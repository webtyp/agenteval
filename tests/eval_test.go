//go:build eval

package tests

import (
	"testing"

	"webtyp.com/agent"
	"webtyp.com/agentcontext"
	"webtyp.com/agenteval"
	"webtyp.com/model"
)

// TestIntegration_ClinicHours is an end-to-end scenario evaluation matching TestIntegration_ClinicHours.
// Note: Qwen3.5-0.8B reaches the tool only about half of the time through search_tools, so MinPass 1
// only proves the pipeline works end to end.
func TestIntegration_ClinicHours(t *testing.T) {
	agenteval.Scenario{
		Given: agenteval.Given{
			At: agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{
				Name:        "clinic_hours",
				Description: "Opening hours of the clinic (horario de atención de la clínica) for each day of the week",
				InputSchema: `{"type":"object","properties":{"day":{"type":"string","description":"day of the week"}}}`,
				Action:      model.Read,
				Returns:     "Open Monday to Friday, 8:00 to 17:00. Closed on weekends.",
			}},
		},
		When: "What time does the clinic open on Monday?",
		Then: []agenteval.Check{
			agenteval.Calls("clinic_hours"),
			agenteval.Contains("8"),
			agenteval.Faithful(),
		},
		Runs:    10,
		MinPass: 1,
	}.Run(t, func(env agenteval.Env) (*agent.Agent, error) {
		cfg := env.Config()
		cfg.Identity = agentcontext.Identity{
			Name:         "Recepcionista",
			Role:         "Recepcionista de Clínica San Miguel",
			Instructions: "Responde siempre en español, de forma concisa. Nunca inventes datos de la clínica (horarios, precios, citas): consíguelos siempre con una herramienta. Si no tienes la herramienta adecuada, llama primero a search_tools.",
			Goals:        []string{"Informar horarios", "Gestionar citas"},
		}
		return agent.New(cfg)
	})
}
