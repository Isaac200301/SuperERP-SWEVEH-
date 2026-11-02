---
name: patrones-gof
description: Catálogo de los 23 patrones de diseño GoF (creacionales, estructurales, de comportamiento) con su uso concreto en SWEVEH en Go y TypeScript, cuándo aplicarlos y cuándo NO. Úsala al diseñar proveedores de IA, canal de WhatsApp, máquina de estados de la orden, automatizaciones, outbox, middlewares y reglas por tipo de vehículo.
---

# Patrones GoF en SWEVEH

Fuente: Gamma, Helm, Johnson, Vlissides, *Design Patterns* (1994); catálogo de referencia: https://refactoring.guru/es/design-patterns/catalog

**Regla de oro:** un patrón se introduce cuando hay un **dolor concreto** (dos variantes reales, una regla que cambia, un acoplamiento que duele), no por adelantado (YAGNI, ver `clean-code-solid`). En Go muchos patrones se reducen a una interfaz pequeña, una función o un struct; no copies la forma de Java. Si dudas entre aplicar un patrón o no, **no lo apliques** y deja una nota en la PR.

## 0. Mapa rápido: problema → patrón → dónde vive en SWEVEH

| Dolor | Patrón | Dónde (ver `docs/06-arquitectura.md`) |
|---|---|---|
| Cambiar de proveedor de IA sin tocar el dominio | **Strategy** + **Adapter** | puerto `LLM`, adaptadores por proveedor |
| Cambiar BSP/Meta directo de WhatsApp | **Adapter** | puerto `CanalMensajeria` |
| Reglas distintas para carro y moto (placa, campos) | **Strategy** / **Abstract Factory** | `PlateRule`, `VehicleSpec` |
| Transiciones de la orden de servicio | **State** | `Order.Transition` |
| Casos de uso como objetos auditables y reintentables | **Command** | `app/` de cada módulo |
| Publicar eventos de dominio de forma fiable | **Observer** + Outbox | tabla `outbox`, worker |
| Cross-cutting (log, métricas, auth, metering, circuit breaker) | **Decorator** / **Chain of Responsibility** | middlewares HTTP, decoradores de puertos |
| Validar la salida de la IA en pasos | **Chain of Responsibility** | `ValidadorSalidaIA` |
| Automatizaciones con pasos fijos y variantes | **Template Method** | motor de automatizaciones |
| Cotización con líneas y subtotales | **Composite** | `Quote` + `QuoteLine` |
| Exponer un módulo a otro sin filtrar internals | **Facade** | puertos públicos del módulo |
| Construir objetos complejos con opciones | **Builder** / opciones funcionales | `NewServer(opts...)` |
| Una sola instancia de config/pool | **Singleton** (evitar) → DI en composition root | `cmd/api/main.go` |

## 1. Creacionales

### 1.1 Factory Method
**Intención:** delegar la creación a un método que decide el tipo concreto.
**SWEVEH:** crear la regla de placa según el tipo de vehículo.

```go
// internal/vehicles/domain/plate.go
type PlateRule interface {
	Validate(raw string) (Plate, error)
}

func PlateRuleFor(kind VehicleKind) (PlateRule, error) {
	switch kind {
	case Car:
		return carPlate{re: regexp.MustCompile(`^[A-Z]{3}[0-9]{3}$`)}, nil
	case Motorbike:
		return bikePlate{re: regexp.MustCompile(`^[A-Z]{3}[0-9]{2}[A-Z]$`)}, nil
	default:
		return nil, ErrUnknownVehicleKind
	}
}
```
**Cuándo no:** si solo hay una variante, usa un constructor `NewPlate`.

### 1.2 Abstract Factory
**Intención:** crear familias de objetos relacionados.
**SWEVEH:** una "familia carro" y una "familia moto" (regla de placa, campos del motor, catálogo de servicios típicos). Solo si de verdad hay 3 o más piezas que varían juntas; si no, basta Strategy.

```go
type VehicleKindKit interface {
	Plate() PlateRule
	Engine() EngineSpecRule // cilindraje_cc y turbo (sí/no), obligatorios para todo vehículo
	DefaultChecklist() []ChecklistItem
}
```

