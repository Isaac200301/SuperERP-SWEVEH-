# ADR-0022 · Cómputo en Google Cloud Run (reemplaza la propuesta Fly.io de ADR-19)

- **Estado:** Aceptado (decisión del equipo); detalles de configuración: Propuesto
- **Fecha:** 2026-11-01
- **Quién decide / aprueba:** Isaac Cano Orozco (respuesta a AQ-01 en la sesión del 2026-11-01)
- **Relacionado con:** AQ-01, ADR-18, ADR-19, ADR-20, `docs/06-arquitectura.md` §12

## Contexto
`docs/06` §12.3 comparaba Fly.io (`gru`) y Cloud Run (`southamerica-east1`) y recomendaba Fly.io por la simplicidad de correr `api` y `worker` juntos. AQ-01 estaba marcada ⛔. El equipo eligió **Cloud Run**.

Lo que `docs/06` ya dejó anotado y sigue vigente: con Cloud Run, el proceso `worker` (sondeo del outbox cada ~1 s) "exige facturación con CPU siempre asignada o un patrón distinto (Cloud Run Jobs/Scheduler)". Costos: **`Sin dato`** (no se cotizó; no se inventan cifras).

## Decisión
El cómputo de staging y producción corre en **Google Cloud Run**, región `southamerica-east1` (São Paulo, junto a Supabase `sa-east-1`, AQ-02), con **la misma imagen** (`deploy/Dockerfile`) para dos servicios:

| Servicio | Comando | Configuración propuesta (a confirmar al crear la cuenta) |
|---|---|---|
| `sweveh-api` | `/app/api` | Mínimo de instancias ≥ 1 en producción (sin arranque en frío para el taller); `/readyz` como verificación |
| `sweveh-worker` | `/app/worker` | Una instancia con CPU siempre asignada, porque el outbox sondea en bucle. Alternativa si el costo no cierra: Cloud Run Jobs disparados por Cloud Scheduler (latencia de avisos de minutos, no segundos) |
| Migraciones | `/app/migrate up` | Cloud Run Job ejecutado por el pipeline antes de desplegar, con el rol `sweveh_migrator` |

Credenciales de despliegue por **federación OIDC** desde GitHub Actions (sin claves de larga vida), como ya proponía `docs/06` §12.6 para Cloud Run.

> **Nota 2026-11-05 (ADR-0036):** para la demo en Render, el api puede correr el worker en el mismo proceso (`RUN_WORKER=true`) y así usar un solo servicio gratis. En producción siguen separados como dice este ADR.

## Alternativas descartadas
| Alternativa | Por qué no |
|---|---|
| Fly.io `gru` (ADR-19, Propuesta) | El equipo eligió Cloud Run (respaldo corporativo y facturación en Google Cloud). La imagen sigue siendo portable |
| Render / Railway | No estaban en la comparación de `docs/06` y no hay evaluación propia |

## Consecuencias
- **Lo que ya funciona sin cambios:** la imagen es un contenedor sin estado, escucha en `HTTP_ADDR` y se configura por variables de entorno; el worker y el api son binarios separados de la misma imagen.
- **Pendiente antes de crear la cuenta (requiere aprobación, docs/02 §3.3):** cotizar Cloud Run para `api` + `worker` con CPU siempre asignada; definir proyecto de GCP, Artifact Registry e identidad federada para GitHub. Hasta entonces no hay despliegue a staging.
- **Riesgo:** si el `worker` se pasa a Jobs + Scheduler, los avisos (AU-01…AU-03) dejan de ser casi inmediatos; habría que revisar las escenas 6–8 de la demo.
- **Reversión:** volver a Fly.io solo cambia el manifiesto de despliegue y el workflow; el código no depende de la plataforma.

## Evidencia
- `docs/06-arquitectura.md` §12.3 (comparación y nota sobre el worker en Cloud Run) y §16.1 (ubicaciones de Cloud Run: https://docs.cloud.google.com/run/docs/locations).
- Precios de Cloud Run: **`Sin dato`**; se verifican en la página oficial el día de la cotización.
