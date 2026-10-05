// Package lab is the agent this repository's laboratory runs (web/client.go, web/workers/agent):
// its model files and its Worker scripts. The agent itself is lab/worker.
package lab

import "webtyp.com/agentworker"

const (
	Dir       = "agent"      // the OPFS directory of the lab's models (D-PWA-10)
	ModelsDir = "models"     // where the developer puts the model files (README, "Laboratorio")
	OutDir    = "web/public" // the framework's output; the release build replaces it
	Port      = "8443"

	DeciderWeights = "decider-0.8b"
	DeciderMerges  = "decider-0.8b-merges"
	WriterWeights  = "lfm2.5-350m"
	WriterMerges   = "lfm2.5-350m-merges"
	ModelsVersion  = "int8-2026-10" // a new version is a new download

	// DecideTemperature is decider-0.8b's decision temperature (the same as agenteval's).
	DecideTemperature = 1.03
)

// Scripts are the Worker scripts sitec builds from web/workers/agent.
var Scripts = agentworker.Scripts{Plain: "/agent.worker.js", SIMD: "/agent.simd.worker.js"}
