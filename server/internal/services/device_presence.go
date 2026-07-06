package services

import "time"

// DeviceOnlineWithin is the maximum age of last_seen_at for a device to count as online.
//
// Android 15+ reports via WorkManager on a ~15 minute cadence (plus OS scheduling jitter).
// Android 14 and below use a foreground-service heartbeat about every 5 minutes.
// Inbound SMS uploads also refresh last_seen_at regardless of platform.
//
// Allow two missed 15-minute heartbeats before marking offline.
const DeviceOnlineWithin = 30 * time.Minute

func IsDeviceOnline(lastSeen *time.Time, now time.Time) bool {
	if lastSeen == nil {
		return false
	}
	return now.Sub(*lastSeen) < DeviceOnlineWithin
}
