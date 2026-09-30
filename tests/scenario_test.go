//go:build !wasm

package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/agent"
	"webtyp.com/agentcontext"
	"webtyp.com/agenteval"
	"webtyp.com/model"
)

func TestScenarioValidation(t *testing.T) {
	validGiven := agenteval.Given{
		At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
		Tools: []agenteval.FakeTool{
			{
				Name:        "list_business_hours",
				Description: "Hours",
				InputSchema: `{"type":"object"}`,
				Action:      model.ActionRead,
				Returns:     "08:00-18:00",
			},
		},
	}
	validCheck := []agenteval.Check{agenteval.Contains("18:00")}

	tests := []struct {
		name    string
		scen    agenteval.Scenario
		wantErr string
	}{
		{
			name: "Given.At required",
			scen: agenteval.Scenario{
				Given:   agenteval.Given{},
				When:    "test",
				Then:    validCheck,
				Runs:    10,
				MinPass: 9,
			},
			wantErr: "agenteval: Given.At is required",
		},
		{
			name: "MinPass required",
			scen: agenteval.Scenario{
				Given:   validGiven,
				When:    "test",
				Then:    validCheck,
				Runs:    10,
				MinPass: 0,
			},
			wantErr: "agenteval: MinPass is required (1..Runs)",
		},
		{
			name: "MinPass greater than Runs",
			scen: agenteval.Scenario{
				Given:   validGiven,
				When:    "test",
				Then:    validCheck,
				Runs:    5,
				MinPass: 9,
			},
			wantErr: "agenteval: MinPass 9 is greater than Runs 5",
		},
		{
			name: "When required",
			scen: agenteval.Scenario{
				Given:   validGiven,
				When:    "",
				Then:    validCheck,
				Runs:    10,
				MinPass: 9,
			},
			wantErr: "agenteval: When is required",
		},
		{
			name: "Then check required",
			scen: agenteval.Scenario{
				Given:   validGiven,
				When:    "test",
				Then:    nil,
				Runs:    10,
				MinPass: 9,
			},
			wantErr: "agenteval: Then needs at least one check",
		},
		{
			name: "FakeTool needs Name",
			scen: agenteval.Scenario{
				Given: agenteval.Given{
					At: validGiven.At,
					Tools: []agenteval.FakeTool{
						{Name: "", Action: model.ActionRead},
					},
				},
				When:    "test",
				Then:    validCheck,
				Runs:    10,
				MinPass: 9,
			},
			wantErr: "agenteval: FakeTool needs a Name",
		},
		{
			name: "FakeTool needs Action",
			scen: agenteval.Scenario{
				Given: agenteval.Given{
					At: validGiven.At,
					Tools: []agenteval.FakeTool{
						{Name: "tool1", Action: 0},
					},
				},
				When:    "test",
				Then:    validCheck,
				Runs:    10,
				MinPass: 9,
			},
			wantErr: `agenteval: FakeTool "tool1" needs an Action (model.ActionRead, ActionCreate, ActionUpdate or ActionDelete)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.scen.Validate()
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestScenarioExecutionWithFakeServer(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/props":
			resp := map[string]any{
				"default_generation_settings": map[string]any{
					"n_ctx": 4096,
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/tokenize":
			resp := map[string]any{
				"tokens": []int{1, 2, 3},
			}
			json.NewEncoder(w).Encode(resp)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)

			if bytesContains(bodyStr, "critic") {
				resp := map[string]any{
					"choices": []map[string]any{
						{
							"message": map[string]any{
								"content": "SUFFICIENT",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			callCount++
			switch callCount {
			case 1:
				// Step 1: Call search_tools to discover list_business_hours
				resp := map[string]any{
					"choices": []map[string]any{
						{
							"message": map[string]any{
								"tool_calls": []map[string]any{
									{
										"id":   "call_search",
										"type": "function",
										"function": map[string]any{
											"name":      "search_tools",
											"arguments": `{"query":"business hours"}`,
										},
									},
								},
							},
							"finish_reason": "tool_calls",
						},
					},
					"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
				}
				json.NewEncoder(w).Encode(resp)
			case 2:
				// Step 2: Call list_business_hours
				resp := map[string]any{
					"choices": []map[string]any{
						{
							"message": map[string]any{
								"tool_calls": []map[string]any{
									{
										"id":   "call_hours",
										"type": "function",
										"function": map[string]any{
											"name":      "list_business_hours",
											"arguments": "{}",
										},
									},
								},
							},
							"finish_reason": "tool_calls",
						},
					},
					"usage": map[string]any{"prompt_tokens": 15, "completion_tokens": 5},
				}
				json.NewEncoder(w).Encode(resp)
			default:
				// Step 3: Return final answer
				resp := map[string]any{
					"choices": []map[string]any{
						{
							"message": map[string]any{
								"content": "Hasta las 18:00.",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]any{"prompt_tokens": 20, "completion_tokens": 10},
				}
				json.NewEncoder(w).Encode(resp)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("AGENTEVAL_MODEL_URL", server.URL)

	scen := agenteval.Scenario{
		Given: agenteval.Given{
			At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{
				{
					Name:        "list_business_hours",
					Description: "Opening hours of the clinic for every day of the week.",
					InputSchema: `{"type":"object","properties":{}}`,
					Action:      model.ActionRead,
					Returns:     "Monday to Friday 08:00-18:00.",
				},
			},
		},
		When: "¿Hasta qué hora atendemos hoy?",
		Then: []agenteval.Check{
			agenteval.Calls("list_business_hours"),
			agenteval.DoesNotModify(),
			agenteval.Contains("18:00"),
		},
		Runs:    1,
		MinPass: 1,
	}

	scen.Run(t, func(env agenteval.Env) (*agent.Agent, error) {
		cfg := agent.Config{
			Identity: agentcontext.Identity{
				Name:         "Jose",
				Role:         "Recepcionista",
				Instructions: "Responde de forma concisa.",
			},
			LLMs: agent.LLMConfig{
				Primary: env.Model,
			},
			Tokens:     env.Tokens,
			Budget:     env.Budget,
			Clock:      env.Clock,
			Memory:     env.Memory,
			IDGen:      env.IDGen,
			ToolIndex:  env.ToolIndex,
			LocalTools: env.Tools,
		}
		return agent.New(cfg)
	})
}

func bytesContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
