# Orden de construcción y plan de arranque

> Todas las HU son Must (decisión del equipo, 1-nov-2026), así que **ya no hay un corte automático**: el orden lo dan las **dependencias** y los **tramos**. Sin fecha de demo ni dedicación semanal (Q-26/AQ-12, `Sin dato`) no se puede poner calendario ni juzgar si todo cabe: este plan solo ordena.

## 1. Cifras que mandan el orden

- **101 HU vigentes**, 53 necesarias para correr las 10 escenas de la demo (contando lo que esas necesitan por dependencias); las otras 48 no aparecen en ninguna escena, **pero también se construyen**.
- **Cadena de dependencias más larga: 9 niveles**: HU-10 → HU-16 → HU-20 → HU-48 → HU-28 → HU-29 → HU-33 → HU-38 → HU-26. Es la que no se acorta con más manos.
- Cota (no estimación) con la regla propia "1 a 3 días por HU": de 101 a 303 días-persona. Sin medición propia que la respalde.
- HU necesarias para la demo: HU-01, HU-02, HU-03, HU-04, HU-05, HU-07, HU-08, HU-09, HU-10, HU-16, HU-17, HU-18, HU-19, HU-20, HU-21, HU-22, HU-23, HU-24, HU-25, HU-27, HU-28, HU-29, HU-32, HU-33, HU-34, HU-40, HU-41, HU-43, HU-45, HU-46, HU-48, HU-51, HU-52, HU-53, HU-55, HU-56, HU-62, HU-68, HU-69, HU-70, HU-71, HU-73, HU-74, HU-75, HU-76, HU-80, HU-81, HU-85, HU-90, HU-93, HU-97, HU-98, HU-99.

## 2. Tramos de construcción (T0 a T7)


### T0 · Cimientos

Cuenta, taller aislado, roles, bitácora, política publicada.

Historias (8): HU-01, HU-02, HU-79, HU-80, HU-81, HU-88, HU-89, HU-90

### T1 · Taller sin WhatsApp

Registro con precarga, placa de carro y moto, motor, orden, checklist, fotos, estados, inventario y cotización. Ya sirve como papel digital.

Historias (22): HU-03, HU-04, HU-05, HU-07, HU-08, HU-09, HU-10, HU-11, HU-12, HU-13, HU-14, HU-16, HU-17, HU-18, HU-20, HU-21, HU-23, HU-25, HU-27, HU-32, HU-34, HU-91

### T2 · Avisos por WhatsApp

Número de prueba, autorización, ventana de 24 h, AU-01 a AU-03, aprobación por botón, conversación en la orden.

Historias (13): HU-85, HU-86, HU-95, HU-47, HU-48, HU-46, HU-19, HU-28, HU-29, HU-33, HU-24, HU-41, HU-43

### T3 · Agente

AU-04 con bandeja y validación; AU-05 con estado real.

Historias (15): HU-51, HU-52, HU-53, HU-54, HU-55, HU-56, HU-57, HU-58, HU-59, HU-62, HU-63, HU-64, HU-65, HU-99, HU-102

### T4 · Números y catálogo

KPIs, medidor del administrador, catálogo con interruptores y textos.

Historias (8): HU-40, HU-42, HU-68, HU-69, HU-70, HU-71, HU-73, HU-74

### T5 · Ensayo

Taller ficticio con moto, script de reinicio, dos pasadas completas.

Historias (2): HU-97, HU-98

### T6 · El resto

Lo que antes era Should: asistente, migración por plantilla y foto, AU-06 a AU-08, recepción, cancelación, cotización adicional, fusión de duplicados, devolución de inventario y las demás.

Historias (26): HU-06, HU-15, HU-22, HU-26, HU-30, HU-31, HU-35, HU-36, HU-37, HU-38, HU-44, HU-45, HU-60, HU-61, HU-66, HU-72, HU-75, HU-76, HU-82, HU-83, HU-84, HU-87, HU-92, HU-93, HU-100, HU-101

### T7 · Con riesgo

Lo que antes era Later: compras y bodegas, garantía, auto-envío, asistente del mecánico, preguntas libres, Excel propio y cuadernos. Cada una tiene una pregunta abierta o una contradicción.

Historias (7): HU-39, HU-49, HU-67, HU-77, HU-78, HU-94, HU-96


## 3. Niveles de dependencia

N0 no depende de otra HU; N1 depende solo de N0, y así. Las del mismo nivel se pueden construir en paralelo entre Isaac y Pablo.

