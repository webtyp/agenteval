//go:build !wasm

package agenteval

import (
	"time"
)

// Moment is a local date and time.
type Moment struct {
	Year, Month, Day, Hour, Minute int
	UTCOffsetMinutes               int // e.g. -180 for UTC-3
}

// Now returns the unix nanoseconds of that local moment in UTC.
func (m Moment) Now() int64 {
	loc := time.FixedZone("", m.UTCOffsetMinutes*60)
	t := time.Date(m.Year, time.Month(m.Month), m.Day, m.Hour, m.Minute, 0, 0, loc)
	return t.UnixNano()
}

type momentClock struct {
	m Moment
}

func (mc momentClock) Now() int64 {
	return mc.m.Now()
}

func (mc momentClock) UTCOffsetMinutes() int {
	return mc.m.UTCOffsetMinutes
}

func (m Moment) clock() momentClock {
	return momentClock{m: m}
}
