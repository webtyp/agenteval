Los tres planes quedaron publicados. La cadena de 4 bits está completa y el demonio ya está listo para correr el agente con `webtyp dev`.

| Repo | Cómo vino | Qué hice | Versión |
|---|---|---|---|
| `decoder` | bien: matrices de 4 bits, pruebas con los modelos chicos de Qwen y LFM2, sin asignaciones de memoria en `Step` | verifiqué que las pruebas comparan de verdad; verde también con TinyGo | **v0.6.0** |
| `weightsc` | otra vez sin implementación, solo 3 archivos sueltos de prueba | la opción `-quant int4-block32` (una matriz con filas que no son múltiplo de 32 queda en int8), pruebas y README | **v0.3.0** |
| `app` | Jules se trabó al bajar dependencias | lo ejecuté en local | **v0.6.113** |

**Por qué Jules no pudo con `app`:** `webtyp/depfind` es un repo **privado** y `app` depende de él. Ninguna sesión de Jules en `app` va a poder hacer `go get`; esos planes van en local, o hay que hacer público `depfind`. Lo dejé anotado en el master del agente.

**Lo que hace ahora `app`:**
- **Workers:** el demonio compila `web/workers/<name>` en segundo plano después del escaneo SSR. Cuando cambia un archivo `.go` de un paquete que el Worker importa, lo recompila (si llegan varios cambios seguidos, hace una sola recompilación) y recarga el navegador.
- **Perfil del navegador:** Chrome usa un perfil fijo por proyecto, así lo que se copió a OPFS sobrevive entre sesiones.
- **Pruebas:** agregué las del perfil y las de recompilación, y la suite completa está verde.

**Pendiente:** `webtyp dev` con el agente todavía no lo probé en vivo. El binario `webtyp` ya se reinstaló, pero tu demonio sigue corriendo la versión anterior. **Si lo reinicias desde `agenteval`, hago la prueba:**
1. Se ve la interfaz.
2. El Worker se compila solo.
3. `/artifacts.json` y los modelos se sirven desde `models/`.
4. Al cerrar y volver a abrir, el agente arranca sin repetir la copia de 1,2 GB.

**Lo que falta, en orden:**
1. Esa prueba en vivo de `webtyp dev` en `agenteval`.
2. **Medir 4 bits:** convertir decider-0.8b y LFM2.5-350M con `-quant int4-block32`. Decider tiene que seguir en ≥32 de 36 preguntas, y hay que volver a medir la memoria de la pestaña (hoy 2,9 GB).
3. El cuelgue del primer arranque (`call to released function`). Está en una librería, así que va con plan.
4. Velocidad: segundo arranque con la caché de decisiones guardada, y plano contra SIMD.
5. Diseño de los argumentos de texto (D2): hoy cada argumento de texto recibe el mensaje completo.
6. Cote en el laboratorio, y después sus escenarios (`mjosefa-cote/docs/PLAN.md`).
7. Cote en `mjosefa-cms`.