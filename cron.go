// Package cron parses standard five-field cron expressions and computes
// their next run time, refusing by default to guess at the ambiguous or
// non-portable constructs different cron implementations disagree on.
package cron

import (
	"fmt"
	"strings"
	"time"
)

// Schedule is a parsed cron expression: a set of allowed minutes, hours,
// days of month, months, and days of week.
type Schedule struct {
	minute, hour, dom, month, dow fieldSet
	domRestricted, dowRestricted  bool
}

type config struct {
	lenient bool
}

// Option configures how Parse interprets a cron expression.
type Option func(*config)

// Lenient disables the strict checks Parse otherwise applies: it accepts
// day-of-week 7 as an alias for Sunday, lower-case month/day names,
// descending (wrap-around) ranges such as "22-2", and expressions that
// restrict both day-of-month and day-of-week at once (combined with OR,
// per traditional cron behavior, which is easy to misread as AND).
func Lenient() Option {
	return func(c *config) { c.lenient = true }
}

// Parse parses a standard five-field cron expression: minute hour
// day-of-month month day-of-week. Fields accept "*", single values,
// comma-separated lists, ranges ("a-b"), and steps ("a-b/c" or "*/c").
// Months and days of week also accept three-letter names (JAN, SUN, ...).
//
// By default Parse is strict: it rejects anything a portable cron reader
// could interpret two different ways. Pass Lenient() to relax that.
func Parse(expr string, opts ...Option) (*Schedule, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron: expected 5 fields (minute hour dom month dow), got %d in %q", len(fields), expr)
	}

	minute, err := parseField(minuteSpec, fields[0], cfg.lenient)
	if err != nil {
		return nil, err
	}
	hour, err := parseField(hourSpec, fields[1], cfg.lenient)
	if err != nil {
		return nil, err
	}
	dom, err := parseField(domSpec, fields[2], cfg.lenient)
	if err != nil {
		return nil, err
	}
	month, err := parseField(monthSpec, fields[3], cfg.lenient)
	if err != nil {
		return nil, err
	}
	dow, err := parseField(dowSpec, fields[4], cfg.lenient)
	if err != nil {
		return nil, err
	}

	domRestricted := fields[2] != "*"
	dowRestricted := fields[4] != "*"
	if domRestricted && dowRestricted && !cfg.lenient {
		return nil, fmt.Errorf("cron: restricting both day-of-month and day-of-week is ambiguous (traditional cron OR's them together); restrict only one, or pass Lenient()")
	}

	return &Schedule{
		minute:        minute,
		hour:          hour,
		dom:           dom,
		month:         month,
		dow:           dow,
		domRestricted: domRestricted,
		dowRestricted: dowRestricted,
	}, nil
}

// maxSearch bounds how far into the future Next will look before giving up.
// Five years covers every legitimate schedule; anything further out means
// the expression can never match (e.g. day-of-month 31 in a month field
// restricted to February).
const maxSearch = 5 * 366 * 24 * time.Hour

// Next returns the first time strictly after from that the schedule
// matches, at minute resolution. It returns the zero Time if no match
// occurs within the next five years.
func (s *Schedule) Next(from time.Time) time.Time {
	loc := from.Location()
	t := from.Truncate(time.Minute).Add(time.Minute)
	deadline := from.Add(maxSearch)

	for t.Before(deadline) {
		if !s.month.has(int(t.Month())) {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)
			continue
		}
		if !s.matchesDay(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
			continue
		}
		if !s.hour.has(t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Add(time.Hour)
			continue
		}
		if !s.minute.has(t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// matchesDay applies the day-of-month / day-of-week combination rule:
// if only one is restricted, it alone decides; if both are restricted,
// either one matching is enough (the traditional, if surprising, cron
// behavior that Parse warns about in strict mode).
func (s *Schedule) matchesDay(t time.Time) bool {
	domMatch := s.dom.has(t.Day())
	dowMatch := s.dow.has(int(t.Weekday()))

	switch {
	case s.domRestricted && s.dowRestricted:
		return domMatch || dowMatch
	case s.domRestricted:
		return domMatch
	case s.dowRestricted:
		return dowMatch
	default:
		return true
	}
}
