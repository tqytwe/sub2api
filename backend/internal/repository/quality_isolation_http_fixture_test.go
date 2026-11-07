//go:build integration

package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Test-only bridge: HTTP tests live in repository_test to avoid a production
// dependency cycle. They reuse the same isolated PostgreSQL harness and fixture.
func NewQualityIsolationHTTPFixture(t *testing.T) (service.ScheduledTestPlanRepository, *service.ScheduledTestPlan, *sql.DB) {
	f := newIsolationFixture(t)
	return f.repo, f.plan, integrationDB
}
