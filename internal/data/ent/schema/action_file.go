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

// ActionFile is a single file in an action package (e.g. index.js). The
// action.yaml manifest itself lives on the Action.manifest column; ActionFile
// holds the supporting files for scripted/composite actions.
type ActionFile struct {
	ent.Schema
}

// Annotations of the ActionFile.
func (ActionFile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "executor_action_files"},
		entsql.WithComments(true),
	}
}

// Fields of the ActionFile.
func (ActionFile) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("action_id").
			NotEmpty().
			MaxLen(36).
			Comment("Owning action ID"),

		field.String("path").
			NotEmpty().
			MaxLen(512).
			Comment("Relative path within the action package (e.g. index.js)"),

		field.Text("content").
			Comment("File content"),
	}
}

// Edges of the ActionFile.
func (ActionFile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("action", Action.Type).
			Ref("files").
			Field("action_id").
			Unique().
			Required(),
	}
}

// Mixin of the ActionFile.
func (ActionFile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Indexes of the ActionFile.
func (ActionFile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("action_id"),
		index.Fields("action_id", "path").Unique(),
	}
}
