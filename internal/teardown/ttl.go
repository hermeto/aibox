// Package teardown holds the TTL rules for cloud runs: when the automatic teardown fires
// and how far it may be extended.
package teardown

import (
	"errors"
	"fmt"
	"time"
)

// Limits for a run's lifetime.
const (
	MinTTL      = 30 * time.Minute
	MaxLifetime = 24 * time.Hour
)

// ErrTooLong is returned when an expiry would exceed MaxLifetime after the start.
var ErrTooLong = errors.New("run lifetime would exceed 24h after its start")

// Expiry returns start + ttl after validating the TTL.
func Expiry(start time.Time, ttl time.Duration) (time.Time, error) {
	if ttl < MinTTL {
		return time.Time{}, fmt.Errorf("ttl %s is shorter than the minimum %s", ttl, MinTTL)
	}
	if ttl > MaxLifetime {
		return time.Time{}, fmt.Errorf("ttl %s: %w", ttl, ErrTooLong)
	}
	return start.Add(ttl).UTC().Truncate(time.Second), nil
}

// Extend moves the current expiry by d, keeping it within MaxLifetime of start.
func Extend(start, current time.Time, d time.Duration) (time.Time, error) {
	if d <= 0 {
		return time.Time{}, fmt.Errorf("extension must be positive, got %s", d)
	}
	next := current.Add(d).UTC().Truncate(time.Second)
	if next.Sub(start) > MaxLifetime {
		return time.Time{}, fmt.Errorf("extend by %s to %s: %w (latest allowed %s)", d, next.Format(time.RFC3339), ErrTooLong, start.Add(MaxLifetime).UTC().Format(time.RFC3339))
	}
	return next, nil
}

// ScheduleExpression renders an EventBridge Scheduler one-time expression (UTC).
func ScheduleExpression(t time.Time) string {
	return "at(" + t.UTC().Format("2006-01-02T15:04:05") + ")"
}

// Remaining returns the time left before expiry (0 if expired).
func Remaining(expiry, now time.Time) time.Duration {
	if now.After(expiry) {
		return 0
	}
	return expiry.Sub(now).Truncate(time.Second)
}
