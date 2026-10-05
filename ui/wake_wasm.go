//go:build wasm

package ui

import "webtyp.com/agentworker"

// wake starts the agent's Worker from the click (a user gesture: agentworker asks the browser to
// keep the downloads first, D-PWA-9).
func (p *Panel) wake() {
	p.State.Wake()
	p.update()
	go func() {
		c, err := agentworker.Start(p.scripts, p.onEvent)
		if err != nil {
			p.onEvent(agentworker.Event{Kind: agentworker.EventFailed, Text: err.Error()})
			return
		}
		p.client = c
	}()
}
