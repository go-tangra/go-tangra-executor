package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// ScriptAssignment holds the schema definition for the ScriptAssignment entity.
type ScriptAssignment struct {
	ent.Schema
}

// Annotations of the ScriptAssignment.
func (ScriptAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_script_assignments"},
		entsql.WithComments(true),
	}
}

// Fields of the ScriptAssignment.
func (ScriptAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("script_id").
			NotEmpty().
			MaxLen(36).
			Comment("FK to executor_scripts"),

		field.String("client_id").
			NotEmpty().
			MaxLen(255).
			Comment("mTLS client CN"),
	}
}

// Edges of the ScriptAssignment.
func (ScriptAssignment) Edges() []ent.Edge {
	return nil
}

// Mixin of the ScriptAssignment.
func (ScriptAssignment) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the ScriptAssignment.
func (ScriptAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("script_id", "client_id", "tenant_id").
			Unique().
			StorageKey("executor_assignment_unique"),
		index.Fields("script_id"),
		index.Fields("client_id"),
		index.Fields("tenant_id"),
	}
}
