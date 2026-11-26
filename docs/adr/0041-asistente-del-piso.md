# ADR-0041 · El asistente del piso

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-11 y V-13, 24-nov-2026); Isaac revisa en el PR
- **Fecha:** 2026-11-25 (R8)
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Relacionado con:** HU-107 (nueva), HU-17, HU-18, HU-20, HU-27, HU-64, HU-87, HU-103, HU-106, ADR-0026, ADR-0029, ADR-0033, ADR-0040, R-07, R-08, RF-17

## Contexto
Sin WhatsApp (V-01), la IA del producto pasa al piso del taller: el mecánico **le habla, le escribe o le manda fotos** a un asistente para recibir el vehículo, anotar el diagnóstico, cotizar y anotar gastos sin llenar formularios. Pablo pidió (V-11):
- empezar en un chat general;
- seguir dentro de la orden cuando hay placa;
- una **ficha en vivo** con "N de M datos · falta…";
- un **Confirmar** que guarda todo.

V-13 limita la IA a **modelos gratuitos** de OpenRouter, con Groq para la voz.

## Decisión

### Un agente interno, distinto del asesor
- Vive en el módulo `assistant` (`app.Floor`), junto al asesor de WhatsApp. Reutiliza:
  - su puerto de modelo (`Advisor`, con la cadena de respaldo de OpenRouter);
  - el bucle de tools (`Converse`);
  - el **tope mensual** y el **registro de cada llamada** en `llamada_ia`, con propósito `piso` (ADR-0026, ADR-0029);
  - la transcripción de Groq (ADR-0033).
- Tiene **su propio prompt y sus 8 tools**: `buscar_vehiculo`, `crear_orden`, `proponer_ficha`, `proponer_recepcion`, `proponer_hallazgo`, `proponer_cotizacion`, `anotar_gasto` y `mover_estado`.
  - **Ninguna tool escribe en la orden.** Todas cambian solo la propuesta de la conversación y devuelven datos reales del taller.
- Sus modelos son `AI_PISO_MODELS`; si está vacía, usa `AI_MODEL`. `config` **no arranca** si algún modelo de `AI_PISO_MODELS` no termina en `:free` (V-13).
  - Para las fotos, el adaptador de OpenRouter manda el mensaje como partes `text` + `image_url` en base64.
  - La cadena debe aceptar imágenes. Las de la entrega sí: Nemotron 3.5 Super VL y Gemma 4.

### La IA propone, el código valida, la persona confirma (R-08, RF-17)
- **Un precio solo sale de dos lugares:**
  - la **lista del taller** (si el servicio tiene precio, manda la lista aunque el modelo diga otro);
  - una cifra que la persona **escribió o dijo literalmente** en la conversación (`domain.SaidAmounts`: "$120.000", "120000", "120 mil").

  Lo demás queda en `null` y la ficha dice **"Sin precio · ponlo"** (`domain.PriceLine`). Una cifra leída en una foto **no** cuenta como dicha.
- **El valor de un gasto también tiene que estar dicho**, o la persona marcó que sale de la factura de una foto de ese mensaje.
- **Los otros datos tampoco se inventan:**
  - el celular y el nombre del cliente solo valen si están en lo que dijo la persona;
  - la placa, si la dijo o se ve en una foto del mensaje;
  - el kilometraje, si lo dijo o se lee en una foto.
- **Si el texto que el asistente muestra trae una cifra de plata** (con $, "95 mil" o "95.000"; no cuenta el kilometraje) que no es de la lista ni la dijo la persona, se cambia por un texto fijo ("Te dejé la propuesta en la ficha…").
- **El texto de la persona va delimitado (`<<< >>>`)**: es un dato, no una instrucción. Lo que se lea en una foto tampoco es instrucción.
- **Nada de lo que viene de la persona va en el prompt del sistema.** La propuesta actual (con el síntoma, las notas, el cliente) le llega al modelo como resultado de la tool `ver_ficha`, que el código consulta antes de cada mensaje; el prompt del sistema solo lleva datos del taller (nombre, servicios con su precio, ítems del checklist, el código y el estado de la orden). Lo encontró el revisor en la primera ronda.
- **La IA no diagnostica.** El prompt solo le deja anotar hallazgos que el mecánico dijo o mostró. Las pruebas adversariales lo miden.

