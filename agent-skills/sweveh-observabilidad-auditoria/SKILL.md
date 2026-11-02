---
name: sweveh-observabilidad-auditoria
description: Observabilidad (logs estructurados slog, métricas y trazas OpenTelemetry, SLO/alertas) y auditoría de solo inserción con hashes en SWEVEH, sin PII en registros. Úsala al añadir logs, métricas, trazas, eventos de auditoría, historial de órdenes o medición de costo de IA.
---

# Observabilidad y auditoría

Fuente: `docs/06` §10 (observabilidad y auditoría) y `docs/02` R-08 (costo por conversación), R-10 (datos personales). Los umbrales numéricos (SLO, alertas) son `Propuesta` hasta medir. Referencias: https://opentelemetry.io/docs/languages/go/ · https://pkg.go.dev/log/slog · https://sre.google/sre-book/monitoring-distributed-systems/

## 1. Tres señales + auditoría (son cosas distintas)

| Señal | Para qué | Quién la lee | Retención |
|---|---|---|---|
| **Logs** | diagnosticar un evento | desarrollo/operación | corta, sin PII |
| **Métricas** | tendencias, alertas, SLO | operación | media |
| **Trazas** | seguir una solicitud entre capas | desarrollo | corta (muestreo) |
| **Auditoría** | quién hizo qué y cuándo (prueba) | negocio/legal/seguridad | larga, inmutable |

No uses logs como auditoría ni auditoría como depuración.

## 2. Logs estructurados con `slog`

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
logger.InfoContext(ctx, "orden_transicionada",
	slog.String("request_id", rid), slog.String("trace_id", tid),
	slog.String("taller_id", tid2), slog.String("orden_id", id.String()),
	slog.String("de", from.String()), slog.String("a", to.String()))
```
- JSON a stdout (la plataforma recolecta). Niveles: `ERROR` accionable; `WARN` degradación; `INFO` eventos de negocio; `DEBUG` solo en local.
- Campos fijos: `request_id`, `trace_id`, `taller_id`, `actor_id`, `modulo`, `evento`.
- **Nunca** en logs: tokens, cookies, OTP, secretos, cuerpo completo de mensajes de clientes, celular completo (enmascarar `+57300***1234`), placas junto con nombre. Un `Redactor` central en el handler de slog evita filtraciones por descuido.
- Nombres de evento en snake_case estable (`webhook_firma_invalida`), no frases libres.

## 3. Métricas (OpenTelemetry / Prometheus)

Mínimas del MVP (`Propuesta`):
- HTTP: tasa, errores, latencia p50/p95/p99 por ruta (plantilla de ruta, no URL con ids).
- Base de datos: conexiones en uso, latencia de consultas, reintentos 40001/40P01, conflictos de versión (409).
- Outbox: pendientes, edad del más antiguo, muertos (DLQ).
- WhatsApp: mensajes enviados/fallidos/entregados, firmas inválidas, duplicados por `wamid`.
- IA: tokens in/out, **costo por conversación y por taller**, tasa de salidas inválidas, escalados a humano, topes alcanzados.
- Negocio (para KPIs del producto): órdenes por estado, tiempo en cada estado.
- **Cardinalidad:** nunca uses `taller_id` + `orden_id` como etiquetas de métricas sin control (explosión). `taller_id` solo si hay un número acotado de talleres; ids de orden jamás.

## 4. Trazas

- Instrumenta `net/http` (entrada y salida), `pgx` (otelpgx) y llamadas a Meta/IA.
- Propaga `traceparent` (W3C). El `trace_id` va en los logs y en la respuesta de error (`instance` o encabezado) para soporte.
- Atributos útiles: `taller.id` (si es acotado), `orden.estado`, `ia.proveedor`, `ia.modelo`, `ia.tokens`.
- Muestreo: 100 % de errores, porcentaje bajo del resto (`Propuesta`, ajustar por costo).
- Backend de observabilidad (Grafana Cloud, otro) = **decisión pendiente / `Sin dato` de precio**: exporta por OTLP para no atarte (Protected Variations).

## 5. SLO y alertas (propuesta para empezar)

| SLI | Objetivo inicial `Propuesta` |
|---|---|
| Disponibilidad de la API | 99,5 % mensual |
| Latencia p95 de lecturas | < 500 ms |
| Entrega de notificación de "orden lista" | 99 % en < 2 min |
| Outbox sin atender | edad máxima < 5 min |

Alertas por **síntoma** (errores/latencia/colas), no por causa; cada alerta enlaza un runbook corto en `docs/` (qué mirar, cómo mitigar). Evita alertas ruidosas: si nadie actúa, se elimina.

## 6. Auditoría (solo inserción, con hashes)

Qué se audita (mínimo): login/logout/refresh reutilizado, cambios de rol/permisos, cambios de estado de órdenes, aprobación de cotizaciones, ajustes de inventario, cambios de precio, autorizaciones de datos (consentimiento), envíos y borradores de IA y su aprobación/edición (RF-19), accesos del Administrador del ERP a datos de un taller, borrados por retención.

```sql
CREATE TABLE auditoria.eventos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    taller_id     uuid,                 -- NULL solo para eventos de plataforma
    ocurrido_en   timestamptz NOT NULL DEFAULT now(),
    actor_tipo    text NOT NULL,        -- usuario | agente_ia | sistema | admin_erp
    actor_id      uuid,
    accion        text NOT NULL,        -- 'orden.transicion', 'cotizacion.aprobar'
    objeto_tipo   text NOT NULL,
    objeto_id     uuid,
    detalle       jsonb NOT NULL,       -- sin PII innecesaria
    request_id    text,
    hash_previo   bytea,
    hash          bytea NOT NULL        -- sha256(hash_previo || contenido canónico)
);
REVOKE UPDATE, DELETE, TRUNCATE ON auditoria.eventos FROM sweveh_api, sweveh_worker;
-- Solo INSERT; además un trigger que rechaza UPDATE/DELETE como defensa adicional
```
- **Solo inserción:** sin `UPDATE/DELETE` para los roles de la app; la verificación de la cadena de hashes (por taller) es una tarea periódica que alerta si se rompe.
- Un evento de auditoría se escribe en la **misma transacción** que el cambio (si falla la auditoría, falla el cambio).
- Historial de estados de una orden: usuario y hora, **inmutable** (docs/02 §10).
- Lectura de auditoría = permiso propio (`audit:read`) y también auditada.
- Retención y borrado según política (Ley 1581): la auditoría no guarda más PII de la necesaria; si hay que anonimizar, se hace con un evento que referencia, no editando filas (`Sin dato`: periodos exactos, consultar asesoría — AQ-10).

## 7. Medición de costo de IA (RNF-06, RNF-14)

Cada llamada al LLM registra: `taller_id`, `conversacion_id`, proveedor, modelo, tokens entrada/salida, costo estimado (calculado con tarifas **configurables**, nunca constantes en código), latencia, resultado de validación. Un agregador por taller y mes alimenta el panel del **Administrador del ERP** (HU-74) y los topes (R-08). Precios de proveedor: consultar la fuente vigente o marcar `Sin dato`.

## 8. Checklist

- [ ] Logs JSON con `request_id`/`trace_id`, sin PII ni secretos.
- [ ] Métricas con cardinalidad controlada y nombres estables.
- [ ] Traza propagada hasta BD y llamadas salientes.
- [ ] Evento de auditoría en la misma transacción para acciones sensibles.
- [ ] Sin `UPDATE/DELETE` sobre la tabla de auditoría para el rol de la app.
- [ ] Alerta + runbook para toda condición nueva que pueda dejar al taller sin servicio.
