---
PLAN: "feat!: agenteval on agent v0.8 — Reply and pending confirmations, typed actions, llm.Decider judge and critic, Env.Config"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 11491948376786452849
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `webtyp.com/agenteval` v0.2.0: follow `agent` v0.8

## 0. Context

`agenteval` runs evaluation scenarios of a `webtyp.com/agent` agent against a real local model
in `llama-server`, N times, and reports a success rate (see `README.md` and `docs/JUDGE.md`).
**It does not build today** against `webtyp.com/agent` v0.8, which changed three things:

1. `Agent.Run(ctx, sessionID, message)` returns `(agent.Reply, error)` instead of
   `(string, error)`:

   ```go
   type Reply struct {
   	Text    string         // the answer, or what the model wrote before its pending calls
   	Pending []llm.ToolCall // tool calls that modify data and wait for the person; they have NOT run
   }
   ```

   After a `Reply` with `Pending`, the application calls `a.Confirm(ctx, sessionID)` or
   `a.Decline(ctx, sessionID)`. A scenario attempt **ends at the first `Reply`**: it never
   confirms. What the agent asked to confirm is part of what is judged.
2. `agent.Tool` has `Action() model.Action`. `model.Action` (from `webtyp.com/model`) is a
   bitmask with `model.Create`, `model.Read`, `model.Update` and `model.Delete`. A tool is
   read-only only when its action is exactly `model.Read`. Today `FakeTool.Action` is a `byte`
   (`model.ActionRead`, …), which no longer satisfies `agent.Tool`.
3. `agent.Config` has `Critic llm.Decider`, a closed question with a probability per option. The
   contract comes from `webtyp.com/llm` v0.2.0:

   ```go
   type Decider interface {
   	Decide(ctx *context.Context, q Question) (Decision, error)
   }
   type Question struct {
   	Context string   // what the model reads before the question
   	Text    string   // the question itself
   	Options []string // 2 to 10 possible answers; yes/no questions use exactly {"no", "yes"}
   }
   type Decision struct {
   	Choice     int       // index into Question.Options of the most probable option
   	Confidence float64   // the probability of Choice
   	Probs      []float64 // one per option, in the order given
   }
   ```

   `agenteval`'s own `Judge`, `Decision` and `QuestionKind` duplicate it and must go.

## Development rules (inline)

- **Host-only tooling:** every file starts with `//go:build !wasm`. It legitimately uses the
  standard library (`testing`, `net/http`, `encoding/json`, `time`, `math`, `os`, `errors`,
  `fmt`, `strings`). Do **not** convert those imports to webtyp packages.
- `go get webtyp.com/agent@latest webtyp.com/llm@v0.2.0 webtyp.com/model@latest` first. The
  agent version must be v0.8.0 or newer.
- All tests in `tests/` (`package tests`), exported API only. None may need a real server: use
  `httptest` fakes and `testdata/judge/recorded.json`.
- Repeated strings are constants. No `TODO`. No silent fallbacks.
- `gotest` green, and `go vet -tags eval ./...` passes.

## Design gate (api-design — five answers)

1. **Prior art.** Inspect AI's scorers receive the whole task state, including tool calls, and
   ours receive `Attempt` with `Pending`. promptfoo's `javascript` and `llm-rubric` asserts map
   to our `Check`. OpenAI Agents SDK run results expose "interruptions" (approvals pending) for
   the caller to assert on, which is our `Attempt.Pending`.
2. **Novice-name test.** `AsksToConfirm("x")` ("it asks to confirm x"), `AsksNothingToConfirm()`,
   `env.Config()` ("the agent config this environment provides"), and `Env.Critic`.
3. **Complexity ledger.**

   ```
   Concepts the developer must learn   +1 (Pending checks) / −3 (Judge, Decision, QuestionKind → llm's)
   Files they must touch to do X       −0
   Lines at the call site              −8 per builder (env.Config() instead of copying fields)
   Ways to do the same thing           −1 (one decision contract in the ecosystem)
   ```

4. **Where it belongs.** Evaluation-specific checks stay here. The decision contract is `llm`'s.
5. **What it deletes.** `Judge`, `Decision`, `QuestionKind`, `Choice`/`YesNo` constants, and
   the byte form of `FakeTool.Action` / `ToolCall.Action`.

## Stage 1 — typed actions (`tool.go`, `check.go`)

- `FakeTool.Action` becomes `model.Action`. `Validate`: zero, or any bit outside
  `model.AllActions`, is the error
  `agenteval: FakeTool %q needs an Action (model.Read, model.Create, model.Update, model.Delete or a combination)`.
  A combination such as `model.Create|model.Update` is valid (an upsert).
