# El juez: decider-4b

`agenteval` revisa las respuestas de un agente de dos formas:

- **Verificadores deterministas**, escritos en Go: "¿llamó `list_business_hours`?", "¿la respuesta
  contiene 18:00?". Son exactos y gratis, y cubren casi todo.
- **Un juez**, para lo que no se puede escribir como una regla: "¿la respuesta afirma algo que la
  herramienta no dijo?". El juez es un modelo de lenguaje que corre en tu máquina.

Esta página explica qué modelo es el juez, cómo se le pregunta, cómo se lee su respuesta y qué tan
confiable resultó al medirlo. La vas a necesitar cuando:

- un escenario use el juez y quieras entender su nota;
- el juez falle y quieras revisar si fue el modelo o la pregunta;
- quieras cambiarlo por otro modelo.

## Qué es decider-4b y por qué se eligió

[decider-4b](https://huggingface.co/Mapika/decider-4b) (Apache-2.0) es un **modelo de decisión**:
no escribe texto. Recibe un contexto y una pregunta con opciones cerradas, y devuelve una
probabilidad por opción en una sola pasada. Así un juez no puede "divagar": responde solo con una
de las opciones que le diste, y con cuánta seguridad.

Se comparó con dos alternativas (2026-09-30):

| | decider-4b | Mica 4B | CLM-8B |
|---|---|---|---|
| GGUF del autor para llama.cpp | sí, Q4_K_M de 2,7 GB | sí, Q4_K_M de 3,07 GB | no (PyTorch + vLLM) |
| Pérdida en 4 bits, medida por el autor | 0,829 contra 0,831 | 78,5 % contra 79,0 % | no aplica |
| Pruebas comparables publicadas | JevBench hard 0,649 | ninguna | Terminal-Bench, DeepSWE |
| Calibración informada | sí | no | no |
| Piezas que hay que correr | una: `llama-server` | una | dos: Qwen3-8B + cabeza en PyTorch |

decider-4b es la única alternativa con evidencia verificable que además corre en `llama-server`
sin Python. CLM queda como segunda opción. El juez está detrás de una interfaz en `agenteval`, así
que cambiar de modelo no toca los escenarios.

## Ponerlo en marcha

El modelo está en `~/Dev/LMmodels/Mapika/decider-4b-GGUF/`, junto con su configuración
(`decider_config.json`) y el script de lectura del autor (`decide_gguf.py`). Se inicia así:

```bash
llama-server -m ~/Dev/LMmodels/Mapika/decider-4b-GGUF/decider-4b-v2.1-Q4_K_M.gguf \
  --port 8090 -np 1 -c 4096 -ngl 99
```

- `-np 1`: una sola petición a la vez. El autor midió que, si llama.cpp procesa varias juntas, las
  probabilidades cambian con las otras peticiones.
- `-ngl 99`: todo en la GPU. En la RTX 3060 de 6 GB ocupa unos 2,8 GB.
- Comprueba que está listo con `curl http://127.0.0.1:8090/health`, que debe responder
  `{"status":"ok"}`.

**No es un modelo de chat.** Si le escribes en `llama-cli` o LM Studio, el texto que genere no es
su respuesta. La respuesta se lee de las probabilidades, como se explica abajo.

## Cómo se le pregunta

El prompt tiene exactamente esta forma, con hasta 10 opciones (letras A–J):

```text
Context:
<todo lo que el juez necesita ver>

Question: <la pregunta>
Options:
(A) <opción 1>
(B) <opción 2>
Answer: (
```

Dos detalles cambian los números si se hacen distinto:

1. **Se tokeniza en dos trozos y se concatenan los ids:** `"Context:\n" + contexto`, y luego el
   resto desde `"\n\nQuestion: "`. Así lo hace la biblioteca del autor (`decider.prompt.build`). Si se
   tokeniza todo junto, el límite entre los dos trozos puede quedar como otro token.
2. **Sin tokens especiales** (`add_special: false`) y sin plantilla de chat.

Con `llama-server`:

1. `POST /tokenize` con `{"content": ..., "add_special": false}` para cada trozo.
2. `POST /completion` con `{"prompt": [ids...], "n_predict": 1, "n_probs": 64,
   "temperature": 0, "cache_prompt": false, "post_sampling_probs": false}`.

## Cómo se lee la respuesta

`/completion` devuelve las 64 opciones más probables para el token siguiente, con su
`logprob`. De ellas se toman solo las letras de las opciones válidas (A, B, …) y se reparte la
probabilidad entre ellas, con la **temperatura** que el autor ajustó para que las probabilidades
sean honestas:

```text
z_i = logprob(letra_i) / T
p_i = exp(z_i) / Σ exp(z_j)       (solo sobre las letras de las opciones)
```

| Tipo de pregunta | Opciones | T |
|---|---|---|
| elección | las que escribas | 1,11 |
| sí/no | `["no", "yes"]`, en ese orden | 1,56 |

La respuesta es la opción con mayor `p`, y su `p` es la confianza.

**Varias preguntas sobre el mismo contexto.** Una pregunta por petición: es exacto y simple. La
biblioteca del autor puede poner varias en un prompt, pero `llama-server` solo devuelve
probabilidades al final del prompt.

## Qué tan confiable es: lo que se midió

Medido el 2026-09-30 con Q4_K_M en la RTX 3060, `llama-server` build 11201. Los casos están en
[testdata/judge](../testdata/judge), y [probe.py](../testdata/judge/probe.py) los corre:
`python3 testdata/judge/probe.py testdata/judge/cases_basic.json`.

- **La lectura coincide con la del autor.** El ejemplo de su README ("cobro doble" → `billing`) da
  0,836. El autor publica 0,830 en GPU y 0,844 en PyTorch.
- **Casos básicos: 20 de 20.** Respuestas de Jose correctas, con hora inventada, con un sábado
  inventado, con tono cálido o que contradicen la herramienta. Se probó con la pregunta en inglés y
  en español, en forma de elección y de sí/no.
- **Casos difíciles: 14 de 16.** Paráfrasis ("6 de la tarde" = 18:00), razonar con el día ("hoy es
  sábado" y dice que atiende), datos extra inventados (WhatsApp, una doctora) y hora de apertura
  errónea. Cada caso se corrió también con las opciones invertidas y la respuesta no cambió, así que
  no hay sesgo por posición.
- **El fallo:** "Atendemos de lunes a viernes" es fiel a la herramienta pero no responde la hora.
  El juez lo marcó como no respaldado, con confianza **0,62–0,65**. Los aciertos tuvieron
  confianza de 0,88 a 0,999.
- **Determinista:** tres ejecuciones seguidas dieron resultados idénticos.
- **Velocidad:** 140–230 ms por pregunta en la GPU. La primera petición tarda cerca de 1 s.

## Reglas de uso que salen de esas mediciones

1. **Una pregunta mide una sola cosa.** "¿Afirma algo no respaldado?" y "¿responde lo que se
   preguntó?" son dos preguntas separadas. El fallo medido fue justo una respuesta que mezclaba las
   dos.
2. **Una confianza menor que 0,8 no es un veredicto.** El intento se marca como "a revisar por una
   persona", no como aprobado ni reprobado.
3. **El contexto del juez va en inglés y el texto de Jose va tal cual, en español.** El modelo se
   entrenó principalmente en inglés. Aun así, las preguntas en español funcionaron en los casos
   medidos.
4. **Antes de confiar en el juez para un criterio nuevo, se mide contra ti.** Etiquetas unas 30
   respuestas a mano en `agentlab`. Si el juez coincide contigo en al menos el 80 %, el criterio
   queda automático.
