package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptWhereImageOnlyFiltersImagePromptTraits(t *testing.T) {
	where, args := buildPromptWhere(service.PromptListFilter{
		ImageOnly: true,
		Query:     "product",
	}, nil, true)

	require.Contains(t, where, "COALESCE(array_length(v.sizes, 1), 0) > 0")
	require.Contains(t, where, "LOWER(v.purpose) IN ('image', 'image_studio', 'image-studio')")
	require.Contains(t, where, "v.reference_requirement <> 'none'")
	require.Contains(t, where, "v.requires_reference")
	require.Contains(t, where, "v.models && $2::text[]")
	require.Contains(t, where, "LIKE ANY($3::text[])")
	require.Contains(t, where, "p.status IN ('published', 'pending_review')")
	require.Len(t, args, 3)
	require.Equal(t, "%product%", args[0])
}
