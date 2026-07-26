package schema

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MobilePushDelivery tracks one outbox event for one concrete device.
type MobilePushDelivery struct {
	ent.Schema
}

func (MobilePushDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "mobile_push_deliveries"}}
}

func (MobilePushDelivery) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (MobilePushDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("outbox_id").Positive().Immutable(),
		field.UUID("device_id", uuid.UUID{}).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.String("status").
			MaxLen(16).
			Default("pending").
			Validate(validateMobilePushDeliveryStatus),
		field.Int("attempts").NonNegative().Default(0),
		field.String("last_error_code").Optional().Nillable().MaxLen(64),
		field.Time("available_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("sent_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MobilePushDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("outbox_id", "device_id").Unique(),
		index.Fields("outbox_id", "status", "available_at"),
		index.Fields("device_id", "created_at"),
	}
}

func validateMobilePushDeliveryStatus(value string) error {
	switch value {
	case "pending", "sent", "failed", "skipped":
		return nil
	default:
		return fmt.Errorf("mobile push delivery status %q is not allowed", value)
	}
}