- The wrapper's `Action()` returns `model.Action`. `ToolCall.Action` becomes `model.Action`.
- `DoesNotModify()`: every executed call has `Action == model.Read`.

## Stage 2 — `Reply` and pending calls (`scenario.go`, `check.go`)

- `Attempt` gains `Pending []llm.ToolCall // what the agent asked the person to confirm; not executed`.
  `Answer` is `reply.Text`.
- New checks in `check.go`:
  - `AsksToConfirm(name string) Check`: some pending call has that name. Reason when it fails:
    `the agent did not ask to confirm %q`.
  - `AsksNothingToConfirm() Check`: `Pending` is empty. The reason names the pending calls.
- `Faithful()` and `AnswersTheQuestion()`: when `Answer` is empty and `Pending` is not, the
  verdict is `Unsure` with the reason `the agent is waiting for confirmation; there is no answer to judge`.

## Stage 3 — the judge is an `llm.Decider` (`judge.go`)

- Delete `Judge`, `Decision`, `QuestionKind`, `Choice`, `YesNo`.
- `NewDeciderJudge(url string) llm.Decider`. Its `Decide(ctx, q)` is today's readout with
  `state = q.Context`, `question = q.Text`, `options = q.Options`. The temperature is 1.56 when
  `q.Options` is exactly `["no", "yes"]` and 1.11 otherwise (constants). It returns
  `llm.Decision{Choice: index, Confidence, Probs}`.
- `Check` becomes `Check(a Attempt, j llm.Decider) Result`. `Faithful()` and
  `AnswersTheQuestion()` build an `llm.Question` and compare `Decision.Choice` by index (0 = the
  first option).
- `tests/judge_test.go`: same recorded cases. The kind is no longer passed; the result must
  still match every record (records with `"T": 1.56` have options `["no","yes"]`).

## Stage 4 — the agent's critic and `Env.Config()` (`llamaserver.go`, `scenario.go`)

- The model server client gets `Decide(ctx, q llm.Question) (llm.Decision, error)`: the same
  model under test answers the critic's question, as it will in the browser.
  1. `POST /apply-template` with `{"messages":[{"role":"user","content": C}]}`, where `C` is
     `"Context:\n" + q.Context + "\n\nQuestion: " + q.Text + "\nOptions:\n(A) …\n(B) …\nAnswer with the letter of one option."`.
     llama-server answers `{"prompt": "..."}`, the chat-formatted text ending in the
     assistant's turn.
  2. `POST /completion` with `{"prompt": prompt + "(", "n_predict": 1, "n_probs": 64, "temperature": 0, "cache_prompt": false}`.
  3. Softmax over the option letters' logprobs, at temperature 1. Tokenize `A`..`J` once with
     `/tokenize`, as the judge does; share that code with `judge.go` instead of copying it.
- `Env` gains `Critic llm.Decider`, the model server for this attempt.
- `func (e Env) Config() agent.Config` returns an `agent.Config` with `LLMs.Primary: e.Model`,
  `Tokens`, `Budget`, `Clock`, `Memory`, `IDGen`, `ToolIndex`, `LocalTools: e.Tools` and
  `Critic: e.Critic`. A builder then only adds its identity:
  `cfg := env.Config(); cfg.Identity = …; return agent.New(cfg)`.
- `tests/scenario_test.go`: the fake model server also answers `/apply-template` and a critic
  `/completion` (letter `A` most probable, which accepts). Add one scenario whose script makes the
  model call a `model.Create` FakeTool after discovering it with `search_tools`. The attempt ends
  with `Pending` holding that call, `AsksToConfirm` passes, and the tool was not executed.

## Stage 5 — callers and docs

- `tests/eval_test.go` (tag `eval`): the builder becomes `cfg := env.Config(); cfg.Identity = …`.
  `FakeTool.Action: model.Read`.
- `README.md`: the example uses `Action: model.Read` and mentions `AsksToConfirm`.
  `docs/JUDGE.md`: the judge is an `llm.Decider`, and the temperature is chosen from the options.

## Stages

| Stage | Files | Acceptance |
|---|---|---|
| 1 | `tool.go`, `check.go` | builds against agent ≥ v0.8.0 |
| 2 | `scenario.go`, `check.go` | pending-call tests |
| 3 | `judge.go` | `tests/judge_test.go`: 36/36 recorded cases; `grep -rn "QuestionKind\|type Judge\|type Decision" --include=*.go .` → empty |
| 4 | `llamaserver.go`, `scenario.go` | critic + `Env.Config` tests |
| 5 | `tests/eval_test.go`, `README.md`, `docs/JUDGE.md` | `go vet -tags eval ./...` |
| all | — | `gotest` green |