| Nivel | Cantidad | Historias |
|---|---|---|
| N0 | 58 | HU-01, HU-03, HU-07, HU-08, HU-09, HU-10, HU-13, HU-14, HU-21, HU-22, HU-24, HU-32, HU-35, HU-40, HU-41, HU-42, HU-44, HU-45, HU-46, HU-47, HU-52, HU-54, HU-58, HU-59, HU-60, HU-62, HU-63, HU-64, HU-65, HU-66, HU-68, HU-69, HU-70, HU-71, HU-72, HU-73, HU-75, HU-76, HU-79, HU-80, HU-81, HU-82, HU-83, HU-84, HU-85, HU-87, HU-88, HU-89, HU-90, HU-92, HU-93, HU-95, HU-97, HU-98, HU-99, HU-100, HU-101, HU-102 |
| N1 | 20 | HU-02, HU-04, HU-06, HU-11, HU-12, HU-15, HU-16, HU-23, HU-34, HU-36, HU-37, HU-39, HU-53, HU-55, HU-56, HU-61, HU-67, HU-74, HU-78, HU-94 |
| N2 | 8 | HU-05, HU-17, HU-18, HU-20, HU-27, HU-51, HU-57, HU-91 |
| N3 | 5 | HU-25, HU-48, HU-77, HU-86, HU-96 |
| N4 | 4 | HU-19, HU-28, HU-43, HU-49 |
| N5 | 3 | HU-29, HU-30, HU-31 |
| N6 | 1 | HU-33 |
| N7 | 1 | HU-38 |
| N8 | 1 | HU-26 |

## 4. Sprint 0: cimientos y prueba de la arquitectura (T0)

Objetivo: dejar el repositorio, el pipeline y **una rebanada vertical completa** funcionando antes de multiplicar el patrón.

1. **Repositorio y estructura:** monorepo `SuperERP-SWEVEH-/` con `cmd/api`, `cmd/worker`, `internal/<modulo>/{domain,app,ports,adapters}`, `internal/platform`, `internal/shared`, `api/openapi.yaml`, `db/migrations`, `db/queries`, `web/`, `deploy/`, `docs/`. Reglas de dependencia forzadas con `depguard` y un test de arquitectura (`go list -deps`). Detalle en `docs/06-arquitectura.md`, sección 4.1.
2. **CI mínimo en GitHub Actions:** lint (golangci-lint), pruebas con `-race` y Postgres real, ESLint + `tsc`, SonarQube Cloud, Snyk (SCA), gitleaks, govulncheck; reglas de rama; Dependabot; acciones fijadas por SHA. Secciones 13.2 y 13.6 de la arquitectura.
3. **Proyecto Supabase de staging** (separado de producción) y **prueba técnica de RLS + `SET LOCAL app.taller_id` + pooler** (riesgo R-A3). Si falla: plan B, **preguntando** (reglas 3.3 punto 6).
4. **Rebanada vertical:** iniciar sesión con OTP (HU-01) → crear cliente y vehículo con motor (HU-09, HU-10, HU-11) → abrir una orden (HU-16) → auditoría → verla en el panel. Atraviesa todas las capas y valida el patrón.
5. **Despliegue a staging** con migraciones, `/healthz`, `/readyz`, logs JSON y trazas básicas (OpenTelemetry).
6. **Pruebas fundacionales:** aislamiento entre dos talleres (CA-10) y la primera prueba de concurrencia (números de orden por taller, 8.3).
7. **Decidir** AQ-01, AQ-02, AQ-04, AQ-10, AQ-12 con Isaac y Pablo (marcadas ⛔ en `docs/07-preguntas-abiertas.md`).

**Criterio de salida del Sprint 0:** `make ci` en verde en local y en GitHub Actions; la rebanada corre en staging; la prueba de aislamiento pasa; las decisiones ⛔ están tomadas o registradas como bloqueo.

## 5. Después del Sprint 0

T1 taller sin WhatsApp (E02–E05, E11) → T2 avisos y WhatsApp (E06 parcial) → T3 agente (E07–E08) → T4 números y catálogo (E09, E06) → T5 ensayo de demo (E13–E14) → T6 el resto → T7 las de riesgo. Detalle en la sección 2.

**Antes de empezar cada tramo:** revisar que las HU estén "listas para construir" (`docs/02`, R-03), que las preguntas abiertas que las afectan tengan respuesta o recomendación aceptada, y actualizar `docs/09-estado-de-construccion.md`.

