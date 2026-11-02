---
name: sweveh-arquitectura-hexagonal
description: Cómo construir un módulo de SWEVEH en el monolito modular hexagonal de Go (domain/app/ports/adapters), reglas de dependencia, composition root, errores y contratos OpenAPI. Úsala al crear o modificar cualquier módulo backend (identity, tenancy, customers, vehicles, orders, quotes, inventory).
---

# Arquitectura hexagonal en SWEVEH (backend Go)

Fuente de verdad: `docs/06-arquitectura.md` (ADR-01 a ADR-07 son decisión del equipo; el resto son `Propuesta`). Esta skill es la receta; si algo contradice el documento, aplica `docs/02` §8.

## 1. Forma de cada módulo

```
internal/<modulo>/
├── domain/            # entidades, objetos de valor, reglas, eventos. SIN imports de infraestructura
├── app/               # casos de uso (comandos y consultas) + puertos (interfaces)
│   ├── commands/  queries/
│   └── ports.go       # interfaces definidas AQUÍ, por el consumidor
├── adapters/
│   ├── in/http/       # handlers generados por oapi-codegen + traducción
│   ├── in/worker/     # consumidores de outbox / jobs
│   └── out/postgres/  # repositorios con sqlc; out/whatsapp, out/llm, etc.
└── module.go          # fachada pública del módulo: constructores y tipos expuestos
```

Módulos del MVP: `identity`, `tenancy`, `customers`, `vehicles`, `orders`, `quotes`, `inventory`; transversal en `internal/platform` (config, db, logging, otel, httpx, authn, clock) y `internal/shared` (**solo** IDs, `Money`, errores base, eventos).

## 2. Regla de dependencia (se hace cumplir con herramientas)

```
adapters ──► app ──► domain
   │          │
   └──────────┴──► shared
```

- `domain` no importa `app`, `adapters`, `pgx`, `net/http`, `slog`, ni SDK de terceros.
- `app` importa `domain` y define los puertos; no importa `adapters`.
- Un módulo **no importa paquetes internos de otro**: solo su `module.go` (fachada) o escucha sus eventos.
- `golangci-lint` con `depguard` + test de arquitectura (`internal/arch_test.go`) fallan el PR si se viola. **No desactives** la regla; corrige el diseño (R-11).

Test de arquitectura sugerido (ejemplo con `go/packages` o `archtest`; elegir una librería es decisión de bajo impacto, anótala en el PR):

```go
func TestDomainHasNoInfraImports(t *testing.T) {
	forbidden := []string{"github.com/jackc/pgx", "net/http", "log/slog", "go.opentelemetry.io"}
	for _, pkg := range listPackages(t, "./internal/*/domain/...") {
		for _, imp := range importsOf(t, pkg) {
			for _, f := range forbidden {
				if strings.HasPrefix(imp, f) { t.Errorf("%s importa %s", pkg, imp) }
			}
		}
	}
}
```

## 3. Receta para añadir un caso de uso (de adentro hacia afuera)

1. **Dominio:** modela el comportamiento como método del agregado o función pura (`order.Transition(...)`). Pruebas unitarias sin base de datos. Ver `principios-poo` y `grasp`.
2. **Puerto:** declara en `app/ports.go` solo lo que el caso de uso necesita (interfaz pequeña; ISP).
3. **Caso de uso:** un `Handler` por comando/consulta con dependencias inyectadas.
4. **Adaptador de salida:** SQL en `db/queries/*.sql` → `sqlc generate` → repositorio que implementa el puerto (ver `sweveh-db-migraciones-sqlc`).
5. **Contrato:** edita `api/openapi.yaml` primero (es la fuente de verdad) → `make gen`.
6. **Adaptador de entrada:** el handler implementa la interfaz generada; solo traduce, autentica/autoriza y llama al caso de uso.
7. **Cableado:** en `cmd/api/main.go` (composition root).
8. **Pruebas:** dominio (unitarias), aplicación (con fakes), adaptadores (integración con Postgres real, `testcontainers` o la instancia local), HTTP (contra el contrato), aislamiento y concurrencia si aplica (`sweveh-testing`).

