# strict-cron

A Go library for parsing standard five-field cron expressions
(`minute hour day-of-month month day-of-week`) and computing when they
next run.

## Why

Most cron parsers quietly accept things that mean different things to
different implementations, or that look reasonable but are almost
certainly a typo:

- `7` in the day-of-week field: Sunday, or an out-of-range value? Depends
  on the implementation.
- `0 0 15 * 1` (day-of-month 15 *and* day-of-week Monday): traditional
  cron treats restricted day-of-month and day-of-week fields as an OR,
  which means this runs on the 15th *or* every Monday, not just Mondays
  that fall on the 15th. That surprises almost everyone who writes it.
- `jan` vs `JAN`: some parsers are case-insensitive, some aren't.
- `22-2` as an hour range: does it wrap around midnight, or is it just
  invalid?

This library refuses to guess. By default, `Parse` rejects all of the
above with a specific error. If you actually want the traditional,
looser behavior, opt in with `Lenient()`.

## Usage

```go
package main

import (
    "fmt"
    "time"

    cron "github.com/mibarra83/strict-cron"
)

func main() {
    sched, err := cron.Parse("*/15 9-17 * * MON-FRI")
    if err != nil {
        panic(err)
    }

    next := sched.Next(time.Now())
    fmt.Println("next run:", next)
}
```

Strict mode rejects the ambiguous cases above:

```go
_, err := cron.Parse("0 0 15 * 1")
// err: cron: restricting both day-of-month and day-of-week is ambiguous
// (traditional cron OR's them together); restrict only one, or pass Lenient()

_, err = cron.Parse("0 0 * * 7")
// err: day-of-week: value 7 not allowed in strict mode; use 0 for Sunday
```

Pass `Lenient()` when you're parsing expressions from a system that
relies on that traditional behavior:

```go
sched, err := cron.Parse("0 0 15 * 1", cron.Lenient())
// no error; runs on the 15th of every month, and every Monday
```

## Supported syntax

- `*` — any value
- `5` — a single value
- `1,3,5` — a list
- `1-5` — a range (inclusive)
- `*/15`, `1-10/2` — a step, optionally over a range
- `JAN`-`DEC`, `SUN`-`SAT` — three-letter names for month and
  day-of-week fields (case-sensitive unless `Lenient()` is passed)

## Status

Early skeleton. The parser and `Next()` are functional and covered by a
table-driven test suite (`go test ./...`), but the public API may still
change.

## License

MIT, see [LICENSE](LICENSE).
