---
name: clean-code-solid
description: Código limpio y principios SOLID, DRY, KISS, YAGNI y Ley de Demeter aplicados a Go y TypeScript en SWEVEH. Úsala al escribir, refactorizar o revisar cualquier código del backend (Go) o del frontend (React + TS), al nombrar funciones, partir módulos grandes, manejar errores o decidir si una abstracción se justifica.
---

# Código limpio y SOLID en SWEVEH

Guía de decisión, no un catálogo para recitar. Regla madre: **el código se escribe para que lo lea la otra persona del equipo dentro de seis meses**. Si dudas entre "más abstracto" y "más obvio", elige lo obvio (KISS) y deja que una segunda necesidad real justifique la abstracción (YAGNI).

Complementa a: `principios-poo`, `patrones-gof`, `grasp`, `sweveh-arquitectura-hexagonal`, `sweveh-revision-codigo`. Las reglas del proyecto (`docs/02-reglas-de-decision.md`) mandan sobre cualquier gusto personal.

## 1. Nombres

| Regla | Mal | Bien |
|---|---|---|
| Nombra por **intención de negocio**, no por mecánica | `processData`, `handle2` | `ApproveQuote`, `reserveStock` |
| El nombre dice **qué** hace, el cuerpo dice **cómo** | `doStuff` | `markOrderAsReady` |
| Un concepto = una palabra en todo el código | `Client`, `Customer`, `Cliente` mezclados | Siempre el término del glosario (`docs/08`) |
| Booleanos como pregunta | `flag`, `status` | `isOverdue`, `hasAuthorization` |
| Sin abreviaturas crípticas | `usr`, `qte` | `user`, `quote` (en Go se admiten las cortas convencionales: `ctx`, `tx`, `err`, `id`) |
| Longitud proporcional al alcance | `i` en un ciclo de 3 líneas; `ordersPendingApproval` en un campo | |

Idioma: identificadores Go en inglés técnico; términos del dominio sin traducción natural pueden quedar en español (`taller`, `placa`). Ver `docs/08-glosario-y-convenciones.md`.

## 2. Funciones y métodos

- **Hacen una cosa** y están en un solo nivel de abstracción. Señal de alarma: un comentario que separa "secciones" dentro de una función → son funciones distintas.
- **Pocos parámetros** (≤ 3 como guía). Más → agrupa en un struct con nombre (`ApproveQuoteCmd`) u *options*.
- **Sin parámetros booleanos** que cambian el comportamiento (`send(msg, true)`): divide en dos funciones o usa un tipo.
- **Sin efectos ocultos.** Si `GetX` modifica estado, miente. Separa consulta de comando (CQS).
- **Retorno temprano** (*guard clauses*) en vez de `if` anidados: máximo 2 niveles de indentación como guía.
- Una función que no cabe en una pantalla probablemente mezcla responsabilidades. No es una regla rígida: un `switch` largo y plano de mapeo puede ser claro.

```go
// ❌ anidada y con dos niveles de abstracción mezclados
func (h ApproveQuote) Handle(ctx context.Context, cmd Cmd) error {
    if cmd.QuoteID != "" {
        q, err := h.repo.ByID(ctx, cmd.QuoteID)
        if err == nil {
            if q.Status == "pending" {
                q.Status = "approved"
                // ... 40 líneas más de SQL, WhatsApp y cálculos
            }
        }
    }
    return nil
}

// ✅ plana, un nivel de abstracción, la regla vive en el dominio
func (h ApproveQuote) Handle(ctx context.Context, cmd Cmd) error {
    return h.uow.Do(ctx, func(tx Tx) error {
        q, err := h.repo.ByID(ctx, tx, cmd.QuoteID)
        if err != nil {
            return fmt.Errorf("load quote: %w", err)
        }
        if err := q.Approve(cmd.By, h.clock.Now()); err != nil {
            return err
        }
        if err := h.repo.Save(ctx, tx, q); err != nil {
            return fmt.Errorf("save quote: %w", err)
        }
        return h.events.Publish(ctx, tx, QuoteApproved{QuoteID: q.ID})
    })
}
```

## 3. Comentarios

- El código bueno **se explica solo**; un comentario que repite el código es ruido.
- Comenta el **por qué** (una regla de negocio, una norma, una decisión): `// Ley 1480 art. 18: el recibo debe llevar la garantía`.
- Cita el id de la HU/RF/ADR cuando una regla viene de ahí: `// HU-27, RF-18`.
- Nunca dejes código comentado: está en git.
- `TODO` solo con dueño y referencia (`TODO(isaac) HU-33: devolver stock`).

