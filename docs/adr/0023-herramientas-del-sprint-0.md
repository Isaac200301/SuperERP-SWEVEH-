# ADR-0023 · Herramientas concretas del Sprint 0

- **Estado:** Propuesto (decisiones de bajo impacto que `docs/06` dejaba "por elegir")
- **Fecha:** 2026-11-01
- **Quién decide / aprueba:** agente constructor; revisión de Isaac y Pablo en el PR
- **Relacionado con:** ADR-08, ADR-09, ADR-10, ADR-14, `docs/06` §4.2, §5.2, §13

## Contexto
La arquitectura propone librerías pero deja varias elecciones abiertas ("golang-migrate o atlas", "depguard y la prueba concreta quedan por elegir", cliente TypeScript "generado desde OpenAPI"). El Sprint 0 necesita una elección para poder construir la rebanada.

## Decisión
| Necesidad | Elección | Nota |
|---|---|---|
| Migraciones | `pressly/goose` v3 (archivos SQL con `-- +goose Up/Down`, embebidos) | sqlc entiende el formato; el mismo `embed.FS` lo usan `cmd/migrate` y las pruebas |
| Consultas | `sqlc` 1.29.0 por Docker | En Windows sqlc necesita cgo; con Docker todos generan igual |
| Router | `go-chi/chi` v5 | Rutas declaradas por módulo con su permiso (`guard.Require`) |
| Contrato | `api/openapi.yaml` (OpenAPI 3.0.3) + prueba que compara rutas del router con el contrato | No se usa `oapi-codegen` todavía: el router declara cada ruta con su permiso y la prueba `TestRouterMatchesOpenAPIContract` impide divergencias |
| Cliente web | `openapi-typescript` + `openapi-fetch` | Tipos generados del contrato (`make gen`) |
| Reglas de arquitectura | `internal/arch_test.go` (con `go list`) + `depguard` en `golangci-lint` v2.15.0 | Ambos verificados con una violación de prueba |
| Módulo Go | `sweveh` | Ruta interna corta; no es una librería pública |
| Frontend | React 19, React Router 8, TanStack Query 5, RHF 7 + Zod 4, Tailwind 4, Vite 8, Vitest 5, **TypeScript 5.9** | TS 7 no es compatible todavía con `typescript-eslint` (< 6.1) ni con `openapi-typescript` (^5) |
| Fuentes | Manrope y JetBrains Mono autoalojadas (`@fontsource`) | La CSP queda sin excepciones a terceros |
| Postgres local/CI | `postgres:16-alpine` | Alinear con la versión del proyecto Supabase cuando exista (`Sin dato`) |

## Alternativas descartadas
| Alternativa | Por qué no |
|---|---|
| `oapi-codegen` (servidor) | Genera una interfaz única con todas las operaciones y no permite declarar el permiso por ruta; se puede adoptar después para tipos sin cambiar el contrato |
| `golang-migrate` / `atlas` | Equivalentes; goose permite un archivo por migración y embebido simple |
| `testcontainers-go` | Requiere el daemon de Docker desde la prueba; se usa una base nueva por paquete en el Postgres del compose/servicio de CI (`internal/platform/testdb`) |

## Consecuencias
Cambiar cualquiera de estas piezas es local (un paquete o un archivo de configuración). Las versiones quedan fijadas en `go.mod`, `web/package.json` (exactas), `Makefile` y `.github/workflows/ci.yml` (acciones por SHA de commit).

## Evidencia
`make ci` equivalente corrido en local el 2026-11-01 (salida en la bitácora de `docs/09`).
