package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type agModelStatsRepo struct {
	usageBatchLogRepoStub
	starts []time.Time
	stats  map[string]*usagestats.AccountStats
}

func (r *agModelStatsRepo) GetAccountWindowStatsByModel(_ context.Context, _ int64, start time.Time) (map[string]*usagestats.AccountStats, error) {
	r.starts = append(r.starts, start)
	return r.stats, nil
}

func TestAntigravityQuotaGroupKeyForModel(t *testing.T) {
	withArgon := []string{"gemini", "gemini-argon-wtu", "3p"}
	require.Equal(t, "gemini", antigravityQuotaGroupKeyForModel("gemini-4-neon-xhigh", withArgon))
	require.Equal(t, "gemini-argon-wtu", antigravityQuotaGroupKeyForModel("models/gemini-4-argon-high", withArgon))
	require.Equal(t, "3p", antigravityQuotaGroupKeyForModel("claude-sonnet-4-6", withArgon))
	require.Equal(t, "3p", antigravityQuotaGroupKeyForModel("gpt-oss-120b-medium", withArgon))

	// Plans without a separate Argon group count Argon under Gemini.
	require.Equal(t, "gemini", antigravityQuotaGroupKeyForModel("gemini-4-argon-high", []string{"gemini", "3p"}))
	require.Equal(t, "", antigravityQuotaGroupKeyForModel("claude-sonnet-4-6", []string{"gemini"}))
}

func TestAntigravityQuotaWindowStats(t *testing.T) {
	now := time.Now()
	fiveReset, weeklyReset, expired := now.Add(2*time.Hour), now.Add(3*24*time.Hour), now.Add(-time.Minute)
	repo := &agModelStatsRepo{stats: map[string]*usagestats.AccountStats{
		"gemini-4-neon-xhigh": {Requests: 3, Tokens: 300, Cost: 1.5, StandardCost: 1.5, UserCost: 1.5},
		"gemini-4-hydrogen":   {Requests: 1, Tokens: 100, Cost: 0.5, StandardCost: 0.5, UserCost: 0.5},
		"claude-sonnet-4-6":   {Requests: 2, Tokens: 50, Cost: 4, StandardCost: 4, UserCost: 4},
	}}
	svc := &AccountUsageService{usageLogRepo: repo, cache: NewUsageCache()}
	usage := &UsageInfo{AntigravityQuotaGroups: []*AntigravityQuotaGroup{
		{Key: "gemini", Windows: []*AntigravityQuotaWindow{
			{Window: "5h", Utilization: 25, ResetsAt: &fiveReset},
			{Window: "weekly", Utilization: 10, ResetsAt: &weeklyReset},
		}},
		{Key: "3p", Windows: []*AntigravityQuotaWindow{
			{Window: "5h", Utilization: 50, ResetsAt: &expired},
			{Window: "weekly", Utilization: 5},
		}},
	}}

	out := svc.withAntigravityQuotaWindowStats(context.Background(), &Account{ID: 7}, usage)

	gemini5h := out.AntigravityQuotaGroups[0].Windows[0].WindowStats
	require.NotNil(t, gemini5h)
	require.Equal(t, int64(4), gemini5h.Requests)
	require.InDelta(t, 2.0, gemini5h.Cost, 1e-9)
	require.NotNil(t, out.AntigravityQuotaGroups[0].Windows[1].WindowStats)
	// An expired window or one without a reset time is not paired with local cost.
	require.Nil(t, out.AntigravityQuotaGroups[1].Windows[0].WindowStats)
	require.Nil(t, out.AntigravityQuotaGroups[1].Windows[1].WindowStats)
	require.Equal(t, []time.Time{fiveReset.Add(-5 * time.Hour), weeklyReset.Add(-7 * 24 * time.Hour)}, repo.starts)

	// The cached usage shared by other callers is left untouched.
	require.Nil(t, usage.AntigravityQuotaGroups[0].Windows[0].WindowStats)

	// A second call within the TTL reuses the per-window query.
	svc.withAntigravityQuotaWindowStats(context.Background(), &Account{ID: 7}, usage)
	require.Len(t, repo.starts, 2)
}
