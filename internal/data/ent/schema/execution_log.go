package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// ExecutionLog holds the schema definition for the ExecutionLog entity.
type ExecutionLog struct {
	ent.Schema
}

// Annotations of the ExecutionLog.
func (ExecutionLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_execution_logs"},
		entsql.WithComments(true),
	}
}

// Fields of the ExecutionLog.
func (ExecutionLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("script_id").
			NotEmpty().
			MaxLen(36).
			Comment("FK to executor_scripts"),

		field.String("script_name").
			NotEmpty().
			MaxLen(255).
			Comment("Denormalized script name for audit readability"),

		field.String("client_id").
			NotEmpty().
			MaxLen(255).
			Comment("mTLS client CN"),

		field.String("script_hash").
			NotEmpty().
			MaxLen(64).
			Comment("Script content hash at execution time"),

		field.Enum("trigger_type").
			Values("CLIENT_PULL", "UI_PUSH").
			Comment("Who initiated the execution"),

		field.Enum("status").
			Values("PENDING", "RUNNING", "COMPLETED", "FAILED",
				"REJECTED_HASH_MISMATCH", "REJECTED_NOT_APPROVED", "CLIENT_OFFLINE").
			Default("PENDING").
			Comment("Current execution status"),

		field.Int("exit_code").
			Optional().
			Nillable().
			Comment("Process exit code"),

		field.Text("output").
			Optional().
			Comment("Script stdout"),

		field.Text("error_output").
			Optional().
			Comment("Script stderr"),

		field.String("rejection_reason").
			Optional().
			MaxLen(1024).
			Comment("Why the client rejected execution"),

		field.Time("started_at").
			Optional().
			Nillable().
			Comment("When execution started on client"),

		field.Time("completed_at").
			Optional().
			Nillable().
			Comment("When execution completed on client"),

		field.Int64("duration_ms").
			Optional().
			Nillable().
			Comment("Execution duration in milliseconds"),
	}
}

// Edges of the ExecutionLog.
func (ExecutionLog) Edges() []ent.Edge {
	return nil
}

// Mixin of the ExecutionLog.
func (ExecutionLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the ExecutionLog.
func (ExecutionLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
		index.Fields("script_id"),
		index.Fields("client_id"),
		index.Fields("status"),
		index.Fields("tenant_id", "script_id"),
		index.Fields("tenant_id", "client_id"),
		index.Fields("tenant_id", "status"),
	}
}
