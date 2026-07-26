package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MobileSkill defines a server-managed skill exposed to mobile clients.
type MobileSkill struct {
	ent.Schema
}

func (MobileSkill) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "mobile_skills"},
	}
}

func (MobileSkill) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (MobileSkill) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			MaxLen(100).
			NotEmpty().
			Unique(),
		field.Enum("status").
			Values("draft", "published", "offline").
			Default("draft"),
		field.String("name_zh").
			MaxLen(120).
			NotEmpty(),
		field.Text("description_zh").
			Default(""),
		field.String("category").
			MaxLen(80).
			Default(""),
		field.Text("icon_url").
			Default(""),
		field.Text("cover_url").
			Default(""),
		field.Int("current_version").
			Positive().
			Default(1),
		field.Int("published_version").
			Positive().
			Optional().
			Nillable(),
		field.Bool("featured").
			Default(false),
		field.Int("sort_order").
			Default(0),
	}
}

func (MobileSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", MobileSkillVersion.Type),
		edge.To("user_installs", UserMobileSkill.Type),
	}
}

func (MobileSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "sort_order"),
		index.Fields("category", "status"),
		index.Fields("featured", "status"),
	}
}
