# Documentación de SWEVEH

Orden de lectura para quien llega nuevo (persona o agente):

| # | Documento | Qué contiene | Cuándo leerlo |
|---|---|---|---|
| — | [`../CLAUDE.md`](../CLAUDE.md) | Punto de entrada del agente: contexto, reglas, mapa y protocolo de sesión | Siempre primero |
| 01 | [`01-producto-y-contexto.md`](01-producto-y-contexto.md) | Qué es SWEVEH, decisiones del equipo (DE-xx), competencia, demo, lo legal | Antes de tocar código |
| 02 | [`02-reglas-de-decision.md`](02-reglas-de-decision.md) | Quién manda, qué decide el agente solo, cuándo parar, reglas R-01…R-17, árbol de decisión | **Antes de cada decisión** |
| 03 | [`03-mvp-requisitos.md`](03-mvp-requisitos.md) | Módulos, RF-01…22, RNF-01…14, taller ficticio, guion de 10 escenas, riesgos, CA-01…12 (+ **errata vigente**) | Al diseñar una funcionalidad o la demo |
| 04 | [`04-historias-de-usuario.md`](04-historias-de-usuario.md) | Las 101 HU con criterios Dado/Cuando/Entonces, dependencias y trazabilidad | **Al construir cada HU** |
| 05 | [`05-orden-de-construccion.md`](05-orden-de-construccion.md) | Tramos T0…T7, niveles de dependencia, Sprint 0, variables, comandos, plantilla de PR | Al planear |
| 06 | [`06-arquitectura.md`](06-arquitectura.md) | Arquitectura completa: capas, Go, Supabase, JWT/RBAC, seguridad, concurrencia, resiliencia, observabilidad, front, infra, CI/CD, ADR | Al escribir código |
| 07 | [`07-preguntas-abiertas.md`](07-preguntas-abiertas.md) | Q-xx, D-xx, AQ-xx; decisiones provisionales y tomadas | Antes de cada HU y al encontrar dudas |
| 08 | [`08-glosario-y-convenciones.md`](08-glosario-y-convenciones.md) | Glosario, nombres, formatos (placa, celular, COP, fechas), pruebas | Al nombrar o validar |
| 09 | [`09-estado-de-construccion.md`](09-estado-de-construccion.md) | Estado de cada HU, qué sigue, bitácora de sesiones | Al abrir y al cerrar cada sesión |
| 10 | [`10-mockups-y-diseno.md`](10-mockups-y-diseno.md) | Sistema visual, reglas de diseño DR-xx, patrones por superficie, trazabilidad HU → pantalla | **Antes de construir cualquier pantalla** |
| — | [`mockups/`](mockups/) | 11 mockups HTML + imágenes de referencia | Junto con `10` |
| — | [`adr/`](adr/) | Índice y plantilla de decisiones de arquitectura | Al decidir algo difícil de revertir |
| — | [`data/historias.json`](data/historias.json) | Las HU en JSON (para scripts, pruebas de trazabilidad, herramientas) | Para automatizar |

## Cómo se mantienen estos documentos

1. **Se actualizan en el mismo PR** que el cambio de código que los afecta (R-12).
2. `04-historias-de-usuario.md` y `data/historias.json` cuentan lo mismo: si cambias uno, cambia el otro.
3. Los originales viven en artefactos del proyecto (URLs en `01-producto-y-contexto.md`, sección 12). Las copias del repositorio son las de trabajo; si difieren, avisa a Isaac o Pablo.
4. Los números de HU, Q, D y AQ **no se reutilizan**.

## Skills del proyecto

Las recetas operativas (cómo construir una HU, módulos hexagonales, RLS, concurrencia, seguridad, WhatsApp/IA, pruebas, CI/CD, revisión, clean code, SOLID, POO, GoF y GRASP) viven en `../.claude/skills/<nombre>/SKILL.md`. El índice con cuándo usar cada una está en `CLAUDE.md` §3.1. Cuando cambie una regla de `docs/02` o una decisión de `docs/06`, actualiza la skill afectada en el mismo PR (R-12).