## 4. Manejo de errores

**Go**
- Errores como valores; **envuelve con contexto** (`fmt.Errorf("save quote: %w", err)`) y compara con `errors.Is` / `errors.As`.
- Errores de dominio **tipados y estables** (`ErrVersionConflict`, `ErrStockInsuficiente`); el adaptador HTTP los traduce a RFC 9457 (409, 422…). Nunca devuelvas `err.Error()` de la base al usuario.
- `panic` solo para errores de programación al arrancar; nunca para flujo de negocio.
- Registra **una vez**, en el borde (middleware o worker), no en cada capa; si registras y devuelves el error, se duplica.
- `context.Context` es el primer parámetro; respeta cancelación y plazos.

**TypeScript**
- `strict: true`. Prohibido `any` (usa `unknown` y estrecha). Prohibidos `@ts-ignore` sin justificación.
- Modela estados con **uniones discriminadas**, no con booleanos sueltos:

```ts
type QuoteView =
  | { kind: 'draft' }
  | { kind: 'sent'; sentAt: string }
  | { kind: 'approved'; approvedAt: string; channel: 'whatsapp' | 'panel' }
  | { kind: 'rejected'; reason: string };
```

## 5. Principios SOLID aplicados

### S · Responsabilidad única
Un módulo/clase/función cambia **por una sola razón** (un actor del negocio). En SWEVEH: `quotes` cambia por reglas de cotización; `messaging` cambia por el canal. Si calcular el total, enviar el WhatsApp y guardar auditoría viven en el mismo método, tres razones de cambio chocan.
*Señal:* nombres con "And", "Manager", "Helper", "Util" o archivos de > 400 líneas.

### O · Abierto/cerrado
Se extiende agregando código, no editando el existente. En SWEVEH: agregar un **proveedor de IA** nuevo = nueva implementación del puerto, sin tocar los casos de uso; agregar una **automatización** (AU-xx) = nueva entrada en el registro, no un `if` más en el despachador.

```go
// ❌ cada proveedor nuevo obliga a editar la función
func Complete(provider string, p Prompt) (Result, error) {
    switch provider {
    case "openai": ...
    case "anthropic": ...
    }
}

// ✅ el caso de uso depende del puerto; los proveedores se registran en el arranque
type LLM interface{ Complete(ctx context.Context, p Prompt) (Result, error) }
```
*No abuses:* abre solo donde ya hay **dos variantes reales**. Un `switch` sobre un enum cerrado y estable (los 7 estados de la orden) está bien.

### L · Sustitución de Liskov
Cualquier implementación de un puerto debe poder reemplazar a otra **sin sorpresas**: mismas precondiciones, postcondiciones y errores. Un `FakeChannel` de pruebas y el `WhatsAppChannel` real cumplen el mismo contrato (por eso se prueban con la misma *suite de contrato*). Si una implementación lanza "no soportado" para un método del puerto, el puerto está mal diseñado.

### I · Segregación de interfaces
Interfaces **pequeñas y definidas por quien las consume**. En Go: "acepta interfaces, devuelve structs"; la interfaz vive en el paquete que la **usa** (`ports/`), con 1–3 métodos.

```go
// ❌ repositorio gordo que todos deben implementar
type QuoteStore interface { ByID; Save; Delete; ListByTaller; Stats; Export /* … */ }

// ✅ cada caso de uso pide solo lo que usa
type QuoteLoader interface{ ByID(ctx context.Context, tx Tx, id QuoteID) (Quote, error) }
type QuoteSaver  interface{ Save(ctx context.Context, tx Tx, q Quote) error }
```

### D · Inversión de dependencias
El dominio y la aplicación **no importan** infraestructura (pgx, HTTP, Meta, el proveedor de IA): dependen de **puertos**; los adaptadores los implementan; el ensamblado ocurre en `cmd/` (*composition root*). Es la base de la arquitectura hexagonal de SWEVEH (ADR-01) y se verifica con `depguard` + test de arquitectura. **Prohibido** el *singleton* global y el *service locator*: las dependencias entran por constructor.

## 6. DRY, KISS, YAGNI, Demeter

