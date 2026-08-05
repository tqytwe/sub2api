package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SubscriptionEntitlement struct {
	ent.Schema
}

func (SubscriptionEntitlement) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subscription_entitlements"}}
}

func (SubscriptionEntitlement) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.Int64("plan_id").Optional().Nillable(),
		field.Int64("payment_order_id"),
		field.String("quota_mode").MaxLen(24),
		field.Float("quota_limit_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("quota_used_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.Float("quota_reserved_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.Int("duration_hours"),
		field.String("status").MaxLen(20).Default("pending"),
		field.Time("starts_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("activated_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("exhausted_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ended_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SubscriptionEntitlement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("payment_order_id").Unique(),
		index.Fields("user_id", "group_id", "status"),
		index.Fields("expires_at"),
	}
}