### 1.3 Builder
**Intención:** construir paso a paso objetos con muchas partes.
**SWEVEH:** construir el cuerpo de una plantilla de WhatsApp o un reporte; en tests, `anOrder().WithVehicle(...).Build()`. En Go, para configuración usa **opciones funcionales** (ver `clean-code-solid` §Go idiomático).

```go
func newOrderForTest(opts ...func(*Order)) Order {
	o := Order{ID: NewOrderID(), State: Received}
	for _, opt := range opts { opt(&o) }
	return o
}
```

### 1.4 Prototype
**Intención:** copiar un objeto existente.
**SWEVEH:** duplicar una plantilla de cotización o una automatización preestablecida al taller (copia profunda; recuerda que los slices y mapas en Go se comparten si copias el struct). Úsalo solo si el copiado es una funcionalidad real (clonar plantilla).

### 1.5 Singleton
**Intención:** una única instancia global.
**SWEVEH: evitar.** Es estado global oculto, rompe tests y concurrencia. La "única instancia" del pool de pgx, del logger o del cliente HTTP se logra **creándola una vez en `main` e inyectándola** (DIP). No uses `var instance` ni `sync.Once` salvo para inicializaciones puras.

## 2. Estructurales

### 2.1 Adapter
**Intención:** traducir una interfaz ajena a la que espera el dominio.
**SWEVEH:** la API de WhatsApp Cloud (Meta) y de cada proveedor de IA nunca entra al dominio; entra por un adaptador que traduce DTOs externos a tipos del dominio.

```go
// puerto (lo define el consumidor, en app/ports.go)
type CanalMensajeria interface {
	EnviarPlantilla(ctx context.Context, to Celular, tpl PlantillaID, params map[string]string) (MensajeID, error)
}

// adaptador (adapters/whatsapp/meta.go)
type MetaCloud struct{ http *http.Client; token Secret; phoneNumberID string }
func (m *MetaCloud) EnviarPlantilla(ctx context.Context, to Celular, tpl PlantillaID, p map[string]string) (MensajeID, error) {
	// traduce a la forma JSON de Meta, maneja 429/5xx, devuelve error de dominio
}
```
Verifica contrato del adaptador con pruebas contra un servidor falso (`httptest`).

### 2.2 Bridge
**Intención:** separar abstracción e implementación para que varíen por separado.
**SWEVEH:** "qué notificación enviar" (abstracción: aviso de orden lista) vs "por qué canal" (WhatsApp hoy, SMS/correo mañana). En la práctica se resuelve con una interfaz `Notificador` + puerto de canal; no crees jerarquías paralelas.

### 2.3 Composite
**Intención:** tratar de igual forma objetos individuales y composiciones.
**SWEVEH:** una cotización con líneas (mano de obra, repuestos, paquetes de servicio que contienen líneas).

```go
type LineItem interface{ Total() Money }
type Labor struct{ Hours int; Rate Money }
type Part  struct{ Qty int; UnitPrice Money }
type Package struct{ Items []LineItem }

func (l Labor) Total() Money   { return l.Rate.Times(l.Hours) }
func (p Part) Total() Money    { return p.UnitPrice.Times(p.Qty) }
func (p Package) Total() Money { t := Zero(); for _, i := range p.Items { t = t.Add(i.Total()) }; return t }
```

### 2.4 Decorator
**Intención:** añadir comportamiento envolviendo un objeto con la misma interfaz.
**SWEVEH:** envolver el puerto `LLM` con medición de costo por conversación, reintentos, circuit breaker y trazas, sin tocar el adaptador.

```go
type LLM interface { Complete(ctx context.Context, req Prompt) (Completion, error) }

type meteredLLM struct{ next LLM; meter UsageMeter }
func (m meteredLLM) Complete(ctx context.Context, req Prompt) (Completion, error) {
	out, err := m.next.Complete(ctx, req)
	m.meter.Record(ctx, req.TenantID, out.Usage) // costo por conversación (DE de precio)
	return out, err
}

// composition root
llm := WithTracing(WithBreaker(WithMetering(openai.New(cfg), meter)))
```
Los middlewares HTTP de `chi` son Decorators de `http.Handler`.

