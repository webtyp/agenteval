---
PLAN: "feat!: hybrid Env — Decider (decider-0.8b with qwen's prompt) and Writer (LFM2.5-350M) over llama-server"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `agenteval` v0.3.0: scenarios for the hybrid agent

Master plan: [AGENT_ECOSYSTEM_MASTER_PLAN.md](https://github.com/webtyp/agent/blob/main/docs/AGENT_ECOSYSTEM_MASTER_PLAN.md)
(D24 hybrid agent, D2 llama.cpp is the reference). **Read [AGENTS.md](../AGENTS.md) first: this
repo is host-only and uses the standard library on purpose.**

## Why

`webtyp.com/agent` v1 (published, now v1.1.0) is hybrid: `agent.Config` has no `LLMs`, `Critic`,
`Budget` or `Identity` any more. It takes a **`Decider llm.Decider`** (decider-0.8b answers closed
questions by the probability of each option letter), an optional **`Writer llm.Client`**
(LFM2.5-350M phrases answers from data), `Tokens`, `Texts`, `Templates`, `Guard`. agenteval still
builds the v0.10 config, so no scenario compiles against agent v1.

Two facts the new code must respect:

1. **The decision prompt is `qwen.DecidePrompt(q) []string`** (`webtyp.com/qwen` v0.4.7). It returns
   pieces; the browser tokenizes **each piece on its own** (no special tokens) and joins the ids.
   No chat template, no system prompt. The model's next token after the last piece (which ends in
   `"("`) is read at the option letters `A`, `B`, … The current `modelServer.Decide`
   (`formatCriticContent` + `/apply-template` + `"Answer with the letter of one option."`) is a
   **different prompt** and is deleted.
2. **Decision temperature 1.03** for decider-0.8b (`qwen.Config.DecideTemperature`): the option
   probabilities are `softmax(logprob_i / 1.03)` over the letters.

## Design gate

1. **Prior art.** promptfoo / OpenAI Evals run a fixed prompt against a provider N times and
   grade; lm-evaluation-harness scores multiple choice by the log-likelihood of each choice — what
   `Decide` does with the letter logprobs. We keep agenteval's `Scenario` and change only what the
   scenario hands the agent.
2. **Novice-name test.** `Env.Decider`, `Env.Writer`: the two models of the hybrid agent, by the
   names `agent.Config` uses.
