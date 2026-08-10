package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Canonical request signing shared with the NodeBB plugin
// (plugins/nodebb-plugin-sub2api-sso/webhook-handlers.js).
//
// WHY NOT SIGN THE RAW BODY
// -------------------------
// NodeBB mounts body-parser before any plugin hook runs, so by the time the
// forum's webhook handler executes the raw bytes are gone. Re-serialising with
// JSON.stringify() is not byte-identical to what we sent: `10.00` comes back as
// `10`, key order is not guaranteed, and unicode escaping differs between
// implementations. Signing an explicit canonical form removes that entire class
// of mismatch.
//
// CONTRACT (must stay identical on both sides)
//
//	canonical = timestamp + "\n" + nonce + "\n" + sorted_pairs.join("&")
//	signature = hex(hmac_sha256(secret, canonical))
//	headers   = x-sub2api-timestamp, x-sub2api-nonce, x-sub2api-signature
//
// sorted_pairs is the payload flattened to "path=value" strings and sorted:
//   - nested objects join with "."   -> d.n.k=v
//   - arrays index with "[i]"        -> d.ids[0]=1&d.ids[1]=2
//   - nil/absent values are skipped entirely (JS skips null and undefined)
//   - values are stringified, never quoted or escaped
//
// MONEY MUST BE A STRING. The JS side stringifies whatever JSON.parse produced,
// so a numeric 10.00 arrives as the string "10" and the signature will not
// match a Go side that formatted "10.00". Every monetary field in an outbound
// payload is therefore built as a Go string, which both sides stringify
// identically.

// forumSSOFlatten renders value into sorted-pair components, mirroring the JS
// flatten() exactly, including its skip rules.
func forumSSOFlatten(value any, prefix string, out *[]string) {
	switch typed := value.(type) {
	case nil:
		// JS: `if (value === null || value === undefined) return out`
		return
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		// Order here is irrelevant because the whole slice is sorted later, but
		// iterating deterministically keeps failures reproducible.
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			forumSSOFlatten(typed[key], next, out)
		}
	case []any:
		for i, item := range typed {
			forumSSOFlatten(item, fmt.Sprintf("%s[%d]", prefix, i), out)
		}
	case string:
		*out = append(*out, prefix+"="+typed)
	case bool:
		*out = append(*out, prefix+"="+strconv.FormatBool(typed))
	case int:
		*out = append(*out, prefix+"="+strconv.Itoa(typed))
	case int64:
		*out = append(*out, prefix+"="+strconv.FormatInt(typed, 10))
	case float64:
		// Matches JS String(Number): integral values lose the fractional part.
		// Monetary values must be passed as strings, never as float64.
		*out = append(*out, prefix+"="+strconv.FormatFloat(typed, 'f', -1, 64))
	default:
		*out = append(*out, fmt.Sprintf("%s=%v", prefix, typed))
	}
}

// ForumSSOCanonicalize builds the exact string both sides sign.
func ForumSSOCanonicalize(payload map[string]any, timestamp, nonce string) string {
	pairs := make([]string, 0, 16)
	forumSSOFlatten(payload, "", &pairs)
	// JS Array.prototype.sort() with no comparator sorts by UTF-16 code units.
	// Go sorts by UTF-8 bytes. The two agree for all ASCII, and every key in
	// this contract is ASCII, so ordering is decided before any non-ASCII value
	// byte can be reached.
	sort.Strings(pairs)
	return timestamp + "\n" + nonce + "\n" + strings.Join(pairs, "&")
}

// ForumSSOSign returns the hex HMAC-SHA256 of the canonical string.
func ForumSSOSign(payload map[string]any, timestamp, nonce, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ForumSSOCanonicalize(payload, timestamp, nonce)))
	return hex.EncodeToString(mac.Sum(nil))
}
