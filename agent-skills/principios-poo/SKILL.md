---
name: principios-poo
description: Principios de programación orientada a objetos (encapsulación, abstracción, herencia vs composición, polimorfismo, invariantes, value objects, agregados, "tell don't ask") traducidos a Go y TypeScript para el dominio SWEVEH. Úsala al modelar entidades (Orden, Cotización, Vehículo, Inventario), objetos de valor (Placa, Celular, Money) y reglas de negocio.
---

# Principios de POO en SWEVEH (Go y TypeScript)

Go no tiene clases ni herencia; tiene **tipos con métodos, interfaces implícitas y composición**. TypeScript sí tiene clases, pero se usan con medida. Los principios son los mismos; cambia la forma de expresarlos. Complementa `clean-code-solid`, `grasp` y `patrones-gof`.

## 1. Los cuatro pilares, traducidos

| Pilar | Idea | En Go | En TypeScript |
|---|---|---|---|
| **Encapsulación** | El objeto protege sus invariantes; nadie los rompe desde afuera | Campos no exportados + constructor con validación + métodos que expresan intención | Campos `private`/`#`, fábrica estática `create()` que valida |
| **Abstracción** | Mostrar *qué* hace, ocultar *cómo* | Interfaces (puertos) pequeñas | `interface`/`type` y módulos con `index.ts` público |
| **Herencia** | Reutilizar y especializar | **No existe**: usa composición/embedding con cuidado | Evítala; prefiere composición y uniones |
| **Polimorfismo** | Mismo mensaje, distinto comportamiento | Interfaces satisfechas implícitamente | Interfaces + uniones discriminadas |

**Composición sobre herencia:** la herencia acopla fuerte y rompe Liskov con facilidad. Reutiliza comportamiento *teniendo* un colaborador, no *siendo* uno.

## 2. Encapsulación e invariantes: el dominio protege sus reglas

Un **invariante** es algo que siempre debe ser verdad (una cotización aprobada tiene total > 0; el stock nunca es negativo; un vehículo tiene motor antes de pasar a Recibido). El constructor y los métodos **garantizan** el invariante; el resto del código no puede violarlo.

```go
// internal/quotes/domain/quote.go
type Quote struct {
    id       QuoteID
    tallerID TallerID
    lines    []Line
    status   Status
    version  int // bloqueo optimista
}

func NewQuote(id QuoteID, taller TallerID, lines []Line) (Quote, error) {
    if len(lines) == 0 {
        return Quote{}, ErrQuoteWithoutLines
    }
    return Quote{id: id, tallerID: taller, lines: lines, status: StatusDraft}, nil
}

// Tell, don't ask: la regla vive aquí, no en el caso de uso.
func (q *Quote) Approve(by Approver, now time.Time) error {
    switch q.status {
    case StatusApproved:
        return ErrAlreadyApproved
    case StatusExpired, StatusCancelled:
        return ErrQuoteNotApprovable
    }
    q.status = StatusApproved
    q.recordApproval(by, now)
    return nil
}

func (q Quote) Total() Money { /* suma de líneas */ }
```

