package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/tidwall/gjson"
)

// Ambiguous duplicate keys can make gateway billing disagree with upstream JSON.
func validateStarframeUniqueJSON(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 128 {
			return fmt.Errorf("StarFrame JSON nesting exceeds gateway limit")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, composite := token.(json.Delim)
		if !composite {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("StarFrame JSON must not contain duplicate object fields")
				}
				seen[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		}
		_, err = decoder.Token()
		return err
	}
	return walk(0)
}

func validateStarframeMaterialURL(value gjson.Result) error {
	invalid := fmt.Errorf("StarFrame materials require public HTTP(S) URLs without userinfo or fragments")
	if value.Type != gjson.String {
		return invalid
	}
	raw := value.String()
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" {
		return invalid
	}
	if _, err := urlvalidator.ValidateHTTPURL(raw, true, urlvalidator.ValidationOptions{}); err != nil {
		return invalid
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		if _, public := imageStudioPublicRemoteAddr(ip); !public {
			return invalid
		}
	} else {
		if !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
			return invalid
		}
		numeric := true
		for _, c := range host {
			if (c < '0' || c > '9') && c != '.' {
				numeric = false
				break
			}
		}
		if numeric {
			return invalid
		}
	}
	return nil
}

func validateStarframeReferences(refs gjson.Result) error {
	if !refs.Exists() {
		return nil
	}
	if !refs.IsObject() {
		return fmt.Errorf("StarFrame references must be an object")
	}
	for _, singular := range []string{"image", "video", "audio"} {
		one, many := refs.Get(singular), refs.Get(singular+"s")
		if one.Exists() && many.Exists() {
			return fmt.Errorf("StarFrame references %s and %ss are mutually exclusive", singular, singular)
		}
		if one.Exists() {
			if err := validateStarframeMaterialURL(one); err != nil {
				return err
			}
		}
		if many.Exists() {
			if !many.IsArray() || len(many.Array()) < 2 {
				return fmt.Errorf("StarFrame plural references require at least two URLs")
			}
			for _, value := range many.Array() {
				if err := validateStarframeMaterialURL(value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
