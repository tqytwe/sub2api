package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// MobileTask stores the privacy-safe task projection shared by chat, image,
// and file workflows. Request content and generated content live elsewhere.
type MobileTask struct {
	ent.Schema
}

func (MobileTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "mobile_tasks"},
	}
}

func (MobileTask) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.Int64("user_id").
			Positive(),
		field.Enum("kind").
			Values("chat", "image", "file"),
		field.String("operation").
			MaxLen(100).
			NotEmpty(),
		field.Enum("status").
			Values("queued", "running", "streaming", "completed", "partial", "failed", "cancelled").
			Default("queued"),
		field.Int("progress").
			Min(0).
			Max(100).
			Default(0),
		field.UUID("parent_task_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("retry_of", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("client_request_id").
			MaxLen(128).
			NotEmpty(),
		field.JSON("resource", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("artifacts", []map[string]any{}).
			Default([]map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("error", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("protocol_version").
			Positive().
			Default(1),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("started_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("finished_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MobileTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "client_request_id").Unique(),
		index.Fields("user_id", "created_at"),
		index.Fields("user_id", "kind", "created_at"),
		index.Fields("user_id", "status", "created_at"),
		index.Fields("retry_of"),
		index.Fields("parent_task_id"),
	}
}
