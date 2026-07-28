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

type SubscriptionEntitlementHold struct {
	ent.Schema
}

func (SubscriptionEntitlementHold) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subscription_entitlement_holds"}}
}

func (SubscriptionEntitlementHold) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("entitlement_id"),
		field.String("request_id").MaxLen(200),
		field.String("request_fingerprint").MaxLen(128),
		field.Float("reserved_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("captured_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.String("status").MaxLen(20).Default("reserved"),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("captured_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("released_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SubscriptionEntitlementHold) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entitlement_id", "request_id").Unique(),
		index.Fields("entitlement_id", "status"),
		index.Fields("expires_at"),
	}
}
