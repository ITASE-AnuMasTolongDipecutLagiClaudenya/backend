package scanbudget

import (
	"testing"
	"time"
)

func TestCacheLookupAndStore(t *testing.T) {
	c := NewCache(time.Hour)
	key := Key("8991002101234", "original", "Susu")
	if _, ok := c.Lookup(key); ok {
		t.Fatal("empty cache should miss")
	}
	c.Store(key, Entry{Name: "Susu", Barcode: "8991002101234", Calories: 120})
	hit, ok := c.Lookup(key)
	if !ok || hit.Calories != 120 {
		t.Fatalf("stored entry should hit: %#v, %v", hit, ok)
	}
}

func TestCacheSameBarcodeDifferentVariant(t *testing.T) {
	c := NewCache(time.Hour)
	a := Key("8991002101234", "original", "Susu")
	b := Key("8991002101234", "cokelat", "Susu")
	c.Store(a, Entry{Calories: 120})
	if _, ok := c.Lookup(b); ok {
		t.Fatal("different variant must not share a cache entry")
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache(time.Millisecond)
	key := Key("", "", "Apel")
	c.Store(key, Entry{Name: "Apel"})
	time.Sleep(5 * time.Millisecond)
	if _, ok := c.Lookup(key); ok {
		t.Fatal("expired entry should miss")
	}
}

func TestCacheKeyFallsBackToNameWithoutBarcode(t *testing.T) {
	if Key("", "", "Susu") == Key("", "", "Teh") {
		t.Fatal("distinct names without barcode must produce distinct keys")
	}
	if Key("", "", "Susu") != Key("", "", " susu ") {
		t.Fatal("name key should be case/space normalized")
	}
}

func TestQuotaDailyLimit(t *testing.T) {
	q := NewQuota(2, 100)
	if !q.Allow("u1") || !q.Allow("u1") {
		t.Fatal("first two requests should be allowed")
	}
	if q.Allow("u1") {
		t.Fatal("third request should exceed daily limit")
	}
	if !q.Allow("u2") {
		t.Fatal("other user should not be affected by u1's limit")
	}
}

func TestQuotaMonthlyLimit(t *testing.T) {
	q := NewQuota(100, 2)
	q.Allow("u1")
	q.Allow("u1")
	if q.Allow("u1") {
		t.Fatal("monthly limit should cap requests")
	}
}

func TestDayKeyUsesWIB(t *testing.T) {
	if got := dayKey(time.Date(2026, time.October, 18, 16, 59, 0, 0, time.UTC)); got != "2026-10-18" {
		t.Fatalf("16:59 UTC is 23:59 WIB, want 2026-10-18, got %s", got)
	}
	if got := dayKey(time.Date(2026, time.October, 18, 17, 0, 0, 0, time.UTC)); got != "2026-10-19" {
		t.Fatalf("17:00 UTC is 00:00 WIB, want 2026-10-19, got %s", got)
	}
	if got := monthKey(time.Date(2026, time.November, 1, 17, 0, 0, 0, time.UTC)); got != "2026-11" {
		t.Fatalf("want 2026-11, got %s", got)
	}
}
