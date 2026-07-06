package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AppConfig holds the schema definition for the AppConfig entity.
// It is treated as a singleton table (a single row) populated externally.
type AppConfig struct {
	ent.Schema
}

// Fields of the AppConfig.
func (AppConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("mikrotik_address"),
		field.String("mikrotik_username"),
		field.String("mikrotik_password").
			Sensitive(),
		field.String("oauth_issuer"),
		field.String("oauth_audience"),
		field.String("vlan_scope"),
		field.Bool("enable_authentication").
			Default(true),
	}
}

// Edges of the AppConfig.
func (AppConfig) Edges() []ent.Edge {
	return nil
}
