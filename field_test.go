package cron

import "testing"

func bits(vals ...int) fieldSet {
	var fs fieldSet
	for _, v := range vals {
		fs |= fieldSet(1) << uint(v)
	}
	return fs
}

func TestFieldSpecResolve(t *testing.T) {
	cases := []struct {
		name    string
		spec    fieldSpec
		token   string
		lenient bool
		want    int
		wantErr bool
	}{
		{"numeric in range", minuteSpec, "45", false, 45, false},
		{"numeric out of range", minuteSpec, "60", false, 0, true},
		{"non-numeric, no names table", minuteSpec, "abc", false, 0, true},
		{"empty token", minuteSpec, "", false, 0, true},
		{"exact-case name", monthSpec, "JAN", false, 1, false},
		{"wrong-case name, strict", monthSpec, "jan", false, 0, true},
		{"wrong-case name, lenient", monthSpec, "jan", true, 1, false},
		{"unrecognized name", monthSpec, "XYZ", false, 0, true},
		{"numeric fallback alongside names", monthSpec, "12", false, 12, false},
		{"dow 7 strict", dowSpec, "7", false, 0, true},
		{"dow 7 lenient maps to Sunday", dowSpec, "7", true, 0, false},
		{"dow 8 lenient still out of range", dowSpec, "8", true, 0, true},
		{"dow name", dowSpec, "SUN", false, 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.spec.resolve(c.token, c.lenient)
			if c.wantErr {
				if err == nil {
					t.Fatalf("resolve(%q, %v) = %d, want error", c.token, c.lenient, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve(%q, %v) unexpected error: %v", c.token, c.lenient, err)
			}
			if got != c.want {
				t.Fatalf("resolve(%q, %v) = %d, want %d", c.token, c.lenient, got, c.want)
			}
		})
	}
}

func TestParseField(t *testing.T) {
	cases := []struct {
		name    string
		spec    fieldSpec
		raw     string
		lenient bool
		want    fieldSet
		wantErr bool
	}{
		{"star", minuteSpec, "*", false, bits(rangeInts(0, 59)...), false},
		{"single value", minuteSpec, "30", false, bits(30), false},
		{"list", minuteSpec, "0,15,30,45", false, bits(0, 15, 30, 45), false},
		{"range", hourSpec, "9-17", false, bits(rangeInts(9, 17)...), false},
		{"star step", minuteSpec, "*/15", false, bits(0, 15, 30, 45), false},
		{"range step", minuteSpec, "0-10/5", false, bits(0, 5, 10), false},
		{"value with step to field max", minuteSpec, "50/5", false, bits(50, 55), false},
		{"empty item in list", minuteSpec, "5,,10", false, 0, true},
		{"empty field", minuteSpec, "", false, 0, true},
		{"invalid step", minuteSpec, "*/0", false, 0, true},
		{"negative step", minuteSpec, "*/-1", false, 0, true},
		{"descending range, strict", hourSpec, "22-2", false, 0, true},
		{"descending range, lenient", hourSpec, "22-2", true, bits(22, 23, 0, 1, 2), false},
		{"descending range with step, lenient", hourSpec, "22-2/2", true, bits(22, 0, 2), false},
		{"month names range", monthSpec, "JAN-MAR", false, bits(1, 2, 3), false},
		{"dow names list", dowSpec, "MON,WED,FRI", false, bits(1, 3, 5), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseField(c.spec, c.raw, c.lenient)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseField(%q, %v) = %#x, want error", c.raw, c.lenient, uint64(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("parseField(%q, %v) unexpected error: %v", c.raw, c.lenient, err)
			}
			if got != c.want {
				t.Fatalf("parseField(%q, %v) = %#x, want %#x", c.raw, c.lenient, uint64(got), uint64(c.want))
			}
		})
	}
}

// rangeInts returns the inclusive integer sequence [lo, hi], used to build
// expected bitmasks for "*" and plain range test cases without repeating
// the full list by hand.
func rangeInts(lo, hi int) []int {
	out := make([]int, 0, hi-lo+1)
	for v := lo; v <= hi; v++ {
		out = append(out, v)
	}
	return out
}
