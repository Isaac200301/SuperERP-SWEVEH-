# CLAUDE.md — SWEVEH (by WEVEH Group)

> Punto de entrada para cualquier agente que construya el MVP de SWEVEH. Está escrito para que **no tengas que preguntar nada que ya esté decidido**, y para que sepas con precisión **cuándo sí debes parar y preguntar**.
> Idioma de trabajo con Isaac y Pablo: **español**. Última actualización: 2026-11-01.

## 1. Qué estás construyendo (en 30 segundos)

**SWEVEH** ("Suebe") es una **suscripción tipo Netflix para talleres de carros y motos**: el taller se registra solo en menos de 5 minutos y recibe todo listo (órdenes de servicio con checklist y fotos, cotización aprobada por WhatsApp, inventario básico, un catálogo de automatizaciones con interruptor, un agente de WhatsApp, 6 KPIs y roles). Resuelve la **barrera de entrada de las pymes a un sistema confiable y a la IA**. **No es un ERP** y no compite con los grandes ERP; **el agente de WhatsApp es el demo** de cómo se automatiza un proceso, no el producto completo.

- Equipo y únicos aprobadores: **Isaac Cano Orozco** y **Pablo Luis Rodríguez Burgos** (estudiantes de Ingeniería de Sistemas, WEVEH Group).
- Fase: preparación de la construcción. **Todavía no hay código de la aplicación.** Meta inmediata: **Sprint 0** y luego la **demo** (10 escenas, ~15 min, taller ficticio "Servitaller El Pistón", número de prueba de Meta).
- **No hay fecha de demo ni dedicación semanal** (`Sin dato`, AQ-12): el plan se ordena por dependencias, no por calendario.

## 2. Mapa de documentos (todo lo necesario está en `docs/`)

| Documento | Para qué |
|---|---|
| `docs/01-producto-y-contexto.md` | Visión, decisiones del equipo **DE-01…DE-23**, competencia, demo, lo legal y de plataforma |
| `docs/02-reglas-de-decision.md` | **Quién manda, qué decides solo, cuándo parar, reglas R-01…R-17** |
| `docs/03-mvp-requisitos.md` | Módulos M1–M11, **RF-01…22**, **RNF-01…14**, taller ficticio, guion de 10 escenas, riesgos, **CA-01…12**. Empieza con la **ERRATA VIGENTE** (manda sobre el resto) |
| `docs/04-historias-de-usuario.md` | **101 HU** (todas Must) en 14 épicas con criterios Dado/Cuando/Entonces. *Lee la HU antes de construirla* |
| `docs/05-orden-de-construccion.md` | Tramos T0…T7, niveles de dependencia, **Sprint 0**, variables de entorno, comandos `make`, plantilla de PR |
| `docs/06-arquitectura.md` | Arquitectura completa (16 secciones, 21 ADR, diagramas Mermaid) |
| `docs/07-preguntas-abiertas.md` | Q-01…Q-28, D-01…D-13, AQ-01…AQ-12, decisiones provisionales y tomadas |
| `docs/08-glosario-y-convenciones.md` | Glosario, nombres, formatos (placa, celular, COP, fechas), pruebas |
| `docs/09-estado-de-construccion.md` | Estado por HU, qué sigue, bitácora de sesiones |
| `docs/10-mockups-y-diseno.md` | **Diseño de la UI:** tokens (paleta WEVEH), reglas DR-01…DR-14, patrones por superficie, trazabilidad HU → pantalla |
| `docs/mockups/` | 11 mockups HTML (landing, registro, panel, orden, Por revisar, automatizaciones, piso ×2, WhatsApp ×2, admin) + imágenes de referencia |
| `docs/adr/` | Plantilla e índice de ADR |
| `docs/data/historias.json` | Las HU en JSON |

**Importados en cada sesión** (son los que mandan; los demás se leen según la tarea):

@docs/02-reglas-de-decision.md
@docs/09-estado-de-construccion.md

**Lee a demanda (no importados por tamaño):** `docs/01` al empezar; `docs/04` la(s) HU que construyes y sus dependencias; `docs/06` las secciones que toquen tu HU; `docs/07` las preguntas que la afecten; `docs/08` al nombrar o validar; `docs/10` y su mockup antes de construir cualquier pantalla.

