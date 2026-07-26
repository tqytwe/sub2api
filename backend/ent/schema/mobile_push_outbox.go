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

// MobilePushOutbox is the durable source for privacy-safe mobile notifications.
type MobilePushOutbox struct {
	ent.Schema
}

func (MobilePushOutbox) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "mobile_push_outbox"}}
}

func (MobilePushOutbox) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (MobilePushOutbox) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Positive().Immutable(),
		field.String("dedupe_key_hash").MaxLen(64).NotEmpty().Immutable(),
		field.String("event_type").MaxLen(64).NotEmpty().Immutable(),
		field.String("source_type").MaxLen(64).Default("").Immutable(),
		field.String("source_id").MaxLen(128).Default("").Immutable(),
		field.String("title_zh").MaxLen(120).NotEmpty(),
		field.String("body_zh").MaxLen(300).NotEmpty(),
		field.JSON("data", map[string]string{}).
			Default(func() map[string]string { return map[string]string{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("status").
			MaxLen(16).
			Default("pending").
			Validate(validateMobilePushStatus),
		field.Int("attempts").NonNegative().Default(0),
		field.String("last_error_code").
			Optional().
			Nillable().
			MaxLen(64),
		field.Time("available_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("sent_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.UUID("claim_token", uuid.UUID{}).
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Time("lease_expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MobilePushOutbox) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("dedupe_key_hash").Unique(),
		index.Fields("status", "available_at"),
		index.Fields("lease_expires_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("source_type", "source_id"),
	}
}

func validateMobilePushStatus(value string) error {
	switch value {
	case "pending", "processing", "sent", "failed", "skipped":
		return nil
	default:
		return fmt.Errorf("mobile push status %q is not allowed", value)
	}
}
