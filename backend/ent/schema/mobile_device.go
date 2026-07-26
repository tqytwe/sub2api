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

// MobileDevice stores a user installation and its encrypted push token.
type MobileDevice struct {
	ent.Schema
}

func (MobileDevice) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "mobile_devices"}}
}

func (MobileDevice) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (MobileDevice) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Int64("user_id").Positive().Immutable(),
		field.UUID("installation_id", uuid.UUID{}).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.String("platform").
			MaxLen(16).
			Validate(validateMobileDevicePlatform),
		field.String("push_provider").
			MaxLen(16).
			Default("fcm").
			Validate(validateMobilePushProvider),
		field.String("token_ciphertext").
			NotEmpty().
			Sensitive().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("token_hash").
			MaxLen(64).
			NotEmpty().
			Sensitive(),
		field.String("app_version").
			MaxLen(64).
			Default(""),
		field.String("locale").
			MaxLen(32).
			Default("zh-CN"),
		field.Bool("enabled").Default(true),
		field.Time("last_seen_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("revoked_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MobileDevice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "installation_id").Unique(),
		index.Fields("token_hash"),
		index.Fields("user_id", "enabled", "last_seen_at"),
		index.Fields("revoked_at"),
	}
}

func validateMobileDevicePlatform(value string) error {
	switch value {
	case "android", "ios", "web":
		return nil
	default:
		return fmt.Errorf("mobile device platform %q is not allowed", value)
	}
}

func validateMobilePushProvider(value string) error {
	if value == "fcm" {
		return nil
	}
	return fmt.Errorf("mobile push provider %q is not allowed", value)
}
