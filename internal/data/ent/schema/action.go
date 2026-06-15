package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Action holds the schema definition for a reusable go-tangra-actions action
// package (an action.yaml manifest plus optional package files). Actions are the
// building blocks referenced by workflows via `uses:`.
type Action struct {
	ent.Schema
}

// Annotations of the Action.
func (Action) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_actions"},
		entsql.WithComments(true),
	}
}

// Fields of the Action.
func (Action) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Action name (the reference used by workflows' uses:)"),

		field.Int("version").
			Default(1).
			Comment("Content version, incremented on update"),

		field.String("description").
			Optional().
			MaxLen(2048).
			Comment("Action description"),

		field.String("using").
			Optional().
			MaxLen(32).
			Comment("runs.using from the manifest: composite, javascript, lua, ..."),

		field.Text("manifest").
			Comment("The action.yaml manifest content"),

		field.String("content_hash").
			NotEmpty().
			MaxLen(64).
			Comment("SHA256 hex digest of the manifest + package files"),

		field.Bool("enabled").
			Default(true).
			Comment("Whether the action is active"),
	}
}

// Edges of the Action.
func (Action) Edges() []ent.Edge {
	return []ent.Edge{
		// Package files (e.g. index.js for scripted actions). Cascade-delete
		// with the action.
		edge.To("files", ActionFile.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Mixin of the Action.
func (Action) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the Action.
func (Action) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id", "using"),
		index.Fields("tenant_id", "enabled"),
	}
}
