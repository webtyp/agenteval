---
PLAN: "feat: agenteval — scenarios run N times against llama-server, deterministic checks, decider-4b judge"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> Part of
> [`AGENT_ECOSYSTEM_MASTER_PLAN.md`](https://github.com/webtyp/agent/blob/main/docs/AGENT_ECOSYSTEM_MASTER_PLAN.md).

# Plan — `webtyp.com/agenteval`: measure how well an agent behaves

## 0. Context

`webtyp.com/agent` runs an AI agent: it takes a user message, lets a language model choose
tools, runs them, and answers. A language model does not answer the same way twice, so a
pass/fail unit test cannot say whether an agent "works". This library runs one **scenario**
(a use case: the situation, the user's message, what a good answer must satisfy) **N times**
against a real local model, and reports a **success rate**. A scenario passes when at least
`MinPass` of its `Runs` attempts pass.

An attempt is judged by **checks**. Most checks are deterministic Go code ("the agent called
`list_business_hours`", "the answer contains `18:00`"). A few need judgment ("the answer states
nothing the tool did not say"); those ask a **judge**: the decision model decider-4b, running in
`llama-server` on the developer's machine. How the judge is asked and read, and how reliable it
measured, is already written in [docs/JUDGE.md](JUDGE.md). **Read it before stage 4.**

The first consumer is the clinic assistant Jose (`veltylabs/mjosefa-jose`, a private repository
you do not have). Its scenarios will look like this; this plan builds the API it uses:

```go
//go:build eval

package evals

// Caso de uso: un funcionario pregunta hasta qué hora se atiende hoy.
func TestHorarioDeHoy(t *testing.T) {
	agenteval.Scenario{
		Given: agenteval.Given{
			At:    agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{
				Name:        "list_business_hours",
				Description: "Opening hours of the clinic for every day of the week.",
				InputSchema: `{"type":"object","properties":{}}`,
				Action:      model.ActionRead,
				Returns:     "Monday to Friday 08:00-18:00. Saturday and Sunday closed.",
			}},
		},
		When: "¿Hasta qué hora atendemos hoy?",
		Then: []agenteval.Check{
			agenteval.Calls("list_business_hours"),
			agenteval.DoesNotModify(),
			agenteval.Contains("18:00"),
			agenteval.Faithful(),
		},
		Runs: 10, MinPass: 9,
	}.Run(t, jose.New)
}
```

### Where the model and the judge come from

Both run in `llama-server` (llama.cpp's HTTP server) on the developer's machine:

| | default URL | env var that overrides it | started with |
|---|---|---|---|
| model under test | `http://127.0.0.1:8080` | `AGENTEVAL_MODEL_URL` | `llama-server -m <model.gguf> --port 8080 --jinja -c 4096 --reasoning-budget 0` |
| judge | `http://127.0.0.1:8090` | `AGENTEVAL_JUDGE_URL` | `llama-server -m decider-4b-v2.1-Q4_K_M.gguf --port 8090 -np 1 -c 4096 -ngl 99` |

**None of your tests may need a real server.** Every test in this plan runs against fakes built
with `net/http/httptest`, from the recorded answers in `testdata/judge/recorded.json`, except the
one file behind the `eval` build tag (stage 7), which the developer runs by hand.

## Development rules (inline)

- **This repository is host-only developer tooling**, like a test framework. It never compiles to
  WASM and never ships to a browser. It **legitimately uses the standard library**: `testing`,
  `net/http`, `encoding/json`, `time`, `os`, `math`, `strings`. Do **not** "fix" those imports into
  `webtyp.com/fmt` / `webtyp.com/json`. Every `.go` file starts with `//go:build !wasm`.
- It uses the webtyp ecosystem where it talks to the agent: `webtyp.com/agent`,
  `webtyp.com/agentcontext`, `webtyp.com/llm`, `webtyp.com/context`, `webtyp.com/model`,
  `webtyp.com/unixid`. Get them with
  `go get webtyp.com/agent@v0.7.0 webtyp.com/agentcontext@v0.2.0 webtyp.com/llm@v0.1.0 webtyp.com/model@v0.2.0 webtyp.com/unixid@v0.2.28 webtyp.com/context@v0.0.23`.
- Flat layout: library files in the root. **Every test in `tests/`**, `package tests`, importing
  `webtyp.com/agenteval` and using only its exported API. Do not export anything only for tests.
- Max 500 lines per file.
- No string literal repeated in logic: env var names, default URLs, paths and error messages are
  named constants.
- No `TODO`, no commented-out code, no silent fallback. A missing server is a loud test failure
  that says how to start it, never a skip and never a pass.
- Run `gotest` (the repository's test runner) for the whole suite; it must be green.

## Design gate (api-design — five answers)

**1. Prior art.** *Inspect AI* (UK AISI): a `Task` = dataset + solver + scorer, repeated with
`epochs`; our `Scenario` + `Checks` + `Runs`. *promptfoo*: YAML test cases with `assert` entries
(`contains`, `llm-rubric`); our checks, but typed Go instead of YAML. *DeepEval*: pytest-style
test cases with metrics such as faithfulness judged by an LLM; our `Faithful()`. *Cucumber/BDD*:
Given/When/Then, the vocabulary a non-programmer reads. This ecosystem differs in three ways: the
scenarios are **Go code** checked by the compiler (the owner's decision), the judge is a local
**decision model** that returns calibrated probabilities instead of free text, and a judgment
below 0.8 confidence is reported as **unsure**, not as pass or fail.

**2. Novice-name test.** `Scenario`, `Given`, `When`, `Then` (BDD's words), `Runs`, `MinPass`,
`Check`, `Calls("x")`, `Contains("x")`, `DoesNotModify()`, `Faithful()`, `AnswersTheQuestion()`,
`FakeTool`, `Moment`, `Judge`. Each reads as a sentence: "then it calls list_business_hours, does
not modify anything, contains 18:00, and is faithful".

**3. Complexity ledger.**

```
Concepts the developer must learn   +5 (Scenario, Given, Check, FakeTool, Moment) / −0
Files they must touch to do X       +1 per domain test file / −0
Lines at the call site              ~20 per scenario / −0
Ways to do the same thing           +0 (one way to write a scenario) / −0
```

**4. Where it belongs.** A new repository, `agenteval`: evaluation is a different concern from
running an agent, and `agent` ships to the browser while this never does. The client for
`llama-server` lives here (owner's decision), not in `webtyp/llm`, which is a contract and may not
ship an implementation.

**5. What it deletes.** `agenteval.go` (the `Agenteval` placeholder that `gonew` generated). The
copy of `agent`'s integration-test client in `_temp/` is deleted in the last stage. The original
file in the `agent` repository is replaced by scenarios in a later plan of that repository.

## Stage 1 — the scenario (`scenario.go`, `moment.go`)

```go
// Scenario is one use case of an agent. Run executes it Runs times and fails the test when
// fewer than MinPass attempts pass.
type Scenario struct {
	Given   Given
	When    string  // the user's message
	Then    []Check // every check must pass for an attempt to pass
	Runs    int     // attempts; 0 means DefaultRuns
	MinPass int     // required: attempts that must pass, 1..Runs
}

// Given is the situation before the user speaks.
type Given struct {
	At    Moment     // required: the date and time the user speaks
	Tools []FakeTool // the tools the agent can use in this scenario
}

// Builder builds the agent under test from what the scenario provides.
type Builder func(Env) (*agent.Agent, error)

// Env is everything a scenario provides to the agent. A Builder passes each field to
// agent.Config unchanged and adds only its own identity and settings.
type Env struct {
	Model     llm.Client
	Tokens    llm.TokenCounter
	Budget    agentcontext.Budget
	Clock     agent.Clock
	Memory    agent.MemoryStore // fresh for every attempt
	IDGen     model.IDGenerator
	ToolIndex agent.ToolIndex
	Tools     []agent.Tool // the scenario's FakeTools; pass them as Config.LocalTools
}

const DefaultRuns = 10

func (s Scenario) Run(t *testing.T, build Builder)
```

`Moment` (in `moment.go`) is a local date and time that implements `agent.Clock`:

```go
type Moment struct {
	Year, Month, Day, Hour, Minute int
	UTCOffsetMinutes               int // e.g. -180 for UTC-3
}
func (m Moment) Now() int64            // unix nanoseconds of that local moment, UTC
func (m Moment) UTCOffsetMinutes() int // the same field
```

`Now` is `time.Date(Year, Month, Day, Hour, Minute, 0, 0, time.FixedZone("", off*60)).UnixNano()`.

`Run`, step by step:

1. `s.Validate()`; on error `t.Fatal(err)`. `Validate` returns these exact messages (constants):
   `agenteval: Given.At is required`, `agenteval: MinPass is required (1..Runs)`,
   `agenteval: MinPass %d is greater than Runs %d`, `agenteval: When is required`,
   `agenteval: Then needs at least one check`, and the FakeTool errors of stage 2.
2. Connect to the model server (stage 3). Unreachable → `t.Fatalf` with
   `agenteval: no model server at %s; start one with: llama-server -m <model.gguf> --port 8080 --jinja -c 4096 --reasoning-budget 0 (or set AGENTEVAL_MODEL_URL)`.
3. For each attempt `i` in `1..Runs`: build a fresh `Env` (new `agent.NewMemMemory()`,
   `agent.NewMemToolIndex()`, new FakeTool recorders, `Clock: s.Given.At`, one
   `unixid.NewUnixID()` per `Run`), call `build(env)`, then
   `a.Run(context.Background(), fmt.Sprintf("attempt-%d", i), s.When)`. Record an `Attempt`
   (stage 4) and evaluate every check.
4. Log one line per attempt that did not pass:
   `attempt 3: FAIL Contains("18:00"): the answer does not contain "18:00"` (or `UNSURE ...`), then
   the answer on the next line, and a final summary:
   `TestHorarioDeHoy: 9/10 passed, 1 failed, 0 unsure (MinPass 9)`. Fail the test with
   `t.Errorf` when passed < MinPass. An unsure attempt does not count as passed.

## Stage 2 — tools that answer what the scenario says (`tool.go`)

```go
// FakeTool is a tool with a fixed answer. Its definition should be the real tool's, so the
// agent sees exactly what it will see in production.
type FakeTool struct {
	Name, Description, InputSchema string
	Action  byte   // required: model.ActionRead, ActionCreate, ActionUpdate or ActionDelete
	Returns string // what the tool answers, whatever the arguments
}
```

Validation (stage 1, step 1): empty `Name` → `agenteval: FakeTool needs a Name`; `Action` not one
of the four `model.Action*` bytes → `agenteval: FakeTool %q needs an Action (model.ActionRead, ActionCreate, ActionUpdate or ActionDelete)`.
There is no default action: a tool nobody classified must not be treated as harmless.

For every attempt, each FakeTool becomes an unexported value implementing `agent.Tool` that
returns `Returns` and records every call (name, arguments JSON, what it returned) into the
attempt.

## Stage 3 — the model client (`llamaserver.go`, unexported)

Port the `llamaServer` type of `_temp/agent/integration_test.go` (the agent repository's
integration test, copied here for you): `Generate` over `POST /v1/chat/completions` with tools,
`CountTokens` over `POST /tokenize`. Keep its behavior; rename it `modelServer`, take the base URL
as a field, and use the URL constants. Add `budget()`: `GET /props`, read
`default_generation_settings.n_ctx`, and return
`agentcontext.Budget{ContextTokens: n_ctx, OutputTokens: DefaultOutputTokens}` with
`const DefaultOutputTokens = 512`. The same call is the reachability check of stage 1, step 2.
Use a `net/http.Client` with a 120 s timeout.

## Stage 4 — attempts and checks (`check.go`)

```go
// Attempt is what happened in one run of a scenario.
type Attempt struct {
	Question string     // Scenario.When
	At       Moment
	Answer   string     // what agent.Run returned
	Err      error      // what agent.Run returned
	Calls    []ToolCall // every tool call, in order
}

type ToolCall struct {
	Name, Input, Output string
	Action              byte
}

// Check judges one attempt.
type Check interface {
	Name() string // how the check reads in the report, e.g. `Contains("18:00")`
	Check(a Attempt, j Judge) Result
}

type Result struct {
	Verdict Verdict
	Reason  string // why, in one line
}

type Verdict int

const (
	Pass Verdict = iota + 1
	Fail
	Unsure
)
```

An attempt passes when `Err == nil` and every check returns `Pass`. `Err != nil` fails the attempt
with the reason `agent.Run failed: <err>` and no check is evaluated.

Deterministic checks (exact behavior and `Reason` text are yours, one line each):

- `Calls(name string) Check` — some call has that `Name`.
- `DoesNotModify() Check` — every call has `Action == model.ActionRead`.
- `Contains(text string) Check` / `NotContains(text string) Check` — exact substring of `Answer`.

## Stage 5 — the judge (`judge.go`)

Implement exactly what [docs/JUDGE.md](JUDGE.md) describes.

```go
// Judge answers a closed question about a text with a probability for every option.
type Judge interface {
	Decide(state, question string, options []string, kind QuestionKind) (Decision, error)
}

type QuestionKind int

const (
	Choice QuestionKind = iota + 1 // temperature 1.11
	YesNo                          // options must be exactly ["no", "yes"]; temperature 1.56
)

type Decision struct {
	Choice     string    // the most probable option
	Confidence float64   // its probability
	Probs      []float64 // one per option, in the order given
}

// MinConfidence is the confidence under which a judgment is reported as Unsure.
const MinConfidence = 0.8
```

`NewDeciderJudge(url string) Judge` (exported; tests and `agentlab` use it): 1 to 10 options (else the error
`agenteval: the judge takes 1 to 10 options, got %d`), tokenize `"Context:\n" + state` and the rest
(from `"\n\nQuestion: "` to `"\nAnswer: ("`) **as two separate `/tokenize` calls** with
`"add_special": false`, concatenate the ids, `POST /completion` with the body in JUDGE.md, read
`completion_probabilities[0].top_logprobs`, take the letter ids (`A`..`J` tokenized once and kept),
a missing letter has logprob −1e9, apply `z_i = logprob_i / T`, softmax over the options. The judge
is created lazily by `Run` only when a check uses it; unreachable →
`t.Fatalf` with `agenteval: no judge at %s; start it with: llama-server -m ~/Dev/LMmodels/Mapika/decider-4b-GGUF/decider-4b-v2.1-Q4_K_M.gguf --port 8090 -np 1 -c 4096 -ngl 99 (or set AGENTEVAL_JUDGE_URL)`.

Judge checks. Their state is always built by one unexported function, in English (JUDGE.md, rule 3):

```text
Today is Tuesday 2026-09-29 10:00.
User asked: <Attempt.Question>
Tool list_business_hours returned: <Output>        (one line per call, in order)
Assistant answered: <Attempt.Answer>
```

- `Faithful() Check` — question
  `Does the assistant's answer state anything that the tool result does not support?`, options
  `["no, everything it says is supported by the tool result", "yes, it states something the tool result does not support"]`, `Choice`.
  First option → `Pass`, second → `Fail`.
- `AnswersTheQuestion() Check` — question
  `Does the assistant's answer actually answer the user's question?`, options `["yes", "no"]`,
  `Choice`. `yes` → `Pass`.
- Both: `Confidence < MinConfidence` → `Unsure`, with the reason
  `the judge chose %q with confidence %.2f (below 0.80)`.

## Stage 6 — tests (`tests/`)

All with fakes; none needs a real server.

- `tests/judge_test.go`: an `httptest` server that answers `/tokenize` and `/completion` from
  `testdata/judge/recorded.json` (for each record: the two `tokenize` entries map `content` →
  `tokens`; the `completion_probabilities` answer the `/completion` whose `prompt` ids equal the
  concatenation). Create the judge with the exported `NewDeciderJudge(url string) Judge` (stage 5;
  `agentlab` will use it too) and call `Decide` with each record's `state`, `question`, `options`
  and kind (`YesNo` when the record has `"T": 1.56`, else `Choice`). For **every** record the
  choice must equal `result.choice`, and every probability must be within `1e-3` of
  `result.probs` (probe.py rounds to 3 decimals).
- `tests/check_test.go`: every deterministic check, both verdicts; `Faithful` and
  `AnswersTheQuestion` with a fake `Judge` returning chosen decisions, including `Unsure` at 0.79
  and `Pass` at 0.80.
- `tests/scenario_test.go`: an `httptest` fake of the model server (`/props`,
  `/v1/chat/completions`, `/tokenize`) whose chat answers are scripted: first a tool call to
  `list_business_hours`, then the answer `Hasta las 18:00.`; the critic prompt (system contains
  `critic`) gets `SUFFICIENT`. Point `AGENTEVAL_MODEL_URL` at it with `t.Setenv`. Assert: a scenario
  with `Calls`, `DoesNotModify` and `Contains("18:00")` passes 3/3. Test the validation messages
  of stages 1 and 2 through `func (s Scenario) Validate() error` (exported; `Run` calls it first
  and passes its error to `t.Fatal`): one case per message.
- `tests/moment_test.go`: `Moment{2026, 9, 29, 10, 0, -180}.Now()` is `1790686800 * 1e9`.

## Stage 7 — the real evaluation (`tests/eval_test.go`, build tag `eval`)

`//go:build eval`. One scenario equivalent to `TestIntegration_ClinicHours` in
`_temp/agent/integration_test.go`: a `clinic_hours` FakeTool (`Action: model.ActionRead`,
`Returns: "Open Monday to Friday, 8:00 to 17:00. Closed on weekends."`), `When: "What time does the clinic open on Monday?"`,
`Then: Calls("clinic_hours"), Contains("8"), Faithful()`, `Runs: 10, MinPass: 1`, and a builder that
uses the identity of that test. It is run by hand with `go test -tags eval ./tests/`; it is not
part of `gotest`. Write in a comment above it that Qwen3.5-0.8B reaches the tool only about half of
the time through `search_tools`, so MinPass 1 only proves the pipeline works end to end.

## Stage 8 — docs and cleanup

- `README.md`: keep its Spanish text; replace the `STATUS` note with an "Escribir un escenario"
  section showing the example of §0 and the two `llama-server` commands, and list
  `docs/JUDGE.md`.
- Delete `agenteval.go`.
- Delete `_temp/` (`test ! -e _temp`).

## Stages

| Stage | Files | Acceptance |
|---|---|---|
| 1 | `scenario.go`, `moment.go` | `tests/moment_test.go`, validation tests |
| 2 | `tool.go` | FakeTool validation and recording tests |
| 3 | `llamaserver.go` | exercised by `tests/scenario_test.go` |
| 4 | `check.go` | `tests/check_test.go` |
| 5 | `judge.go` | `tests/judge_test.go`: all 36 recorded cases match |
| 6 | `tests/` | `ls *_test.go` in the root → nothing |
| 7 | `tests/eval_test.go` | `go vet -tags eval ./...` passes (not run in CI) |
| 8 | `README.md`; `agenteval.go`, `_temp/` deleted | `test ! -e _temp && test ! -e agenteval.go` |
| all | — | `gotest` green; `grep -rn "TODO\|FIXME" --include=*.go .` → empty |
