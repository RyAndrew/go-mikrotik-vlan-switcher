package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// InterfaceVlanState holds the schema definition for the InterfaceVlanState entity.
// It caches the last known interface-list membership for a MikroTik interface.
type InterfaceVlanState struct {
	ent.Schema
}

// Fields of the InterfaceVlanState.
func (InterfaceVlanState) Fields() []ent.Field {
	return []ent.Field{
		field.String("interface").
			Unique(),
		field.String("current_list"),
		field.Int("current_vlan_id"),
		field.Time("last_synced_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the InterfaceVlanState.
func (InterfaceVlanState) Edges() []ent.Edge {
	return nil
}
