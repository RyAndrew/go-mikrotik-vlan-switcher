package api

import (
	"encoding/json"
	"net/http"
	"time"

	"go-mikrotik-vlan-switcher/ent"
	"go-mikrotik-vlan-switcher/internal/mikrotik"
	"go-mikrotik-vlan-switcher/internal/reqctx"
	"go-mikrotik-vlan-switcher/internal/store"
)

// Deps holds the dependencies the HTTP handlers need.
type Deps struct {
	Ent      *ent.Client
	Mikrotik *mikrotik.Client
}

type switchVlanRequest struct {
	Interface string `json:"interface"`
	VlanID    int    `json:"vlan_id"`
}

type switchVlanResponse struct {
	Interface      string `json:"interface"`
	PreviousVlanID int    `json:"previous_vlan_id"`
	VlanID         int    `json:"vlan_id"`
	Status         string `json:"status"`
}

// HandleSwitchVlan handles POST /vlan: move an interface into the
// interface-list for the requested VLAN id.
func (d *Deps) HandleSwitchVlan(w http.ResponseWriter, r *http.Request) {
	var req switchVlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.fail(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	fields := reqctx.From(r.Context())
	fields.Interface = req.Interface
	fields.VlanID = req.VlanID

	if err := mikrotik.ValidateInterface(req.Interface); err != nil {
		d.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	list, err := mikrotik.ListForVlanID(req.VlanID)
	if err != nil {
		d.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}

	previous, err := d.Mikrotik.SwitchVlan(r.Context(), req.Interface, req.VlanID)
	if err != nil {
		d.fail(w, r, http.StatusBadGateway, err.Error())
		return
	}

	if err := store.SetCachedVlanState(r.Context(), d.Ent, req.Interface, list, req.VlanID); err != nil {
		// The switch already succeeded on the device; a cache write failure
		// shouldn't fail the request, just note it on the log entry.
		fields.Error = "cache update failed: " + err.Error()
	}

	writeJSON(w, http.StatusOK, switchVlanResponse{
		Interface:      req.Interface,
		PreviousVlanID: previous,
		VlanID:         req.VlanID,
		Status:         "ok",
	})
}

type vlanStateResponse struct {
	Interface    string     `json:"interface"`
	VlanID       int        `json:"vlan_id"`
	List         string     `json:"list"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// HandleGetVlan handles GET /vlan/{interface}: return the cached VLAN
// membership, falling back to a live device query if nothing is cached yet.
func (d *Deps) HandleGetVlan(w http.ResponseWriter, r *http.Request) {
	iface := r.PathValue("interface")
	reqctx.From(r.Context()).Interface = iface

	if err := mikrotik.ValidateInterface(iface); err != nil {
		d.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}

	cached, err := store.GetCachedVlanState(r.Context(), d.Ent, iface)
	if err == nil {
		lastSynced := cached.LastSyncedAt
		writeJSON(w, http.StatusOK, vlanStateResponse{
			Interface:    iface,
			VlanID:       cached.CurrentVlanID,
			List:         cached.CurrentList,
			LastSyncedAt: &lastSynced,
		})
		return
	}
	if !ent.IsNotFound(err) {
		d.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	vlanID, err := d.Mikrotik.CurrentVlan(r.Context(), iface)
	if err != nil {
		d.fail(w, r, http.StatusBadGateway, err.Error())
		return
	}
	if vlanID == 0 {
		d.fail(w, r, http.StatusNotFound, "interface has no known vlan membership")
		return
	}

	list, _ := mikrotik.ListForVlanID(vlanID)
	if err := store.SetCachedVlanState(r.Context(), d.Ent, iface, list, vlanID); err != nil {
		reqctx.From(r.Context()).Error = "cache write failed: " + err.Error()
	}

	writeJSON(w, http.StatusOK, vlanStateResponse{Interface: iface, VlanID: vlanID, List: list})
}

// HandleSyncVlan handles POST /vlan/{interface}/sync: always query the
// device for iface's current VLAN membership and overwrite the cache with
// it, regardless of what (if anything) was cached before. Use this to
// correct the cache after out-of-band changes on the device.
func (d *Deps) HandleSyncVlan(w http.ResponseWriter, r *http.Request) {
	iface := r.PathValue("interface")
	reqctx.From(r.Context()).Interface = iface

	if err := mikrotik.ValidateInterface(iface); err != nil {
		d.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}

	vlanID, err := d.Mikrotik.CurrentVlan(r.Context(), iface)
	if err != nil {
		d.fail(w, r, http.StatusBadGateway, err.Error())
		return
	}

	if vlanID == 0 {
		if err := store.DeleteCachedVlanState(r.Context(), d.Ent, iface); err != nil {
			reqctx.From(r.Context()).Error = "cache delete failed: " + err.Error()
		}
		d.fail(w, r, http.StatusNotFound, "interface has no known vlan membership")
		return
	}

	list, _ := mikrotik.ListForVlanID(vlanID)
	if err := store.SetCachedVlanState(r.Context(), d.Ent, iface, list, vlanID); err != nil {
		d.fail(w, r, http.StatusInternalServerError, "cache write failed: "+err.Error())
		return
	}

	now := time.Now()
	writeJSON(w, http.StatusOK, vlanStateResponse{
		Interface:    iface,
		VlanID:       vlanID,
		List:         list,
		LastSyncedAt: &now,
	})
}

type vlanStateItem struct {
	Interface    string     `json:"interface"`
	VlanID       int        `json:"vlan_id,omitempty"`
	List         string     `json:"list,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// HandleSyncAllVlans handles POST /vlan/sync: force a live re-query of
// every allowed port's VLAN membership, overwrite the cache for each, and
// return the full list. A failure on one port (device error, cache write
// error) is reported in that port's Error field rather than aborting the
// rest.
func (d *Deps) HandleSyncAllVlans(w http.ResponseWriter, r *http.Request) {
	ifaces := mikrotik.AllInterfaces()
	results := make([]vlanStateItem, 0, len(ifaces))

	for _, iface := range ifaces {
		vlanID, err := d.Mikrotik.CurrentVlan(r.Context(), iface)
		if err != nil {
			results = append(results, vlanStateItem{Interface: iface, Error: err.Error()})
			continue
		}

		if vlanID == 0 {
			if err := store.DeleteCachedVlanState(r.Context(), d.Ent, iface); err != nil {
				results = append(results, vlanStateItem{Interface: iface, Error: "cache delete failed: " + err.Error()})
				continue
			}
			results = append(results, vlanStateItem{Interface: iface})
			continue
		}

		list, _ := mikrotik.ListForVlanID(vlanID)
		if err := store.SetCachedVlanState(r.Context(), d.Ent, iface, list, vlanID); err != nil {
			results = append(results, vlanStateItem{Interface: iface, VlanID: vlanID, List: list, Error: "cache write failed: " + err.Error()})
			continue
		}

		now := time.Now()
		results = append(results, vlanStateItem{Interface: iface, VlanID: vlanID, List: list, LastSyncedAt: &now})
	}

	writeJSON(w, http.StatusOK, results)
}

func (d *Deps) fail(w http.ResponseWriter, r *http.Request, status int, msg string) {
	reqctx.From(r.Context()).Error = msg
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
