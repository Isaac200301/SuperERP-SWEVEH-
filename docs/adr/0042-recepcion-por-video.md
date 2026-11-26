# ADR-0042 · Recepción por video

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-12, 24-nov-2026); Isaac revisa en el PR
- **Fecha:** 2026-11-26 (R9)
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Relacionado con:** HU-108 (nueva), HU-107, HU-17, HU-18, HU-21, HU-64, HU-87, ADR-0033, ADR-0041, R-07, R-08, R-10, RNF-13

## Contexto
Pablo pidió (V-12) recibir el vehículo con **un video de hasta 90 s**:
- con un contador y una guía de lados;
- el asistente anota todo: el audio pasa a texto y de 8 a 10 cuadros van al modelo de visión;
- si falta algo, se graba otro video o el asistente lo pregunta.

El plan preveía un prototipo previo (**P1**) para medir con 3 o 4 videos de vehículos del equipo, en Chrome Android y Safari iOS, y decidir entre **video** y el **recorte** (fotos por lado + nota de voz).

**P1 no se corrió.** No hubo sesión aparte ni videos de prueba, y el agente no tiene un celular real. Por eso este ADR **no trae mediciones**: qué datos acierta, cuánto tarda y qué formatos acepta cada navegador quedan como `Sin dato` (Q-47). En lugar de elegir a ciegas, se construyeron **los dos caminos sobre el mismo endpoint**.

## Decisión

### El video nunca sale del celular
- La pantalla graba con la cámara y **solo se queda con**:
  - el **audio**, con un `MediaRecorder` que graba solo la pista de audio;
  - **un cuadro cada 10 s**, tomado con `<canvas>` en JPEG de 1280 px como máximo, hasta 10.
- El video completo no se graba ni se manda (minimización, Ley 1581, y menos almacenamiento). El servidor **rechaza una parte `video`** con 400 `video_completo`.
- Del plan se descartó la **segunda grabadora de video**: si el video no se guarda, sobra.
- **Desviación de la ficha:** la transcripción **no** se guarda como nota de la orden. Queda como texto del mensaje en la conversación de la orden, que se ve en su pestaña Asistente. A la orden solo llegan, al confirmar, las notas que propuso el modelo y que la persona revisó.
  - Motivo: las notas de la recepción son salvedades de máximo 300 letras (golpes, faltantes), y una transcripción de 90 s no cabe ni es una salvedad. Así también se guardan menos datos (minimización).
  - Riesgo: si el modelo omite algo que se dijo, no llega a la orden. La persona lo ve en la ficha antes de confirmar y puede decírselo al asistente.

### El mismo asistente de R8, con origen "video"
- `POST /orders/{id}/reception/video` recibe en multipart:
  - `segundos` (0 a 90);
  - `audio` (opcional, máx. 10 MB);
  - `cuadros` (1 a 10, cada uno de máx. 2 MB, validado por su contenido).
- `app.Floor.Video` sigue el mismo camino que un mensaje del chat (`chatFor`, `userMessage`, `answer`, `save`), con dos cambios:
  - el prompt suma un párrafo fijo del código (`floorVideoSystem`);
  - el turno marca `video: true`, así que todo lo que sale de ese mensaje queda con **origen `video`** en la ficha en vivo (HU-64).
- El texto de la persona sigue entrando como dato, nunca como instrucción (R-08).
- **Solo para órdenes "Por recibir"** (409 `orden_ya_recibida` si no) y del taller de la sesión (404 a otro taller, por RLS).
- **Los cuadros siempre van a la propuesta**, aunque la IA no responda (RNF-13): no se pierden y se confirman como fotos de la orden.
- **Nada se guarda hasta Confirmar** (R-08). Al confirmar, los cuadros pasan a evidencias de la orden con su lado (HU-21), por el mismo `Apply` de R8.

### La guía de lados
- La guía es **✓ Placa · ✓ Tablero · ○ Frente · ○ Lado izq. · ○ Atrás · ○ Lado der.**, igual para carro y moto.
- `proponer_recepcion` acepta `placa` como lado. En un video, cada lado que reporta el modelo se suma a `propuesta.vistos`.
  - La placa (y el tablero de un carro) marcan la guía, pero la foto va **sin lado**: no son lados con foto en la orden.
- Un paso de la guía se marca si:
  - el modelo lo vio;
  - una foto de la propuesta tiene ese lado;
  - o la orden ya lo tiene guardado.
