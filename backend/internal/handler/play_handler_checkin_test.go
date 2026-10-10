package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type checkinHandlerRepo struct {
	blindboxExplorerHandlerRepo
	energy int64
}

func (*checkinHandlerRepo) InsertCheckin(context.Context, int64, time.Time, float64, int) error {
	return nil
}

func (r *checkinHandlerRepo) InsertGrowthEnergyLedger(_ context.Context, entry service.PlayGrowthEnergyLedgerEntry) error {
	r.energy += entry.Amount
	return nil
}

// Exercise the ordinary HTTP handler and real service, rather than manually
// constructing a DTO that could hide a missing service-to-handler projection.
func TestCheckinHTTPProjectsGrowthReward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectBegin()
	mock.ExpectCommit()
	repo := &checkinHandlerRepo{}
	settings := service.NewSettingService(&blindboxExplorerHandlerSettingRepo{values: map[string]string{
		service.SettingKeyPlayCheckinEnabled: "true",
	}}, nil)
	svc := service.NewPlayService(repo, nil, nil, settings, nil, client)
	svc.RequireGrowthQualification(true)
	router := gin.New()
	router.POST("/api/v1/play/checkin", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	}, NewPlayHandler(svc, nil).Checkin)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/play/checkin", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NoError(t, mock.ExpectationsWereMet())
	require.EqualValues(t, 1, repo.energy)
	var response struct {
		Code int                        `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code)
	require.JSONEq(t, "1", string(response.Data["growth_energy"]))
	var eligibility service.PlayGrowthEligibility
	require.NoError(t, json.Unmarshal(response.Data["growth_eligibility"], &eligibility))
	require.Equal(t, service.PlayGrowthTierExplorer, eligibility.Tier)
	require.Equal(t, service.PlayGrowthRewardEnergy, eligibility.RewardMode)
	require.Equal(t, service.PlayGrowthEligibilityReasonAccountTooNew, eligibility.PrimaryReason)
	require.Equal(t, eligibility.PrimaryReason, eligibility.Progress.NextAction)
	for _, field := range []string{"reward_amount", "balance_added", "daily_reward_amount", "milestone_amount"} {
		require.JSONEq(t, "0", string(response.Data[field]), "legacy field %s", field)
	}
	require.JSONEq(t, `"none"`, string(response.Data["reward_type"]))
	require.NotEmpty(t, response.Data["server_date"])
	require.NotContains(t, response.Data, "coupon")
	require.NotContains(t, response.Data, "redeem_code")
}
