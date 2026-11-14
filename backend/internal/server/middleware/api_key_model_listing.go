package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const modelListingGroupsKey = "api_key.model_listing_groups"

// Explicit model selection and Codex manifests keep request-group semantics.
func isMultiGroupModelListing(c *gin.Context, key *service.APIKey) bool {
	return key != nil && len(key.ConfiguredGroupIDs()) > 1 && c.Request.Method == http.MethodGet &&
		(c.Request.URL.Path == "/v1/models" || c.Request.URL.Path == "/models") &&
		!c.Request.URL.Query().Has("model") && c.Query("client_version") == ""
}

func GetModelListingGroups(c *gin.Context) ([]*service.APIKey, bool) {
	value, ok := c.Get(modelListingGroupsKey)
	groups, valid := value.([]*service.APIKey)
	return groups, ok && valid
}
