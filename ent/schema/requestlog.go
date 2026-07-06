package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RequestLog holds the schema definition for the RequestLog entity.
// One row is written per API request handled by the server.
type RequestLog struct {
	ent.Schema
}

// Fields of the RequestLog.
func (RequestLog) Fields() []ent.Field {
	return []ent.Field{
		field.Time("timestamp").
			Default(time.Now),
		field.String("method"),
		field.String("path"),
		field.String("remote_addr"),
		field.String("subject").
			Optional(),
		field.String("interface").
			Optional(),
		field.Int("vlan_id").
			Optional(),
		field.Int("status_code"),
		field.Int64("duration_ms"),
		field.String("error").
			Optional(),
	}
}

// Edges of the RequestLog.
func (RequestLog) Edges() []ent.Edge {
	return nil
}
