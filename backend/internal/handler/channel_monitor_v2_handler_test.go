package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorV2MatrixRepoStub struct {
	filter service.ChannelMonitorV2Filter
}

func (s *channelMonitorV2MatrixRepoStub) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{Enabled: true}, nil
}
func (s *channelMonitorV2MatrixRepoStub) UpdateConfig(context.Context, service.ChannelMonitorV2Config, int) (*service.ChannelMonitorV2Config, error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetDimensions(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Dimensions, error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetSnapshot(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2Snapshot, error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetModels(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow], error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetMatrix(_ context.Context, filter service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, groupBy service.ChannelMonitorV2GroupBy, _ bool) (*service.ChannelMonitorV2Matrix, error) {
	s.filter = filter
	return &service.ChannelMonitorV2Matrix{GroupBy: groupBy, Items: []service.ChannelMonitorV2MatrixRow{}}, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetErrors(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2ErrorRow], error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetUsers(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2UserRow], error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) GetAggregationWatermark(context.Context) (*service.ChannelMonitorV2AggregationWatermark, error) {
	return nil, nil
}
func (s *channelMonitorV2MatrixRepoStub) RecomputeRange(context.Context, time.Time, time.Time) error {
	return nil
}

type channelMonitorV2GroupAuthorizerStub struct {
	groups []service.Group
	err    error
	calls  []int64
}

func (s *channelMonitorV2GroupAuthorizerStub) GetAvailableGroups(_ context.Context, userID int64) ([]service.Group, error) {
	s.calls = append(s.calls, userID)
	return s.groups, s.err
}

func TestChannelMonitorV2QueryListSupportsRepeatedAndCommaValues(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest("GET", "/?platform=openai,grok&platform=anthropic", nil)
	require.Equal(t, []string{"openai", "grok", "anthropic"}, queryList(c, "platform"))
}

func TestChannelMonitorV2GroupByQueryDefaultsAndRejectsInvalid(t *testing.T) {
	groupBy, err := service.ParseChannelMonitorV2GroupBy("")
	require.NoError(t, err)
	require.Equal(t, service.ChannelMonitorV2GroupByPlatformGroup, groupBy)
	_, err = service.ParseChannelMonitorV2GroupBy("invalid")
	require.Error(t, err)
}

func TestChannelMonitorV2MatrixHandlerRejectsInvalidGroupBy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/matrix?group_by=invalid", nil)
	h := NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(nil), nil)
	h.Matrix(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestChannelMonitorV2MatrixHandlerParsesRecentSamplesBoolean(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{"true", "1"} {
		t.Run(value, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/admin/channel-monitor-v2/matrix?recent_samples="+value, nil)
			repo := &channelMonitorV2MatrixRepoStub{}
			h := NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(repo), nil)

			h.AdminMatrix(c)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.True(t, repo.filter.IncludeRecentSamples)
		})
	}
}

func TestChannelMonitorV2ScopeFilterUsesAvailableGroupsForOrdinaryUser(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	authorizer := &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 3}, {ID: 7}}}
	h := &ChannelMonitorV2Handler{apiKeyService: authorizer}
	filter := service.ChannelMonitorV2Filter{GroupIDs: []int64{7, 9}}

	require.True(t, h.scopeFilter(c, &filter, false))
	require.True(t, filter.RestrictGroups)
	require.Equal(t, []int64{3, 7}, filter.AllowedGroupIDs)
	require.Equal(t, []int64{42}, authorizer.calls)
}

func TestChannelMonitorV2ScopeFilterPreservesEmptyOrdinaryUserScope(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h := &ChannelMonitorV2Handler{apiKeyService: &channelMonitorV2GroupAuthorizerStub{}}
	filter := service.ChannelMonitorV2Filter{}

	require.True(t, h.scopeFilter(c, &filter, false))
	require.True(t, filter.RestrictGroups)
	require.Empty(t, filter.AllowedGroupIDs)
}

func TestChannelMonitorV2ScopeFilterLeavesAdminUnrestricted(t *testing.T) {
	authorizer := &channelMonitorV2GroupAuthorizerStub{}
	h := &ChannelMonitorV2Handler{apiKeyService: authorizer}
	filter := service.ChannelMonitorV2Filter{GroupIDs: []int64{9}}

	require.True(t, h.scopeFilter(nil, &filter, true))
	require.False(t, filter.RestrictGroups)
	require.Nil(t, filter.AllowedGroupIDs)
	require.Empty(t, authorizer.calls)
}
