package catalog

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func formatCount(n int) string {
	if n >= 1000 {
		v := float64(n) / 1000
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return strings.TrimSuffix(fmt.Sprintf("%.1fk", v), ".0k") + "k"
	}
	return fmt.Sprintf("%d", n)
}

func metricFor(item Item) Metric {
	switch item.Kind {
	case "creator":
		return Metric{Key: "followers", Label: "followers", Value: item.Stats.Followers, Display: formatCount(item.Stats.Followers) + " followers"}
	case "api":
		if item.Stats.Runs > item.Stats.Pulls {
			return Metric{Key: "runs", Label: "runs", Value: item.Stats.Runs, Display: formatCount(item.Stats.Runs) + " runs"}
		}
		if item.Stats.Pulls > 0 {
			return Metric{Key: "pulls", Label: "pulls", Value: item.Stats.Pulls, Display: formatCount(item.Stats.Pulls) + " pulls"}
		}
	default:
		if item.Type == "modded_apk" || item.Type == "custom_rom" {
			if item.Stats.Pulls > 0 {
				return Metric{Key: "pulls", Label: "pulls", Value: item.Stats.Pulls, Display: formatCount(item.Stats.Pulls) + " pulls"}
			}
		}
		if item.Stats.Forks > item.Stats.Saves {
			return Metric{Key: "forks", Label: "forks", Value: item.Stats.Forks, Display: formatCount(item.Stats.Forks) + " forks"}
		}
	}
	return Metric{Key: "saves", Label: "saves", Value: item.Stats.Saves, Display: formatCount(item.Stats.Saves) + " saves"}
}

func trendingScore(item Item, now time.Time) float64 {
	ageHours := now.Sub(item.PublishedAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	ageDays := ageHours / 24

	engagement := float64(item.Stats.Saves)*2.4 +
		float64(item.Stats.Likes)*1.6 +
		float64(item.Stats.Forks)*3.1 +
		float64(item.Stats.Pulls)*1.2 +
		float64(item.Stats.Runs)*0.4 +
		float64(item.Stats.Followers)*0.8

	recency := math.Exp(-ageDays / 10)
	freshBoost := 1.0
	if ageDays <= 3 {
		freshBoost = 1.45
	} else if ageDays <= 7 {
		freshBoost = 1.2
	}

	return engagement * recency * freshBoost
}

func sortTrending(items []Item, now time.Time) []Item {
	out := append([]Item(nil), items...)
	for i := range out {
		out[i].TrendingScore = math.Round(trendingScore(out[i], now)*100) / 100
		out[i].Metric = metricFor(out[i])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TrendingScore == out[j].TrendingScore {
			if out[i].Stats.Saves == out[j].Stats.Saves {
				return out[i].PublishedAt.After(out[j].PublishedAt)
			}
			return out[i].Stats.Saves > out[j].Stats.Saves
		}
		return out[i].TrendingScore > out[j].TrendingScore
	})
	return out
}

func sortFresh(items []Item) []Item {
	out := append([]Item(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PublishedAt.After(out[j].PublishedAt)
	})
	for i := range out {
		out[i].Metric = metricFor(out[i])
	}
	return out
}

func cloneItems(items []Item) []Item {
	out := make([]Item, len(items))
	copy(out, items)
	return out
}
