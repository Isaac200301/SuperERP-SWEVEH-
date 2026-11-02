# Estado de construcción

> **Este archivo se actualiza en el mismo PR que cambia el estado de una HU** (R-03 y R-17 de `docs/02-reglas-de-decision.md`). Es lo primero que lee el próximo agente.
> Leyenda de estado: ⬜ Pendiente · 🟦 En curso · 🟨 En revisión (PR abierto) · ✅ Terminada (cumple la definición de terminado, R-03) · 🟥 Bloqueada · ⛔ Cancelada.

## Resumen actual

| Dato | Valor |
|---|---|
| Fecha de la última actualización | 2026-11-02 |
| Tramo actual | T0 (Sprint 0) — **en curso**: pasos 1, 2, 4 y 6 hechos; CI corriendo en GitHub (PR #1, `developer` → `main`); 3 y 5 esperan cuentas de GCP y Supabase |
| HU terminadas | 0 de 101 (ninguna cumple aún la definición de R-03: falta staging y pantallas). Hay código para ~80 HU; 21 con prueba |
| Bloqueos ⛔ abiertos | Solo AQ-10 (asesoría jurídica; necesaria antes de datos reales). AQ-01 (Cloud Run, ADR-0022), AQ-02 (`sa-east-1`) y AQ-04 (auth propia) decididas el 2026-11-01; AQ-12 provisional (ver `docs/07-preguntas-abiertas.md`) |
| Repositorio | Monolito Go (`cmd/`, `internal/` con identity, tenancy, customers, vehicles, orders, audit, outbox), migraciones, contrato OpenAPI, frontend React en `web/`, `Makefile`, `deploy/Dockerfile` |
| Pipeline | GitHub Actions en `developer` y PR a `main`, con Snyk y Sonar configurados (2026-11-02). Ramas: se trabaja en `developer` y se integra a `main` por PR |

## Qué sigue (lo escribe el agente al cerrar cada sesión)

1. Isaac/Pablo: revisar y hacer merge del PR #1 cuando el CI esté en verde.
2. Pruebas de los módulos sin cobertura vigilada: cotización (HU-27…29), inventario (HU-32…38), servicios (HU-07), mensajería/agente/automatizaciones y KPIs; sumar esos paquetes a `scripts/coverage.sh`.
3. Montar en el router de pruebas las rutas de `wire` y `analytics` y documentarlas en OpenAPI.
4. Pantallas que faltan para la demo (piso del mecánico, cotización, Por revisar, automatizaciones, KPIs).
5. Isaac/Pablo: cuentas de GCP y Supabase staging; repetir `rls_test.go` contra el pooler; AQ-10 antes de datos reales.

## Estado por historia

"Demo" = la HU hace falta para correr las 10 escenas (53 de 101).

| HU | Título | Tramo | Demo | Estado | PR | Notas |
|---|---|---|---|---|---|---|
| HU-01 | Crear la cuenta con mi celular | T0 | sí | 🟦 En curso | | Criterios 1–3 con prueba (`internal/server/server_test.go`, `errors_test.go`). Falta: canal real del código (Q-13), medir el criterio 4 (< 5 min) en la demo |
| HU-02 | Autorizar el tratamiento de mis propios datos | T0 | sí | 🟦 En curso | | Casilla sin marcar, bloqueo sin ella y prueba guardada. Política en borrador: depende de HU-90 y AQ-10 |
| HU-03 | Contar qué tipo de taller tengo | T1 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-04 | Recibir el taller ya armado | T1 | sí | ⬜ Pendiente | | |
| HU-05 | Hacer mi primera orden antes de terminar | T1 | sí | ⬜ Pendiente | | |
| HU-06 | Que me pidan los datos solo cuando hagan falta | T6 | no | ⬜ Pendiente | | |
| HU-07 | Poner el precio y la duración de mis servicios | T1 | sí | ⬜ Pendiente | | |
| HU-08 | Cargar el horario del taller | T1 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-09 | Registrar al cliente por su celular | T1 | sí | 🟦 En curso | | Los 3 criterios con prueba, incluida concurrencia del mismo celular. Construida como parte de la rebanada del Sprint 0 |
| HU-10 | Registrar el vehículo por su placa, sea carro o moto | T1 | sí | 🟦 En curso | | Los 6 criterios con prueba (dominio + HTTP). Formatos de moto provisionales (Q-01) |
| HU-11 | Guardar los datos del vehículo y de su motor | T1 | no | 🟦 En curso | | Criterios 1–5 con prueba. Falta el criterio 6 (aviso de kilometraje menor al de la última orden) |
| HU-12 | Un cliente con varios vehículos y vehículos que cambian de dueño | T1 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-13 | Ver todo lo que se le ha hecho a un vehículo | T1 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-14 | Buscar desde cualquier pantalla | T1 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-15 | Unir clientes o vehículos repetidos | T6 | no | ⬜ Pendiente | | |
| HU-16 | Abrir una orden desde la placa | T1 | sí | 🟦 En curso | | Los 2 criterios con prueba; números consecutivos probados con 25 órdenes simultáneas |
| HU-17 | Hacer el checklist de recepción en el celular (carro) | T1 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-18 | Hacer el checklist de recepción de una moto | T1 | sí | ⬜ Pendiente | | |
| HU-19 | Recibir la constancia con fotos y confirmarla | T2 | sí | ⬜ Pendiente | | |
| HU-20 | Mover la orden por sus estados | T1 | sí | 🟦 En curso | | Código ya existe (dominio, caso de uso, HTTP). Criterios 1–4 con prueba: dominio (`orders/domain/flow_test.go`) y HTTP (`server/flow_test.go`: saltos, 409 por versión, falta de cotización, historial con usuario y hora). 2026-11-02: corregido que devolverse a Recibido pedía el checklist. Falta: prueba del permiso del mecánico (sin anulación del dueño) y pantalla del piso |
| HU-21 | Agregar fotos y notas en cualquier momento | T1 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-22 | Asignar mecánico y fecha prometida | T6 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-23 | Ver mis órdenes en el celular | T1 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-24 | Ver la conversación de WhatsApp dentro de la orden | T2 | sí | ⬜ Pendiente | | |
| HU-25 | Entregar el vehículo | T1 | sí | ⬜ Pendiente | | |
| HU-26 | Cancelar una orden que no siguió | T6 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-27 | Armar la cotización con servicios y repuestos | T1 | sí | ⬜ Pendiente | | |
| HU-28 | Aprobar la cotización desde WhatsApp | T2 | sí | ⬜ Pendiente | | |
| HU-29 | Registrar la respuesta del cliente y mover la orden | T2 | sí | ⬜ Pendiente | | |
| HU-30 | Aprobar ítem por ítem | T6 | no | ⬜ Pendiente | | |
| HU-31 | Cotización adicional durante la reparación | T6 | no | ⬜ Pendiente | | |
| HU-32 | Crear, editar y desactivar repuestos | T1 | sí | ⬜ Pendiente | | |
| HU-33 | Descontar el repuesto al aprobarse la orden | T2 | sí | ⬜ Pendiente | | |
| HU-34 | Buscar repuestos al cotizar | T1 | sí | ⬜ Pendiente | | |
| HU-35 | Registrar entrada de mercancía | T6 | no | ⬜ Pendiente | | |
| HU-36 | Recibir alerta de repuesto bajo el mínimo | T6 | no | ⬜ Pendiente | | |
| HU-37 | Importar inventario desde Excel o CSV | T6 | no | ⬜ Pendiente | | |
| HU-38 | Devolver al inventario lo que no se usó | T6 | no | ⬜ Pendiente | | |
| HU-39 | Proveedores, órdenes de compra y bodegas | T7 | no | ⬜ Pendiente | | |
| HU-40 | Ver el catálogo de automatizaciones | T4 | sí | ⬜ Pendiente | | |
| HU-41 | Encender y apagar una automatización | T2 | sí | ⬜ Pendiente | | |
| HU-42 | Editar el texto de un aviso | T4 | no | ⬜ Pendiente | | |
| HU-43 | Saber que mi vehículo está listo | T2 | sí | ⬜ Pendiente | | |
| HU-44 | Recordatorio de la cita | T6 | no | ⬜ Pendiente | | |
| HU-45 | Recordatorio de vehículo sin recoger | T6 | sí | ⬜ Pendiente | | |
| HU-46 | Registrar cada ejecución de una automatización | T2 | sí | ⬜ Pendiente | | |
| HU-47 | Respetar la ventana de 24 horas | T2 | no | ⬜ Pendiente | | |
| HU-48 | Enviar el aviso correcto al cambiar el estado | T2 | sí | ⬜ Pendiente | | |
| HU-49 | Seguimiento de garantía y garantía por orden | T7 | no | ⬜ Pendiente | | |
| HU-50 | Recordatorio de técnico-mecánica por tipo de vehículo | — | — | ⛔ Cancelada | | |
| HU-51 | Escribirle al taller en mis palabras | T3 | sí | ⬜ Pendiente | | |
| HU-52 | Entender qué pide el cliente | T3 | sí | ⬜ Pendiente | | |
| HU-53 | Redactar el borrador solo con datos del taller | T3 | sí | ⬜ Pendiente | | |
| HU-54 | Validar todo lo que sale de la IA | T3 | no | ⬜ Pendiente | | |
| HU-55 | Crear la orden cuando llega la placa | T3 | sí | ⬜ Pendiente | | |
| HU-56 | Preguntar "¿ya está mi carro?" o "¿ya está mi moto?" | T3 | sí | ⬜ Pendiente | | |
| HU-57 | Preguntar cuál vehículo si hay más de uno | T3 | no | ⬜ Pendiente | | |
| HU-58 | Hablar solo del taller | T3 | no | ⬜ Pendiente | | |
| HU-59 | Pedir hablar con una persona | T3 | no | ⬜ Pendiente | | |
| HU-60 | Mandar notas de voz | T6 | no | ⬜ Pendiente | | |
| HU-61 | Respuesta fuera de horario | T6 | no | ⬜ Pendiente | | |
| HU-62 | Revisar y enviar el borrador | T3 | sí | ⬜ Pendiente | | |
| HU-63 | Editar el borrador antes de enviarlo | T3 | no | ⬜ Pendiente | | |
| HU-64 | Saber de dónde salió cada dato | T3 | no | ⬜ Pendiente | | |
| HU-65 | Ver qué tan seguido edito los borradores | T3 | no | ⬜ Pendiente | | |
| HU-66 | Enterarme de que hay algo por revisar | T6 | no | ⬜ Pendiente | | |
| HU-67 | Auto-envío para respuestas exactas | T7 | no | ⬜ Pendiente | | |
| HU-68 | Ver órdenes abiertas por estado | T4 | sí | ⬜ Pendiente | | |
| HU-69 | Ver aprobaciones pendientes y cuánto llevan | T4 | sí | ⬜ Pendiente | | |
| HU-70 | Ver el tiempo promedio por orden | T4 | sí | ⬜ Pendiente | | |
| HU-71 | Ver el ticket promedio | T4 | sí | ⬜ Pendiente | | |
| HU-72 | Ver cuántos clientes vuelven | T6 | no | ⬜ Pendiente | | |
| HU-73 | Ver cuánto me demoro en contestar WhatsApp | T4 | sí | ⬜ Pendiente | | |
| HU-74 | Ver el consumo de mensajes y el costo de IA de cada taller | T4 | sí | ⬜ Pendiente | | |
| HU-75 | Preguntar qué significa un número | T6 | sí | ⬜ Pendiente | | |
| HU-76 | Ver qué tengo pendiente hoy | T6 | sí | ⬜ Pendiente | | |
| HU-77 | Que el asistente también guíe al mecánico | T7 | no | ⬜ Pendiente | | |
| HU-78 | Hacerle preguntas libres al asistente | T7 | no | ⬜ Pendiente | | |
| HU-79 | Que mis datos no los vea otro taller | T0 | no | 🟦 En curso | | RLS forzado + `SET LOCAL` probado (incluida meta-prueba de todas las tablas) y 404 entre talleres por HTTP. Falta: "el intento queda registrado" (criterio 1) y repetir contra el pooler de Supabase |
| HU-80 | Agregar a mi mecánico | T0 | sí | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-81 | Ver solo lo que me toca como mecánico | T0 | sí | ⬜ Pendiente | | |
| HU-82 | Trabajar como recepción | T6 | no | ⬜ Pendiente | | |
| HU-83 | Ocultar la gestión de usuarios si trabajo solo | T6 | no | ⬜ Pendiente | | |
| HU-84 | Desactivar a un usuario que se va | T6 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-85 | Dar mi autorización antes de que procesen mi mensaje | T2 | sí | ⬜ Pendiente | | |
| HU-86 | Pedir la autorización en el mostrador | T2 | no | 🟦 En curso | | Código y prueba HTTP de punta a punta (2026-11-02). Falta revisar criterio por criterio, pantalla y permisos por rol |
| HU-87 | Pedir mis datos o que los borren | T6 | no | ⬜ Pendiente | | |
| HU-88 | Ver quién cambió qué | T0 | no | 🟦 En curso | | Bitácora solo-inserción escrita en la misma transacción (taller, usuario, cliente, vehículo, motor, orden). Falta la pantalla con filtros (criterio 2) y la cadena de hashes (ADR-17) |
| HU-89 | Aceptar el contrato de transmisión con SWEVEH | T0 | no | ⬜ Pendiente | | |
| HU-90 | Publicar la política de tratamiento de datos | T0 | sí | ⬜ Pendiente | | |
| HU-91 | Llenar el sistema mientras trabajo | T1 | no | ⬜ Pendiente | | |
| HU-92 | Subir mis clientes y vehículos con una plantilla | T6 | no | ⬜ Pendiente | | |
| HU-93 | Pasar mi lista de precios con una foto | T6 | sí | ⬜ Pendiente | | |
| HU-94 | Subir mi propio Excel con mapeo de columnas | T7 | no | ⬜ Pendiente | | |
| HU-95 | No escribirle a clientes importados sin autorización | T2 | no | ⬜ Pendiente | | |
| HU-96 | Traer cuadernos completos y chats viejos | T7 | no | ⬜ Pendiente | | |
| HU-97 | Reiniciar el taller ficticio entre demos | T5 | sí | ⬜ Pendiente | | |
| HU-98 | Agregar una moto al taller ficticio | T5 | sí | ⬜ Pendiente | | |
| HU-99 | Ver costo, tokens y latencia de cada llamada a la IA | T3 | sí | ⬜ Pendiente | | |
| HU-100 | Ver el registro técnico de mensajes | T6 | no | ⬜ Pendiente | | |
| HU-101 | Cambiar del número de prueba al número del taller | T6 | no | ⬜ Pendiente | | |
| HU-102 | Probar el agente con 30 mensajes reales de estilo | T3 | no | ⬜ Pendiente | | |

## Bitácora de sesiones (la más reciente arriba)

| Fecha | Quién (persona o agente) | Qué se hizo | Qué quedó pendiente |
|---|---|---|---|
| 2026-11-01 | Claude | Se halló que el repo va por delante de este cuadro (existen `flow`, checklist, stats y cotizaciones en `orders`; el cuadro de HU puede estar desactualizado). Se añadieron pruebas de dominio de HU-20 (`go test` verde; `-race` no corrió: esta máquina no tiene cgo). | Auditar qué HU de T1 ya están en código y corregir este cuadro; pruebas HTTP de HU-20 |
| 2026-11-02 | Claude | `arch_test.go` regla 5: `internal/wire` exento como composition root (igual que `server`). Suite completa con Postgres local y `-race` (gcc WinLibs instalado; `CGO_ENABLED=1` con `go env -w`): todo verde **salvo** `TestRouterMatchesOpenAPIContract`, porque hay 36 rutas en el router que faltan en `api/openapi.yaml` (cotización, inventario, servicios, usuarios, taller, checklist, evidencias, transición, búsqueda…) | Documentar las 36 rutas en OpenAPI y regenerar el cliente TS; corregir el cuadro de HU |
| 2026-11-02 | Claude (rama `developer`, PR #1) | Commits por sesión y push; CI en GitHub. Arreglos para ponerlo en verde: modelos sqlc regenerados, 9 hallazgos de golangci-lint (incluye el `hub.challenge` del webhook que se reflejaba sin validar), Go en el job de Snyk. Cobertura R-11 de 26–70 % a ≥ 82 % en todos los paquetes vigilados con pruebas HTTP de recepción, evidencias, asignación, historial, cancelación, taller, usuarios, clientes y cambio de dueño; se quitó `Service.Recent` sin uso | Merge del PR #1; pruebas de cotización, inventario, servicios, mensajería y KPIs |
| 2026-11-02 | Claude | `api/openapi.yaml` con las 36 rutas faltantes (órdenes: flujo, checklist, evidencias; cotización; inventario; servicios; usuarios; taller; búsqueda; historial) y campos nuevos de `Orden`. Cliente TS regenerado. **Verde:** `go test -race ./...` con Postgres (incluido el contrato), `tsc`, ESLint y Vitest (14). Enums sin confirmar en el código (`decision`, `tipo` de línea, `atiende`, `tamano`) quedaron como texto libre | Las rutas de `wire` y `analytics` (conversaciones, bandeja, automatizaciones, auditoría, KPIs) no se montan en el router de pruebas y tampoco están en el contrato; corregir el cuadro de HU |
| 2026-11-01 | Claude (rama `chore/sprint-0-cimientos`, sin commit) | **Sprint 0 en local.** Estructura hexagonal (5 módulos + audit/outbox), 7 migraciones con RLS forzado, rebanada HU-01 → HU-09 → HU-10/11 → HU-16 de punta a punta (API + pantallas de registro, ingreso, lista, nueva orden y detalle), contrato OpenAPI, `Makefile`, `Dockerfile`, CI escrito, ADR-0022 (Cloud Run) y ADR-0023 (herramientas). **Probado:** `go test -race` (dominio, RLS, 8+5 pruebas HTTP de punta a punta, concurrencia), cobertura dominio/aplicación ≥ 80 % en todos los módulos (`scripts/coverage.sh`), `golangci-lint` 0 hallazgos, test de arquitectura y `depguard` verificados con una violación de prueba, `govulncheck` sin vulnerabilidades, `gitleaks` sin filtraciones, `tsc`/ESLint/Vitest (14) y build del frontend en verde, imagen de 52,5 MB construida, prueba de humo con curl y capturas con Playwright del flujo completo | Push y primer run en GitHub; cuentas (GCP, Supabase staging, Sonar, Snyk); RLS contra el pooler; pruebas de pantallas del frontend (cobertura web 16 %); HU-11 criterio 6; Q-29…Q-32 para el equipo |
| 2026-11-01 | Claude | Isaac decidió AQ-01 (Cloud Run), AQ-02 (`sa-east-1`), AQ-04 (auth propia); AQ-12 queda provisional sin fecha. Registrado en `docs/07`. Inconsistencia detectada: este cuadro lista HU-102 pero el alcance dice 101 HU (sin corregir; requiere confirmar alcance) | ADR Cloud Run; AQ-10; aclarar HU-102 |
| 2026-11-01 | Claude | Se redactó el paquete de documentación (`CLAUDE.md` + `docs/`): contexto, reglas de decisión, MVP con errata, 101 HU, arquitectura, preguntas abiertas | Iniciar el Sprint 0 |
