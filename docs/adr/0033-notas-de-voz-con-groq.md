# ADR-0033 · Notas de voz transcritas con Groq (HU-60)

- **Estado:** Propuesto (demo)
- **Fecha:** 2026-11-05
- **Quién decide / aprueba:** Pablo eligió Groq el 2026-11-05; revisión de Isaac en el PR
- **Relacionado con:** HU-60, Q-12, ADR-0026 (IA por proveedor), ADR-0027 (WAHA), ADR-0028, AQ-09, AQ-10

## Contexto
HU-60 pide que una nota de voz se transcriba, que la transcripción se vea en la tarjeta junto al audio y que siga el mismo flujo del texto. Hasta ahora el audio no se guardaba: la tarjeta decía "escúchala en la conversación" y el dueño tenía que abrir WhatsApp. Faltaba el proveedor de transcripción (Q-12, Sin dato).

## Decisión
- **Proveedor:** Groq, endpoint compatible con OpenAI (`POST /openai/v1/audio/transcriptions`, modelo `whisper-large-v3-turbo`, idioma `es`). Acepta `ogg`, que es el formato de las notas de voz de WhatsApp (ogg/opus), así que no hay que convertir el audio. Referencia: https://console.groq.com/docs/speech-to-text (consultada el 2026-11-05).
- **Puerto `assistant/app.Transcriber`** con el adaptador `assistant/adapters/out/stt/groq`. Sin `STT_PROVIDER`, no hay transcripción y la nota de voz queda como tarjeta para el dueño (como antes, ahora con el audio).
- **El audio se descarga del canal** por su referencia, que se guarda al recibir el mensaje (`payload.media`): el id de Meta (`GET /{media-id}` y luego la URL temporal, con el token) o la URL de WAHA (se usa solo la ruta `/api/files/...` contra `WAHA_URL`, con la clave, para no llamar a otro servidor con ella). Es un puerto opcional (`messaging/app.MediaFetcher`): el simulador no tiene audios.
- **Flujo (worker):** descargar → transcribir (máx. 60 s, fuera de la transacción) → guardar la transcripción como texto del mensaje (`payload.transcripcion`) → seguir como si el cliente la hubiera escrito (asesor, reglas, borradores). El borrador queda marcado `nota_de_voz` (migración 00031). Si algo falla (canal, Groq, audio vacío), la tarjeta dice que no se pudo transcribir; no se reintenta en bucle ni se inventa texto.
- **Escuchar en el panel (api):** `GET /conversations/{id}/messages/{mid}/audio` (`conversation:read`). La primera vez el api lo descarga del canal y lo guarda en su almacenamiento (`taller/mensaje`); después sale de ahí. No depende del disco del worker.
- **Solo demo:** `STT_PROVIDER=groq` no arranca en producción hasta cerrar AQ-09 (voz de clientes a un tercero) y AQ-10.

## Alternativas descartadas
| Alternativa | Por qué no |
|---|---|
| Modelo con audio en OpenRouter | No se confirmó un modelo gratuito que acepte ogg; Groq transcribe ogg directo con Whisper |
| Whisper propio (servidor) | Costo y operación de un servidor con GPU para la demo |
| Guardar el audio en el worker | El worker y el api no comparten disco (`fotos-worker`); en la nube serían servicios distintos |

## Consecuencias
- La IA recibe la transcripción, recortada a 2.000 caracteres; la conversación guarda el texto completo.
- Sin costo por mensaje en el plan gratuito de Groq (con límites de uso); no se registra costo en `MonthCost`.
- **Pendiente:** confirmar la política de datos de Groq para AQ-09; medir la calidad con audios reales de clientes (acentos, ruido del taller); con Meta real, las URLs de medios vencen (se descarga al llegar y al escucharlo la primera vez).

## Evidencia
- `TestTranscribe` (adaptador, contra una API falsa: archivo `.ogg`, modelo, idioma y clave).
- `TestWhatsApp_VoiceNoteTranscribed` (de punta a punta con WAHA y Groq falsos: transcripción en la conversación, borrador marcado, audio desde el panel y la tarjeta cuando Groq falla).
- Vitest: la tarjeta y el hilo muestran el reproductor con la transcripción.
