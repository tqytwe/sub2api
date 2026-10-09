package repository

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// External mode is deliberately restricted to disposable, password-free local
// fixtures. Never include supplied URLs in errors: they may contain credentials.
// Opt in with SUB2API_TEST_EXTERNAL_MODE=local, SUB2API_TEST_POSTGRES_DSN
// (postgres URL, user sub2api_test, database sub2api_test_<suffix>, sslmode=disable),
// and SUB2API_TEST_REDIS_ADDR. Both services require numeric loopback addresses
// and explicit non-default unprivileged ports. Start fresh fixtures for each run;
// ApplyMigrations modifies the selected test database. Docker remains the default.
type externalIntegrationConfig struct {
	enabled     bool
	postgresDSN string
	redisAddr   string
}

var externalTestDatabaseName = regexp.MustCompile(`^sub2api_test_[a-z0-9_]+$`)

func parseExternalIntegrationConfig(mode, dsn, redisAddr string) (externalIntegrationConfig, error) {
	invalid := func() (externalIntegrationConfig, error) {
		return externalIntegrationConfig{}, errors.New("external integration mode requires explicit local mode, an isolated test database/user, and numeric loopback addresses with non-default ports")
	}
	if mode == "" && dsn == "" && redisAddr == "" {
		return externalIntegrationConfig{}, nil
	}
	if mode != "local" {
		return invalid()
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" || u.Opaque != "" || u.Fragment != "" || u.User == nil || u.User.Username() != "sub2api_test" {
		return invalid()
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return invalid()
	}
	if !isolatedLoopbackAddress(u.Host, 5432) || !strings.HasPrefix(u.Path, "/") || len(strings.TrimPrefix(u.Path, "/")) > 63 || !externalTestDatabaseName.MatchString(strings.TrimPrefix(u.Path, "/")) {
		return invalid()
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) != 1 || len(query["sslmode"]) != 1 || query.Get("sslmode") != "disable" {
		return invalid()
	}
	if !isolatedLoopbackAddress(redisAddr, 6379) {
		return invalid()
	}
	return externalIntegrationConfig{enabled: true, postgresDSN: dsn, redisAddr: redisAddr}, nil
}

func isolatedLoopbackAddress(addr string, standardPort int) bool {
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	port, err := strconv.Atoi(portString)
	return err == nil && ip != nil && ip.IsLoopback() && port > 1024 && port <= 65535 && port != standardPort
}
