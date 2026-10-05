package ui

import (
	"webtyp.com/agentworker"
	"webtyp.com/components/bubblethread"
	"webtyp.com/fmt"
)

// Phase is where the agent is, from the page's point of view.
type Phase uint8

const (
	PhaseSleeping Phase = iota // before the click that wakes it
	PhaseWaking                // started: downloading or loading
	PhaseReady
	PhaseAsleep // the device cannot run the agent
)

// State is what the panel shows. Every change comes from the person (Wake, Said) or from the
// Worker (Apply), so it is tested without a browser.
type State struct {
	Phase   Phase
	Status  string   // the line under the title
	Notes   []string // warnings when ready; the shortfalls when asleep
	Bubbles []bubblethread.Bubble
	Pending []agentworker.Pending // calls waiting for Confirm or Decline
	Busy    bool                  // a request was sent and its reply has not arrived
	next    int
}

// Wake marks the agent as starting.
func (s *State) Wake() {
	s.Phase, s.Status, s.Notes = PhaseWaking, Loading, nil
}

// Said records a message from the person.
func (s *State) Said(text string) {
	s.add(Me, text, true)
	s.Pending, s.Busy = nil, true
}

// Answered records that the person confirmed or declined the pending calls.
func (s *State) Answered() {
	s.Pending, s.Busy = nil, true
}

// Apply records what the Worker said.
func (s *State) Apply(e agentworker.Event) {
	switch e.Kind {
	case agentworker.EventProgress:
		var pct int64
		if e.Total > 0 {
			pct = e.Done * 100 / e.Total
		}
		s.Status = fmt.Sprintf(Downloading, e.Artifact, pct)
	case agentworker.EventReady:
		s.Phase, s.Status, s.Notes = PhaseReady, ReadyNote, nil
		if e.WriterOff {
			s.Notes = append(s.Notes, WriterOffNote)
		}
		if !e.Persisted {
			s.Notes = append(s.Notes, NotPersisted)
		}
	case agentworker.EventAsleep:
		s.Phase, s.Status, s.Notes = PhaseAsleep, AsleepTitle, nil
		for _, name := range e.Shortfalls {
			s.Notes = append(s.Notes, ShortfallText(name))
		}
	case agentworker.EventReply:
		s.add(Agent, e.Text, false)
		s.Pending, s.Busy = e.Pending, false
	case agentworker.EventFailed:
		s.add(Agent, FailedPrefix+e.Text, false)
		s.Busy = false
		if s.Phase == PhaseWaking {
			s.Phase = PhaseSleeping // the start failed: offer the button again
		}
	}
}

func (s *State) add(author, body string, mine bool) {
	s.next++
	s.Bubbles = append(s.Bubbles, bubblethread.Bubble{
		ID: fmt.Sprintf("b%d", s.next), Author: author, Body: body, Mine: mine,
	})
}
