package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	_ "golang.org/x/image/webp"
)

const imageStudioMaxDownloadBytes = 32 << 20 // 32MB

var imageStudioSpecialUseRemotePrefixes = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.31.196.0/24"),
	netip.MustParsePrefix("192.52.193.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("192.175.48.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2620:4f:8000::/48"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func FetchImageStudioRemoteURL(ctx context.Context, rawURL string) ([]byte, string, error) {
	validatedURL, err := validateImageStudioRemoteURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := newImageStudioRemoteHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("fetch image failed: status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, imageStudioMaxDownloadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > imageStudioMaxDownloadBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}

func validateImageStudioRemoteURL(rawURL string) (string, error) {
	validated, err := urlvalidator.ValidateHTTPSURL(rawURL, urlvalidator.ValidationOptions{})
	if err != nil {
		return "", fmt.Errorf("image studio remote url is not allowed: %w", err)
	}
	parsed, err := url.Parse(validated)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("image studio remote url is not allowed")
	}
	return validated, nil
}

func newImageStudioRemoteHTTPClient() *http.Client {
	transport := cloneImageStudioHTTPTransport(http.DefaultTransport)
	transport.Proxy = nil
	transport.DialContext = imageStudioSafeDialContext(
		net.DefaultResolver.LookupIP,
		(&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	)
	return &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("image studio remote redirect limit exceeded")
			}
			if _, err := validateImageStudioRemoteURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

func cloneImageStudioHTTPTransport(base http.RoundTripper) *http.Transport {
	if transport, ok := base.(*http.Transport); ok && transport != nil {
		return transport.Clone()
	}
	return &http.Transport{}
}

func imageStudioSafeDialContext(
	lookup func(context.Context, string, string) ([]net.IP, error),
	dial func(context.Context, string, string) (net.Conn, error),
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("image studio remote address is invalid: %w", err)
		}
		ips, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("image studio remote dns resolution failed: %w", err)
		}
		if len(ips) == 0 {
			return nil, errors.New("image studio remote dns resolution returned no addresses")
		}
		addrs := make([]netip.Addr, 0, len(ips))
		for _, ip := range ips {
			addr, ok := imageStudioPublicRemoteAddr(ip)
			if !ok {
				return nil, fmt.Errorf("image studio remote url is not allowed: resolved ip %s", ip)
			}
			addrs = append(addrs, addr)
		}
		var dialErr error
		for _, addr := range addrs {
			conn, err := dial(ctx, network, net.JoinHostPort(addr.String(), port))
			if err == nil {
				return conn, nil
			}
			dialErr = errors.Join(dialErr, err)
		}
		return nil, dialErr
	}
}

func isPublicImageStudioRemoteIP(ip net.IP) bool {
	_, ok := imageStudioPublicRemoteAddr(ip)
	return ok
}

func imageStudioPublicRemoteAddr(ip net.IP) (netip.Addr, bool) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return netip.Addr{}, false
	}
	for _, prefix := range imageStudioSpecialUseRemotePrefixes {
		if prefix.Contains(addr) {
			return netip.Addr{}, false
		}
	}
	return addr, true
}

func DecodeImageStudioDataURL(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "data:") {
		return nil, "", fmt.Errorf("invalid data url")
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid data url")
	}
	meta := parts[0]
	payload := parts[1]
	ct := "image/png"
	if semi := strings.Index(meta, ";"); semi > 5 {
		ct = meta[5:semi]
	} else if len(meta) > 5 {
		ct = meta[5:]
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > imageStudioMaxDownloadBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	return data, ct, nil
}

func NormalizeImageStudioPayloads(ctx context.Context, images []ImageStudioImagePayload) ([]ImageStudioImagePayload, error) {
	out := make([]ImageStudioImagePayload, 0, len(images))
	for _, img := range images {
		if len(img.Data) == 0 {
			continue
		}
		ct, err := detectImageStudioContentType(img.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, ImageStudioImagePayload{Data: img.Data, ContentType: ct})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no image data")
	}
	return out, nil
}

func detectImageStudioContentType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("image data is empty")
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid image data: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png", nil
	case "jpeg":
		return "image/jpeg", nil
	case "webp":
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unsupported image format %q", format)
	}
}
