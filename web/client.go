//go:build wasm

// web/client.go — the agent laboratory's page: a chat with the agent of web/workers/agent.
// Run it with the real models: go run ./cmd/lab (README, "Laboratorio").

package main

import (
	. "webtyp.com/dom"

	"webtyp.com/agenteval/lab"
	"webtyp.com/agenteval/ui"
)

func main() {
	p := ui.New(lab.Scripts)
	p.Init(nil)
	Render("app", p)

	// select{} keeps the WASM goroutine alive so JS event callbacks keep working.
	select {}
}
