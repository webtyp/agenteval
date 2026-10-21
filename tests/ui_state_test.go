//go:build !wasm

package tests

import (
	"strings"
	"testing"

	"webtyp.com/agentworker"
	"webtyp.com/agenteval/ui"
)

func TestState_ProgressThenReady(t *testing.T) {
	var s ui.State
	s.Wake()
	s.Apply(agentworker.Event{Kind: agentworker.EventProgress, Artifact: "decider-0.8b", Done: 50, Total: 100})
	if !strings.Contains(s.Status, "50 %") {
		t.Errorf("Status = %q, want 50 %%", s.Status)
	}
	s.Apply(agentworker.Event{Kind: agentworker.EventReady, WriterOff: true})
	if s.Phase != ui.PhaseReady || len(s.Notes) != 2 || s.Notes[0] != ui.WriterOffNote || s.Notes[1] != ui.NotPersisted {
		t.Errorf("ready state = %+v", s)
	}
}

func TestState_Asleep(t *testing.T) {
	var s ui.State
	s.Wake()
	s.Apply(agentworker.Event{Kind: agentworker.EventAsleep, Shortfalls: []string{"secure", "other"}})
	if s.Phase != ui.PhaseAsleep || s.Notes[0] != ui.ShortfallText("secure") || s.Notes[1] != "other" {
		t.Errorf("asleep state = %+v", s)
	}
}

func TestState_ReplyAndPending(t *testing.T) {
	var s ui.State
	s.Said("cambia el horario")
	if !s.Busy || len(s.Bubbles) != 1 || !s.Bubbles[0].Mine {
		t.Fatalf("after Said: %+v", s)
	}
	s.Apply(agentworker.Event{Kind: agentworker.EventReply, Text: "¿Confirmas?", Pending: []agentworker.Pending{{ID: "1", Name: "clinic.change_business_hours"}}})
	if s.Busy || len(s.Pending) != 1 || s.Bubbles[1].Mine || s.Bubbles[1].ID == s.Bubbles[0].ID {
		t.Fatalf("after reply: %+v", s)
	}
	s.Answered()
	if len(s.Pending) != 0 || !s.Busy {
		t.Errorf("after Answered: %+v", s)
	}
}

func TestState_FailedStartSleepsAgain(t *testing.T) {
	var s ui.State
	s.Wake()
	s.Apply(agentworker.Event{Kind: agentworker.EventFailed, Text: "no worker"})
	if s.Phase != ui.PhaseSleeping || s.Bubbles[0].Body != ui.FailedPrefix+"no worker" {
		t.Errorf("failed start: %+v", s)
	}
}

func TestState_Timings(t *testing.T) {
	var s1 ui.State
	s1.Apply(agentworker.Event{Kind: agentworker.EventReply, Text: "ok", DecideMs: []int64{5120, 6010}, TurnMs: 13400})
	if len(s1.Timings) != 1 || s1.Timings[0] != "Turno 1: 13400 ms · decisiones (2): 5120, 6010 ms · herramientas y redactor: 2270 ms" {
		t.Errorf("worked example timings: %q", s1.Timings)
	}

	var s2 ui.State
	s2.Apply(agentworker.Event{Kind: agentworker.EventReply, TurnMs: 900})
	if len(s2.Timings) != 1 || s2.Timings[0] != "Turno 1: 900 ms · decisiones (0): — ms · herramientas y redactor: 900 ms" {
		t.Errorf("0 decisions timings: %q", s2.Timings)
	}

	var s3 ui.State
	s3.Apply(agentworker.Event{Kind: agentworker.EventReply, TurnMs: 100})
	s3.Apply(agentworker.Event{Kind: agentworker.EventReply, TurnMs: 200})
	if len(s3.Timings) != 2 || !strings.Contains(s3.Timings[0], "Turno 1") || !strings.Contains(s3.Timings[1], "Turno 2") {
		t.Errorf("two replies timings: %q", s3.Timings)
	}

	var s4 ui.State
	s4.Apply(agentworker.Event{Kind: agentworker.EventFailed, Text: "failed"})
	if len(s4.Timings) != 0 {
		t.Errorf("EventFailed added timing: %q", s4.Timings)
	}
}
