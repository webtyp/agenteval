//go:build !wasm

package tests

import (
	"testing"

	"webtyp.com/agenteval"
	"webtyp.com/fmt"
	"webtyp.com/llm"
	"webtyp.com/model"
	"webtyp.com/qwen"
)

// The decider is asked with the prompt the browser reads (D30): the first question of a turn, the
// tool choice, reaches llama-server as exactly the pieces of qwen.DecidePrompt, tokenized in order,
// with no chat template around them.
func TestDecider_SendsQwenPrompt(t *testing.T) {
	var prompts [][]int
	dec := fakeDecider(t, &prompts)
	wr := fakeWriter("Hasta las 18:00.", nil)
	t.Cleanup(dec.Close)
	t.Cleanup(wr.Close)
	t.Setenv("AGENTEVAL_DECIDER_URL", dec.URL)
	t.Setenv("AGENTEVAL_WRITER_URL", wr.URL)

	const msg = "¿Hasta qué hora atendemos hoy?"
	tool := agenteval.FakeTool{
		Name: "list_business_hours", Description: "Opening hours of the clinic.",
		InputSchema: `{"type":"object","properties":{}}`, Action: model.Read, Returns: "08:00-18:00",
	}
	agenteval.Scenario{
		Given:   agenteval.Given{At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180}, Tools: []agenteval.FakeTool{tool}},
		When:    msg,
		Then:    []agenteval.Check{agenteval.Calls("list_business_hours")},
		Runs:    1,
		MinPass: 1,
	}.Run(t, buildWithTexts)

	if len(prompts) == 0 {
		t.Fatal("the decider was never asked")
	}
	want := ""
	for _, p := range qwen.DecidePrompt(llm.Question{
		Context: fmt.Sprintf("%s wrote: %s", texts().Speaker, msg),
		Text:    "Which tool should the assistant use?",
		Options: []string{tool.Name + ": " + tool.Description, "none: " + texts().NoToolOption},
	}) {
		want += p
	}
	got := make([]byte, len(prompts[0]))
	for i, id := range prompts[0] {
		got[i] = byte(id)
	}
	if string(got) != want {
		t.Errorf("first prompt\n got %q\nwant %q", got, want)
	}
}
