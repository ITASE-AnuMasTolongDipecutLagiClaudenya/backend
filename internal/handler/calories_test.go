package handler

import (
	"testing"
	"time"
)

func TestCalorieBucketsAndAggregation(t *testing.T) {
	location := time.FixedZone("WIB", 7*60*60)
	now := time.Date(2026, time.October, 12, 12, 0, 0, 0, location)
	day := calorieBuckets("day", now)
	if len(day) != 7 || day[0].Label != "Sen, 12 Okt" || day[6].Label != "Min, 18 Okt" {
		t.Fatalf("daily buckets = %#v", day)
	}
	week := calorieBuckets("week", now)
	if len(week) != 4 || !week[3].End.Equal(time.Date(2026, time.November, 1, 0, 0, 0, 0, location)) {
		t.Fatalf("weekly buckets = %#v", week)
	}
	if got := len(calorieBuckets("month", now)); got != 12 {
		t.Fatalf("monthly bucket count = %d, want 12", got)
	}
	year := calorieBuckets("year", now)
	if len(year) != 6 || year[0].Label != "2027" || year[5].Label != "2032" {
		t.Fatalf("yearly buckets = %#v", year)
	}
	points := aggregateCalories([]map[string]any{
		{"created_at": "2026-10-12T01:00:00Z", "calories": float64(400)},
		{"created_at": "2026-10-12T18:00:00Z", "calories": float64(250)},
		{"created_at": "2026-10-19T00:00:00Z", "calories": float64(900)},
	}, day)
	if points[0].Calories != 400 || points[1].Calories != 250 {
		t.Fatalf("daily calorie totals = %#v", points)
	}
}

func TestAggregateCaloriesHandlesDatabaseTimestampOffsets(t *testing.T) {
	buckets := calorieBuckets("day", time.Date(2026, time.October, 12, 12, 0, 0, 0, time.FixedZone("WIB", 7*60*60)))
	points := aggregateCalories([]map[string]any{{
		"created_at": "2026-10-12T23:30:00-05:00",
		"calories":   float64(321.5),
	}}, buckets)
	if points[1].Calories != 321.5 {
		t.Fatalf("offset timestamp should map to Tuesday WIB, got %#v", points)
	}
}
