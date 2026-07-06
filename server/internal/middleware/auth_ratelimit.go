package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	authRateLimitWindow       = time.Minute
	authRateLimitMaxFailures  = 10
	authRateLimitBackoffAfter = 3
	authRateLimitBaseBackoff  = time.Second
	authRateLimitMaxBackoff   = 15 * time.Minute
)

type authRateEntry struct {
	failures            []time.Time
	consecutiveFailures int
	blockedUntil        time.Time
}

// AuthRateLimiter limits auth endpoint abuse by client IP with exponential backoff
// after failed credential checks.
type AuthRateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*authRateEntry
	clientIP *ClientIPResolver
}

func NewAuthRateLimiter(clientIP *ClientIPResolver) *AuthRateLimiter {
	return &AuthRateLimiter{
		entries:  make(map[string]*authRateEntry),
		clientIP: clientIP,
	}
}

func (l *AuthRateLimiter) AuthRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := l.clientIP.ClientIP(c)
		if retryAfter, blocked := l.check(ip); blocked {
			c.Set("Retry-After", strconv.Itoa(retryAfter))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"detail": "too many requests"})
		}

		err := c.Next()

		switch c.Response().StatusCode() {
		case fiber.StatusUnauthorized:
			l.recordFailure(ip)
		case fiber.StatusOK, fiber.StatusCreated:
			l.recordSuccess(ip)
		}
		return err
	}
}

func (l *AuthRateLimiter) check(ip string) (retryAfterSec int, blocked bool) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[ip]
	if entry == nil {
		return 0, false
	}
	l.pruneFailures(entry, now)
	if entry.blockedUntil.After(now) {
		return int(entry.blockedUntil.Sub(now).Seconds()) + 1, true
	}
	if len(entry.failures) >= authRateLimitMaxFailures {
		entry.blockedUntil = now.Add(authRateLimitWindow)
		return int(authRateLimitWindow.Seconds()), true
	}
	return 0, false
}

func (l *AuthRateLimiter) recordFailure(ip string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.getEntry(ip)
	l.pruneFailures(entry, now)
	entry.failures = append(entry.failures, now)
	entry.consecutiveFailures++
	if entry.consecutiveFailures < authRateLimitBackoffAfter {
		return
	}
	failures := entry.consecutiveFailures - authRateLimitBackoffAfter + 1
	backoff := authRateLimitBaseBackoff << failures
	if backoff > authRateLimitMaxBackoff {
		backoff = authRateLimitMaxBackoff
	}
	until := now.Add(backoff)
	if until.After(entry.blockedUntil) {
		entry.blockedUntil = until
	}
}

func (l *AuthRateLimiter) recordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, ip)
}

func (l *AuthRateLimiter) getEntry(ip string) *authRateEntry {
	entry := l.entries[ip]
	if entry == nil {
		entry = &authRateEntry{}
		l.entries[ip] = entry
	}
	return entry
}

func (l *AuthRateLimiter) pruneFailures(entry *authRateEntry, now time.Time) {
	cutoff := now.Add(-authRateLimitWindow)
	i := 0
	for _, t := range entry.failures {
		if t.After(cutoff) {
			entry.failures[i] = t
			i++
		}
	}
	entry.failures = entry.failures[:i]
}
