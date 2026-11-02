---
name: sweveh-implementar-historia
description: Flujo paso a paso para construir una historia de usuario (HU) de SWEVEH de principio a fin: leer estado y dependencias, preguntas abiertas, rebanada vertical, pruebas, PR y actualización de docs/09. Úsala cada vez que vayas a implementar una HU o abrir/cerrar una sesión de trabajo.
---

# Implementar una HU de SWEVEH

Esta skill aplica `docs/02` (R-01, R-03, R-04, R-12, R-14, R-15, R-17) y `docs/05` (tramos T0..T7). Léela completa al iniciar una HU.

## 1. Abrir la sesión (R-17)

1. Lee `CLAUDE.md` y `docs/09-estado-de-construccion.md` (qué está hecho, qué sigue, bloqueos).
2. Localiza la HU en `docs/04-historias-de-usuario.md` (y `docs/data/historias.json`): rol, superficie (panel/piso/WhatsApp/motor), criterios, dependencias, RF/escenas relacionadas.
3. Confirma el **tramo** (T0..T7) en `docs/05`. Si las dependencias no están terminadas, **no** construyas: construye primero la dependencia (nunca por conveniencia, `docs/02` §6).
4. Busca preguntas abiertas (Q-xx, D-xx, AQ-xx) en `docs/07` que toquen la HU. Aplica R-04.

## 2. Clasificar la HU: "lista para construir" o no (R-03)

| Estado | Condición | Acción |
|---|---|---|
| **Lista** | criterios claros, dependencias hechas, sin pregunta ⛔ | construir |
| **Con decisión provisional** | hay una Q-xx con recomendación | aplicar la recomendación, dejar el comportamiento fácil de cambiar, anotar en el PR y en `docs/07` |
| **Bloqueada** | depende de AQ-01/02/04/10/12 (⛔) o cambia alcance/seguridad/dinero | **parar y preguntar** a Isaac o Pablo (`docs/02` §3.3) y trabajar en otra HU |

Recuerda las erratas vigentes (`docs/03`): **sin VIN**; todo vehículo tiene motor (`cilindraje_cc`, `turbo`); HU-74 es del Administrador del ERP; HU-50 cancelada; todas las HU son *Must*.

## 3. Diseñar la rebanada vertical (R-01)

Una HU = una rebanada que atraviesa: contrato OpenAPI → dominio → caso de uso → SQL/migración → adaptador HTTP → UI → pruebas. Evita capas a medias.

Anota antes de codificar (en el PR o en una nota corta):
- Qué criterios de aceptación cubre (copiar los de la HU).
- Qué reglas de dominio y qué errores tipados.
- Si toca: multi-tenant (`sweveh-multitenant-rls`), concurrencia (`sweveh-concurrencia-idempotencia`), seguridad (`sweveh-seguridad-auth`), WhatsApp/IA (`sweveh-whatsapp-ia`), datos personales (R-10).

## 4. Orden de trabajo recomendado

1. **Rama:** `feat/HU-27-aprobar-cotizacion` (R-14).
2. **Contrato:** `api/openapi.yaml` → `make gen`.
3. **Pruebas primero (o en paralelo):** convierte cada criterio de aceptación en al menos una prueba con nombre legible (`TestApproveQuote_RejectsWhenVersionMismatch`). Ver `sweveh-testing`.
4. **Dominio** (sin I/O) → **caso de uso** → **migración + consultas sqlc** (`sweveh-db-migraciones-sqlc`) → **adaptadores**.
5. **Frontend** (`sweveh-frontend-react`) con estados de carga/error/vacío.
6. **Auditoría y observabilidad** (`sweveh-observabilidad-auditoria`): evento de auditoría, métricas, logs sin PII.
7. **Refactor** con las pruebas en verde (`clean-code-solid`, `grasp`, `patrones-gof` solo si hay dolor real).
8. **`make ci`** local completo. Solo entonces abre el PR.

## 5. Definición de "Terminada" (R-03, R-11)

- [ ] Todos los criterios de aceptación tienen prueba que pasa.
- [ ] Pruebas de aislamiento (si hay tablas nuevas) y de concurrencia (si aplica) incluidas.
- [ ] Cobertura de código nuevo ≥ 80 % en dominio y aplicación.
- [ ] `make ci` verde: `go test -race`, lint (depguard), arquitectura, `tsc`, ESLint, Vitest, govulncheck, gitleaks.
- [ ] Contrato OpenAPI y código generado sincronizados.
- [ ] Documentación actualizada en el **mismo PR** (R-12): `docs/09` (estado), `docs/04`/`historias.json` solo si el equipo cambió alcance, `docs/07` si hay decisión provisional nueva, ADR si cumple R-13.
- [ ] Sin secretos, sin datos reales, sin PII en logs.
- [ ] Probada la ruta feliz y los errores en un entorno real (local o staging), no solo en pruebas.
- [ ] Si es parte de la demo: la escena correspondiente funciona (`docs/05`, guion de 10 escenas).

## 6. El PR (R-14)

- Título: `HU-27: aprobar cotización por botón de WhatsApp` (español, imperativo, id de HU).
- Descripción: qué HU/criterios cubre, cómo se probó (comandos y salida real), riesgos, decisiones provisionales, docs tocados, capturas si hay UI.
- Pequeño (< 400 líneas orientativo); si crece, divide en rebanadas.
- **No hagas merge** ni despliegues a producción; la revisión la hace la otra persona del equipo.
- Si CI falla: reproducir, corregir la causa, **nunca** desactivar pruebas ni bajar umbrales (`docs/02` §7).

## 7. Cuándo PARAR y preguntar (resumen de `docs/02` §3.3)

Cambia alcance o criterios de una HU; gasta dinero o activa un plan de pago; usa datos reales de personas; relaja seguridad; despliega a producción; toca una pregunta ⛔ (AQ-01, AQ-02, AQ-04, AQ-10, AQ-12); contradice una decisión del equipo; el plan B de RLS. Pregunta con: contexto, opciones, recomendación y qué haces mientras tanto.

## 8. Qué NO hacer

Saltarte dependencias; añadir funcionalidad "ya que estamos"; renumerar o reutilizar ids de HU (R-12); cambiar decisiones del equipo (R-02); marcar terminada sin correr pruebas (R-15); dejar TODO sin Q-xx asociado; construir contabilidad/DIAN/nómina (fuera de alcance).

## 9. Cerrar la sesión (R-17)

1. Actualiza `docs/09`: estado de las HU tocadas (por construir / en curso / en revisión / terminada / bloqueada), enlace al PR.
2. Anota decisiones provisionales y preguntas nuevas (Q-xx) en `docs/07`.
3. Escribe en la bitácora qué sigue y qué quedó pendiente, con honestidad (R-15).
4. Deja el pipeline en verde o explica con evidencia por qué no.

## 10. Plantilla de nota de bitácora (ejemplo ilustrativo: ids y preguntas inventados para mostrar el formato)

```
### 2026-11-DD · HU-27 aprobar cotización
- Hecho: dominio, caso de uso, endpoint POST /quotes/{id}/approve, pruebas (12), concurrencia (If-Match).
- Decisión provisional: Q-12 → recomendación aplicada (monto límite configurable).
- Falta: botón en WhatsApp (depende de HU-33).
- Riesgo: ninguno nuevo. CI: verde (enlace).
- Siguiente: HU-33.
```
