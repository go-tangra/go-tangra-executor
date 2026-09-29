package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// Setting is a platform-wide executor setting stored as a JSON document under
// a well-known key (e.g. "inventory_agent"). Values may hold secrets: the
// field is sensitive (never printed) and services never return them to the
// admin API.
type Setting struct {
	ent.Schema
}

// Annotations of the Setting.
func (Setting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_settings"},
		entsql.WithComments(true),
	}
}

// Fields of the Setting.
func (Setting) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			MaxLen(64).
			Comment("Setting key"),

		field.Text("value").
			Sensitive().
			Comment("JSON document"),

		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Last change"),

		field.Uint32("updated_by").
			Optional().
			Nillable().
			Comment("User who last changed it"),
	}
}
