package schema

import (
	"encoding/hex"
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

const (
	mobileAssetKindImage    = "image"
	mobileAssetKindAudio    = "audio"
	mobileAssetKindVideo    = "video"
	mobileAssetKindPDF      = "pdf"
	mobileAssetKindDocument = "document"
	mobileAssetKindFile     = "file"

	mobileAssetSourceUpload      = "upload"
	mobileAssetSourceShare       = "share"
	mobileAssetSourceImageResult = "image_result"
	mobileAssetSourceVideoResult = "video_result"
	mobileAssetSourceChatExport  = "chat_export"
	mobileAssetSourceVoice       = "voice"

	mobileAssetStatusUploading = "uploading"
	mobileAssetStatusReady     = "ready"
	mobileAssetStatusFailed    = "failed"
	mobileAssetStatusDeleted   = "deleted"
)

// MobileAsset stores user-owned material metadata. Binary content remains in
// object storage and is referenced through storage_key.
type MobileAsset struct {
	ent.Schema
}

func (MobileAsset) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "mobile_assets"},
	}
}

func (MobileAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (MobileAsset) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Int64("user_id").
			Positive().
			Immutable(),
		field.String("kind").
			MaxLen(32).
			Validate(validateMobileAssetKind),
		field.String("source").
			MaxLen(32).
			Validate(validateMobileAssetSource),
		field.String("storage_key").
			NotEmpty().
			MaxLen(1024),
		field.String("original_name").
			NotEmpty().
			MaxLen(255),
		field.String("content_type").
			NotEmpty().
			MaxLen(255),
		field.Int64("byte_size").
			NonNegative(),
		field.String("sha256").
			Optional().
			Nillable().
			MaxLen(64).
			Validate(validateMobileAssetSHA256),
		field.String("status").
			MaxLen(32).
			Default(mobileAssetStatusUploading).
			Validate(validateMobileAssetStatus),
		field.String("source_type").
			Optional().
			Nillable().
			NotEmpty().
			MaxLen(64),
		field.String("source_id").
			Optional().
			Nillable().
			NotEmpty().
			MaxLen(128),
		field.JSON("metadata", map[string]any{}).
			Default(map[string]any{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (MobileAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("user_id", "kind", "created_at"),
		index.Fields("user_id", "sha256").
			Annotations(entsql.IndexWhere("deleted_at IS NULL AND sha256 IS NOT NULL")),
		index.Fields("source_type", "source_id").
			Annotations(entsql.IndexWhere("source_type IS NOT NULL AND source_id IS NOT NULL")),
		index.Fields("user_id", "status", "created_at"),
		index.Fields("deleted_at"),
	}
}

func validateMobileAssetKind(value string) error {
	switch value {
	case mobileAssetKindImage,
		mobileAssetKindAudio,
		mobileAssetKindVideo,
		mobileAssetKindPDF,
		mobileAssetKindDocument,
		mobileAssetKindFile:
		return nil
	default:
		return fmt.Errorf("mobile asset kind %q is not allowed", value)
	}
}

func validateMobileAssetSource(value string) error {
	switch value {
	case mobileAssetSourceUpload,
		mobileAssetSourceShare,
		mobileAssetSourceImageResult,
		mobileAssetSourceVideoResult,
		mobileAssetSourceChatExport,
		mobileAssetSourceVoice:
		return nil
	default:
		return fmt.Errorf("mobile asset source %q is not allowed", value)
	}
}

func validateMobileAssetStatus(value string) error {
	switch value {
	case mobileAssetStatusUploading,
		mobileAssetStatusReady,
		mobileAssetStatusFailed,
		mobileAssetStatusDeleted:
		return nil
	default:
		return fmt.Errorf("mobile asset status %q is not allowed", value)
	}
}

func validateMobileAssetSHA256(value string) error {
	if len(value) != 64 {
		return fmt.Errorf("mobile asset sha256 must contain exactly 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("mobile asset sha256 must be hexadecimal: %w", err)
	}
	return nil
}