### 2.5 Facade
**Intención:** una interfaz simple frente a un subsistema.
**SWEVEH:** cada módulo del monolito expone **una fachada pequeña** (un paquete `orders` con `Service`) y oculta el resto en `internal/`. Otro módulo solo importa esa fachada; `depguard` y el test de arquitectura lo hacen cumplir.

### 2.6 Flyweight
**Intención:** compartir estado inmutable para ahorrar memoria.
**SWEVEH:** casi nunca. Catálogos de solo lectura (tipos de servicio, estados) se cargan una vez en memoria; no lo llames patrón ni lo sobrediseñes.

### 2.7 Proxy
**Intención:** sustituto que controla el acceso.
**SWEVEH:** repositorio con caché de lectura (proxy de caché), o un cliente que aplica límites de tasa antes de llamar a Meta. Si añade funcionalidad transversal, es Decorator; si controla el acceso/ciclo de vida, Proxy.

## 3. De comportamiento

### 3.1 Strategy
**Intención:** intercambiar algoritmos tras una interfaz.
**SWEVEH:** proveedor de IA (OpenAI, Anthropic, otro), política de reintentos, cálculo de impuestos/redondeo, regla de placa por tipo de vehículo. La estrategia se elige en el composition root o por configuración del taller, no con `if` repartidos.

### 3.2 State
**Intención:** el comportamiento depende del estado y las transiciones están encapsuladas.
**SWEVEH:** la orden de servicio. Una tabla de transiciones permitidas en el dominio, no `if estado == ...` dispersos. Los 7 estados más Cancelada están en `docs/02` §10 (Q-04, Q-05); las transiciones exactas salen de HU-20 y HU-26 (`docs/04`). El código es ilustrativo.

```go
var allowed = map[OrderState][]OrderState{
	ToReceive:   {Received, Cancelled},
	Received:    {InDiagnosis, Cancelled},
	InDiagnosis: {AwaitingApproval, Cancelled},
	// ... Esperando aprobación → En reparación → Listo → Entregado (HU-20/HU-26)
}

func (o *Order) Transition(to OrderState, actor Actor) error {
	if !slices.Contains(allowed[o.State], to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, o.State, to)
	}
	if to == Received && o.Vehicle.EnginePending() { // Q-23: motor pendiente bloquea Recibido
		return ErrEngineRequired
	}
	o.recordEvent(OrderStateChanged{From: o.State, To: to, By: actor.ID})
	o.State = to
	return nil
}
```
Pasa a clases-estado (objetos con métodos por estado) solo si cada estado tiene comportamiento propio y la tabla se queda corta.

### 3.3 Command
**Intención:** encapsular una petición como objeto.
**SWEVEH:** cada caso de uso es un comando con su `Handler` (`ApproveQuoteCmd`). Esto da: validación en un sitio, auditoría (quién ejecutó qué), idempotencia (`Idempotency-Key`) y reintento desde el outbox.

```go
type ApproveQuote struct{ QuoteID QuoteID; Actor Actor; IdempotencyKey string }
type ApproveQuoteHandler struct{ uow UnitOfWork; clock Clock }
func (h ApproveQuoteHandler) Handle(ctx context.Context, c ApproveQuote) error { /* ... */ }
```

### 3.4 Observer
**Intención:** notificar a suscriptores cuando algo cambia.
**SWEVEH:** eventos de dominio (`OrderReady`) escritos en la tabla `outbox` dentro de la misma transacción; un worker (`FOR UPDATE SKIP LOCKED`) los entrega a suscriptores (notificar por WhatsApp, actualizar métricas). Nunca llames un módulo vecino directamente desde el dominio para "avisar": publica un evento.

### 3.5 Chain of Responsibility
**Intención:** una petición pasa por una cadena de manejadores.
**SWEVEH:** validación de la respuesta del agente de IA: (1) esquema JSON válido, (2) solo plantillas aprobadas (AU-05), (3) sin datos sensibles, (4) dentro de límites de monto, (5) si falla alguno, escalar a humano. Cada validador es independiente y la cadena es configurable (RF-17, RF-18: la IA propone, el código valida, el humano aprueba).

