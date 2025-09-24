// utils/timeutil.go
package utils

import "time"

// Vietnam time location (ICT, +07:00)
var vnLoc = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*3600)
}()

// Use explicit "seconds" variant for DB storage (recommended)
func NowUnixSeconds() int64 { return time.Now().Unix() }

// Optional other variants if you truly need them elsewhere
func NowUnixMillis() int64 { return time.Now().UnixMilli() }
func NowUnixMicros() int64 { return time.Now().UnixMicro() }

// Convert an epoch value in **seconds** to VN time.
// Returns zero time if t<=0 to let callers decide how to render.
func FromUnixSecondsVN(t int64) time.Time {
	if t <= 0 {
		return time.Time{}
	}
	return time.Unix(t, 0).In(vnLoc)
}

// If you’re unsure of units (secs/ms/us/ns), use this detector.
// Prefer NOT to rely on this—store seconds consistently instead.
func FromUnixAutoVN(x int64) time.Time {
	if x <= 0 {
		return time.Time{}
	}
	// Heuristic thresholds (approx around current epoch 2025-…)
	switch {
	case x < 1e11: // < ~Sat Mar 3 5138 in seconds; treat as seconds
		return time.Unix(x, 0).In(vnLoc)
	case x < 1e14: // milliseconds since epoch
		sec := x / 1e3
		nsec := (x % 1e3) * 1e6
		return time.Unix(sec, nsec).In(vnLoc)
	case x < 1e17: // microseconds
		sec := x / 1e6
		nsec := (x % 1e6) * 1e3
		return time.Unix(sec, nsec).In(vnLoc)
	default: // nanoseconds
		sec := x / 1e9
		nsec := x % 1e9
		return time.Unix(sec, nsec).In(vnLoc)
	}
}

// Format helpers
func FormatRFC3339VN(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(vnLoc).Format(time.RFC3339) // e.g. 2025-09-24T15:12:00+07:00
}

func FormatDisplayVN(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Customize as you like:
	return t.In(vnLoc).Format("2006-01-02 15:04:05 -0700 MST")
}
