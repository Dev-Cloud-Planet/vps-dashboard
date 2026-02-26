package geo

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// GeoResult holds geolocation data for a single IP address.
type GeoResult struct {
	Country string  `json:"country"`
	City    string  `json:"city"`
	ISP     string  `json:"isp"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Proxy   bool    `json:"proxy"`
}

// ipAPIResponse mirrors the JSON returned by ip-api.com/json/<ip>.
type ipAPIResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Proxy       bool    `json:"proxy"`
	Message     string  `json:"message"`
}

// Locator performs IP geolocation lookups against ip-api.com with an
// in-memory cache and a token-bucket rate limiter (45 requests per minute)
// to stay within the free-tier limits.
type Locator struct {
	cache   map[string]*cacheEntry
	mu      sync.RWMutex
	limiter *rateLimiter
	client  *http.Client
}

type cacheEntry struct {
	result    *GeoResult
	expiresAt time.Time
}

const (
	cacheTTL       = 24 * time.Hour
	rateLimit      = 45 // requests per minute
	ratePeriod     = time.Minute
	requestTimeout = 5 * time.Second
)

// NewLocator creates a Locator with an empty cache and a fresh rate limiter.
func NewLocator() *Locator {
	return &Locator{
		cache:   make(map[string]*cacheEntry),
		limiter: newRateLimiter(rateLimit, ratePeriod),
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Lookup returns geolocation data for the given IP. Private/reserved IPs
// are silently skipped (nil result, nil error). Results are cached for 24h.
func (l *Locator) Lookup(ip string) (*GeoResult, error) {
	if isPrivateIP(ip) {
		return nil, nil
	}

	// Check cache.
	l.mu.RLock()
	if entry, ok := l.cache[ip]; ok && time.Now().Before(entry.expiresAt) {
		l.mu.RUnlock()
		return entry.result, nil
	}
	l.mu.RUnlock()

	// Wait for a rate-limit token.
	l.limiter.wait()

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,city,isp,lat,lon,proxy", ip)
	resp, err := l.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("geo: http get %s: %w", ip, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Back off and return a cache miss rather than an error.
		log.Printf("geo: rate limited by ip-api.com for %s", ip)
		return nil, nil
	}

	var apiResp ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("geo: decode response for %s: %w", ip, err)
	}

	if apiResp.Status != "success" {
		log.Printf("geo: lookup failed for %s: %s", ip, apiResp.Message)
		return nil, nil
	}

	result := &GeoResult{
		Country: apiResp.Country,
		City:    apiResp.City,
		ISP:     apiResp.ISP,
		Lat:     apiResp.Lat,
		Lon:     apiResp.Lon,
		Proxy:   apiResp.Proxy,
	}

	// Store in cache.
	l.mu.Lock()
	l.cache[ip] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(cacheTTL),
	}
	l.mu.Unlock()

	return result, nil
}

// PurgeExpired removes stale entries from the cache. It is safe to call
// from a background goroutine at regular intervals.
func (l *Locator) PurgeExpired() {
	now := time.Now()
	l.mu.Lock()
	for ip, entry := range l.cache {
		if now.After(entry.expiresAt) {
			delete(l.cache, ip)
		}
	}
	l.mu.Unlock()
}

// isPrivateIP returns true for loopback, link-local, and RFC-1918 addresses.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true // unparseable => skip
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	privateRanges := []struct {
		network string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"fc00::/7"},
	}
	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------
// Simple token-bucket rate limiter
// --------------------------------------------------------------------

type rateLimiter struct {
	tokens   int
	maxTokens int
	period    time.Duration
	mu        sync.Mutex
	lastReset time.Time
}

func newRateLimiter(maxTokens int, period time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		period:    period,
		lastReset: time.Now(),
	}
}

// wait blocks until a token is available.
func (r *rateLimiter) wait() {
	for {
		r.mu.Lock()
		r.refill()
		if r.tokens > 0 {
			r.tokens--
			r.mu.Unlock()
			return
		}
		// Calculate how long until the next refill.
		elapsed := time.Since(r.lastReset)
		waitTime := r.period - elapsed
		if waitTime < 0 {
			waitTime = 0
		}
		r.mu.Unlock()

		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}
}

func (r *rateLimiter) refill() {
	now := time.Now()
	if now.Sub(r.lastReset) >= r.period {
		r.tokens = r.maxTokens
		r.lastReset = now
	}
}
