package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// accountModelWindowStatsReader is implemented by the usage log repository. It is optional so
// test stubs of UsageLogRepository need not implement it.
type accountModelWindowStatsReader interface {
	GetAccountWindowStatsByModel(ctx context.Context, accountID int64, startTime time.Time) (map[string]*usagestats.AccountStats, error)
}

type antigravityModelStatsCacheKey struct {
	accountID int64
	duration  time.Duration
}

type antigravityModelStatsCache struct {
	stats     map[string]*usagestats.AccountStats
	timestamp time.Time
	start     time.Time
}

func antigravityQuotaWindowDuration(window string) time.Duration {
	switch window {
	case "5h":
		return 5 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	}
	return 0
}

// antigravityQuotaGroupKeyForModel picks which quota group a model draws from. The summary does
// not list a group's models, so this goes by name: Gemini models use the "gemini" group, except
// Argon on plans that report a separate "gemini-argon-*" group; everything else (Claude, GPT)
// uses the remaining group ("3p").
func antigravityQuotaGroupKeyForModel(model string, keys []string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "models/")
	find := func(match func(string) bool) string {
		for _, k := range keys {
			if match(k) {
				return k
			}
		}
		return ""
	}
	if strings.Contains(m, "gemini") {
		if strings.Contains(m, "argon") {
			if k := find(func(k string) bool { return strings.HasPrefix(k, "gemini-argon") }); k != "" {
				return k
			}
		}
		return find(func(k string) bool { return k == "gemini" })
	}
	return find(func(k string) bool { return !strings.HasPrefix(k, "gemini") })
}

// withAntigravityQuotaWindowStats returns a copy of usage whose quota-group windows carry this
// account's local usage since each window started, so the UI can show cost and a projected
// full-window cost like it does for Codex. usage itself is shared through the quota cache and
// is not modified.
func (s *AccountUsageService) withAntigravityQuotaWindowStats(ctx context.Context, account *Account, usage *UsageInfo) *UsageInfo {
	if usage == nil || account == nil || len(usage.AntigravityQuotaGroups) == 0 {
		return usage
	}
	reader, ok := s.usageLogRepo.(accountModelWindowStatsReader)
	if !ok || reader == nil {
		return usage
	}

	keys := make([]string, 0, len(usage.AntigravityQuotaGroups))
	for _, g := range usage.AntigravityQuotaGroups {
		if g != nil && g.Key != "" {
			keys = append(keys, g.Key)
		}
	}

	now := time.Now()
	out := *usage
	out.AntigravityQuotaGroups = make([]*AntigravityQuotaGroup, len(usage.AntigravityQuotaGroups))
	for i, g := range usage.AntigravityQuotaGroups {
		if g == nil {
			continue
		}
		group := *g
		group.Windows = make([]*AntigravityQuotaWindow, len(g.Windows))
		for j, w := range g.Windows {
			if w == nil {
				continue
			}
			window := *w
			window.WindowStats = nil
			group.Windows[j] = &window

			duration := antigravityQuotaWindowDuration(w.Window)
			// Only pair local cost with a known, current window; otherwise the projection is wrong.
			if duration == 0 || w.ResetsAt == nil || !now.Before(*w.ResetsAt) {
				continue
			}
			start := w.ResetsAt.Add(-duration)
			if start.After(now) {
				continue
			}
			byModel := s.antigravityModelStatsSince(ctx, reader, account.ID, duration, start, now)
			if byModel == nil {
				continue
			}
			sum := &usagestats.AccountStats{}
			for model, st := range byModel {
				if st == nil || antigravityQuotaGroupKeyForModel(model, keys) != g.Key {
					continue
				}
				sum.Requests += st.Requests
				sum.Tokens += st.Tokens
				sum.Cost += st.Cost
				sum.StandardCost += st.StandardCost
				sum.UserCost += st.UserCost
			}
			window.WindowStats = windowStatsFromAccountStats(sum)
		}
		out.AntigravityQuotaGroups[i] = &group
	}
	return &out
}

func (s *AccountUsageService) antigravityModelStatsSince(ctx context.Context, reader accountModelWindowStatsReader, accountID int64, duration time.Duration, start, now time.Time) map[string]*usagestats.AccountStats {
	key := antigravityModelStatsCacheKey{accountID: accountID, duration: duration}
	if cached, ok := s.cache.antigravityModelStatsCache.Load(key); ok {
		if c, ok := cached.(*antigravityModelStatsCache); ok && c.start.Equal(start) && now.Sub(c.timestamp) < windowStatsCacheTTL {
			return c.stats
		}
	}
	stats, err := reader.GetAccountWindowStatsByModel(ctx, accountID, start)
	if err != nil {
		log.Printf("Failed to get antigravity window stats for account %d: %v", accountID, err)
		return nil
	}
	s.cache.antigravityModelStatsCache.Store(key, &antigravityModelStatsCache{stats: stats, timestamp: now, start: start})
	return stats
}
