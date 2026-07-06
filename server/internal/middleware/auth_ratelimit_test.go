package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func testClientIPResolver(t *testing.T) *ClientIPResolver {
	t.Helper()
	resolver, err := NewClientIPResolver("0.0.0.0/0,::/0")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}
	return resolver
}

func TestAuthRateLimiterFailureWindowCap(t *testing.T) {
	limiter := NewAuthRateLimiter(testClientIPResolver(t))
	ip := "203.0.113.10"

	for i := 0; i < authRateLimitMaxFailures; i++ {
		limiter.recordFailure(ip)
	}
	if _, blocked := limiter.check(ip); !blocked {
		t.Fatal("expected block after max failures in window")
	}
}

func TestAuthRateLimiterDoesNotCountSuccessfulAuth(t *testing.T) {
	limiter := NewAuthRateLimiter(testClientIPResolver(t))
	app := fiber.New()
	app.Post("/login", limiter.AuthRateLimit(), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	ip := "203.0.113.14"
	for i := 0; i < authRateLimitMaxFailures+5; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
}

func TestAuthRateLimiterExponentialBackoffOnFailures(t *testing.T) {
	limiter := NewAuthRateLimiter(testClientIPResolver(t))
	app := fiber.New()
	app.Post("/login", limiter.AuthRateLimit(), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusUnauthorized)
	})

	ip := "203.0.113.11"
	for i := 0; i < authRateLimitBackoffAfter; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failure request %d failed: %v", i+1, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("failure request %d: expected 401, got %d", i+1, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("backoff request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected backoff 429 after %d failures, got %d", authRateLimitBackoffAfter, resp.StatusCode)
	}
}

func TestAuthRateLimiterResetsAfterSuccess(t *testing.T) {
	limiter := NewAuthRateLimiter(testClientIPResolver(t))
	ip := "203.0.113.12"

	for i := 0; i < authRateLimitBackoffAfter-1; i++ {
		limiter.recordFailure(ip)
	}
	limiter.recordSuccess(ip)

	if retry, blocked := limiter.check(ip); blocked {
		t.Fatalf("expected no block after success reset, retry=%d", retry)
	}

	for i := 0; i < authRateLimitBackoffAfter; i++ {
		limiter.recordFailure(ip)
	}
	if _, blocked := limiter.check(ip); !blocked {
		t.Fatal("expected backoff after repeated failures post-reset")
	}
}

func TestAuthRateLimiterPrunesOldFailures(t *testing.T) {
	limiter := NewAuthRateLimiter(testClientIPResolver(t))
	now := time.Now()
	entry := limiter.getEntry("203.0.113.13")
	entry.failures = []time.Time{
		now.Add(-2 * authRateLimitWindow),
		now.Add(-30 * time.Second),
	}
	limiter.pruneFailures(entry, now)
	if len(entry.failures) != 1 {
		t.Fatalf("expected 1 failure after prune, got %d", len(entry.failures))
	}
}

func TestAuthRateLimiterIgnoresSpoofedIPWithoutTrustedProxy(t *testing.T) {
	resolver, err := NewClientIPResolver("10.0.0.0/8")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	peer := "203.0.113.50"
	first := resolver.Resolve(peer, "198.51.100.7", "")
	second := resolver.Resolve(peer, "198.51.100.8", "")
	if first != second || first != peer {
		t.Fatalf("expected untrusted peer to ignore spoofed headers, got %q and %q", first, second)
	}
}
