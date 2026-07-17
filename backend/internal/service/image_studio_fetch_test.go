package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsPublicImageStudioRemoteIPRejectsSpecialUseAddresses(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"0.0.0.0",
		"0.1.2.3",
		"10.0.0.1",
		"100.64.0.1",
		"100.127.255.254",
		"127.0.0.1",
		"169.254.1.1",
		"172.16.0.1",
		"192.0.0.1",
		"192.0.2.1",
		"192.31.196.1",
		"192.52.193.1",
		"192.88.99.1",
		"192.168.1.1",
		"192.175.48.1",
		"198.18.0.1",
		"198.51.100.1",
		"203.0.113.1",
		"224.0.0.1",
		"240.0.0.1",
		"255.255.255.255",
		"::",
		"::1",
		"64:ff9b::1",
		"64:ff9b:1::1",
		"100::1",
		"100:0:0:1::1",
		"2001::1",
		"2001:db8::1",
		"2002::1",
		"2620:4f:8000::1",
		"3fff::1",
		"5f00::1",
		"fc00::1",
		"fe80::1",
		"ff02::1",
	}

	for _, rawIP := range blocked {
		rawIP := rawIP
		t.Run(rawIP, func(t *testing.T) {
			t.Parallel()
			require.False(t, isPublicImageStudioRemoteIP(net.ParseIP(rawIP)))
		})
	}
	require.False(t, isPublicImageStudioRemoteIP(nil))
}

func TestIsPublicImageStudioRemoteIPAllowsPublicAddresses(t *testing.T) {
	t.Parallel()

	for _, rawIP := range []string{
		"1.1.1.1",
		"8.8.8.8",
		"93.184.216.34",
		"2001:4860:4860::8888",
		"2606:4700:4700::1111",
	} {
		require.True(t, isPublicImageStudioRemoteIP(net.ParseIP(rawIP)), rawIP)
	}
}

func TestImageStudioSafeDialContextRejectsMixedResolverResultsBeforeDial(t *testing.T) {
	t.Parallel()

	dialCalled := false
	safeDial := imageStudioSafeDialContext(
		func(_ context.Context, network, host string) ([]net.IP, error) {
			require.Equal(t, "ip", network)
			require.Equal(t, "cdn.example.com", host)
			return []net.IP{
				net.ParseIP("93.184.216.34"),
				net.ParseIP("100.64.0.8"),
			}, nil
		},
		func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("must not dial")
		},
	)

	conn, err := safeDial(context.Background(), "tcp", "cdn.example.com:443")

	require.Nil(t, conn)
	require.ErrorContains(t, err, "resolved ip 100.64.0.8")
	require.False(t, dialCalled)
}

func TestImageStudioSafeDialContextDialsOnlyResolvedPublicCandidates(t *testing.T) {
	t.Parallel()

	var dialed []string
	safeDial := imageStudioSafeDialContext(
		func(context.Context, string, string) ([]net.IP, error) {
			return []net.IP{
				net.ParseIP("93.184.216.34"),
				net.ParseIP("2606:4700:4700::1111"),
			}, nil
		},
		func(_ context.Context, network, address string) (net.Conn, error) {
			require.Equal(t, "tcp", network)
			dialed = append(dialed, address)
			if len(dialed) == 1 {
				return nil, errors.New("first candidate unavailable")
			}
			client, server := net.Pipe()
			t.Cleanup(func() {
				_ = client.Close()
				_ = server.Close()
			})
			return client, nil
		},
	)

	conn, err := safeDial(context.Background(), "tcp", "cdn.example.com:443")

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, []string{
		"93.184.216.34:443",
		"[2606:4700:4700::1111]:443",
	}, dialed)
}

func TestImageStudioRemoteRedirectPolicyRevalidatesEveryTarget(t *testing.T) {
	t.Parallel()

	checkRedirect := newImageStudioRemoteHTTPClient().CheckRedirect
	require.NotNil(t, checkRedirect)

	safeRequest, err := http.NewRequest(http.MethodGet, "https://example.com/image.png", nil)
	require.NoError(t, err)
	require.NoError(t, checkRedirect(safeRequest, []*http.Request{{}}))

	for _, rawURL := range []string{
		"http://example.com/image.png",
		"https://localhost/image.png",
		"https://user@example.com/image.png",
		"https://example.com/image.png#fragment",
	} {
		req, reqErr := http.NewRequest(http.MethodGet, rawURL, nil)
		require.NoError(t, reqErr)
		require.Error(t, checkRedirect(req, []*http.Request{{}}), rawURL)
	}

	require.ErrorContains(t, checkRedirect(safeRequest, make([]*http.Request, 5)), "redirect limit")
}
