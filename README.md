# agenteval
<img src="docs/img/badges.svg">

`agenteval` mide qué tan bien se comporta un agente de webtyp. Un modelo de lenguaje no responde
siempre igual, así que un test de pasa/no pasa no alcanza. Cada escenario, escrito en Go, se corre N
veces contra un modelo local, y el resultado es una **tasa de éxito**.

Lo vas a usar cuando cambies el prompt de un agente (por ejemplo, Jose en `veltylabs/mjosefa-jose`),
su modelo o su manejo de contexto, y quieras saber si mejoró o empeoró sin revisar las respuestas
una por una.

> **STATUS (borrar esta nota cuando exista el primer código):** por ahora hay solo documentación y
> los datos de verificación del juez. La API de escenarios se define en el primer plan.

## Cómo se decide si un intento aprobó

1. **Verificadores deterministas en Go:** qué tools se llamaron y qué contiene la respuesta.
2. **Un juez local** para lo que no es una regla. Hoy es decider-4b sobre `llama-server`.

## Documentación

- [docs/JUDGE.md](docs/JUDGE.md): el modelo juez, cómo se le pregunta, cómo se lee y qué tan
  confiable resultó.
