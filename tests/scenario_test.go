//go:build !wasm

package tests

import (
	"testing"

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
				Action:      model.Read,
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
						{Name: "", Action: model.Read},
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
			wantErr: `agenteval: FakeTool "tool1" needs an Action (model.Read, model.Create, model.Update, model.Delete or a combination)`,
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
	useFakeServers(t, "Hoy atendemos hasta las 18:00.", nil)

	agenteval.Scenario{
		Given: agenteval.Given{
			At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{
				Name:        "list_business_hours",
				Description: "Opening hours of the clinic for every day of the week.",
				InputSchema: `{"type":"object","properties":{}}`,
				Action:      model.Read,
				Returns:     "Monday to Friday 08:00-18:00.",
			}},
		},
		When: "¿Hasta qué hora atendemos hoy?",
		Then: []agenteval.Check{
			agenteval.Calls("list_business_hours"),
			agenteval.DoesNotModify(),
			agenteval.Contains("18:00"),
			agenteval.AsksNothingToConfirm(),
		},
		Runs:    1,
		MinPass: 1,
	}.Run(t, buildWithTexts)
}

func TestScenarioPendingCallWithFakeServer(t *testing.T) {
	useFakeServers(t, "unused", nil)

	agenteval.Scenario{
		Given: agenteval.Given{
			At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{
				Name:        "create_appointment",
				Description: "Create a new clinic appointment.",
				InputSchema: `{"type":"object","properties":{"time":{"type":"string"}}}`,
				Action:      model.Create,
				Returns:     "appointment created",
			}},
		},
		When: "Quiero agendar una cita para las 10:00",
		Then: []agenteval.Check{
			agenteval.AsksToConfirm("create_appointment"),
			agenteval.DoesNotModify(),
		},
		Runs:    1,
		MinPass: 1,
	}.Run(t, buildWithTexts)
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
