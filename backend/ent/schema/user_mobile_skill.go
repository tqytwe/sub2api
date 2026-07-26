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

// UserMobileSkill records a user's installed version and preferences for a skill.
type UserMobileSkill struct {
	ent.Schema
}

func (UserMobileSkill) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_mobile_skills"},
	}
}

func (UserMobileSkill) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserMobileSkill) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Positive(),
		field.Int64("skill_id").
			Positive(),
		field.Int("installed_version").
			Positive(),
		field.Bool("pinned").
			Default(false),
		field.Time("last_used_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserMobileSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", MobileSkill.Type).
			Ref("user_installs").
			Field("skill_id").
			Unique().
			Required(),
	}
}

func (UserMobileSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "skill_id").Unique(),
		index.Fields("user_id", "pinned", "last_used_at"),
		index.Fields("skill_id"),
	}
}
