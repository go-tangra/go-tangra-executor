package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Workflow holds the schema definition for a saved, reusable go-tangra-actions
// workflow (jobs/steps YAML) that can be run on a client on demand.
type Workflow struct {
	ent.Schema
}

// Annotations of the Workflow.
func (Workflow) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_workflows"},
		entsql.WithComments(true),
	}
}

// Fields of the Workflow.
func (Workflow) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Workflow name"),

		field.String("description").
			Optional().
			MaxLen(2048).
			Comment("Workflow description"),

		field.Text("content").
			Comment("Workflow YAML (jobs/steps)"),

		field.String("content_hash").
			NotEmpty().
			MaxLen(64).
			Comment("SHA256 hex digest of content"),

		field.Int("version").
			Default(1).
			Comment("Content version, incremented on update"),

		field.Bool("enabled").
			Default(true).
			Comment("Whether the workflow is active"),
	}
}

// Edges of the Workflow.
func (Workflow) Edges() []ent.Edge {
	return nil
}

// Mixin of the Workflow.
func (Workflow) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the Workflow.
func (Workflow) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id", "enabled"),
	}
}
