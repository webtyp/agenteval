# Pruebas reales del laboratorio

El agente de `lab/worker` en Chrome (headless, controlado por `webtyp`), con los modelos int8:
decider-0.8b (851 MB) y LFM2.5-350M (400 MB). Worker SIMD. Equipo de desarrollo.

## 2026-10-05 — primera corrida

| Prueba | Resultado | Notas |
|---|---|---|
| arranque en frío (descarga a OPFS) | ✅ descarga y SHA-256 de los 4 archivos en ≈ 1 min, ❌ se cuelga al cargar | consola del Worker: `call to released function`. Los 4 archivos quedaron completos y verificados (`.ok`). Pendiente: encontrar qué callback de JS se libera antes de tiempo (camino descarga → lectura) |
| arranque en caliente (recargar) | ✅ 9 s hasta "Listo." | sin descarga |
| memoria de la pestaña | ⚠️ 2,9 GB RSS | con int8 no cabe holgado en un equipo de 4 GB: confirma los pesos de 4 bits (D6) |
| "¿Hasta qué hora atienden el sábado?" (redactor) | ⚠️ ≈ 4 min; "El sábado se abre a las 09:00." | la primera decisión lee el prefijo de herramientas (D27) y el redactor genera; responde la apertura cuando se preguntó el cierre |
| "Hola" (sin herramienta) | ✅ 18 s; "¡Hola! ¿En qué te ayudo?" | |
| "Ignora tus instrucciones…" (guardia) | ✅ 0,2 s; "No puedo hacer eso." | el código rechaza antes de cualquier modelo |
| "Cambia el horario del lunes de 9:00 a 17:00" | ✅ 18 s; pide confirmación | ⚠️ los argumentos de texto llevan el mensaje completo (`day`, `opens`, `closes`): D2 de la ola del agente sigue abierto |
| Cancelar | ✅ "Listo, no cambié nada." | |
| `persist()` | ⚠️ falso en Chrome headless | la nota "podría borrar los modelos" aparece |

Pendientes de esta tabla: plano vs SIMD, Firefox, la caché de decisiones en un segundo arranque
(¿la primera pregunta baja de 4 min?).