## 4. Caso de uso con transacción, eventos y auditoría

```go
type ApproveQuoteHandler struct {
	uow    UnitOfWork        // abre tx con SET LOCAL app.taller_id
	clock  Clock
	audit  AuditLog
}

func (h ApproveQuoteHandler) Handle(ctx context.Context, c ApproveQuote) error {
	return h.uow.Do(ctx, func(ctx context.Context, tx Tx) error {
		q, err := tx.Quotes().Get(ctx, c.QuoteID)          // con RLS: solo ve su taller
		if err != nil { return err }
		if err := q.Approve(c.Actor, h.clock.Now(), c.ExpectedVersion); err != nil {
			return err                                        // errores de dominio tipados
		}
		if err := tx.Quotes().Save(ctx, q); err != nil { return err } // UPDATE ... WHERE version = $n
		tx.Outbox().Add(q.PullEvents()...)                    // mismo commit que el cambio
		return h.audit.Record(ctx, tx, c.Actor, "quote.approve", q.ID)
	})
}
```

Reglas: una transacción por caso de uso; **un agregado por transacción**; eventos al outbox dentro de la misma transacción; nada de llamadas a Meta, a la IA ni a HTTP externo dentro de la transacción.

## 5. Errores

- Dominio devuelve errores **tipados** (`ErrInvalidTransition`, `ErrEngineRequired`, `ErrVersionConflict`) con `errors.Is/As`.
- Los adaptadores HTTP los mapean a respuestas RFC 9457 (`application/problem+json`): 400 validación, 401, 403, 404 (también para recursos de otro taller, no filtra existencia), 409 conflicto de versión/estado, 422 regla de negocio, 429, 5xx genérico sin detalles internos.
- Envuelve con contexto `fmt.Errorf("aprobar cotización %s: %w", id, err)`; nunca devuelvas `err.Error()` de la base al cliente.
- Nunca `panic` en flujo normal; un middleware de recuperación registra y responde 500.

## 6. Contexto, cancelación y tiempos

- `ctx` como primer parámetro; propaga `taller_id`, `actor`, `request_id` y `trace_id` por el contexto con tipos privados.
- Tiempo límite por solicitud en el borde; los llamados salientes (Meta, IA) con `context.WithTimeout` propio.
- Usa `Clock` inyectado, no `time.Now()` en dominio/app (pruebas deterministas).

## 7. Composition root

Todo se instancia en `cmd/api/main.go` y `cmd/worker/main.go`: configuración (variables de entorno validadas al arrancar, falla rápido), pool, repositorios, decoradores (`patrones-gof` §2.4), handlers, router. Sin variables globales ni `init()` con efectos.

## 8. Qué NO hacer

- Entidades con etiquetas `json`/`db` para "ahorrar" mapeos: los DTO viven en adaptadores.
- Repositorio genérico `Repository[T]` que expone SQL; cada puerto es específico del caso de uso.
- Lógica de negocio en handlers HTTP, en SQL o en triggers.
- Compartir tablas entre módulos o hacer `JOIN` a esquemas ajenos.
- Importar `internal/<otro>/domain`. Si dos módulos necesitan un concepto, es evento o dato en una fachada.
- Decidir un cambio estructural sin ADR (R-13).

## 9. Checklist

- [ ] `make lint` (depguard) y test de arquitectura en verde.
- [ ] El contrato OpenAPI cambió primero y `make gen` no deja diferencias.
- [ ] Pruebas unitarias del dominio sin I/O.
- [ ] Errores tipados mapeados a problem+json.
- [ ] Docs (`docs/04`/`09`) actualizados en el mismo PR (R-12).
