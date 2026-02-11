package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Script holds the schema definition for the Script entity.
type Script struct {
	ent.Schema
}

// Annotations of the Script.
func (Script) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_scripts"},
		entsql.WithComments(true),
	}
}

// Fields of the Script.
func (Script) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Script name"),

		field.String("description").
			Optional().
			MaxLen(2048).
			Comment("Script description"),

		field.Enum("script_type").
			Values("BASH", "JAVASCRIPT", "LUA").
			Comment("Script execution engine type"),

		field.Text("content").
			Comment("Script body content"),

		field.String("content_hash").
			NotEmpty().
			MaxLen(64).
			Comment("SHA256 hex digest of content"),

		field.Int("version").
			Default(1).
			Comment("Content version, incremented on update"),

		field.Bool("enabled").
			Default(true).
			Comment("Whether the script is active"),
	}
}

// Edges of the Script.
func (Script) Edges() []ent.Edge {
	return nil
}

// Mixin of the Script.
func (Script) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the Script.
func (Script) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
		index.Fields("tenant_id", "name"),
		index.Fields("tenant_id", "script_type"),
		index.Fields("tenant_id", "enabled"),
	}
}
