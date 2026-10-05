//go:build !wasm

package agenteval

import (
	"errors"
	"fmt"
	"testing"

	"webtyp.com/agent"
	"webtyp.com/context"
	"webtyp.com/llm"
	"webtyp.com/model"
	"webtyp.com/unixid"
)

const DefaultRuns = 10

var (
	ErrGivenAtRequired   = errors.New("agenteval: Given.At is required")
	ErrMinPassRequired   = errors.New("agenteval: MinPass is required (1..Runs)")
	ErrWhenRequired      = errors.New("agenteval: When is required")
	ErrThenCheckRequired = errors.New("agenteval: Then needs at least one check")
)

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
		Decider:    e.Decider,
		Writer:     e.Writer,
		Tokens:     e.Tokens,
		Clock:      e.Clock,
		Memory:     e.Memory,
		IDGen:      e.IDGen,
		ToolIndex:  e.ToolIndex,
		LocalTools: e.Tools,
	}
}

// Validate returns validation errors for Scenario.
func (s Scenario) Validate() error {
	if s.Given.At.Year == 0 {
		return ErrGivenAtRequired
	}
	runs := s.Runs
	if runs == 0 {
		runs = DefaultRuns
	}
	if s.MinPass <= 0 {
		return ErrMinPassRequired
	}
	if s.MinPass > runs {
		return fmt.Errorf("agenteval: MinPass %d is greater than Runs %d", s.MinPass, runs)
	}
	if s.When == "" {
		return ErrWhenRequired
	}
	if len(s.Then) == 0 {
		return ErrThenCheckRequired
	}
	for _, ft := range s.Given.Tools {
		if err := ft.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Run executes the scenario Runs times and fails t when fewer than MinPass attempts pass.
func (s Scenario) Run(t *testing.T, build Builder) {
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}

	runs := s.Runs
	if runs == 0 {
		runs = DefaultRuns
	}

	dec := newDeciderServer("")
	if !dec.available() {
		t.Fatalf(errDeciderUnreachable, dec.url)
	}
	wr := newWriterServer("")
	if !wr.available() {
		t.Fatalf(errWriterUnreachable, wr.url)
	}

	var judge llm.Decider
	needsJudge := false
	for _, chk := range s.Then {
		if requiresJudge(chk) {
			needsJudge = true
			break
		}
	}

	if needsJudge {
		judge = NewDeciderJudge("")
		if dj, ok := judge.(*deciderJudge); ok {
			if !dj.available() {
				t.Fatalf(errJudgeUnreachable, dj.url)
			}
		}
	}

	passedCount := 0
	failedCount := 0
	unsureCount := 0

	for i := 1; i <= runs; i++ {
		sessionID := fmt.Sprintf("attempt-%d", i)

		idGen, err := unixid.NewUnixID()
		if err != nil {
			t.Fatalf("unixid.NewUnixID failed: %v", err)
		}

		mem := agent.NewMemMemory()
		toolIdx := agent.NewMemToolIndex()

		var recCalls []ToolCall
		var wrappedTools []agent.Tool
		var toolDefs []llm.ToolDef

		for _, ft := range s.Given.Tools {
			wt := ft.wrap(&recCalls)
			wrappedTools = append(wrappedTools, wt)
			toolDefs = append(toolDefs, llm.ToolDef{
				Name:        ft.Name,
				Description: ft.Description,
				InputSchema: ft.InputSchema,
			})
		}

		ctx := context.Background()
		if err := toolIdx.IndexTools(ctx, toolDefs); err != nil {
			t.Fatalf("toolIdx.IndexTools failed: %v", err)
		}

		env := Env{
			Decider:   dec,
			Writer:    wr.forAttempt(i),
			Tokens:    dec,
			Clock:     s.Given.At.clock(),
			Memory:    mem,
			IDGen:     idGen,
			ToolIndex: toolIdx,
			Tools:     wrappedTools,
		}

		a, err := build(env)
		var reply agent.Reply
		if err == nil {
			reply, err = a.Run(ctx, sessionID, s.When)
		}

		attempt := Attempt{
			Question: s.When,
			At:       s.Given.At,
			Answer:   reply.Text,
			Pending:  reply.Pending,
			Err:      err,
			Calls:    recCalls,
		}

		attemptPass := true
		attemptUnsure := false

		if err != nil {
			attemptPass = false
			failedCount++
			t.Logf("attempt %d: FAIL agent.Run failed: %v", i, err)
			t.Logf("%s", reply.Text)
		} else {
			for _, chk := range s.Then {
				res := chk.Check(attempt, judge)
				switch res.Verdict {
				case Pass:
					// pass
				case Fail:
					attemptPass = false
					t.Logf("attempt %d: FAIL %s: %s", i, chk.Name(), res.Reason)
					t.Logf("%s", reply.Text)
				case Unsure:
					attemptPass = false
					attemptUnsure = true
					t.Logf("attempt %d: UNSURE %s: %s", i, chk.Name(), res.Reason)
					t.Logf("%s", reply.Text)
				}
				if !attemptPass {
					break
				}
			}
			if attemptPass {
				passedCount++
			} else if attemptUnsure {
				unsureCount++
			} else {
				failedCount++
			}
		}
	}

	summary := fmt.Sprintf("%s: %d/%d passed, %d failed, %d unsure (MinPass %d)", t.Name(), passedCount, runs, failedCount, unsureCount, s.MinPass)
	t.Log(summary)

	if passedCount < s.MinPass {
		t.Errorf("%s", summary)
	}
}

func requiresJudge(c Check) bool {
	if jc, ok := c.(interface{ NeedsJudge() bool }); ok {
		return jc.NeedsJudge()
	}
	return false
}
