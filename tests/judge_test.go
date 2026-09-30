//go:build !wasm

package tests

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"webtyp.com/agenteval"
	"webtyp.com/llm"
)

type recordedFile struct {
	Letters map[string]int `json:"letters"`
	Records []recordedCase `json:"records"`
}

type recordedCase struct {
	Case struct {
		Name     string   `json:"name"`
		State    string   `json:"state"`
		Question string   `json:"question"`
		Options  []string `json:"options"`
		Expect   string   `json:"expect"`
		T        float64  `json:"T"`
	} `json:"case"`
	Tokenize []struct {
		Content string `json:"content"`
		Tokens  []int  `json:"tokens"`
	} `json:"tokenize"`
	CompletionProbabilities []struct {
		TopLogprobs []struct {
			ID      int     `json:"id"`
			Logprob float64 `json:"logprob"`
		} `json:"top_logprobs"`
	} `json:"completion_probabilities"`
	Result struct {
		Choice string             `json:"choice"`
		Probs  map[string]float64 `json:"probs"`
	} `json:"result"`
}

func TestDeciderJudgeRecordedCases(t *testing.T) {
	data, err := os.ReadFile("../testdata/judge/recorded.json")
	if err != nil {
		t.Fatalf("failed to read testdata/judge/recorded.json: %v", err)
	}

	var recFile recordedFile
	if err := json.Unmarshal(data, &recFile); err != nil {
		t.Fatalf("failed to parse recorded.json: %v", err)
	}

	cases := recFile.Records
	if len(cases) != 36 {
		t.Fatalf("expected 36 recorded cases, got %d", len(cases))
	}

	for i, c := range cases {
		tokMap := make(map[string][]int)
		for _, tok := range c.Tokenize {
			tokMap[tok.Content] = tok.Tokens
		}

		for l, id := range recFile.Letters {
			tokMap[l] = []int{id}
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/tokenize" {
				var req struct {
					Content string `json:"content"`
				}
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &req)

				tokens, ok := tokMap[req.Content]
				if !ok {
					if id, ok := recFile.Letters[req.Content]; ok {
						tokens = []int{id}
					} else {
						tokens = []int{100}
					}
				}
				resp := map[string]any{"tokens": tokens}
				json.NewEncoder(w).Encode(resp)
				return
			}

			if r.URL.Path == "/completion" {
				resp := map[string]any{
					"completion_probabilities": c.CompletionProbabilities,
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			http.NotFound(w, r)
		}))

		judge := agenteval.NewDeciderJudge(server.URL)
		q := llm.Question{
			Context: c.Case.State,
			Text:    c.Case.Question,
			Options: c.Case.Options,
		}

		dec, err := judge.Decide(nil, q)
		server.Close()

		if err != nil {
			t.Errorf("case %d (%s) failed: %v", i, c.Case.Name, err)
			continue
		}

		gotChoiceOption := c.Case.Options[dec.Choice]
		if gotChoiceOption != c.Result.Choice {
			t.Errorf("case %d (%s): Choice = %q (%d), want %q", i, c.Case.Name, gotChoiceOption, dec.Choice, c.Result.Choice)
		}

		for optIdx, optName := range c.Case.Options {
			wantProb, ok := c.Result.Probs[optName]
			if !ok {
				t.Errorf("case %d (%s): option %q not found in expected probs map", i, c.Case.Name, optName)
				continue
			}
			gotProb := dec.Probs[optIdx]
			if math.Abs(gotProb-wantProb) > 1e-3 {
				t.Errorf("case %d (%s): Probs for %q = %f, want %f (diff > 1e-3)", i, c.Case.Name, optName, gotProb, wantProb)
			}
		}
	}
}
