package teardown

import (
	"errors"
	"testing"
	"time"
)

var start = time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)

func TestExpiry(t *testing.T) {
	e, err := Expiry(start, 4*time.Hour)
	if err != nil || !e.Equal(start.Add(4*time.Hour)) {
		t.Fatalf("%v %v", e, err)
	}
	if _, err := Expiry(start, 10*time.Minute); err == nil {
		t.Fatal("too short")
	}
	if _, err := Expiry(start, 25*time.Hour); !errors.Is(err, ErrTooLong) {
		t.Fatalf("too long: %v", err)
	}
}

func TestExtend(t *testing.T) {
	cur := start.Add(4 * time.Hour)
	next, err := Extend(start, cur, 2*time.Hour)
	if err != nil || !next.Equal(start.Add(6*time.Hour)) {
		t.Fatalf("%v %v", next, err)
	}
	if _, err := Extend(start, start.Add(23*time.Hour), 2*time.Hour); !errors.Is(err, ErrTooLong) {
		t.Fatalf("over 24h: %v", err)
	}
	if _, err := Extend(start, cur, 0); err == nil {
		t.Fatal("non-positive extension")
	}
	if got, err := Extend(start, start.Add(20*time.Hour), 4*time.Hour); err != nil || !got.Equal(start.Add(24*time.Hour)) {
		t.Fatalf("exactly 24h must be allowed: %v %v", got, err)
	}
}

func TestScheduleAndRemaining(t *testing.T) {
	if got := ScheduleExpression(time.Date(2026, 11, 5, 5, 0, 0, 0, time.FixedZone("x", 3600))); got != "at(2026-11-05T04:00:00)" {
		t.Fatal(got)
	}
	if Remaining(start, start.Add(time.Hour)) != 0 || Remaining(start.Add(time.Hour), start) != time.Hour {
		t.Fatal("remaining")
	}
}