## 3. Protocolo de sesión

**Al abrir (R-17):**
1. Lee este archivo, `docs/02` y `docs/09`.
2. Confirma el **tramo actual** y cuál es la **siguiente HU**; no saltes dependencias (`docs/05`).
3. Lee esa HU completa, sus dependencias, las preguntas abiertas que la afectan (`docs/07`) y las secciones de `docs/06` que apliquen.
4. Verifica que está "lista para construir" (R-03). Si no, di qué falta.

**Mientras trabajas:** una rebanada vertical a la vez (R-01); PR pequeños; pruebas junto al código; documentación en el mismo PR.

**Al cerrar:**
1. Actualiza `docs/09` (estado de las HU, "qué sigue", bitácora) y, si aplica, `docs/07` y los ADR.
2. Deja el pipeline en verde o explica con la salida real por qué no.
3. Resume a Isaac/Pablo: qué hiciste, qué probaste, qué decidiste provisionalmente, qué necesitas de ellos.

## 3.1 Skills del proyecto (`.claude/skills/`)

Claude Code las carga solas según su descripción; también puedes invocarlas por nombre. **Antes de escribir código, abre la skill que corresponda**: traducen las reglas de `docs/02` y la arquitectura de `docs/06` a recetas con ejemplos en Go y TypeScript. Si una skill contradice un documento, gana el documento (`docs/02` §8) y se corrige la skill.

| Skill | Úsala cuando… |
|---|---|
| `sweveh-implementar-historia` | vayas a construir una HU o abrir/cerrar sesión (flujo completo, definición de "Terminada") |
| `sweveh-arquitectura-hexagonal` | crees o cambies un módulo backend (capas, puertos, errores, composition root) |
| `sweveh-multitenant-rls` | toques tablas o acceso a datos (`taller_id`, RLS, prueba con dos talleres) |
| `sweveh-db-migraciones-sqlc` | escribas migraciones o consultas sqlc |
| `sweveh-concurrencia-idempotencia` | toques inventario, órdenes, cotizaciones, webhooks, colas o llamadas externas |
| `sweveh-seguridad-auth` | toques login, sesiones, permisos, cabeceras o endpoints |
| `sweveh-whatsapp-ia` | toques WhatsApp, el agente de IA o automatizaciones |
| `sweveh-observabilidad-auditoria` | añadas logs, métricas, trazas o eventos de auditoría |
| `sweveh-frontend-react` | crees o cambies pantallas del panel o del piso |
| `sweveh-testing` | escribas o revises pruebas |
| `sweveh-ci-cd-calidad` | toques workflows, Sonar, Snyk, Docker o despliegue |
| `sweveh-revision-codigo` | vayas a abrir un PR o a revisar el de la otra persona |
| `clean-code-solid` | escribas o refactorices código (Clean Code, SOLID, DRY/KISS/YAGNI, Demeter) |
| `principios-poo` | modeles entidades, objetos de valor, agregados y reglas de negocio |
| `patrones-gof` | dudes si aplicar un patrón de diseño (los 23 GoF y cuándo NO usarlos) |
| `grasp` | decidas qué módulo o tipo debe tener una responsabilidad |

## 4. Las 10 reglas que más importan (el detalle y el resto están en `docs/02`)

