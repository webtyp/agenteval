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
