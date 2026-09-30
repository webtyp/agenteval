package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/agent"
	"webtyp.com/agentcontext"
	"webtyp.com/agenteval"
	"webtyp.com/model"
)

// Qwen's chat template accepts one system block, at the start (llama-server answers 500
// "System message must be at the beginning" otherwise). The agent adds system messages later in
// the conversation (the critic's retry note, summaries), so the client folds every system
// message into the first one.
func TestModelClient_SendsOneSystemMessageFirst(t *testing.T) {
	chats := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/props":
			json.NewEncoder(w).Encode(map[string]any{"default_generation_settings": map[string]any{"n_ctx": 4096}})
		case "/apply-template":
			json.NewEncoder(w).Encode(map[string]any{"prompt": "<critic prompt>"})
		case "/tokenize":
			var body struct {
				Content string `json:"content"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := 100
			if len(body.Content) == 1 {
				id = int(body.Content[0]-'A') + 1 // A=1, B=2, …
			}
			json.NewEncoder(w).Encode(map[string]any{"tokens": []int{id}})
		case "/completion":
			// The critic is sure the answer is not supported (option B): the agent retries once.
			json.NewEncoder(w).Encode(map[string]any{"completion_probabilities": []map[string]any{{
				"top_logprobs": []map[string]any{{"id": 2, "logprob": 0.0}, {"id": 1, "logprob": -9.0}},
			}}})
		case "/v1/chat/completions":
			chats++
			var body struct {
				Messages []struct {
					Role string `json:"role"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for i, m := range body.Messages {
				if m.Role == "system" && i != 0 {
					t.Errorf("chat request %d has a system message at position %d", chats, i)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"content": "Hasta las 18:00."}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
			})
		}
	}))
	defer server.Close()
	t.Setenv("AGENTEVAL_MODEL_URL", server.URL)

	agenteval.Scenario{
		Given: agenteval.Given{
			At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{Name: "list_business_hours", InputSchema: `{"type":"object","properties":{}}`,
				Action: model.Read, Returns: "08:00-18:00"}},
		},
		When:    "¿Hasta qué hora atendemos hoy?",
		Then:    []agenteval.Check{agenteval.Contains("18:00")},
		Runs:    1,
		MinPass: 1,
	}.Run(t, func(env agenteval.Env) (*agent.Agent, error) {
		cfg := env.Config()
		cfg.Identity = agentcontext.Identity{Name: "Jose"}
		return agent.New(cfg)
	})
	if chats < 2 {
		t.Fatalf("the critic's rejection must cause a retry, got %d chat requests", chats)
	}
}
