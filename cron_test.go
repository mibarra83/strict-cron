package cron

import (
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	exprs := []string{
		"*/15 9-17 * * MON-FRI",
		"0 0 1 * *",
		"0 0 * * *",
		"0 0 * 1,6 *",
		"0 0 * JAN-DEC *",
		"0 0 * * SUN,SAT",
	}
	for _, expr := range exprs {
		if _, err := Parse(expr); err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", expr, err)
		}
	}
}

func TestParseScheduleAliases(t *testing.T) {
	cases := map[string]string{
		"@yearly":   "0 0 1 1 *",
		"@annually": "0 0 1 1 *",
		"@monthly":  "0 0 1 * *",
		"@weekly":   "0 0 * * 0",
		"@daily":    "0 0 * * *",
		"@midnight": "0 0 * * *",
		"@hourly":   "0 * * * *",
	}
	for alias, expanded := range cases {
		got, err := Parse(alias)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", alias, err)
			continue
		}
		want, err := Parse(expanded)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", expanded, err)
		}
		if *got != *want {
			t.Errorf("Parse(%q) = %+v, want %+v (expansion of %q)", alias, got, want, expanded)
		}
	}
}

func TestParseScheduleAliasUnrecognized(t *testing.T) {
	cases := []string{"@reboot", "@Daily", "@every_minute", "@"}
	for _, expr := range cases {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) = nil error, want error for unrecognized alias", expr)
		}
	}
}

func TestParseScheduleAliasLenientCase(t *testing.T) {
	got, err := Parse("@Daily", Lenient())
	if err != nil {
		t.Fatalf("Parse(%q, Lenient()) unexpected error: %v", "@Daily", err)
	}
	want := mustParse(t, "@daily")
	if *got != *want {
		t.Errorf("Parse(%q, Lenient()) = %+v, want %+v", "@Daily", got, want)
	}
}

func TestParseFieldCount(t *testing.T) {
	cases := []string{"* * * *", "* * * * * *", "*"}
	for _, expr := range cases {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) = nil error, want error for wrong field count", expr)
		}
	}
}

func TestParseStrictRejections(t *testing.T) {
	cases := []string{
		"0 0 15 * 1",   // both dom and dow restricted
		"0 0 * * 7",    // dow alias for Sunday
		"0 0 1 jan *",  // lower-case month name
		"0 22-2 * * *", // descending range
	}
	for _, expr := range cases {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) in strict mode = nil error, want error", expr)
		}
	}
}

func TestParseLenientAccepts(t *testing.T) {
	cases := []string{
		"0 0 15 * 1",
		"0 0 * * 7",
		"0 0 1 jan *",
		"0 22-2 * * *",
	}
	for _, expr := range cases {
		if _, err := Parse(expr, Lenient()); err != nil {
			t.Errorf("Parse(%q, Lenient()) unexpected error: %v", expr, err)
		}
	}
}

func TestParseDowSevenEquivalence(t *testing.T) {
	strict, err := Parse("0 0 * * 0", Lenient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	viaSeven, err := Parse("0 0 * * 7", Lenient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strict.dow != viaSeven.dow {
		t.Fatalf("dow set for %q = %#x, want %#x (same as dow 0)", "0 0 * * 7", uint64(viaSeven.dow), uint64(strict.dow))
	}
}

func mustParse(t *testing.T, expr string, opts ...Option) *Schedule {
	t.Helper()
	s, err := Parse(expr, opts...)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", expr, err)
	}
	return s
}

func TestNextWeekdayAtFixedHour(t *testing.T) {
	// 2024-01-01 was a Monday.
	s := mustParse(t, "0 9 * * MON-FRI")

	cases := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{
			name: "same day, before the slot",
			from: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			want: time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		},
		{
			name: "exactly at the slot rolls to the next matching day",
			from: time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
			want: time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC),
		},
		{
			name: "Friday rolls over the weekend to Monday",
			from: time.Date(2024, 1, 5, 9, 0, 0, 0, time.UTC),
			want: time.Date(2024, 1, 8, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.Next(c.from)
			if !got.Equal(c.want) {
				t.Errorf("Next(%v) = %v, want %v", c.from, got, c.want)
			}
		})
	}
}

func TestNextDomDowOrLenient(t *testing.T) {
	// "0 0 15 * 1" lenient: midnight on the 15th of any month, OR every
	// Monday. 2024-01-01 and 2024-01-15 are both Mondays.
	s := mustParse(t, "0 0 15 * 1", Lenient())

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	want := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)

	got := s.Next(from)
	if !got.Equal(want) {
		t.Errorf("Next(%v) = %v, want %v", from, got, want)
	}
}

func TestNextMonthRestriction(t *testing.T) {
	s := mustParse(t, "0 0 1 6 *") // midnight, June 1st

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	got := s.Next(from)
	if !got.Equal(want) {
		t.Errorf("Next(%v) = %v, want %v", from, got, want)
	}
}

func TestNextNoMatchReturnsZero(t *testing.T) {
	// February never has a 30th day.
	s := mustParse(t, "0 0 30 2 *")

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := s.Next(from)
	if !got.IsZero() {
		t.Errorf("Next(%v) = %v, want zero Time", from, got)
	}
}
