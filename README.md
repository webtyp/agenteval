# agenteval
<img src="docs/img/badges.svg">

`agenteval` mide qué tan bien se comporta un agente de webtyp. Un modelo de lenguaje no responde
siempre igual, así que un test de pasa/no pasa no alcanza. Cada escenario, escrito en Go, se corre N
veces contra un modelo local, y el resultado es una **tasa de éxito**.

Lo vas a usar cuando cambies el prompt de un agente (por ejemplo, Jose en `veltylabs/mjosefa-jose`),
su modelo o su manejo de contexto, y quieras saber si mejoró o empeoró sin revisar las respuestas
una por una.

## Escribir un escenario

```go
//go:build eval

package evals

// Caso de uso: un funcionario pregunta hasta qué hora se atiende hoy.
func TestHorarioDeHoy(t *testing.T) {
	agenteval.Scenario{
		Given: agenteval.Given{
			At:    agenteval.Moment{Year: 2026, Month: 9, Day: 29, Hour: 10, UTCOffsetMinutes: -180},
			Tools: []agenteval.FakeTool{{
				Name:        "list_business_hours",
				Description: "Opening hours of the clinic for every day of the week.",
				InputSchema: `{"type":"object","properties":{}}`,
				Action:      model.Read,
				Returns:     "Monday to Friday 08:00-18:00. Saturday and Sunday closed.",
			}},
		},
		When: "¿Hasta qué hora atendemos hoy?",
		Then: []agenteval.Check{
			agenteval.Calls("list_business_hours"),
			agenteval.DoesNotModify(),
			agenteval.Contains("18:00"),
			agenteval.Faithful(),
			agenteval.AsksNothingToConfirm(),
		},
		Runs: 10, MinPass: 9,
	}.Run(t, jose.New)
}
```

Para correr los escenarios se inician el modelo y el juez en `llama-server`:

```bash
# Modelo bajo prueba
llama-server -m <model.gguf> --port 8080 --jinja -c 4096 --reasoning-budget 0 --temp 0.7 --top-p 0.8 --top-k 20 --min-p 0

# Juez
llama-server -m decider-4b-v2.1-Q4_K_M.gguf --port 8090 -np 1 -c 4096 -ngl 99
```

Los parámetros de muestreo (`--temp`, `--top-p`, `--top-k`, `--min-p`) son los que Qwen3.5
recomienda para responder sin razonamiento; otro modelo usa los suyos. **No se usa temperatura
0:** con 0, los N intentos serían el mismo intento repetido y la tasa de éxito no mediría nada.
Cada intento usa su propia semilla (1, 2, …), así que los intentos varían entre sí y repetir la
suite completa da el mismo resultado.

## Cómo se decide si un intento aprobó

1. **Verificadores deterministas en Go:** qué tools se llamaron y qué contiene la respuesta.
2. **Un juez local** para lo que no es una regla. Hoy es decider-4b sobre `llama-server`.

## Documentación

- [docs/JUDGE.md](docs/JUDGE.md): el modelo juez, cómo se le pregunta, cómo se lee y qué tan
  confiable resultó.
