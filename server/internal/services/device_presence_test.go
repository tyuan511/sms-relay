package services

import (
	"testing"
	"time"
)

func TestIsDeviceOnline(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	t.Run("nil last seen", func(t *testing.T) {
		if IsDeviceOnline(nil, now) {
			t.Fatal("expected offline when last_seen is nil")
		}
	})

	t.Run("within threshold", func(t *testing.T) {
		lastSeen := now.Add(-29 * time.Minute)
		if !IsDeviceOnline(&lastSeen, now) {
			t.Fatal("expected online within 30 minutes")
		}
	})

	t.Run("at threshold", func(t *testing.T) {
		lastSeen := now.Add(-30 * time.Minute)
		if IsDeviceOnline(&lastSeen, now) {
			t.Fatal("expected offline at exactly 30 minutes")
		}
	})

	t.Run("beyond threshold", func(t *testing.T) {
		lastSeen := now.Add(-31 * time.Minute)
		if IsDeviceOnline(&lastSeen, now) {
			t.Fatal("expected offline after 30 minutes")
		}
	})
}
