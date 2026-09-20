package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithBuiltInPluginPublishers(t *testing.T) {
	t.Run("adds the Jisudeng State Kit publisher", func(t *testing.T) {
		publishers := withBuiltInPluginPublishers(nil)

		assert.Equal(t, jisudengStateKitPublisherPublicKey, publishers[jisudengStateKitPublisherID])
	})

	t.Run("preserves an explicit key for rotation and other publishers", func(t *testing.T) {
		publishers := withBuiltInPluginPublishers(map[string]string{
			jisudengStateKitPublisherID: "rotated-key",
			"another-publisher":         "another-key",
		})

		assert.Equal(t, "rotated-key", publishers[jisudengStateKitPublisherID])
		assert.Equal(t, "another-key", publishers["another-publisher"])
	})
}
