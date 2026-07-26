package schema

import (
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/stretchr/testify/require"
)

func TestMobileAssetFields(t *testing.T) {
	descriptors := mobileAssetFieldDescriptors(MobileAsset{}.Fields())

	require.ElementsMatch(t, []string{
		"id",
		"user_id",
		"kind",
		"source",
		"storage_key",
		"original_name",
		"content_type",
		"byte_size",
		"sha256",
		"status",
		"source_type",
		"source_id",
		"metadata",
	}, mobileAssetFieldNames(descriptors))

	require.Equal(t, field.TypeUUID, descriptors["id"].Info.Type)
	require.NotNil(t, descriptors["id"].Default)
	require.True(t, descriptors["id"].Immutable)
	require.True(t, descriptors["user_id"].Immutable)
	require.True(t, descriptors["sha256"].Optional)
	require.True(t, descriptors["sha256"].Nillable)
	require.True(t, descriptors["source_type"].Optional)
	require.True(t, descriptors["source_id"].Optional)
	require.Equal(t, mobileAssetStatusUploading, descriptors["status"].Default)
}

func TestMobileAssetEnumValidation(t *testing.T) {
	for _, value := range []string{"image", "audio", "video", "pdf", "document", "file"} {
		require.NoError(t, validateMobileAssetKind(value))
	}
	require.Error(t, validateMobileAssetKind("archive"))

	for _, value := range []string{"upload", "share", "image_result", "chat_export", "voice"} {
		require.NoError(t, validateMobileAssetSource(value))
	}
	require.Error(t, validateMobileAssetSource("unknown"))

	for _, value := range []string{"uploading", "ready", "failed", "deleted"} {
		require.NoError(t, validateMobileAssetStatus(value))
	}
	require.Error(t, validateMobileAssetStatus("complete"))
}

func TestMobileAssetSHA256Validation(t *testing.T) {
	require.NoError(t, validateMobileAssetSHA256("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	require.Error(t, validateMobileAssetSHA256(""))
	require.Error(t, validateMobileAssetSHA256("abcd"))
	require.Error(t, validateMobileAssetSHA256("z123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
}

func TestMobileAssetIndexes(t *testing.T) {
	indexes := MobileAsset{}.Indexes()

	requireMobileAssetIndex(t, indexes, "user_id", "created_at")
	requireMobileAssetIndex(t, indexes, "user_id", "kind", "created_at")
	requireMobileAssetIndex(t, indexes, "user_id", "sha256")
	requireMobileAssetIndex(t, indexes, "source_type", "source_id")
	requireMobileAssetIndex(t, indexes, "user_id", "status", "created_at")
	requireMobileAssetIndex(t, indexes, "deleted_at")
}

func mobileAssetFieldDescriptors(fields []ent.Field) map[string]*field.Descriptor {
	descriptors := make(map[string]*field.Descriptor, len(fields))
	for _, entField := range fields {
		descriptor := entField.Descriptor()
		descriptors[descriptor.Name] = descriptor
	}
	return descriptors
}

func mobileAssetFieldNames(descriptors map[string]*field.Descriptor) []string {
	names := make([]string, 0, len(descriptors))
	for name := range descriptors {
		names = append(names, name)
	}
	return names
}

func requireMobileAssetIndex(t *testing.T, indexes []ent.Index, fields ...string) {
	t.Helper()
	for _, entIndex := range indexes {
		descriptor := entIndex.Descriptor()
		if len(descriptor.Fields) != len(fields) {
			continue
		}
		matched := true
		for i, name := range fields {
			if descriptor.Fields[i] != name {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	require.Failf(t, "missing index", "expected index on %v", fields)
}
