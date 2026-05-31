package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// parseTimeParam accepts a bare date (YYYY-MM-DD), full RFC3339, or a naive
// datetime (treated as UTC). isDate reports whether the input was a bare date,
// so callers can expand an end bound to next-midnight.
func parseTimeParam(s string) (t time.Time, isDate bool, err error) {
	if d, e := time.Parse("2006-01-02", s); e == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), true, nil
	}
	if d, e := time.Parse(time.RFC3339, s); e == nil {
		return d.UTC(), false, nil
	}
	if d, e := time.Parse("2006-01-02T15:04:05", s); e == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), time.UTC), false, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid time %q", s)
}

// parseRange reads the required from/to params and returns a half-open [from, to)
// UTC range. A bare-date `to` expands to the next midnight (whole day included).
func parseRange(r *http.Request) (from, to time.Time, err error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("from and to are required")
	}
	from, _, err = parseTimeParam(fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var toIsDate bool
	to, toIsDate, err = parseTimeParam(toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if toIsDate {
		to = to.AddDate(0, 0, 1)
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be after from")
	}
	return from, to, nil
}

// currentMonthUTC returns [first-of-this-month, first-of-next-month) in UTC.
func currentMonthUTC(now time.Time) (from, to time.Time) {
	y, m, _ := now.UTC().Date()
	from = time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	to = from.AddDate(0, 1, 0)
	return from, to
}

// parseID parses a positive int64 path value.
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return id, nil
}