- **DRY** es sobre **conocimiento duplicado**, no sobre líneas parecidas. Dos bloques iguales hoy que cambian por razones distintas (la validación de placa de carro y la de moto) **no** se unifican a la fuerza. Regla de tres: abstrae a la tercera repetición *real*.
- **KISS:** la solución más simple que cumple los criterios de aceptación de la HU.
- **YAGNI:** no construyas lo que el equipo dejó fuera (ver `CLAUDE.md`, sección 9) ni "por si acaso" (multi-moneda, multi-idioma, plugins).
- **Ley de Demeter:** habla con tus colaboradores directos. `order.Customer().Phone().Value()` expone la estructura interna; mejor `order.NotifyPhone()`.
- **Tell, don't ask:** en vez de `if q.Status == X { q.Status = Y }` desde afuera, `q.Approve(...)` y la clase protege su regla.

## 7. Olores de código y cómo actuar

| Olor | Qué hacer |
|---|---|
| Función/clase gigante | Extraer por responsabilidad; empezar por lo que cambia con distinta frecuencia |
| Lista de parámetros larga | Objeto de parámetros / comando con nombre |
| Obsesión por primitivos (`string` para placa, `int` para pesos) | **Value objects**: `Placa`, `Celular`, `Money`, `CilindrajeCC` (ver `principios-poo`) |
| `switch`/`if` repetido sobre un tipo | Polimorfismo o tabla (ver `patrones-gof`: Strategy/State) |
| Envidia de características (un método usa más datos de otro objeto que de su dueño) | Mover el método al dueño de los datos (GRASP: Experto) |
| Dependencia circular entre paquetes | Introducir un puerto o mover el tipo compartido a `shared/` |
| Código muerto | Borrarlo |
| Números/cadenas mágicas | Constantes con nombre o configuración (`MaxOtpAttempts`) |
| Comentarios que explican código oscuro | Renombrar y extraer |
| Estado global mutable | Inyectar dependencias; datos inmutables |

## 8. Refactorizar con seguridad

1. **Primero pruebas** que fijen el comportamiento actual (o confirma que existen).
2. Pasos pequeños, cada uno compila y pasa pruebas; **un refactor no cambia comportamiento** y no se mezcla en el mismo commit con una funcionalidad nueva.
3. Regla del *boy scout*: deja el archivo que tocas un poco mejor, **sin** refactors masivos fuera del alcance de la HU (R-01 de `docs/02`).
4. Si el refactor cambia un contrato público o la arquitectura → ADR y avisar (R-13).

## 9. Idiomas propios de Go

- **Composición sobre herencia** (Go no tiene herencia): incrustar (`embed`) para reutilizar, interfaces para polimorfismo.
- **Opciones funcionales** para constructores con muchos parámetros opcionales:

```go
type Option func(*Sender)
func WithRetries(n int) Option { return func(s *Sender) { s.retries = n } }
func NewSender(ch Channel, opts ...Option) *Sender { s := &Sender{ch: ch, retries: 3}; for _, o := range opts { o(s) }; return s }
```
- Cero valores útiles; evita constructores que no validan: si hay invariantes, el constructor **devuelve error** (`NewPlaca(string) (Placa, error)`).
- `internal/` para ocultar; paquetes pequeños y con nombre en singular, sin `utils`/`common`/`base`.
- Concurrencia: toda goroutine tiene dueño y forma de terminar (`errgroup`, `context`); `go test -race` siempre.

## 10. Idiomas propios de TypeScript/React

- Componentes **pequeños y puros**; la lógica de datos en *hooks* (`useQuote`), no en JSX.
- Props explícitas y tipadas; sin *prop drilling* profundo: contexto o composición.
- Evita `useEffect` para derivar estado (calcúlalo en render) o para pedir datos (usa TanStack Query).
- Un archivo = un componente exportado; estilos con Tailwind y tokens del proyecto, sin CSS en línea ad hoc.
- Nunca `dangerouslySetInnerHTML` sin sanear (regla de ESLint del proyecto).

## 11. Lista de verificación antes de pedir revisión

- [ ] Los nombres dicen la intención de negocio y usan el glosario.
- [ ] Ninguna función mezcla niveles de abstracción; sin anidación profunda.
- [ ] Las interfaces son pequeñas y están del lado del consumidor.
- [ ] El dominio no importa infraestructura.
- [ ] Sin duplicación de **conocimiento**; sin abstracciones sin segunda necesidad real.
- [ ] Errores envueltos con contexto, tipados en el dominio, registrados una sola vez.
- [ ] Sin código comentado, sin `TODO` huérfanos, sin números mágicos.
- [ ] Pruebas que explican el comportamiento (ver `sweveh-testing`).
