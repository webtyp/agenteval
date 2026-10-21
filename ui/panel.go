package ui

import (
	"webtyp.com/agentworker"
	"webtyp.com/avatar"
	"webtyp.com/components/bubblethread"
	"webtyp.com/components/composebar"
	. "webtyp.com/dom"
	. "webtyp.com/html"
)

// session is what the panel asks of a started agent (*agentworker.Client in the browser).
type session interface {
	Run(sessionID, text string)
	Confirm(sessionID string)
	Decline(sessionID string)
}

// Panel is a chat with one agent running in a Web Worker: a button wakes it, the status shows
// its downloads and readiness, and the person talks to it and confirms what it wants to change.
type Panel struct {
	Element
	State State

	scripts  agentworker.Scripts
	client   session
	examples []string
	thread   *bubblethread.BubbleThread
	compose  *composebar.ComposeBar
	status   *SignalString
	notes    *SignalNodes
	pending  *SignalNodes
	timings  *SignalNodes
	avatar   *avatar.Avatar
	awake    *SignalBool // hides the wake button
	locked   *SignalBool // the compose bar: until ready, and while a reply is on its way
	idle     *SignalBool // hides Confirm/Decline: no call is waiting
}

// New returns the panel of the agent whose Worker scripts are s (sitec builds them from
// web/workers/<name>: "/<name>.worker.js" and "/<name>.simd.worker.js").
func New(s agentworker.Scripts, examples ...string) *Panel {
	p := &Panel{
		scripts:  s,
		examples: examples,
		status:   NewString(WakeHint),
		notes:    NewNodes(),
		pending:  NewNodes(),
		timings:  NewNodes(),
		awake:    NewBool(false),
		locked:   NewBool(true),
		idle:     NewBool(true),
	}
	p.thread = &bubblethread.BubbleThread{}
	p.compose = &composebar.ComposeBar{
		Placeholder: Placeholder, SendLabel: SendLabel, MaxLength: MaxBodyLength,
		OnSend: p.send, Disabled: p.locked,
	}
	return p
}

func (p *Panel) Init(ctx Ctx) {
	p.thread.Init(ctx)
	p.compose.Init(ctx)
}

// WithAvatar shows av beside the chat: thinking while a reply is on its way, speaking the reply.
func (p *Panel) WithAvatar(av *avatar.Avatar) *Panel {
	p.avatar = av
	return p
}

func (p *Panel) Render() *Element {
	var children []Component
	children = append(children, H2().Text(Title))

	if p.avatar != nil {
		children = append(children, Div().Child(p.avatar))
	}

	children = append(children,
		P().BindText(p.status),
		Div().BindChildren(p.notes),
		Button().Attr("type", "button").Text(WakeLabel).BindAttrBool("hidden", p.awake).
			OnClick(func(Event) { p.wake() }),
		p.thread,
	)

	if len(p.examples) > 0 {
		sel := NewElement("select").Child(Option("", ExamplesHint))
		for _, ex := range p.examples {
			sel.Child(Option(ex, ex))
		}
		sel.BindAttrBool("disabled", p.locked).OnChange(func(e Event) {
			if v := e.TargetValue(); v != "" {
				p.send(v)
			}
		})
		children = append(children, H3().Text(ExamplesLabel), sel)
	}

	children = append(children,
		H3().Text(TimingsLabel),
		Ul().BindChildren(p.timings),
		Div().BindAttrBool("hidden", p.idle).Child(
			Div().BindChildren(p.pending),
			Button().Attr("type", "button").Text(ConfirmLabel).OnClick(func(Event) { p.answer(true) }),
			Button().Attr("type", "button").Text(DeclineLabel).OnClick(func(Event) { p.answer(false) }),
		),
		p.compose,
	)

	return Div().Child(children...)
}

func (p *Panel) send(body string) {
	if p.client == nil {
		return
	}
	p.State.Said(body)
	if p.avatar != nil {
		p.avatar.SetEmotion(avatar.Neutral)
		p.avatar.SetActivity(avatar.Think)
	}
	p.update()
	p.client.Run(SessionID, body)
}

func (p *Panel) answer(confirm bool) {
	if p.client == nil {
		return
	}
	p.State.Answered()
	if p.avatar != nil {
		p.avatar.SetActivity(avatar.Think)
	}
	p.update()
	if confirm {
		p.client.Confirm(SessionID)
	} else {
		p.client.Decline(SessionID)
	}
}

// onEvent is the Worker's events, on the page's goroutine.
func (p *Panel) onEvent(e agentworker.Event) {
	if p.avatar != nil {
		if e.Kind == agentworker.EventReply {
			p.avatar.Speak(e.Text)
		} else if e.Kind == agentworker.EventFailed {
			p.avatar.SetEmotion(avatar.Sad)
			p.avatar.Speak(e.Text)
		}
	}
	p.State.Apply(e)
	p.update()
}

// update pushes State into what the panel shows: the only place the view changes.
func (p *Panel) update() {
	s := &p.State
	p.status.Set(s.Status)
	notes := make([]*Element, 0, len(s.Notes))
	for _, n := range s.Notes {
		notes = append(notes, P().Text(n))
	}
	p.notes.Set(notes)
	calls := make([]*Element, 0, len(s.Pending))
	for _, c := range s.Pending {
		calls = append(calls, P().Text(c.Name+" "+c.Input))
	}
	p.pending.Set(calls)

	timings := make([]*Element, 0, len(s.Timings))
	for _, t := range s.Timings {
		timings = append(timings, Li().Text(t))
	}
	p.timings.Set(timings)

	p.awake.Set(s.Phase != PhaseSleeping)
	p.locked.Set(s.Phase != PhaseReady || s.Busy)
	p.idle.Set(len(s.Pending) == 0 || s.Busy)
	p.thread.SetBubbles(s.Bubbles)
}
