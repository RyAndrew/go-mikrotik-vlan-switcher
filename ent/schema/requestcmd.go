package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RequestCmd holds the schema definition for the RequestCmd entity.
// One row is written per RouterOS API command sent to the MikroTik device.
type RequestCmd struct {
	ent.Schema
}

// Fields of the RequestCmd.
func (RequestCmd) Fields() []ent.Field {
	return []ent.Field{
		field.Time("timestamp").
			Default(time.Now),
		field.String("command"),
		field.String("args").
			Optional(),
		field.String("interface").
			Optional(),
		field.Bool("success"),
		field.String("error").
			Optional(),
		field.Int64("duration_ms"),
	}
}

// Edges of the RequestCmd.
func (RequestCmd) Edges() []ent.Edge {
	return nil
}
