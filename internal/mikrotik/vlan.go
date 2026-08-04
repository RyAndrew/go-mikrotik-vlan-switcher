// Package mikrotik wraps the RouterOS API to move a physical interface
// between the vlan1/vlan101/vlan102 interface-lists, which is how VLAN
// membership is managed on the target device. Every command it sends to
// the device is recorded in the request_cmds table.
package mikrotik

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"

	"go-mikrotik-vlan-switcher/ent"
	"go-mikrotik-vlan-switcher/internal/store"
)

// allInterfaces is the ordered list of physical ports this service is
// permitted to move between VLAN interface-lists.
var allInterfaces = func() []string {
	ifaces := []string{"ether1"}
	for i := 1; i <= 8; i++ {
		ifaces = append(ifaces, fmt.Sprintf("sfp-sfpplus%d", i))
	}
	return ifaces
}()

// AllowedInterfaces is the set of physical ports this service is permitted
// to move between VLAN interface-lists.
var AllowedInterfaces = func() map[string]bool {
	m := make(map[string]bool, len(allInterfaces))
	for _, i := range allInterfaces {
		m[i] = true
	}
	return m
}()

// AllInterfaces returns the ordered list of allowed ports (ether1, then
// sfp-sfpplus1..8).
func AllInterfaces() []string {
	out := make([]string, len(allInterfaces))
	copy(out, allInterfaces)
	return out
}

// listByVlanID maps the VLAN id a caller asks for to the RouterOS
// interface-list name that represents it.
var listByVlanID = map[int]string{
	1:   "vlan1",
	101: "vlan101",
	102: "vlan102",
}

var vlanIDByList = func() map[string]int {
	m := make(map[string]int, len(listByVlanID))
	for id, list := range listByVlanID {
		m[list] = id
	}
	return m
}()

// ValidateInterface reports whether iface is one of the ports this service
// is allowed to touch.
func ValidateInterface(iface string) error {
	if !AllowedInterfaces[iface] {
		return fmt.Errorf("interface %q is not an allowed port", iface)
	}
	return nil
}

// ListForVlanID returns the interface-list name for a supported VLAN id.
func ListForVlanID(vlanID int) (string, error) {
	list, ok := listByVlanID[vlanID]
	if !ok {
		return "", fmt.Errorf("vlan id %d is not supported", vlanID)
	}
	return list, nil
}

// Client holds the RouterOS connection parameters for the target device.
// It does not keep a connection open: every operation dials a fresh
// connection, uses it, and closes it before returning.
type Client struct {
	address  string
	username string
	password string
	ent      *ent.Client
}

// NewClient builds a Client for the target device. entClient is used to
// record every command sent to the device in the request_cmds table.
func NewClient(address, username, password string, entClient *ent.Client) *Client {
	return &Client{address: address, username: username, password: password, ent: entClient}
}

// dial opens a new RouterOS API connection to the target device. Callers
// are responsible for closing it once they're done.
func (c *Client) dial() (*routeros.Client, error) {
	conn, err := routeros.Dial(c.address, c.username, c.password)
	if err != nil {
		return nil, fmt.Errorf("dial mikrotik: %w", err)
	}
	return conn, nil
}

// TestConnection opens a connection to the device and immediately closes
// it, to verify the device is reachable and the credentials are valid.
func (c *Client) TestConnection() error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// run sends one RouterOS command over conn and records it in the
// request_cmds table.
func (c *Client) run(ctx context.Context, conn *routeros.Client, iface, cmd string, args ...string) (*routeros.Reply, error) {
	start := time.Now()
	reply, err := conn.Run(append([]string{cmd}, args...)...)
	duration := time.Since(start)

	if c.ent != nil {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logErr := store.WriteRequestCmd(ctx, c.ent, store.RequestCmdEntry{
			Command:    cmd,
			Args:       strings.Join(args, " "),
			Interface:  iface,
			Success:    err == nil,
			Error:      errMsg,
			DurationMs: duration.Milliseconds(),
		})
		if logErr != nil {
			slog.Default().Error("write request_cmd", slog.Any("error", logErr))
		}
	}

	return reply, err
}