```go
type Validator interface{ Validate(ctx context.Context, out AIOutput) error }
type Chain []Validator
func (c Chain) Validate(ctx context.Context, out AIOutput) error {
	for _, v := range c { if err := v.Validate(ctx, out); err != nil { return err } }
	return nil
}
```
Los middlewares de autenticación, RBAC, limitación de tasa y CSRF forman la misma cadena en HTTP.

### 3.6 Template Method
**Intención:** esqueleto fijo con pasos variables.
**SWEVEH:** una automatización siempre hace: disparador → verificar consentimiento (Ley 1581) → verificar plan/cuota → ejecutar acción → registrar en auditoría. En Go se logra con una función que recibe los pasos variables (composición), no con herencia.

```go
type Automation interface {
	Matches(ev Event) bool
	Run(ctx context.Context, ev Event) error // paso variable
}

func Execute(ctx context.Context, a Automation, ev Event, d Deps) error {
	if !a.Matches(ev) { return nil }
	if err := d.Consent.Check(ctx, ev.Customer); err != nil { return err }
	if err := d.Quota.Reserve(ctx, ev.TenantID); err != nil { return err }
	if err := a.Run(ctx, ev); err != nil { return err }
	return d.Audit.Record(ctx, ev, a)
}
```

### 3.7 Mediator
**Intención:** centralizar la comunicación entre objetos.
**SWEVEH:** el bus de eventos interno entre módulos actúa como mediador. Evita que el módulo `orders` conozca a quien notifica. Mantén el bus mínimo; no hagas un "dios" que contenga reglas.

### 3.8 Memento
**Intención:** guardar y restaurar estado.
**SWEVEH:** el historial/auditoría de cambios y snapshots de cotización aprobada (inmutable: lo que el cliente aprobó). No restaures estado por reflexión; guarda versiones explícitas.

### 3.9 Iterator
**Intención:** recorrer una colección sin exponer su estructura.
**SWEVEH:** paginación por cursor (listados), `iter.Seq` de Go 1.23 para recorrer resultados grandes sin cargar todo en memoria.

### 3.10 Visitor
**Intención:** añadir operaciones a una jerarquía sin modificarla.
**SWEVEH:** raro. En Go se reemplaza por `switch` sobre tipos o por interfaces; en TypeScript, uniones discriminadas con `switch` exhaustivo (más simple y seguro).

### 3.11 Interpreter
**Intención:** gramática y evaluador de un lenguaje.
**SWEVEH:** **no en el MVP**. Una eventual lengua de reglas de automatización configurables por el taller sería candidata, pero requiere decisión del equipo.

## 4. Patrones que SWEVEH usa sin ser GoF (para no confundirlos)

- **Repository**, **Unit of Work**, **Outbox**, **Saga/compensación**, **Circuit Breaker**, **Idempotent Receiver**: patrones de arquitectura/integración. Ver `sweveh-arquitectura-hexagonal` y `sweveh-concurrencia-idempotencia`.
- **Value Object**, **Aggregate**: DDD, ver `principios-poo`.

## 5. Antipatrones a evitar

| Antipatrón | Síntoma | Alternativa |
|---|---|---|
| Singleton global | `var db *sql.DB` en un paquete | inyección en el composition root |
| Factory de una sola variante | `NewFooFactory().Create()` | constructor directo |
| Jerarquías de herencia en TS | `class A extends B extends C` | composición + uniones discriminadas |
| Patrón por ornamento | interfaz con una sola implementación sin necesidad de test/swap | función o struct concreto |
| God Mediator | el bus decide reglas de negocio | reglas en el dominio |
| Visitor en Go | doble despacho artificial | `switch` o interfaz |

## 6. Cómo decidir (checklist rápido)

1. ¿Hay hoy **dos o más** variantes reales o una frontera externa (IA, WhatsApp, pago)? Si no, no hay patrón.
2. ¿Una interfaz pequeña definida por el consumidor lo resuelve? Entonces es Strategy/Adapter y basta.
3. ¿Se repite cross-cutting? Decorator o middleware.
4. ¿Hay un estado con reglas de transición? State (tabla primero).
5. Documenta la decisión en un ADR corto solo si es estructural (`docs/adr/0000-plantilla.md`).
6. Nombra el patrón en el comentario de paquete solo si ayuda a entender; la intención se lee en el código.
