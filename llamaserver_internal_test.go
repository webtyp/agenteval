//go:build !wasm

package agenteval

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/llm"
)

func TestDecider_ReadsLettersWithTemperature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tokenize":
			var body struct{ Content string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			ids := []int{}
			for _, b := range []byte(body.Content) {
				ids = append(ids, int(b))
			}
			json.NewEncoder(w).Encode(map[string]any{"tokens": ids})
		case "/completion":
			json.NewEncoder(w).Encode(map[string]any{"completion_probabilities": []map[string]any{{"top_logprobs": []map[string]any{
				{"id": 'A', "logprob": -2.3}, {"id": 'B', "logprob": -0.1}, {"id": 'C', "logprob": -2.3},
			}}}})
		}
	}))
	defer srv.Close()

	d, err := newDeciderServer(srv.URL).Decide(context.Background(), llm.Question{Context: "c", Text: "q", Options: []string{"a", "b", "c"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Choice != 1 {
		t.Errorf("Choice = %d, want 1", d.Choice)
	}
	lp := []float64{-2.3, -0.1, -2.3}
	var sum float64
	for _, v := range lp {
		sum += math.Exp(v / DecideTemperature)
	}
	for i, v := range lp {
		if want := math.Exp(v/DecideTemperature) / sum; math.Abs(d.Probs[i]-want) > 1e-9 {
			t.Errorf("Probs[%d] = %v, want %v", i, d.Probs[i], want)
		}
	}
}

func TestDecider_OptionRange(t *testing.T) {
	d := newDeciderServer("http://127.0.0.1:1")
	for _, n := range []int{1, 11} {
		_, err := d.Decide(context.Background(), llm.Question{Options: make([]string, n)})
		if err == nil || !strings.Contains(err.Error(), "2 to 10 options") {
			t.Errorf("%d options: err = %v", n, err)
		}
	}
}

func TestWriter_NoToolsAndLength(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "hola"}, "finish_reason": "length"}},
		})
	}))
	defer srv.Close()

	resp, err := newWriterServer(srv.URL).Generate(context.Background(), llm.Request{
		System:   "s",
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: "s2"}, {Role: llm.RoleUser, Content: "u"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["tools"]; ok {
		t.Error("the writer request carries tools")
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || msgs[0].(map[string]any)["content"] != "s\n\ns2" {
		t.Errorf("messages = %v, want one system block first", msgs)
	}
	if resp.StopReason != llm.StopMaxTokens {
		t.Errorf("StopReason = %v, want StopMaxTokens", resp.StopReason)
	}
}
