//go:build !wasm

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/agent"
	"webtyp.com/agenteval"
)

// fakeLetterA is the token id the fake tokenizer gives "A": one id per byte.
const fakeLetterA = 'A'

// fakeDecider is a llama-server whose tokenizer gives one id per byte and whose next token is
// always the letter A (logprob -0.1, every other letter -2.3). prompts receives every prompt.
func fakeDecider(t *testing.T, prompts *[][]int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/tokenize":
			var body struct {
				Content    string `json:"content"`
				AddSpecial bool   `json:"add_special"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.AddSpecial {
				t.Errorf("tokenize with add_special: the browser tokenizes without special tokens")
			}
			ids := []int{}
			for _, b := range []byte(body.Content) {
				ids = append(ids, int(b))
			}
			json.NewEncoder(w).Encode(map[string]any{"tokens": ids})
		case "/completion":
			var body struct {
				Prompt []int `json:"prompt"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if prompts != nil {
				*prompts = append(*prompts, body.Prompt)
			}
			top := []map[string]any{{"id": fakeLetterA, "logprob": -0.1}}
			for l := 'B'; l <= 'J'; l++ {
				top = append(top, map[string]any{"id": int(l), "logprob": -2.3})
			}
			json.NewEncoder(w).Encode(map[string]any{"completion_probabilities": []map[string]any{{"top_logprobs": top}}})
		default:
			http.NotFound(w, r)
		}
	}))
}

// fakeWriter is a llama-server whose chat completion always answers text. requests receives
// every request body.
func fakeWriter(text string, requests *[]map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/chat/completions":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if requests != nil {
				*requests = append(*requests, body)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

// useFakeServers points the scenario at a fake decider and a fake writer for this test.
func useFakeServers(t *testing.T, answer string, writerRequests *[]map[string]any) {
	dec := fakeDecider(t, nil)
	wr := fakeWriter(answer, writerRequests)
	t.Cleanup(dec.Close)
	t.Cleanup(wr.Close)
	t.Setenv("AGENTEVAL_DECIDER_URL", dec.URL)
	t.Setenv("AGENTEVAL_WRITER_URL", wr.URL)
}

// texts are an application's words with every field filled, as agent.New requires.
func texts() agent.Texts {
	return agent.Texts{
		Speaker: "A staff member of a clinic", Assistant: "a clinic assistant",
		NoToolOption: "ninguna herramienta", NoTool: "Hola.", Refused: "No puedo.",
		TooLong: "Muy largo.", Clarify: "¿A qué te refieres?", Confirm: "¿Confirmas?",
		Declined: "Cancelado.", Failed: "Falló.", Yes: "Sí.", No: "No.", Found: "Datos:",
		WriterSystem: "Eres amable.", DataLabel: "Datos:", QuestionLabel: "Pregunta:",
	}
}

func buildWithTexts(env agenteval.Env) (*agent.Agent, error) {
	cfg := env.Config()
	cfg.Texts = texts()
	return agent.New(cfg)
}
