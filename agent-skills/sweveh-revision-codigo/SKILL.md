---
name: sweveh-revision-codigo
description: Lista de revisión de código y de PR para SWEVEH que combina correctitud, arquitectura hexagonal, multi-tenant, seguridad, concurrencia, IA/WhatsApp, clean code, pruebas, documentación y honestidad. Úsala antes de abrir un PR y al revisar el de la otra persona.
---

# Revisión de código en SWEVEH

Úsala como autorrevisión antes de abrir el PR y como guion al revisar a la otra persona (Isaac ↔ Pablo). El agente **no hace merge** (R-14). Revisa en este orden: lo que rompe producto o seguridad primero, estilo al final.

## 0. Contexto
- [ ] ¿Qué HU/criterios cubre? ¿Está enlazada y es la correcta? ¿Dependencias hechas?
- [ ] ¿Cambia alcance, seguridad, dinero o toca AQ ⛔? Si sí, ¿hay aprobación del equipo registrada?
- [ ] ¿PR pequeño (< 400 líneas orientativo) y con una sola intención?

## 1. Correctitud
- [ ] Cumple cada criterio de aceptación; casos límite (vacío, máximos, Unicode, zona horaria America/Bogota, dinero en pesos enteros).
- [ ] Errores manejados y envueltos con contexto; sin `err` ignorados (`_ =`) sin motivo.
- [ ] Sin `panic` en flujo normal; sin nil-pointer evidentes; sin bucles que capturan variables mal.
- [ ] Idempotencia donde hay reintentos; sin efectos externos dentro de transacciones.

## 2. Arquitectura (`sweveh-arquitectura-hexagonal`)
- [ ] `domain` sin imports de infraestructura; puertos definidos por el consumidor; depguard y test de arquitectura en verde.
- [ ] Ningún módulo importa internos de otro; comunicación por fachada/eventos.
- [ ] Lógica de negocio en el dominio/caso de uso, no en handlers ni SQL.
- [ ] DTO ≠ entidad ≠ fila; mapeos explícitos.
- [ ] Cambio estructural con ADR (R-13).

## 3. Multi-tenant y datos (`sweveh-multitenant-rls`, `sweveh-db-migraciones-sqlc`)
- [ ] `taller_id` NOT NULL + RLS (`ENABLE` y `FORCE`) + política + prueba de aislamiento con dos talleres.
- [ ] El `taller_id` sale de la sesión, no del cliente.
- [ ] Migración compatible hacia atrás, sin bloqueos largos; índices con `taller_id` al inicio.
- [ ] Datos de prueba **Ficticio**; sin PII real.

## 4. Seguridad (`sweveh-seguridad-auth`)
- [ ] Permiso `recurso:accion` declarado y probado; recurso ajeno → 404.
- [ ] Entrada validada (OpenAPI + dominio); SQL parametrizado.
- [ ] Sin secretos ni PII en código, logs, fixtures o capturas.
- [ ] Cookies/CSRF/cabeceras intactas; nada guardado en `localStorage`.
- [ ] Firma de webhook en tiempo constante; sin `==` para comparar secretos.

## 5. Concurrencia (`sweveh-concurrencia-idempotencia`)
- [ ] Mecanismo correcto según la tabla de R-07; `version`/`If-Match` en edición.
- [ ] Prueba con N goroutines y `-race`.
- [ ] Sin bloqueos largos ni llamadas externas con filas bloqueadas.
- [ ] Orden de bloqueo consistente (IDs ascendentes).

## 6. IA y WhatsApp (`sweveh-whatsapp-ia`)
- [ ] Texto del cliente tratado como dato; salida de IA validada por esquema; humano aprueba lo sensible.
- [ ] Consentimiento verificado; plantillas/ventana de 24 h respetadas.
- [ ] Costo/tokens registrados; topes; degradación si falla el servicio.
- [ ] Pruebas adversariales actualizadas.

## 7. Clean code, POO y diseño (`clean-code-solid`, `principios-poo`, `grasp`, `patrones-gof`)
- [ ] Nombres que revelan intención; funciones cortas con un solo nivel de abstracción.
- [ ] SRP/ISP/DIP respetados; interfaces pequeñas en el consumidor.
- [ ] Sin duplicación real (DRY) ni abstracción prematura (YAGNI).
- [ ] Invariantes en el dominio (value objects, máquina de estados); "tell, don't ask".
- [ ] Patrón GoF solo si hay dolor real y está nombrado con sentido; nada de Singleton global.
- [ ] Comentarios explican el *porqué*, no el *qué*; sin código muerto ni TODO sin Q-xx.

## 8. Frontend (`sweveh-frontend-react`)
- [ ] Tipos del cliente generado; sin `any`; estados de carga/error/vacío.
- [ ] 401/403/404/409/422 manejados; formularios con Zod alineado.
- [ ] Textos en español de Colombia; COP sin decimales; accesibilidad básica.
- [ ] Piso: móvil, objetivos táctiles grandes, sin costos/KPIs de dinero para el mecánico (RF-21).

## 9. Observabilidad y auditoría (`sweveh-observabilidad-auditoria`)
- [ ] Eventos de auditoría en la misma transacción para acciones sensibles.
- [ ] Logs estructurados sin PII; métricas con cardinalidad controlada.

## 10. Pruebas y CI (`sweveh-testing`, `sweveh-ci-cd-calidad`)
- [ ] Pruebas por criterio, nombradas por comportamiento; cobertura nueva ≥ 80 % en dominio/aplicación.
- [ ] Sin pruebas deshabilitadas, `Skip` ni umbrales bajados.
- [ ] `make ci` verde (adjuntar salida real en el PR).
- [ ] Dependencias nuevas justificadas (licencia, mantenimiento, vulnerabilidades).

## 11. Documentación y honestidad (R-12, R-15)
- [ ] `docs/09` (estado) y, si procede, `docs/07`, ADR y contrato OpenAPI actualizados **en el mismo PR**.
- [ ] La descripción dice qué se probó y qué **no** se pudo probar.
- [ ] Cualquier afirmación sobre terceros (precios, límites, políticas) lleva fuente o `Sin dato`.

## 12. Cómo dar y recibir la revisión
- Comentarios con severidad: **Bloqueante** (seguridad, aislamiento, pérdida de datos, criterio sin cumplir), **Importante** (diseño, pruebas), **Sugerencia** (estilo).
- Criticar el código, no a la persona; proponer alternativa concreta con un ejemplo corto.
- El autor responde a cada comentario (arreglado / no aplica y por qué).
- Si no entiendes un cambio en 5 minutos, pide una explicación: es señal de que falta claridad en el código o en el PR.
- Aprobación = "lo ejecutaría yo en producción". Si no, no apruebes.

## 13. Plantilla de comentario

```
[Bloqueante] internal/quotes/adapters/out/postgres/repo.go:88
La consulta no filtra por taller y la política RLS no está habilitada en esta tabla (falta FORCE).
Riesgo: un taller podría leer cotizaciones de otro.
Propuesta: añadir FORCE RLS + política y la prueba de aislamiento (ver sweveh-multitenant-rls §4).
```
