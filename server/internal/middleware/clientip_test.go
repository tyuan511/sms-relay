package middleware

import (
	"net"
	"testing"
)

func TestClientIPResolverTrustedProxyUsesForwardedHeader(t *testing.T) {
	resolver, err := NewClientIPResolver("10.0.0.0/8")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	got := resolver.Resolve("10.0.0.5", "198.51.100.7, 10.0.0.1", "")
	if got != "198.51.100.7" {
		t.Fatalf("expected forwarded client IP, got %q", got)
	}
}

func TestClientIPResolverUntrustedPeerIgnoresForwardedHeader(t *testing.T) {
	resolver, err := NewClientIPResolver("10.0.0.0/8")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	got := resolver.Resolve("203.0.113.50", "198.51.100.7", "198.51.100.8")
	if got != "203.0.113.50" {
		t.Fatalf("expected peer IP, got %q", got)
	}
}

func TestClientIPResolverTrustedProxyFallsBackToRealIP(t *testing.T) {
	resolver, err := NewClientIPResolver("172.16.0.0/12")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	got := resolver.Resolve("172.18.0.2", "", "198.51.100.9")
	if got != "198.51.100.9" {
		t.Fatalf("expected X-Real-IP fallback, got %q", got)
	}
}

func TestClientIPResolverDefaultsToPrivateNetworks(t *testing.T) {
	resolver, err := NewClientIPResolver("")
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}

	if !resolver.isTrusted(net.ParseIP("127.0.0.1")) {
		t.Fatal("expected loopback to be trusted by default")
	}
	if resolver.isTrusted(net.ParseIP("203.0.113.1")) {
		t.Fatal("expected public IP to be untrusted by default")
	}
}
