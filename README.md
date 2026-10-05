# agenteval
<img src="docs/img/badges.svg">

`agenteval` mide qué tan bien se comporta un agente de webtyp. Un modelo de lenguaje no responde
siempre igual, así que un test de pasa/no pasa no alcanza. Cada escenario, escrito en Go, se corre N
veces contra un modelo local, y el resultado es una **tasa de éxito**.

Lo vas a usar cuando cambies el prompt de un agente (por ejemplo, Cote en `veltylabs/mjosefa-cote`),
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
	}.Run(t, func(env agenteval.Env) (*agent.Agent, error) {
		cfg := env.Config() // el decisor, el redactor y el resto de las piezas del escenario
		cfg.Texts = cote.Texts // las palabras de la aplicación
		return agent.New(cfg)
	})
}
```

`Env` trae las dos piezas del agente híbrido: el **decisor** (decider-0.8b, que elige entre
opciones) y el **redactor** (LFM2.5-350M, que escribe respuestas a partir de datos). Al decisor se
le pregunta con el mismo texto que lee en el navegador (`qwen.DecidePrompt`), así que lo que se
mide aquí es lo que corre en la aplicación.

Para correr los escenarios se inician tres `llama-server`:

```bash
# Decisor (AGENTEVAL_DECIDER_URL)
llama-server -m ~/Dev/LMmodels/mradermacher/decider-0.8b-GGUF/decider-0.8b.Q8_0.gguf --port 8080 -np 1 -c 4096

# Redactor (AGENTEVAL_WRITER_URL)
llama-server -m ~/Dev/LMmodels/LiquidAI/LFM2.5-350M-GGUF/LFM2.5-350M-Q8_0.gguf --port 8081 --jinja -np 1 -c 4096 --temp 0.3

# Juez (AGENTEVAL_JUDGE_URL)
llama-server -m decider-4b-v2.1-Q4_K_M.gguf --port 8090 -np 1 -c 4096 -ngl 99
```

El decisor se lee a temperatura 0 (se toma la probabilidad de cada letra, con la temperatura de
decisión 1,03 que usa el navegador). El redactor varía entre intentos: cada intento usa su propia
semilla (1, 2, …), así que repetir la suite completa da el mismo resultado.

## Cómo se decide si un intento aprobó

1. **Verificadores deterministas en Go:** qué tools se llamaron y qué contiene la respuesta.
2. **Un juez local** para lo que no es una regla. Hoy es decider-4b sobre `llama-server`.

## Laboratorio: el agente en el navegador

Los escenarios miden el agente contra `llama-server`. El laboratorio lo corre **donde va a correr**:
en un Web Worker del navegador, con los modelos reales en WebAssembly, y un chat para hablarle.

| Pieza | Qué es |
|---|---|
| `ui/` | el panel de chat: botón para despertar al agente, progreso de descarga, conversación, confirmar o cancelar cambios. Sirve para cualquier agente que corra en `webtyp.com/agentworker` |
| `lab/` | los archivos de modelos del laboratorio (`ArtifactSources()`) y `Serve` |
| `lab/worker/` | el agente de prueba: un asistente de consultorio con dos herramientas (leer el horario, cambiarlo) |
| `web/client.go` | la página: `ui.New(lab.Scripts)` |
| `web/workers/agent/main.go` | el Worker: `agentworker.Serve(worker.Setup(ids))` |
| `cmd/lab` | build de release + servidor HTTPS |

Cómo correrlo:

```bash
mkdir -p models
ln ~/Dev/LMmodels/Mapika/decider-0.8b/decider-0.8b.{wtypw,merges} models/
ln ~/Dev/LMmodels/LiquidAI/LFM2.5-350M/lfm2.5-350m.{wtypw,merges} models/
go run ./cmd/lab          # build de release en web/public y https://localhost:8443
```

Abrir `https://localhost:8443` y pulsar **Despertar al agente**. La primera vez el Worker copia los
modelos (≈ 1,2 GB) del servidor a su almacenamiento propio (OPFS) y verifica su SHA-256; después
arranca desde ahí sin descargar.

Por qué `go run ./cmd/lab` y no solo `webtyp dev`: el servidor de desarrollo compila la página pero
todavía no compila los Workers (`web/workers/`), y los modelos solo se publican en el build de
release (`/artifacts.json`, `/artifacts/`). Con `webtyp dev` se ve la interfaz; para hablar con el
agente, `cmd/lab`.

Resultados de las pruebas reales: [docs/REAL_TESTS.md](docs/REAL_TESTS.md).

## Documentación

- [docs/JUDGE.md](docs/JUDGE.md): el modelo juez, cómo se le pregunta, cómo se lee y qué tan
  confiable resultó.