1. **Precedencia:** instrucción actual de Isaac/Pablo > decisiones `Equipo` > HU y RF/RNF > arquitectura `Propuesta` > otras propuestas > tu criterio. Los conflictos se registran, no se resuelven en silencio.
2. **Lo del equipo no se toca** (DE-xx, ADR-01…07). Si crees que está mal, lo dices con evidencia y sigues cumpliéndolo.
3. **Para y pregunta si:** cambias alcance (crear/borrar/cambiar prioridad de una HU), gastas dinero o creas cuentas, usas datos reales, relajas seguridad/privacidad/aislamiento, despliegas a producción o haces push forzado, tocas una pregunta ⛔, o algo no se puede deshacer.
4. **Si se puede deshacer** después de que alguien lo vea: actúa, di qué hiciste y qué lectura elegiste.
5. **Preguntas abiertas:** si no son ⛔ y tienen recomendación, aplícala como **decisión provisional** (regístrala en `docs/07`) y déjala fácil de cambiar.
6. **Nada de suposiciones sin respaldo:** afirmaciones con fuente (URL) o `Sin dato`. No inventes precios, normas, cifras ni fuentes.
7. **Multi-tenant en dos capas:** `taller_id` de la sesión + RLS forzado; cada tabla nueva trae su prueba de aislamiento con dos talleres.
8. **Toda entrada externa es idempotente** y todo cambio de estado escribe **auditoría + evento en la misma transacción**.
9. **La IA propone, el código valida, el humano aprueba:** nada redactado por la IA sale sin que el dueño pulse Enviar; nunca un precio fuera de la lista del taller (RF-17, RF-18).
10. **Honestidad:** reporta fallos con su salida real; no digas "terminado" sin haber corrido las pruebas; no desactives pruebas ni SonarQube/Snyk para ponerlas en verde.

## 5. Decisiones del equipo que debes tener presentes (resumen de `docs/01`, sección 5)

- Talleres de **carros y motos**. **Sin VIN.** Todo vehículo tiene **ficha** (DE-17, 3-nov): combustible, caja, cilindraje y aspiración (o 2T/4T en moto), más tracción, carrocería, refrigeración, color y llanta opcionales; "ficha pendiente" hasta pasar a Recibido.
- **Todas las HU son Must** (101 vigentes; **HU-50 cancelada**). **HU-74** (medidor de mensajes y costo de IA) es del **Administrador del ERP**, no del taller.
- **Solo WhatsApp** (demo con número de prueba de Meta). Facturación: **integrar Siigo/Alegra, no construir**.
- **Multi-tenant**, identidad por **celular** + OTP, **Ley 1581** (autorización previa), IA con límites y borradores, capa **multi-proveedor** con costo por conversación.
- **Corrección frente al documento del MVP del 30-oct:** backend = **Go** (no Spring Boot), base = **Supabase**, sin VIN, motos dentro, todo Must (ver errata en `docs/03`).

## 6. Stack y arquitectura en una pantalla (detalle: `docs/06`)

| Capa | Decisión | Origen |
|---|---|---|
| Arquitectura | **Monolito modular** con capas hexagonales (`domain` / `app` / `ports` / `adapters`), reglas de dependencia forzadas con `depguard` y test de arquitectura | **Equipo** |
| Backend | **Go**; propuesta: `chi`, `pgx` v5, `sqlc` (sin ORM), `oapi-codegen`, `golang-jwt` v5, `slog`, OpenTelemetry, outbox con `FOR UPDATE SKIP LOCKED` | Equipo (lenguaje) / Propuesta (librerías) |
| Base de datos | **Supabase** Postgres + Storage privado; un esquema por módulo; RLS; pooler en modo sesión; Data API desactivada | Equipo / Propuesta |
| Frontend | **React + TypeScript + Vite + Tailwind CSS**; TanStack Query, React Router, RHF + Zod, cliente generado desde OpenAPI, PWA para "Piso" | Equipo / Propuesta |
| Sesión | **JWT** ES256 de 10 min + refresh opaco rotativo; cookies `__Host-`/`__Secure-` `HttpOnly` `SameSite=Strict`; CSRF por encabezado + `Origin`; **RBAC** `recurso:acción`; dos *realms* (taller y Admin ERP) | **Equipo** / Propuesta (parámetros) |
| Transversales | Seguridad, observabilidad, auditoría inmutable, resiliencia (timeouts, disyuntores, degradación), control de concurrencia, idempotencia | **Equipo** |
| Despliegue | Una imagen (Go + SPA servido por el api), procesos `api` y `worker`, mismo origen, proxy/WAF delante; **Google Cloud Run** `southamerica-east1` (AQ-01, ADR-0022) | **Equipo** (plataforma) / Propuesta (configuración) |
| CI/CD | **GitHub Actions** + **SonarQube Cloud** + **Snyk**; además govulncheck y gitleaks; acciones fijadas por SHA; producción con aprobación manual | **Equipo** / Propuesta |