Reglas:
- **Sin setters públicos** (`SetStatus`). Métodos con nombre de negocio (`Approve`, `MarkReady`).
- **No devuelvas** slices/maps internos mutables: devuelve copia o vista de solo lectura.
- Los datos que vienen de afuera se validan **al construir el objeto** (parse, don't validate): después de crear un `Placa`, ya es válida.

## 3. Value objects (objetos de valor)

Representan un concepto por su **valor**, son **inmutables** y se comparan por contenido. Eliminan la *obsesión por primitivos*. Candidatos en SWEVEH:

| Value object | Regla que encapsula | Referencia |
|---|---|---|
| `Placa` | Mayúsculas, sin espacios; carro `^[A-Z]{3}[0-9]{3}$`; moto `ABC12D`/`ABC12`; especial provisional | `docs/08`, Q-01 |
| `Celular` | 10 dígitos colombianos que empiezan por 3; se guarda E.164 `+57…` | HU-01, RF-04 |
| `Money` | Pesos COP como entero, **nunca float**; suma/resta seguras | `docs/08` |
| `CilindrajeCC` | Entero positivo en cc | DE-17 |
| `QuoteID`, `TallerID`… | Tipos distintos para IDs (evita pasar el ID de una orden donde va el de un taller) | |

```go
type Placa struct{ v string }

var (
    reCarro   = regexp.MustCompile(`^[A-Z]{3}[0-9]{3}$`)
    reMoto    = regexp.MustCompile(`^[A-Z]{3}[0-9]{2}[A-Z]$`)
    reMotoOld = regexp.MustCompile(`^[A-Z]{3}[0-9]{2}$`)
)

func NewPlaca(raw string) (Placa, error) {
    v := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
    if reCarro.MatchString(v) || reMoto.MatchString(v) || reMotoOld.MatchString(v) {
        return Placa{v}, nil
    }
    return Placa{}, fmt.Errorf("%w: %q", ErrPlacaInvalida, raw)
}
func (p Placa) String() string { return p.v }
```
```ts
// web/src/shared/money.ts — pesos enteros, nunca float
export type Money = Readonly<{ pesos: number }>;
export const money = (pesos: number): Money => {
  if (!Number.isInteger(pesos)) throw new Error('COP no admite decimales');
  return { pesos };
};
```

## 4. Entidades, agregados y raíces

- **Entidad:** tiene identidad que persiste (una `Orden`, un `Vehiculo`); dos entidades con los mismos datos pero distinto ID son distintas.
- **Agregado:** grupo de objetos que se modifican **juntos** como una unidad de consistencia, con una **raíz** que es la única puerta de entrada. En SWEVEH: `Orden` (raíz) con sus ítems, estados y evidencias; `Quote` con sus líneas; `Articulo` de inventario con su libro de movimientos.
- **Reglas:** (1) se referencian otros agregados **por ID**, no por objeto; (2) una transacción modifica **un** agregado (la consistencia entre agregados es eventual, vía outbox); (3) el repositorio carga y guarda **agregados completos** y controla la `version`.
- Esto alinea con "un esquema por módulo, sin FK entre módulos" (ADR-09) y con la concurrencia optimista (`sweveh-concurrencia-idempotencia`).

## 5. Máquinas de estado como objetos

El estado de una orden no es un `string` suelto. Modela las transiciones permitidas en el dominio (ver `patrones-gof`, patrón State):

```go
type OrderState string

const (
    PorRecibir OrderState = "por_recibir"; Recibido = "recibido"; EnDiagnostico = "en_diagnostico"
    EsperandoAprobacion = "esperando_aprobacion"; EnReparacion = "en_reparacion"
    Listo = "listo"; Entregado = "entregado"; Cancelada = "cancelada"
)

// Ilustrativo: las transiciones exactas salen de HU-20 y HU-26; no inventar otras.
var allowed = map[OrderState][]OrderState{
    PorRecibir:          {Recibido, Cancelada},
    Recibido:            {EnDiagnostico, Cancelada},
    EnDiagnostico:       {EsperandoAprobacion, Cancelada},
    EsperandoAprobacion: {EnReparacion, Cancelada},
    EnReparacion:        {Listo, Cancelada},
    Listo:               {Entregado, Cancelada},
}

func (o *Order) MoveTo(next OrderState, by UserID, now time.Time) error {
    if !slices.Contains(allowed[o.state], next) {
        return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, o.state, next)
    }
    if next == Recibido && o.vehicle.Engine.IsPending() {
        return ErrEnginePending // motor con cilindraje y turbo antes de Recibido (Q-23)
    }
    o.history = append(o.history, Transition{From: o.state, To: next, By: by, At: now})
    o.state = next
    return nil
}
```

## 6. Polimorfismo sin herencia

- **Go:** una interfaz pequeña y varias implementaciones; el polimorfismo ocurre en tiempo de ejecución por el valor que se inyecta.
- **Carro vs moto** (HU-17/HU-18, placas, checklists): no uses un `if vehicle.Type == "moto"` en veinte sitios. Define una interfaz (`ChecklistTemplate`, `PlateRule`) con una implementación por tipo y elige **una vez**, en la fábrica (ver `patrones-gof`, Abstract Factory / Strategy).

```go
type PlateRule interface{ Valid(p string) bool }
type CarPlate struct{}  ; func (CarPlate) Valid(p string) bool { return reCarro.MatchString(p) }
type BikePlate struct{} ; func (BikePlate) Valid(p string) bool { return reMoto.MatchString(p) || reMotoOld.MatchString(p) }
```

## 7. Herencia: cuándo (casi) nunca

- En Go no hay; el *embedding* **no** es herencia (no hay `super` ni polimorfismo por jerarquía). Úsalo para **reutilizar implementación** de forma transparente (p. ej., incrustar un `http.Handler` en un decorador), no para modelar "es-un".
- En TypeScript evita `extends` de clases de dominio. Si necesitas variantes, usa **unión discriminada** + función exhaustiva (`never` check) o composición.

## 8. Lista de verificación de diseño de objetos

- [ ] ¿Las reglas están **dentro** del objeto que posee los datos (Tell, don't ask)?
- [ ] ¿Hay constructor/fábrica que impide crear objetos inválidos?
- [ ] ¿Los primitivos con reglas (placa, celular, dinero, cc) son value objects?
- [ ] ¿La raíz del agregado es la única puerta y se guarda con `version`?
- [ ] ¿Referencias entre agregados por ID?
- [ ] ¿Las variantes (carro/moto, proveedor de IA, canal) se resuelven por polimorfismo y no por `if` dispersos?
- [ ] ¿Ninguna jerarquía de herencia innecesaria?
