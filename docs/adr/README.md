# Registro de decisiones de arquitectura (ADR)

Los **ADR-01 a ADR-21** nacen en `docs/06-arquitectura.md`, sección 14.1 (1-nov-2026). Los ADR nuevos se escriben en este directorio con el formato de `0000-plantilla.md` y numeración `0022`, `0023`, … (cuándo escribir uno: R-13 de `docs/02-reglas-de-decision.md`).

| ID | Decisión | Origen | Estado |
|---|---|---|---|
| ADR-01 | Monolito modular con capas internas hexagonales | **Equipo** | Aceptado |
| ADR-02 | Backend en Go (reemplaza Spring Boot del MVP del 30-oct) | **Equipo** | Aceptado |
| ADR-03 | Supabase (Postgres + Storage) como base de datos | **Equipo** | Aceptado |
| ADR-04 | React + TypeScript + Vite + Tailwind CSS | **Equipo** | Aceptado |
| ADR-05 | JWT + RBAC + cookies seguras | **Equipo** | Aceptado |
| ADR-06 | GitHub Actions + SonarQube Cloud + Snyk | **Equipo** | Aceptado |
| ADR-07 | Seguridad, observabilidad, auditoría, resiliencia y concurrencia como requisitos transversales | **Equipo** | Aceptado |
| ADR-08 | Sin ORM: `sqlc` + `pgx` | Propuesta | Propuesto |
| ADR-09 | Un esquema Postgres por módulo; sin FK entre módulos (salvo `taller`) | Propuesta | Propuesto |
| ADR-10 | Multi-tenant por `taller_id` + RLS (rol sin `BYPASSRLS`) | Propuesta | Propuesto (probar en Sprint 0) |
| ADR-11 | Autenticación propia (celular + OTP), sin Supabase Auth | Propuesta | Propuesto (AQ-04 ⛔) |
| ADR-12 | JWT ES256 de 10 min + refresh opaco rotativo con detección de reutilización | Propuesta | Propuesto |
| ADR-13 | Data API de Supabase desactivada; el navegador no habla con la base | Propuesta | Propuesto |
| ADR-14 | Outbox transaccional + worker en el mismo repositorio | Propuesta | Propuesto |
| ADR-15 | Concurrencia: optimista por defecto, pesimista en inventario/contadores, idempotencia en toda entrada externa | Propuesta | Propuesto |
| ADR-16 | OpenTelemetry (OTLP) + `slog` JSON; backend de visualización intercambiable | Propuesta | Propuesto (AQ-08) |
| ADR-17 | Auditoría solo-inserción con cadena de hashes, en la misma transacción | Propuesta | Propuesto |
| ADR-18 | Mismo origen: Go sirve API y SPA; proxy/WAF delante | Propuesta | Propuesto |
| ADR-19 | Cómputo en Fly.io `gru` (Cloud Run como alternativa) | Propuesta | **Reemplazado por ADR-0022** |
| ADR-20 | Construir una vez, promover el mismo artefacto; producción con aprobación manual | Propuesta | Propuesto |
| ADR-21 | Sin Kubernetes, sin broker, sin caché distribuida en el MVP | Propuesta | Propuesto |
| [ADR-0022](0022-computo-en-cloud-run.md) | Cómputo en Google Cloud Run `southamerica-east1` | **Equipo** (AQ-01) | Aceptado; configuración por confirmar |
| [ADR-0023](0023-herramientas-del-sprint-0.md) | Herramientas concretas del Sprint 0 (goose, sqlc por Docker, chi, prueba de contrato, TS 5.9) | Propuesta | Propuesto |
| [ADR-0025](0025-canal-whatsapp-por-proveedor.md) | Canal de WhatsApp por proveedor: Meta oficial, simulador local, WAHA solo para la demo | Pablo / Propuesta | Propuesto |
| [ADR-0026](0026-interprete-de-ia-por-proveedor.md) | Intérprete de IA por proveedor: la IA interpreta y el código redacta; reglas por defecto, OpenRouter para la demo | Pablo / Propuesta | Propuesto |
| [ADR-0027](0027-waha-canal-de-demo.md) | WAHA como WhatsApp de prueba para la demo (no oficial): menú con números en vez de botones, motor GOWS, prohibido en producción | Pablo / Propuesta | Propuesto |
| [ADR-0028](0028-agente-responde-solo.md) | El agente responde solo, encendido por defecto; se detiene si el cliente pide una persona o el taller toma la conversación; las notas para el dueño esperan en Por revisar | Pablo / Propuesta | Propuesto |

Cuando una propuesta se acepta o se cambia: actualizar la columna Estado, crear el ADR en este directorio si el cambio es nuevo y reflejarlo en `docs/06-arquitectura.md`.