### Conversación y propuesta (migración 00036)
- **`assistant.chat_piso`** guarda:
  - la conversación;
  - la orden, si ya la tiene;
  - la **propuesta pendiente** en jsonb;
  - una **versión optimista** (R-07).

  **`assistant.chat_piso_mensaje`** guarda los mensajes, con las fotos como referencias al almacenamiento y la **voz solo como transcripción**: el audio no se guarda (minimización). **`assistant.chat_piso_confirmacion`** guarda cada `Idempotency-Key`.

  Las tres tablas tienen RLS forzado y la prueba con dos talleres. No hay DELETE: la conversación queda como historia de la orden.
- **Cuando aparece una placa, la busca el código antes de llamar al modelo** (como una llamada a `buscar_vehiculo` con su resultado, no en el prompt), si es la única del mensaje y la conversación todavía no tiene vehículo. Así la conversación pasa a la orden abierta aunque el modelo olvide la tool, y se ahorra una ronda. (Lo pedía la medición: el modelo a veces no buscaba la placa.)
- **Con dos placas**, el código pregunta "¿Seguimos con ABC123 o con XYZ98D?" con dos botones, sin gastar IA.
- **Placa con orden abierta:** la conversación se asocia a esa orden y nunca se crea otra.
- **Placa sin orden:** se propone una orden nueva "Por recibir". Si el vehículo es nuevo, también el cliente (celular y nombre) y el tipo.
- **Ficha en vivo:** "7 de 8 datos · falta la parte trasera", con placa, cliente (si es nuevo), qué trae, kilometraje, combustible y una foto por lado (4 en carro, 5 en moto). Lo que ya está guardado en la orden cuenta como hecho. Cada dato muestra su origen ("lo dijiste", "de la foto", "de tu lista", "lo pusiste", "ya estaba", HU-64) y se puede quitar.
- **Lo que la persona edita** (`PATCH`) queda con origen "lo pusiste": un precio puesto por ella vale. Lo que identifica la orden no se edita desde la ficha y no se pueden agregar fotos que no mandó (`domain.HumanEdit`).

### Confirmar
- **`POST /assistant/floor/chats/{id}/confirm`** pide `Idempotency-Key` y la versión. En **una sola transacción** aplica la propuesta por los casos de uso que ya existen, con sus reglas, su bitácora y sus eventos, en este orden:
  1. cliente y vehículo (si son nuevos) y orden;
  2. ficha;
  3. fotos como evidencias por lado;
  4. recepción;
  5. cambio de estado;
  6. hallazgos;
  7. cotización;
  8. gastos.

  El orden importa: Recibido exige las fotos y la cotización exige diagnóstico o reparación.
- Escribe además `asistente.confirmar` en la bitácora, con `origen: asistente`, quien confirmó y lo hecho.
- **Repetir con la misma clave devuelve el mismo resultado.** Dos toques a la vez: el segundo espera al primero por la llave de la confirmación y recibe ese resultado (probado con 6 a la vez). La misma clave en **otra** conversación responde 409 `clave_usada`, no el resultado de la primera.
- **No se confirma** con ítems sin precio (422 `precio_pendiente`), con un vehículo nuevo sin cliente o sin tipo, ni con la versión vieja (409).
- **Permisos (R-05):** las rutas piden `assistant:use` (los 3 roles). Al confirmar, el servidor revisa además el permiso de cada parte: `order:create`, `vehicle:update`, `evidence:upload`, `order:diagnose`, `quote:edit`, `expense:manage` y `order:update_state`.
- **Las fotos se pasan a la orden dentro de la transacción.** Ya están en el almacenamiento desde que se mandaron: se leen y se guardan de nuevo como evidencias. Es I/O dentro de la transacción, acotado a pocas fotos; se eligió así porque pasar a Recibido las exige en la misma confirmación.

### Si la IA falla o se acaba el cupo
- Sin modelo, con el tope agotado, con un error o si pasan 60 s, el chat dice **"El asistente no está disponible, sigue con el formulario"** y la propuesta queda como estaba: nada a medias (RNF-13).
- Los formularios de recepción, diagnóstico, cotización y gastos siguen iguales.

### Datos personales
- Al borrar los datos de un titular (HU-87), se borran los textos, las fotos y la propuesta de las conversaciones del piso de sus órdenes.
- Los logs no llevan el texto de los mensajes ni el celular (probado).
- **Límites (para AQ-10, igual que los de HU-87):** al anonimizar se vacían los adjuntos del chat, pero los archivos de las fotos siguen en el almacenamiento (el almacenamiento no tiene borrado todavía); la copia de datos del titular (`/customers/{id}/export`) no trae las conversaciones del piso; no hay retención para los chats generales sin orden ni para las fotos que nunca se confirmaron.

