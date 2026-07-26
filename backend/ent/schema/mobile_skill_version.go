package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MobileSkillVersion stores the immutable, versioned behavior of a mobile skill.
type MobileSkillVersion struct {
	ent.Schema
}

func (MobileSkillVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "mobile_skill_versions"},
	}
}

func (MobileSkillVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (MobileSkillVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id").
			Positive(),
		field.Int("version").
			Positive(),
		field.Int64("prompt_id").
			Min(0),
		field.Int("prompt_version").
			Positive(),
		field.Text("system_prompt_override").
			Optional().
			Nillable(),
		field.JSON("input_schema", map[string]any{}).
			Default(map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("examples", []map[string]any{}).
			Default([]map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("tool_config", map[string]any{}).
			Default(map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("model_policy", map[string]any{}).
			Default(map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("consumption_note_zh").
			Default(""),
		field.Text("changelog_zh").
			Default(""),
	}
}

func (MobileSkillVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", MobileSkill.Type).
			Ref("versions").
			Field("skill_id").
			Unique().
			Required(),
	}
}

func (MobileSkillVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id", "version").Unique(),
		index.Fields("prompt_id", "prompt_version"),
	}
}