3. **Complexity ledger.** −3 `Env` fields (`Model`, `Critic`, `Budget`), +2 (`Decider`, `Writer`);
   one more server to start (the writer). Ways to ask the decider: 1 (qwen's prompt).
4. **Where it belongs.** Here: agenteval owns how scenarios reach llama-server.
5. **What it deletes.** `Env.Model`, `Env.Critic`, `Env.Budget`, `modelServer.budget`,
   `formatCriticContent`, `modelServer.Decide`'s chat-template path, `llamaApplyTemplatePath`,
   `llamaPropsPath` and their request/response types, the `agentcontext` dependency.

## Stage 1 — `go.mod`

`go get webtyp.com/agent@v1.1.0 webtyp.com/qwen@v0.4.7 webtyp.com/llm@latest` then `go mod tidy`.
`webtyp.com/agentcontext` must disappear from `go.mod`.

## Stage 2 — `scenario.go`: the new `Env`

```go
// Env is everything a scenario provides to the agent. A Builder passes each field to
// agent.Config unchanged and adds only the application's Texts, Templates and Guard.
type Env struct {
	Decider   llm.Decider      // decider-0.8b on llama-server (AGENTEVAL_DECIDER_URL)
	Writer    llm.Client       // LFM2.5-350M on llama-server (AGENTEVAL_WRITER_URL)
	Tokens    llm.TokenCounter // the decider's tokenizer
	Clock     agent.Clock
	Memory    agent.MemoryStore // fresh for every attempt
	IDGen     model.IDGenerator
	ToolIndex agent.ToolIndex
	Tools     []agent.Tool // the scenario's FakeTools; pass them as Config.LocalTools
}

// Config returns an agent.Config with the environment's pieces. A Builder adds the
// application's words:
//
//	cfg := env.Config()
//	cfg.Texts, cfg.Templates, cfg.Guard = ...
//	return agent.New(cfg)
func (e Env) Config() agent.Config {
	return agent.Config{
		Decider: e.Decider, Writer: e.Writer, Tokens: e.Tokens, Clock: e.Clock,
		Memory: e.Memory, IDGen: e.IDGen, ToolIndex: e.ToolIndex, LocalTools: e.Tools,
	}
}
```

In `Scenario.Run`: replace `newModelServer` + `budget()` with
`dec := newDeciderServer("")` and `wr := newWriterServer("")`; check each with its `/health`
(`GET <url>/health` → 200) **before** the attempts and `t.Fatalf` with its error constant when it
does not answer. Per attempt: `Decider: dec.forAttempt(i)`, `Writer: wr.forAttempt(i)`,
`Tokens: dec`. Everything else in `Run` stays.

## Stage 3 — `llamaserver.go`: two servers

Constants (replace `defaultModelURL`, `envModelURL`, `errModelServerUnreachable`):

```go
const (
	defaultDeciderURL = "http://127.0.0.1:8080"
	envDeciderURL     = "AGENTEVAL_DECIDER_URL"
	defaultWriterURL  = "http://127.0.0.1:8081"
	envWriterURL      = "AGENTEVAL_WRITER_URL"
	// DecideTemperature is decider-0.8b's decision temperature, the one the browser uses.
	DecideTemperature = 1.03
	errDeciderUnreachable = "agenteval: no decision model at %s; start it with: llama-server -m ~/Dev/LMmodels/mradermacher/decider-0.8b-GGUF/decider-0.8b.Q8_0.gguf --port 8080 -np 1 -c 4096 (or set AGENTEVAL_DECIDER_URL)"
	errWriterUnreachable  = "agenteval: no writer at %s; start it with: llama-server -m ~/Dev/LMmodels/LiquidAI/LFM2.5-350M-GGUF/LFM2.5-350M-Q8_0.gguf --port 8081 --jinja -np 1 -c 4096 --temp 0.3 (or set AGENTEVAL_WRITER_URL)"
)
```

**Decider** — `type deciderServer struct { url string; client *http.Client; seed int; tok *llamaTokenizer }`,
`newDeciderServer(override string)` (override → env var → default), `forAttempt(i)`,
`CountTokens(text string) int` (the existing `/tokenize` call, moved here), and:

```go
func (d *deciderServer) Decide(ctx *context.Context, q llm.Question) (llm.Decision, error)
```

1. `len(q.Options)` must be 2..10, else error `agenteval: a decision takes 2 to 10 options, got %d`
   (the same range qwen enforces).
2. `ids`: for each piece of `qwen.DecidePrompt(q)`, `d.tok.tokenize(piece)` (it already sends
   `add_special: false`) and append the ids, in order.
3. `POST /completion` with `{"prompt": ids, "n_predict": 1, "n_probs": 100, "temperature": 0,
   "cache_prompt": false}` — `prompt` is the **array of token ids** (llama-server accepts it; see
   `judge.go`, which already does this). Reuse `judgeCompletionReq` (its `Prompt` is `[]int`;
   leave `PostSamplingProbs` false) and `judgeCompletionResp`; delete `llamaCompletionReq` and
   `llamaCompletionResp`.
4. Read `completion_probabilities[0].top_logprobs` into a map id → logprob, and call the existing
   `computeOptionDecision(logprobMap, letterIDs, DecideTemperature)` with
   `letterIDs = d.tok.letterIDs(len(q.Options))`. Return `llm.Decision{Choice, Confidence, Probs}`.

`seed` is kept for symmetry but greedy reading at temperature 0 does not use it.

**Writer** — `type writerServer struct { url string; client *http.Client; seed int }`,
`newWriterServer(override string)`, `forAttempt(i)`, and `Generate(ctx, req llm.Request)`: the
**current** `modelServer.Generate` body moved here unchanged (chat completions, the system block,
`Seed: w.seed`), **minus** the `Tools`/`ToolCalls` handling (the writer never sees tools: drop
`llamaToolDef`, `llamaToolCall`, the `tool_calls` finish reason; `StopReason` is `StopMaxTokens`
for `"length"`, else `StopEndTurn`).

Delete: `type modelServer` and every method on it, `formatCriticContent`, `budget`,
`llamaApplyTemplatePath`, `llamaPropsPath`, `llamaApplyTemplateRequest/Response`,
`chatTemplateKwargs`, `llamaPropsResponse`, `DefaultOutputTokens`, `llamaToolDef`, `llamaToolCall`.

## Stage 4 — tests (`tests/`, httptest only, no real llama-server)

| Test | Proves |
|---|---|
| `TestDecider_SendsQwenPromptIDs` | a fake server whose `/tokenize` returns one id per byte of the content (`[]int{int(b)...}`) and whose `/completion` records the `prompt` it receives: for a yes/no and a 3-option question, the recorded ids equal the concatenation of the per-piece tokenizations of `qwen.DecidePrompt(q)`, in order |
| `TestDecider_ReadsLettersWithTemperature` | `/completion` returns logprobs −0.1 for `B` and −2.3 for `A`, `C`: `Choice == 1` and `Probs` equal `softmax(lp/1.03)` within 1e-9 |
| `TestDecider_OptionRange` | 1 option and 11 options → the range error |
| `TestWriter_NoTools` | `/v1/chat/completions` receives no `tools` key; `"finish_reason":"length"` → `StopMaxTokens` |
| `TestEnv_Config` | `Env{...}.Config()` carries every field; `agent.New` on it with an `agent.Texts` whose fields are all non-empty succeeds |
| `TestScenarioExecutionWithFakeServer`, `TestScenarioPendingCallWithFakeServer` (existing, updated) | they now start **two** httptest servers and set `t.Setenv("AGENTEVAL_DECIDER_URL", decider.URL)` and `t.Setenv("AGENTEVAL_WRITER_URL", writer.URL)`: the decider answers `/health`, `/tokenize` (one id per byte) and `/completion` (logprobs that pick the option the scenario needs); the writer answers `/health` and `/v1/chat/completions`. Their assertions on calls, pending and pass counts stay |

Other existing tests that build `Env{Model: ...}` or call the old decider are updated to the new
fields; tests of `Critic` are deleted. The unreachable-server path calls `t.Fatalf` on a real
`*testing.T` and is not unit-tested.

## Stage 5 — docs

- `README.md`: the example scenario's builder becomes
  `func(env agenteval.Env) (*agent.Agent, error) { cfg := env.Config(); cfg.Texts = ...; return agent.New(cfg) }`;
  the "start the servers" block lists **three** servers: decider (8080, the command in
  `errDeciderUnreachable`), writer (8081, `errWriterUnreachable`), judge (8090, unchanged). Remove
  the Qwen3.5 sampling paragraph (the decider reads at temperature 0; the writer's `--temp 0.3`).
- `docs/JUDGE.md`: unchanged.

## Acceptance

- `gotest` green.
- `grep -rn "modelServer\|formatCriticContent\|apply-template\|Env{Model\|agentcontext" --include=*.go .` → empty.
- `grep -n agentcontext go.mod` → empty.

| Stage | Files | Done when |
|---|---|---|
| 1 | `go.mod` | agent v1.1.0, qwen v0.4.7 |
| 2 | `scenario.go` | new `Env`, `Run` uses two servers |
| 3 | `llamaserver.go`, `tokenize.go` | decider with qwen's prompt, writer without tools |
| 4 | `tests/*` | table green |
| 5 | `README.md` | three servers |
