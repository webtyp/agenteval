//go:build !wasm

package tests

import (
	"fmt"
	"testing"

	"webtyp.com/agenteval"
	"webtyp.com/context"
	"webtyp.com/llm"
	"webtyp.com/model"
)

type mockJudge struct {
	choiceIndex int
	confidence  float64
	err         error
}

func (m mockJudge) Decide(ctx *context.Context, q llm.Question) (llm.Decision, error) {
	if m.err != nil {
		return llm.Decision{}, m.err
	}
	return llm.Decision{
		Choice:     m.choiceIndex,
		Confidence: m.confidence,
		Probs:      []float64{m.confidence, 1.0 - m.confidence},
	}, nil
}

func TestDeterministicChecks(t *testing.T) {
	att := agenteval.Attempt{
		Question: "Hola",
		Answer:   "Atendemos hasta las 18:00.",
		Calls: []agenteval.ToolCall{
			{Name: "list_hours", Input: "{}", Output: "08:00-18:00", Action: model.Read},
		},
	}

	// Calls
	res := agenteval.Calls("list_hours").Check(att, nil)
	if res.Verdict != agenteval.Pass {
		t.Errorf("Calls('list_hours') verdict = %v, want Pass", res.Verdict)
	}
	res = agenteval.Calls("other_tool").Check(att, nil)
	if res.Verdict != agenteval.Fail {
		t.Errorf("Calls('other_tool') verdict = %v, want Fail", res.Verdict)
	}

	// DoesNotModify
	res = agenteval.DoesNotModify().Check(att, nil)
	if res.Verdict != agenteval.Pass {
		t.Errorf("DoesNotModify() verdict = %v, want Pass", res.Verdict)
	}

	attMut := att
	attMut.Calls = []agenteval.ToolCall{
		{Name: "create_appointment", Input: "{}", Output: "ok", Action: model.Create},
	}
	res = agenteval.DoesNotModify().Check(attMut, nil)
	if res.Verdict != agenteval.Fail {
		t.Errorf("DoesNotModify() with mutation verdict = %v, want Fail", res.Verdict)
	}

	// Contains
	res = agenteval.Contains("18:00").Check(att, nil)
	if res.Verdict != agenteval.Pass {
		t.Errorf("Contains('18:00') verdict = %v, want Pass", res.Verdict)
	}
	res = agenteval.Contains("20:00").Check(att, nil)
	if res.Verdict != agenteval.Fail {
		t.Errorf("Contains('20:00') verdict = %v, want Fail", res.Verdict)
	}

	// NotContains
	res = agenteval.NotContains("20:00").Check(att, nil)
	if res.Verdict != agenteval.Pass {
		t.Errorf("NotContains('20:00') verdict = %v, want Pass", res.Verdict)
	}
	res = agenteval.NotContains("18:00").Check(att, nil)
	if res.Verdict != agenteval.Fail {
		t.Errorf("NotContains('18:00') verdict = %v, want Fail", res.Verdict)
	}
}

func TestJudgeChecks(t *testing.T) {
	att := agenteval.Attempt{
		Question: "¿Hasta qué hora atienden?",
		Answer:   "Atendemos hasta las 18:00.",
		Calls: []agenteval.ToolCall{
			{Name: "list_hours", Input: "{}", Output: "08:00-18:00", Action: model.Read},
		},
	}

	// Faithful Pass (0.80)
	jPass := mockJudge{
		choiceIndex: 0,
		confidence:  0.80,
	}
	res := agenteval.Faithful().Check(att, jPass)
	if res.Verdict != agenteval.Pass {
		t.Errorf("Faithful pass verdict = %v, want Pass", res.Verdict)
	}

	// Faithful Unsure (0.79)
	jUnsure := mockJudge{
		choiceIndex: 0,
		confidence:  0.79,
	}
	res = agenteval.Faithful().Check(att, jUnsure)
	if res.Verdict != agenteval.Unsure {
		t.Errorf("Faithful unsure verdict = %v, want Unsure", res.Verdict)
	}
	if want := "the judge chose \"no, everything it says is supported by the tool result\" with confidence 0.79 (below 0.80)"; res.Reason != want {
		t.Errorf("Faithful unsure reason = %q, want %q", res.Reason, want)
	}

	// Faithful Fail
	jFail := mockJudge{
		choiceIndex: 1,
		confidence:  0.95,
	}
	res = agenteval.Faithful().Check(att, jFail)
	if res.Verdict != agenteval.Fail {
		t.Errorf("Faithful fail verdict = %v, want Fail", res.Verdict)
	}

	// AnswersTheQuestion Pass
	jAnswersPass := mockJudge{
		choiceIndex: 0,
		confidence:  0.85,
	}
	res = agenteval.AnswersTheQuestion().Check(att, jAnswersPass)
	if res.Verdict != agenteval.Pass {
		t.Errorf("AnswersTheQuestion pass verdict = %v, want Pass", res.Verdict)
	}

	// AnswersTheQuestion Fail
	jAnswersFail := mockJudge{
		choiceIndex: 1,
		confidence:  0.85,
	}
	res = agenteval.AnswersTheQuestion().Check(att, jAnswersFail)
	if res.Verdict != agenteval.Fail {
		t.Errorf("AnswersTheQuestion fail verdict = %v, want Fail", res.Verdict)
	}

	// AnswersTheQuestion Error
	jErr := mockJudge{err: fmt.Errorf("network error")}
	res = agenteval.AnswersTheQuestion().Check(att, jErr)
	if res.Verdict != agenteval.Unsure {
		t.Errorf("AnswersTheQuestion error verdict = %v, want Unsure", res.Verdict)
	}
}