// membership describes an existing /interface/list/member row for one of
// the managed VLAN lists.
type membership struct {
	id     string
	list   string
	vlanID int
}

// currentMembership finds which (if any) of the managed VLAN lists iface
// currently belongs to.
func (c *Client) currentMembership(ctx context.Context, conn *routeros.Client, iface string) (*membership, error) {
	reply, err := c.run(ctx, conn, iface, "/interface/list/member/print", "?interface="+iface)
	if err != nil {
		return nil, fmt.Errorf("list member print: %w", err)
	}

	for _, re := range reply.Re {
		list := re.Map["list"]
		if vlanID, ok := vlanIDByList[list]; ok {
			return &membership{id: re.Map[".id"], list: list, vlanID: vlanID}, nil
		}
	}
	return nil, nil
}

// bridgeName is the single bridge these ports belong to.
const bridgeName = "bridge"

// setBridgePortPVID sets the pvid on iface's existing /interface/bridge/port
// entry, which is the second half of applying a VLAN change (the interface
// list membership controls VLAN filtering, but the port's pvid controls
// how untagged traffic on it is classified). The bridge port entry is
// expected to already exist; it is not created if missing.
func (c *Client) setBridgePortPVID(ctx context.Context, conn *routeros.Client, iface string, vlanID int) error {
	reply, err := c.run(ctx, conn, iface, "/interface/bridge/port/print", "?interface="+iface, "?bridge="+bridgeName)
	if err != nil {
		return fmt.Errorf("bridge port print: %w", err)
	}
	if len(reply.Re) == 0 {
		return fmt.Errorf("no /interface/bridge/port entry for interface %q on bridge %q", iface, bridgeName)
	}

	id := reply.Re[0].Map[".id"]
	if _, err := c.run(ctx, conn, iface, "/interface/bridge/port/set", "=.id="+id, fmt.Sprintf("=pvid=%d", vlanID)); err != nil {
		return fmt.Errorf("bridge port set pvid: %w", err)
	}
	return nil
}

// SwitchVlan moves iface into the interface-list for targetVlanID, removing
// it from whichever managed list it currently belongs to (if any), and sets
// the matching pvid on its bridge port entry. It returns the VLAN id iface
// belonged to beforehand, or 0 if it wasn't a member of any managed list.
func (c *Client) SwitchVlan(ctx context.Context, iface string, targetVlanID int) (previousVlanID int, err error) {
	if err := ValidateInterface(iface); err != nil {
		return 0, err
	}
	targetList, err := ListForVlanID(targetVlanID)
	if err != nil {
		return 0, err
	}

	conn, err := c.dial()
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	current, err := c.currentMembership(ctx, conn, iface)
	if err != nil {
		return 0, err
	}

	if current != nil {
		previousVlanID = current.vlanID
		if current.list != targetList {
			if _, err := c.run(ctx, conn, iface, "/interface/list/member/remove", "=.id="+current.id); err != nil {
				return previousVlanID, fmt.Errorf("list member remove: %w", err)
			}
			if _, err := c.run(ctx, conn, iface, "/interface/list/member/add", "=list="+targetList, "=interface="+iface); err != nil {
				return previousVlanID, fmt.Errorf("list member add: %w", err)
			}
		}
	} else if _, err := c.run(ctx, conn, iface, "/interface/list/member/add", "=list="+targetList, "=interface="+iface); err != nil {
		return previousVlanID, fmt.Errorf("list member add: %w", err)
	}

	if err := c.setBridgePortPVID(ctx, conn, iface, targetVlanID); err != nil {
		return previousVlanID, err
	}

	return previousVlanID, nil
}

// CurrentVlan returns the VLAN id iface currently belongs to, or 0 if it's
// not a member of any managed list.
func (c *Client) CurrentVlan(ctx context.Context, iface string) (vlanID int, err error) {
	if err := ValidateInterface(iface); err != nil {
		return 0, err
	}

	conn, err := c.dial()
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	current, err := c.currentMembership(ctx, conn, iface)
	if err != nil {
		return 0, err
	}
	if current == nil {
		return 0, nil
	}
	return current.vlanID, nil
}
