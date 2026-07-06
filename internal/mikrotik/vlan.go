// Package mikrotik wraps the RouterOS API to move a physical interface
// between the vlan1/vlan101/vlan102 interface-lists, which is how VLAN
// membership is managed on the target device.
package mikrotik

import (
	"fmt"
	"sync"

	"github.com/go-routeros/routeros/v3"
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

// Client is a serialized wrapper around a single RouterOS API connection.
type Client struct {
	mu   sync.Mutex
	conn *routeros.Client
}

// Dial opens a RouterOS API connection to the target device.
func Dial(address, username, password string) (*Client, error) {
	conn, err := routeros.Dial(address, username, password)
	if err != nil {
		return nil, fmt.Errorf("dial mikrotik: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() {
	c.conn.Close()
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
func (c *Client) currentMembership(iface string) (*membership, error) {
	reply, err := c.conn.Run("/interface/list/member/print", "?interface="+iface)
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

// SwitchVlan moves iface into the interface-list for targetVlanID, removing
// it from whichever managed list it currently belongs to (if any). It
// returns the VLAN id iface belonged to beforehand, or 0 if it wasn't a
// member of any managed list.
func (c *Client) SwitchVlan(iface string, targetVlanID int) (previousVlanID int, err error) {
	if err := ValidateInterface(iface); err != nil {
		return 0, err
	}
	targetList, err := ListForVlanID(targetVlanID)
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	current, err := c.currentMembership(iface)
	if err != nil {
		return 0, err
	}

	if current != nil {
		previousVlanID = current.vlanID
		if current.list == targetList {
			// Already a member of the target list; nothing to do.
			return previousVlanID, nil
		}
		if _, err := c.conn.Run("/interface/list/member/remove", "=.id="+current.id); err != nil {
			return previousVlanID, fmt.Errorf("list member remove: %w", err)
		}
	}

	if _, err := c.conn.Run("/interface/list/member/add", "=list="+targetList, "=interface="+iface); err != nil {
		return previousVlanID, fmt.Errorf("list member add: %w", err)
	}

	return previousVlanID, nil
}

// CurrentVlan returns the VLAN id iface currently belongs to, or 0 if it's
// not a member of any managed list.
func (c *Client) CurrentVlan(iface string) (vlanID int, err error) {
	if err := ValidateInterface(iface); err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	current, err := c.currentMembership(iface)
	if err != nil {
		return 0, err
	}
	if current == nil {
		return 0, nil
	}
	return current.vlanID, nil
}
