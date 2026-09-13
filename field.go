package cron

import (
	"fmt"
	"strconv"
	"strings"
)

// fieldSet is a bitmask over the small integer ranges cron fields use
// (0-59 at most, for minutes), so a uint64 covers every field with room
// to spare.
type fieldSet uint64

func (fs fieldSet) has(v int) bool {
	if v < 0 || v > 63 {
		return false
	}
	return fs&(fieldSet(1)<<uint(v)) != 0
}

type fieldSpec struct {
	name       string
	min, max   int
	names      map[string]int
	aliasSeven bool // day-of-week only: strict rejects 7, lenient maps it to 0 (Sunday)
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

var (
	minuteSpec = fieldSpec{name: "minute", min: 0, max: 59}
	hourSpec   = fieldSpec{name: "hour", min: 0, max: 23}
	domSpec    = fieldSpec{name: "day-of-month", min: 1, max: 31}
	monthSpec  = fieldSpec{name: "month", min: 1, max: 12, names: monthNames}
	dowSpec    = fieldSpec{name: "day-of-week", min: 0, max: 6, names: dowNames, aliasSeven: true}
)

// resolve turns a single token (a number or a name) into an in-range int.
// Name lookups are case-sensitive in strict mode, since silently accepting
// "jan" alongside "JAN" is exactly the kind of leniency this library opts
// callers out of by default.
func (spec fieldSpec) resolve(token string, lenient bool) (int, error) {
	if token == "" {
		return 0, fmt.Errorf("%s: empty value", spec.name)
	}

	var v int
	if spec.names != nil {
		key := token
		if lenient {
			key = strings.ToUpper(token)
		}
		if n, ok := spec.names[key]; ok {
			v = n
		} else if n, err := strconv.Atoi(token); err == nil {
			v = n
		} else {
			return 0, fmt.Errorf("%s: unrecognized value %q", spec.name, token)
		}
	} else {
		n, err := strconv.Atoi(token)
		if err != nil {
			return 0, fmt.Errorf("%s: invalid integer %q", spec.name, token)
		}
		v = n
	}

	if spec.aliasSeven && v == 7 {
		if !lenient {
			return 0, fmt.Errorf("%s: value 7 not allowed in strict mode; use 0 for Sunday", spec.name)
		}
		v = 0
	}

	if v < spec.min || v > spec.max {
		return 0, fmt.Errorf("%s: value %d out of range [%d-%d]", spec.name, v, spec.min, spec.max)
	}
	return v, nil
}

// parseField parses one comma-separated cron field ("1,5-10/2,*") into the
// set of values it matches.
func parseField(spec fieldSpec, raw string, lenient bool) (fieldSet, error) {
	var fs fieldSet
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			return 0, fmt.Errorf("%s: empty item in %q", spec.name, raw)
		}
		bits, err := parseRangeOrStep(spec, part, lenient)
		if err != nil {
			return 0, err
		}
		fs |= bits
	}
	return fs, nil
}

func cutStep(part string) (base, step string, hasStep bool) {
	if idx := strings.IndexByte(part, '/'); idx >= 0 {
		return part[:idx], part[idx+1:], true
	}
	return part, "", false
}

func parseRangeOrStep(spec fieldSpec, part string, lenient bool) (fieldSet, error) {
	base, stepStr, hasStep := cutStep(part)
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepStr)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("%s: invalid step %q", spec.name, stepStr)
		}
		step = n
	}

	var lo, hi int
	switch {
	case base == "*":
		lo, hi = spec.min, spec.max
	case strings.Contains(base, "-"):
		bounds := strings.SplitN(base, "-", 2)
		var err error
		if lo, err = spec.resolve(bounds[0], lenient); err != nil {
			return 0, err
		}
		if hi, err = spec.resolve(bounds[1], lenient); err != nil {
			return 0, err
		}
	default:
		v, err := spec.resolve(base, lenient)
		if err != nil {
			return 0, err
		}
		lo = v
		if hasStep {
			hi = spec.max // "5/2" means 5, 7, 9, ... through the field's max
		} else {
			hi = v
		}
	}

	if lo > hi {
		if !lenient {
			return 0, fmt.Errorf("%s: descending range %q not allowed in strict mode", spec.name, base)
		}
		return wrappedRange(spec, lo, hi, step), nil
	}

	var fs fieldSet
	for v := lo; v <= hi; v += step {
		fs |= fieldSet(1) << uint(v)
	}
	return fs, nil
}

// wrappedRange handles lenient descending ranges like "22-2", which mean
// "22, 23, then wrap to the field minimum and count up to 2".
func wrappedRange(spec fieldSpec, lo, hi, step int) fieldSet {
	var fs fieldSet
	for v := lo; v <= spec.max; v += step {
		fs |= fieldSet(1) << uint(v)
	}

	consumed := (spec.max-lo)%step + 1
	remaining := step - consumed
	if remaining == step {
		remaining = 0
	}
	for v := spec.min + remaining; v <= hi; v += step {
		fs |= fieldSet(1) << uint(v)
	}
	return fs
}
