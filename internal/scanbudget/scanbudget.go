// Package scanbudget enforces the cost controls around the OCR + web-search
// scan pipeline: a product cache keyed by barcode+variant (so re-scanning the
// same product never re-pays Google Search / AI), and per-user daily and
// monthly scan quotas.
//
// Both are process-local (the backend is a single instance). A restart
// resets quota counters and the in-memory cache; cache entries expire after
// their TTL anyway, and quota is a soft guard against runaway spend, not a
// billing ledger.
package scanbudget

import (
	"strings"
	"sync"
	"time"
)

// Entry is a cached nutrition lookup result for a single product variant.
type Entry struct {
	Name     string   `json:"name"`
	Barcode  string   `json:"barcode,omitempty"`
	Variant  string   `json:"variant,omitempty"`
	Calories float64  `json:"calories,omitempty"`
	Protein  float64  `json:"protein_g,omitempty"`
	Carbs    float64  `json:"carbs_g,omitempty"`
	Fat      float64  `json:"fat_g,omitempty"`
	Sugar    *float64 `json:"sugar_g,omitempty"`
	Sodium   *float64 `json:"sodium_mg,omitempty"`
	Source   string   `json:"source,omitempty"`
}

// Cache is a concurrency-safe product cache with TTL eviction.
type Cache struct {
	mu   sync.Mutex
	data map[string]cachedEntry
	ttl  time.Duration
}

type cachedEntry struct {
	entry     Entry
	expiresAt time.Time
}

// NewCache creates a cache with the given entry lifetime.
func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	return &Cache{data: map[string]cachedEntry{}, ttl: ttl}
}

// Key derives a stable cache key from barcode and variant. It falls back to a
// normalized name when no barcode is present (e.g. fresh produce, receipts).
func Key(barcode, variant, name string) string {
	barcode = strings.TrimSpace(barcode)
	variant = strings.TrimSpace(variant)
	if barcode != "" {
		return "b:" + barcode + "|v:" + variant
	}
	return "n:" + strings.ToLower(strings.TrimSpace(name)) + "|v:" + variant
}

// Lookup returns a cached, unexpired entry for the key, if any.
func (c *Cache) Lookup(key string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.data[key]
	if !ok || time.Now().After(hit.expiresAt) {
		if ok {
			delete(c.data, key)
		}
		return Entry{}, false
	}
	return hit.entry, true
}

// Store records an entry, refreshing its TTL.
func (c *Cache) Store(key string, entry Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cachedEntry{entry: entry, expiresAt: time.Now().Add(c.ttl)}
}

// Quota tracks per-user daily and monthly usage against configured limits.
type Quota struct {
	dailyLimit   int
	monthlyLimit int

	mu      sync.Mutex
	daily   map[string]int
	monthly map[string]int
	day     string
	month   string
}

// NewQuota creates a quota tracker. day/month keys roll over automatically.
func NewQuota(dailyLimit, monthlyLimit int) *Quota {
	now := time.Now()
	return &Quota{
		dailyLimit:   dailyLimit,
		monthlyLimit: monthlyLimit,
		daily:        map[string]int{},
		monthly:      map[string]int{},
		day:          dayKey(now),
		month:        monthKey(now),
	}
}

var wib = time.FixedZone("WIB", 7*60*60)

func dayKey(t time.Time) string   { return t.In(wib).Format("2006-01-02") }
func monthKey(t time.Time) string { return t.In(wib).Format("2006-01") }

// Allow consumes one unit for user and reports whether a scan may proceed.
// It returns false when either the daily or monthly limit is exhausted.
func (q *Quota) Allow(userID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	if dk := dayKey(now); dk != q.day {
		q.day = dk
		q.daily = map[string]int{}
	}
	if mk := monthKey(now); mk != q.month {
		q.month = mk
		q.monthly = map[string]int{}
	}
	if q.dailyLimit > 0 && q.daily[userID] >= q.dailyLimit {
		return false
	}
	if q.monthlyLimit > 0 && q.monthly[userID] >= q.monthlyLimit {
		return false
	}
	q.daily[userID]++
	q.monthly[userID]++
	return true
}
