# AGENTS.md — webtyp/agenteval

Working notes for AI agents operating in this repository. End-user docs: [README.md](README.md).

## Mission

`agenteval` measures how well a `webtyp.com/agent` behaves: a scenario written in Go runs N times
against local models served by `llama-server`, and the result is a pass rate. Deterministic
checks (which tools ran, what the answer contains) plus a judge model for what is not a rule.

## This is a HOST-ONLY tool — the standard library is correct here

Every file is `//go:build !wasm`. It never runs in the browser. It legitimately uses `net/http`,
`encoding/json`, `context`, `fmt`, `strings`, `errors`, `os`, `testing`. **Do NOT replace those
imports with `webtyp.com/*` equivalents** — that rule is for browser code, not for this repo.

The one thing it must never do is re-implement what the agent's own models do: the decision
prompt comes from `qwen.DecidePrompt` (one source; what agenteval measures must be what the
browser reads).

## The build that decides

```bash
go install webtyp.com/devflow/cmd/gotest@latest   # once
gotest            # vet + tests; must be green
```

Tests never need a real `llama-server`: they use `net/http/httptest` servers that answer
`/tokenize`, `/completion`, `/v1/chat/completions` and `/health`.

## Layout

| File | Role |
|---|---|
| `scenario.go` | `Scenario`, `Given`, `Env`, `Builder`, `Run` |
| `check.go` | deterministic checks (`Calls`, `DoesNotModify`, `Contains`, …) |
| `judge.go` | the judge (decider-4b) and the checks that need it |
| `llamaserver.go` | the model under test over llama-server |
| `tokenize.go` | `/tokenize` and the option-letter ids |
| `tests/` | black-box tests (`package tests` / `agenteval_test`) |
