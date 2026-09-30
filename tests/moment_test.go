//go:build !wasm

package tests

import (
	"testing"

	"webtyp.com/agenteval"
)

func TestMomentNow(t *testing.T) {
	m := agenteval.Moment{
		Year:             2026,
		Month:            9,
		Day:              29,
		Hour:             10,
		Minute:           0,
		UTCOffsetMinutes: -180,
	}
	expected := int64(1790686800) * 1e9
	if got := m.Now(); got != expected {
		t.Errorf("Moment.Now() = %d, want %d", got, expected)
	}
	if got := m.UTCOffsetMinutes; got != -180 {
		t.Errorf("Moment.UTCOffsetMinutes = %d, want -180", got)
	}
}