## Alternativas descartadas
- **Un módulo `floor` aparte:** obligaba a duplicar el registro de llamadas, el tope y el puerto del modelo, o a cruzar módulos. El asistente es la misma capacidad de IA con otro prompt.
- **Que las tools escriban directo en la orden:** rompe R-08 (la IA propone, la persona confirma) y deja cambios a medias si la IA falla.
- **Guardar el audio:** no hace falta para el trabajo y suma datos personales (Ley 1581, minimización). Se guarda la transcripción.
- **Pasar las fotos a la orden después de confirmar:** no servía para "pasar a Recibido" en la misma confirmación.

## Consecuencias
- **"La IA no diagnostica" lo sostienen el prompt y la evaluación, no el código:** `proponer_hallazgo` no puede saber si la descripción salió de lo que dijo el mecánico. Es aceptable porque nada se guarda sin que la persona lo lea en la ficha y toque Confirmar.
- **Toda tool nueva debe cambiar solo la propuesta** y validar en código. Lo que llega a la orden pasa siempre por los casos de uso de su módulo.
- **La calidad depende de los modelos gratis, que cambian sin aviso.** Antes de una demo se mide con `make agent-eval-floor`; los límites diarios son `Sin dato` (Q-42).
- **El asistente no reemplaza los formularios:** si no está, el taller sigue trabajando.

## Evidencia
- Dominio (`floor_test.go`):
  - precios de la lista y de lo dicho;
  - lo inventado sin precio;
  - cifras habladas ("80 mil");
  - "N de M datos" con lo guardado;
  - validación al confirmar;
  - edición de la persona;
  - el tipo de foto por su contenido.
- Aplicación: el permiso de cada parte al confirmar (`TestFloorAllowedChecksEachPart`).
- Adaptador: las fotos van como `image_url` (`TestChat_ImagesGoAsParts`). Esta prueba encontró que el adaptador no las mandaba.
- HTTP (`internal/server/floor_test.go`, con IA falsa):
  - recibir un vehículo nuevo de punta a punta;
  - precios solo de la lista o de lo dicho, editados por la persona;
  - adversarial con foto e "ignora tus instrucciones";
  - dos placas;
  - sin IA y con la IA fallando;
  - una placa con orden abierta no duplica;
  - 6 confirmaciones a la vez con la misma clave;
  - aislamiento entre talleres (404 en chat, foto, orden, editar y confirmar);
  - 401 sin sesión;
  - nada del mensaje en los logs.
- Evaluación en CI (`eval/floor_test.go`): un modelo falso que inventa precios, gastos y kilometraje en las 10 conversaciones del juego (3 adversariales) deja **0 precios inventados** en la propuesta.
- **Medición con modelos reales** (`make agent-eval-floor`, cadena gratis de Nemotron 3 Ultra, Nemotron 3.5 Super VL y Gemma 4):

  | Corrida | Resultado | Precios inventados |
  |---|---|---|
  | Primera | 5/10 | 0 |
  | Con la búsqueda de placa en código y el prompt ajustado | 8/10 (cumple; las 2 restantes eran fallas de la medición) | 0 |
  | Con las fotos ya enviadas como imagen | **10/10** | **0** |
  | Con la ficha como resultado de `ver_ficha` (fuera del prompt del sistema) | 9/10 (cumple; la que falla es una llamada que pasó de 30 s: el chat dijo "no disponible" y la propuesta quedó igual) | **0** |

  En la primera corrida:
  - el modelo no buscaba la placa;
  - leía "Al GNO667" como "ALGNO667", y ahora se corrige en código;
  - una cadena de 3 tools pasó de 50 s, y el límite subió a 60 s.

  En la última corrida cada mensaje tardó de 13 a 30 s; la mitad, unos 26 s. La foto de la evaluación es gris: el modelo la recibe y dice que no ve una factura (lo correcto). Falta medir con fotos reales de vehículos del equipo (R9).
- e2e (`web/e2e/asistente.spec.ts`, con la IA falsa de `scripts/fake-ai.mjs`): registro, chat general, "Llegó el Mazda…", 4 fotos con el kilometraje, "9 de 9 datos", Confirmar, la orden con su recepción y la conversación en D2.