## 6. Lo que decide la fecha real (bloqueos fuera del código)

La cuenta de Meta y el número de prueba (plantillas propias y duración del token: `Sin dato`), el modelo de IA (D-08), el formato de placa de moto (Q-01), el checklist y los servicios de moto (Q-02), la política de datos publicada, el contrato de transmisión de datos (AQ-10) y la plataforma de cómputo (AQ-01).

## 7. Variables de entorno y secretos (a crear en el Sprint 0; nunca en el repositorio)

| Variable | Uso | Entornos |
|---|---|---|
| `DATABASE_URL` | Conexión del rol de aplicación (sin `BYPASSRLS`, sin DDL), vía pooler en modo sesión | local, staging, prod |
| `MIGRATIONS_DATABASE_URL` | Rol de migraciones (solo en el job de despliegue) | staging, prod |
| `JWT_PRIVATE_KEY_PEM`, `JWT_KID` | Firma ES256; la anterior se conserva para rotación | staging, prod |
| `WHATSAPP_APP_SECRET` | Verificar `X-Hub-Signature-256` | staging, prod |
| `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_VERIFY_TOKEN` | Envío y verificación del webhook | staging, prod |
| `AI_PROVIDER`, `AI_API_KEY`, `AI_MODEL` | Capa multi-proveedor (D-08 abierta) | staging, prod |
| `SUPABASE_URL`, `SUPABASE_SERVICE_KEY` (solo backend) | Storage privado y URLs firmadas | staging, prod |
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS` | Exportación de trazas, métricas y logs | staging, prod |
| `SONAR_TOKEN`, `SNYK_TOKEN` | Solo como secretos de GitHub Actions | CI |

`.env.example` con **valores ficticios** sí va al repositorio; `.env` real, no (en `.gitignore`).

## 8. Comandos objetivo (a crear en el Sprint 0 con un `Makefile`)

| Comando | Qué hace |
|---|---|
| `make dev` | Levanta Postgres local, api y worker con recarga; front con Vite |
| `make test` | `go test -race -cover ./...` y `npm test` |
| `make lint` | golangci-lint (con `depguard`), ESLint, `tsc --noEmit` |
| `make gen` | `sqlc generate`, `oapi-codegen` y cliente TypeScript; falla si hay diferencias sin commitear |
| `make migrate` | Aplica migraciones al entorno local |
| `make e2e` | Playwright contra el entorno local o de staging |
| `make ci` | Todo lo anterior + govulncheck y gitleaks, igual que el pipeline |
| `make seed-demo` | Carga el taller ficticio Servitaller El Pistón (datos **Ficticio**) |
| `make reset-demo` | Reinicia el taller ficticio para correr la demo dos veces seguidas (HU-99) |

> Los nombres de los comandos son una **Propuesta**; si el equipo prefiere otros, se cambian aquí y en `CLAUDE.md`.
>
> **Estado (2026-11-01):** existen `dev` (con `dev-api`, `dev-worker`, `dev-web`), `test`, `lint`, `gen`, `migrate`, `cover`, `ci`, `build` e `image`. `e2e`, `seed-demo` y `reset-demo` fallan a propósito con un mensaje hasta T5 (HU-97, HU-98). `make dev` levanta Postgres y migra, pero api, worker y web se corren en tres terminales.

## 9. Plantilla de trabajo por HU (copiar en la descripción del PR)

```markdown
## HU
HU-XX · título · tramo T? · escenas/RF/CA que cubre

## Qué cambió
- …

## Criterios de aceptación cubiertos
- [ ] Dado … cuando … entonces … (prueba: `ruta/prueba_test.go::Nombre`)

## Reglas de `docs/02`
- [ ] Aislamiento (prueba con dos talleres)   - [ ] Auditoría + evento en la misma transacción
- [ ] Idempotencia (si hay entrada externa)   - [ ] Permiso RBAC definido y probado
- [ ] Sin datos prohibidos en logs            - [ ] Contrato OpenAPI y cliente regenerados
- [ ] Prueba de concurrencia (si aplica)      - [ ] IA/WhatsApp probados (si aplica)

## Decisiones provisionales o preguntas nuevas (docs/07)
- …

## Docs actualizados
- [ ] docs/09-estado-de-construccion.md  - [ ] HU / ADR / arquitectura si cambió
```