- La guía viaja en `progreso.guia` con la ficha en vivo. `vistos` no se edita a mano (`HumanEdit` lo conserva).
- Lo que no se vio lo dice el **código**, no la IA: "No vi la parte trasera ni el lado derecho. Graba otro video o dime cómo están." (`domain.GuideMissing`).

### Idempotencia (R-07)
- El id del mensaje del video es un **UUIDv5** del taller y la `Idempotency-Key` (`shared.NameID`).
- Si ese mensaje ya existe, se devuelve la conversación tal como quedó, sin volver a llamar a la IA ni repetir cuadros.
- La misma clave en **otra orden** del taller responde 409 `clave_usada`, como en R8 (`TestFloorVideo_KeyReusedInAnotherOrder`).
- Con dos envíos a la vez con la misma clave, el segundo choca con la llave primaria del mensaje y devuelve el primero (`TestFloorVideo_ConcurrentSameKey`).
- **Costo aceptado:** en ese choque, el segundo envío ya llamó a la IA y guardó sus cuadros. Las copias quedan sin referencia en el almacenamiento, igual que las fotos de un mensaje que falla en R8.

### El recorte, en la misma pantalla
- Si el navegador no tiene `MediaRecorder` o `getUserMedia`, o la persona lo prefiere, **"Tomar fotos por lado"** manda hasta 10 fotos al mismo endpoint, con `segundos = 0` y sin audio.
- La nota de voz sigue en el chat (R8).
- Así el recorte previsto **no necesita otra rebanada**: ya está.

### Micrófono permitido para el propio origen
- `Permissions-Policy` decía `microphone=()`: **el navegador no daba la pista de audio a nadie**. Esto también apagaba en silencio el botón "Hablar" de R8, que nunca se probó en un celular.
- Lo encontró el e2e de R9: `getUserMedia` devolvía 0 pistas de audio.
- Pasa a `microphone=(self)`, igual que la cámara: solo este origen y siempre con el permiso que pide el navegador. Lo prueba `server_test.go`.
- Es un ajuste de una cabecera de seguridad: queda como decisión provisional en `docs/07`, para que Isaac la revise.

## Alternativas descartadas
- **Subir el video completo y cortarlo en el servidor** (ffmpeg): guarda datos que no se usan (Ley 1581), pesa decenas de MB por recepción y suma una dependencia nativa a la imagen.
- **Mandar el video al modelo:** los modelos gratuitos de la cadena (V-13) reciben imágenes, no video (`Sin dato` para video en el plan gratuito).
- **Una tabla nueva para la idempotencia:** el id derivado de la clave usa la llave primaria del mensaje y no pide migración.
- **Elegir solo el recorte sin P1:** el plan pide video; con los dos caminos, la decisión final se toma con datos del piloto sin tocar el backend.

## Consecuencias
- El taller puede recibir con un video o con fotos, y la ficha en vivo dice de dónde salió cada dato ("del video").
- Las fotos de la orden son cuadros de 1280 px: alcanzan para la evidencia de la recepción, pero son menos nítidas que una foto.
- El audio pasa por Groq y los cuadros por OpenRouter, igual que en R8: aplican AQ-09, AQ-10 y Q-46.
- Al borrar a un titular (HU-87), la conversación de la orden se borra igual que en R8. Los cuadros quedan como fotos de la orden si se confirmaron.

## Evidencia
- **Dominio:** `TestGuide`, `TestGuideMissing`, `TestHumanEditKeepsSeen`.
- **HTTP con IA falsa:**
  - `TestFloorVideo_ReceptionProposalAndConfirm`: 8 cuadros → propuesta con origen video y guía; nada guardado; idempotente; Confirmar guarda 8 fotos y el kilometraje.
  - `TestFloorVideo_RulesAndIsolation`: clave, cuadros, video completo, no imagen, otro taller, sin sesión y orden ya recibida.
  - `TestFloorVideo_WithoutAIKeepsFrames`.
  - `TestFloorVideo_ConcurrentSameKey`.
- **Vitest** (`video.test.tsx`), con `MediaRecorder` simulado: contador, un cuadro cada 10 s, corte a los 90 s con audio y 10 cuadros; el recorte con fotos y su clave; solo "Por recibir".
- **e2e** (`video.spec.ts`): Chromium real con cámara y micrófono falsos. Graba 12 s con `MediaRecorder`, llegan el audio (opus) y 3 cuadros, la guía marca lo visto, el código dice qué falta y Confirmar guarda 3 fotos.
- **Sin medir (Q-47):** Safari iOS y Chrome Android reales, acierto con vehículos reales y tiempo con los modelos gratuitos.
