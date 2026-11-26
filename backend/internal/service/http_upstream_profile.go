package service

import (
	"context"
	"net/http"
)

// HTTPUpstreamProfile marks HTTP upstream requests that need provider-specific
// transport policy.
type HTTPUpstreamProfile string

const (
	HTTPUpstreamProfileDefault    HTTPUpstreamProfile = ""
	HTTPUpstreamProfileOpenAI     HTTPUpstreamProfile = "openai"
	HTTPUpstreamProfileGrok       HTTPUpstreamProfile = "grok"
	HTTPUpstreamProfileLongStream HTTPUpstreamProfile = "long_stream"
)

type plainUpstreamRequestBodyKey struct{}
type openAICodexHTTPRequestKey struct{}

// WithPlainUpstreamRequestBody protects the final body of a single-submission
// or integrity-checked request from transport encoding.
func WithPlainUpstreamRequestBody(ctx context.Context) context.Context {
	return context.WithValue(ctx, plainUpstreamRequestBodyKey{}, true)
}

func UpstreamCompressionRequiresPlainBody(ctx context.Context) bool {
	plain, _ := ctx.Value(plainUpstreamRequestBodyKey{}).(bool)
	return plain || IsOpenAICodexStateProbeRequest(ctx) || OpenAIAgentRecoveryNoReplay(ctx)
}

func markOpenAICodexHTTPRequest(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), openAICodexHTTPRequestKey{}, true))
}

func IsOpenAICodexHTTPRequest(ctx context.Context) bool {
	marked, _ := ctx.Value(openAICodexHTTPRequestKey{}).(bool)
	return marked
}

type httpUpstreamProfileContextKey struct{}
type httpUpstreamDisableRedirectsContextKey struct{}
type httpUpstreamPublicHostsOnlyContextKey struct{}

// WithHTTPUpstreamProfile injects an upstream transport profile into ctx.
func WithHTTPUpstreamProfile(ctx context.Context, profile HTTPUpstreamProfile) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if profile == HTTPUpstreamProfileDefault {
		return ctx
	}
	return context.WithValue(ctx, httpUpstreamProfileContextKey{}, profile)
}

// HTTPUpstreamProfileFromContext resolves the upstream transport profile from ctx.
func HTTPUpstreamProfileFromContext(ctx context.Context) HTTPUpstreamProfile {
	if ctx == nil {
		return HTTPUpstreamProfileDefault
	}
	profile, ok := ctx.Value(httpUpstreamProfileContextKey{}).(HTTPUpstreamProfile)
	if !ok {
		return HTTPUpstreamProfileDefault
	}
	switch profile {
	case HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileGrok, HTTPUpstreamProfileLongStream:
		return profile
	default:
		return HTTPUpstreamProfileDefault
	}
}

// WithHTTPUpstreamRedirectsDisabled prevents credential-bearing probes from
// following redirects through the shared upstream client.
func WithHTTPUpstreamRedirectsDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, httpUpstreamDisableRedirectsContextKey{}, true)
}

func HTTPUpstreamRedirectsDisabled(ctx context.Context) bool {
	return ctx != nil && ctx.Value(httpUpstreamDisableRedirectsContextKey{}) == true
}

// WithHTTPUpstreamPublicHostsOnly marks a request whose destination, and every
// redirect hop after it, must resolve to a public address. The shared upstream
// client enforces it regardless of the security.url_allowlist configuration;
// use it for fetches whose URL comes from an untrusted upstream response.
func WithHTTPUpstreamPublicHostsOnly(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, httpUpstreamPublicHostsOnlyContextKey{}, true)
}

func HTTPUpstreamPublicHostsOnly(ctx context.Context) bool {
	return ctx != nil && ctx.Value(httpUpstreamPublicHostsOnlyContextKey{}) == true
}
