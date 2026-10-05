//go:build wasm

// The agent laboratory's Web Worker: the lab's agent with the real models (lab/worker).
package main

import (
	"webtyp.com/agentworker"
	"webtyp.com/unixid"

	"webtyp.com/agenteval/lab/worker"
)

func main() {
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err)
	}
	agentworker.Serve(worker.Setup(ids))
}
