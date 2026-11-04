# ADR-0025 · Canal de WhatsApp por proveedor (Meta oficial; simulador local; WAHA solo para la demo)

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Pablo (pidió el conector de Meta "bien modularizado" y una opción no oficial para la demo); revisión de Isaac en el PR
- **Relacionado con:** RNF-07 (cambiar de número sin tocar el resto), HU-101, HU-47, HU-19, ADR-14 (outbox), `docs/06` §9.1 y §16.1

## Contexto
El canal de WhatsApp estaba en un solo paquete (`messaging/adapters/out/channel`) que mezclaba el enrutador, el canal de prueba y Meta, y el parser del webhook vivía en los casos de uso (`messaging/app/parse.go`) atado al formato de Meta. Pablo pidió un conector de Meta bien separado y adelantó que la demo usará una opción no oficial (probablemente WAHA) mientras llega la cuenta de Meta.

## Decisión
- Dos puertos en `messaging/app`: `Channel` (salida: texto, botones, plantilla aprobada e imagen) e `InboundParser` (webhook crudo → mensajes entrantes y estados de entrega). Errores comunes: `ErrUnsupported`, `ErrOutsideWindow` e `IsRetryable` (el adaptador marca sus errores con `Retryable()`).
- Un paquete por proveedor bajo `messaging/adapters/out/whatsapp/`:
  - `meta`: Cloud API (Graph API) con timeout propio; imágenes subidas a `/media` y enviadas por id (sin URLs públicas de fotos); errores tipados `APIError{Status, Code}` **sin el mensaje de Meta** (puede traer el número o el texto del cliente); 131047 equivale a `ErrOutsideWindow`; límites de tasa y 5xx son reintentables. Parser del webhook, firma `X-Hub-Signature-256` y verificación de la suscripción.
  - `local`: simulador, no envía nada; números `local-…`.
  - `waha`: **solo para la demo**, cuando se construya, implementando los mismos dos puertos.
- `whatsapp.Router` manda los números `local-…` al simulador y el resto al proveedor configurado.
- Elección por configuración: `WHATSAPP_PROVIDER=meta|local` (más adelante `waha`). Sin la variable: `meta` si hay token, si no `local`. `meta` exige token y secreto. **El simulador no arranca en producción.**
- Fuentes del formato: https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages · https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media · https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components · https://developers.facebook.com/docs/whatsapp/cloud-api/support/error-codes

## Consecuencias
- Agregar WAHA (u otro) es un paquete nuevo más una línea en `wire.Channel`; los casos de uso, el outbox y las automatizaciones no cambian.
- WAHA no es oficial: **solo para la demo**, nunca con datos reales (AQ-10). Si el simulador y WAHA conviven, la cola de webhooks tendrá que guardar de qué proveedor viene cada cuerpo; hoy todos comparten el formato de Meta.
- Las pruebas del adaptador de Meta corren contra un `httptest.Server` que imita la Graph API; validar contra Meta real queda para cuando exista el número de prueba.
