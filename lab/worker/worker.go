// Package worker is the agent the laboratory runs in its Web Worker: a clinic assistant over two
// tools (the opening hours, and changing them), in Spanish. It is generic on purpose: an
// application's own agent (Cote) is tested the same way with its own Setup.
package worker

import (
	"webtyp.com/agent"
	"webtyp.com/agentworker"
	"webtyp.com/context"
	"webtyp.com/lfm"
	"webtyp.com/model"
	"webtyp.com/qwen"

	"webtyp.com/agenteval/lab"
)

// Texts are the lab agent's words. Speaker and Assistant are English: the decision model reads
// them, and its questions were measured in English.
var Texts = agent.Texts{
	Speaker:       "A staff member of a clinic",
	Assistant:     "a clinic assistant",
	NoToolOption:  "ninguna herramienta: saludo, agradecimiento u otra cosa",
	NoTool:        "¡Hola! ¿En qué te ayudo?",
	Refused:       "No puedo hacer eso.",
	TooLong:       "El mensaje es demasiado largo.",
	Clarify:       "¿A cuál de estas te refieres?",
	Confirm:       "¿Confirmas este cambio?",
	Declined:      "Listo, no cambié nada.",
	Failed:        "No pude completar la acción.",
	Yes:           "Sí.",
	No:            "No.",
	Found:         "Esto es lo que encontré:",
	WriterSystem:  "Eres el asistente de un consultorio. Responde en español, breve, solo con los datos entregados.",
	DataLabel:     "Datos del consultorio:",
	QuestionLabel: "Pregunta:",
}

// Guard refuses, before any model, the phrases that try to change the agent's rules.
var Guard = agent.Guard{Phrases: []string{"ignora tus instrucciones", "olvida tus instrucciones", "desde ahora eres", "tus instrucciones"}}

// Setup is the lab agent's agentworker.Setup: the model files of lab's manifest, and an agent with
// in-memory memory (a reload starts a new conversation) and keyword tool search.
func Setup(ids model.IDGenerator) agentworker.Setup {
	return agentworker.Setup{
		Dir: lab.Dir,
		Decider: agentworker.DeciderSpec{
			Weights: lab.DeciderWeights, Merges: lab.DeciderMerges,
			Decoder: qwen.Qwen35_08B, Temperature: lab.DecideTemperature,
		},
		Writer: &agentworker.WriterSpec{
			Weights: lab.WriterWeights, Merges: lab.WriterMerges, Decoder: lfm.LFM25_350M,
		},
		AgentConfig: func(m agentworker.Models) (agent.Config, error) {
			return Config(m, ids), nil
		},
	}
}

// Config is the lab agent over the given models.
func Config(m agentworker.Models, ids model.IDGenerator) agent.Config {
	return agent.Config{
		Decider: m.Decider, Writer: m.Writer, Tokens: m.Tokens,
		Texts: Texts, Guard: Guard,
		Memory: agent.NewMemMemory(), IDGen: ids, ToolIndex: agent.NewMemToolIndex(),
		LocalTools: Tools(),
	}
}

// HoursJSON is what the hours tool returns: the shape of business_calendar's list_business_hours.
const HoursJSON = `[{"day_of_week":1,"is_open":true,"day":"Monday","opens":"08:00","closes":"18:00"},{"day_of_week":2,"is_open":true,"day":"Tuesday","opens":"08:00","closes":"18:00"},{"day_of_week":3,"is_open":true,"day":"Wednesday","opens":"08:00","closes":"18:00"},{"day_of_week":4,"is_open":true,"day":"Thursday","opens":"08:00","closes":"18:00"},{"day_of_week":5,"is_open":true,"day":"Friday","opens":"08:00","closes":"18:00"},{"day_of_week":6,"is_open":true,"day":"Saturday","opens":"09:00","closes":"13:00"},{"day_of_week":0,"is_open":false,"day":"Sunday"}]`

// Tools are the lab agent's tools: one that reads, one that would change data (it waits for the
// person's confirmation and changes nothing real).
func Tools() []agent.Tool {
	return []agent.Tool{
		tool{name: "clinic.list_business_hours", action: model.Read, result: HoursJSON,
			description: "Horario de atención del consultorio para cada día de la semana: día (Monday … Sunday), si abre, y la hora de apertura y de cierre (HH:MM).",
			schema:      `{"type":"object","properties":{}}`},
		tool{name: "clinic.change_business_hours", action: model.Update, result: "ok",
			description: "Cambia el horario de atención de un día de la semana.",
			schema:      `{"type":"object","properties":{"day":{"type":"string"},"opens":{"type":"string"},"closes":{"type":"string"}}}`},
	}
}

type tool struct {
	name, description, schema, result string
	action                            model.Action
}

func (t tool) Name() string         { return t.name }
func (t tool) Description() string  { return t.description }
func (t tool) InputSchema() string  { return t.schema }
func (t tool) Action() model.Action { return t.action }
func (t tool) Execute(ctx *context.Context, argsJSON string) (string, error) {
	return t.result, nil
}