### Estructura objetivo del repositorio (`SuperERP-SWEVEH-/`)

```
├── CLAUDE.md
├── .claude/skills/          # 16 skills del proyecto (ver 3.1)
├── README.md
├── docs/                    # esta documentación (viva)
├── cmd/api/  cmd/worker/    # binarios (composition root)
├── internal/
│   ├── platform/            # config, db, logging, otel, httpx, authn, clock
│   ├── identity/ tenancy/ customers/ vehicles/ orders/ quotes/ inventory/
│   │   └── {domain,app,ports,adapters/{in/http,out/postgres}}   # misma forma por módulo
│   └── shared/              # solo tipos neutros: IDs, Money, errores, eventos
├── api/openapi.yaml         # contrato (fuente de verdad de la API)
├── db/migrations/  db/queries/
├── web/                     # React + TS + Vite + Tailwind
├── deploy/                  # Dockerfile, fly.toml/manifiesto
├── .github/workflows/       # ci.yml, deploy.yml
└── Makefile
```

## 7. Comandos objetivo (se crean en el Sprint 0; `docs/05`, sección 8)

`make dev` · `make test` · `make lint` · `make gen` · `make migrate` · `make cover` · `make ci` · `make image` existen desde el Sprint 0 (`make help` los lista; en Windows, desde Git Bash). `make e2e` corre el recorrido de la orden en Chromium con el api sirviendo la SPA (`scripts/e2e.sh`, lo mismo que el job `e2e` del CI). `make seed-demo` y `make reset-demo` **todavía no existen**: fallan a propósito con un mensaje hasta que lleguen HU-97 y HU-98 (T5).

## 8. Dónde estamos y qué sigue

- Estado detallado: `docs/09-estado-de-construccion.md` (importado arriba).
- Primer paso: **Sprint 0** (`docs/05`, sección 4): estructura del repositorio y reglas de arquitectura → CI mínimo → Supabase de staging y **prueba de RLS + `SET LOCAL` + pooler** → rebanada vertical (HU-01 → HU-09/10/11 → HU-16) → despliegue a staging → pruebas de aislamiento y concurrencia → decidir las preguntas ⛔.
- **Decididas el 2026-11-01:** AQ-01 = **Cloud Run** (ADR-0022), AQ-02 = **Supabase `sa-east-1`** (condicionada a AQ-10 para datos reales), AQ-04 = **auth propia**. AQ-12 provisional: sin fecha, plan por dependencias.
- **Bloqueo ⛔ abierto:** AQ-10 (asesoría jurídica de datos), antes de usar datos reales. Crear cuentas (GCP, Supabase, SonarQube Cloud, Snyk) sigue requiriendo aprobación de Isaac/Pablo por costo.

## 9. Qué NO hacer (lista corta; la completa está en `docs/02`, sección 3.4)

- No inventes datos, fuentes, precios ni normas. No uses datos reales. No pongas secretos en el repo ni en logs.
- No guardes VIN. No guardes tokens en `localStorage`. No quites RLS ni uses `BYPASSRLS` en la API.
- No dejes que la IA envíe sin aprobación ni cite precios fuera de lista.
- No construyas lo que el equipo dejó fuera: ERP completo, DIAN/contabilidad, Instagram/Facebook, diagnóstico con IA, marketplace, constructor visual de automatizaciones, pagos en línea, nómina, multi-sede, app nativa, portal del cliente.
- No hagas merge ni despliegues a producción por tu cuenta. No reescribas la historia de `main`.
- No cambies el alcance (HU) ni las decisiones `Equipo` sin instrucción explícita.

## 10. Referencias del proyecto

Originales (artefactos): MVP https://claude.ai/artifact/9Cbp2G4mjX6rkVbJyUjCcq · Historias de usuario https://claude.ai/artifact/XENupGcm9WgWp9aKvb4rYy · Arquitectura https://claude.ai/artifact/LUQHmSChKP37B22XQAAgXU · Más fuentes y URLs oficiales en `docs/01` (sección 12), `docs/06` (sección 16) y `docs/04` (referencias). Regla heredada del equipo: **toda afirmación factual nueva lleva URL o se marca `Sin dato`.**
