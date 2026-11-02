---
name: grasp
description: Los 9 principios GRASP (Information Expert, Creator, Controller, Low Coupling, High Cohesion, Polymorphism, Pure Fabrication, Indirection, Protected Variations) aplicados a la asignación de responsabilidades en SWEVEH con Go y TypeScript. Úsala al decidir en qué módulo, tipo o capa va cada comportamiento.
---

# GRASP en SWEVEH

GRASP (*General Responsibility Assignment Software Patterns*) viene de Craig Larman, *Applying UML and Patterns*. Responde a una sola pregunta: **¿quién debería tener esta responsabilidad?** Referencia: https://en.wikipedia.org/wiki/GRASP_(object-oriented_design)

Se usa **antes** de elegir un patrón GoF (`patrones-gof`) y junto con `clean-code-solid`.

## Resumen en una tabla

| Principio | Pregunta | Ejemplo SWEVEH |
|---|---|---|
| Information Expert | ¿Quién tiene los datos para hacerlo? | `Quote.Total()` en la cotización |
| Creator | ¿Quién debe crear X? | `Order` crea sus `ServiceItem`; el agregado crea sus partes |
| Controller | ¿Quién recibe el evento externo? | handler HTTP / webhook → caso de uso |
| Low Coupling | ¿Cómo dependo de lo mínimo? | módulos hablan por puertos y eventos |
| High Cohesion | ¿Todo en este tipo trata de lo mismo? | un módulo = un contexto (órdenes, vehículos, clientes) |
| Polymorphism | ¿Varía por tipo? | `PlateRule` carro/moto |
| Pure Fabrication | ¿Necesito un tipo que no existe en el dominio? | `OrderRepository`, `Clock`, `UnitOfWork` |
| Indirection | ¿Qué intermediario desacopla? | puerto `CanalMensajeria` |
| Protected Variations | ¿Qué punto de variación debo aislar? | proveedor de IA, BSP de WhatsApp, región de Supabase |

## 1. Information Expert
Asigna la responsabilidad al tipo que **ya tiene la información** necesaria.

```go
// Bien: el experto en las líneas es la cotización.
func (q Quote) Total() Money {
	t := Zero()
	for _, l := range q.Lines { t = t.Add(l.Total()) }
	return t
}
// Mal: un servicio externo que recorre q.Lines y suma (modelo anémico, viola "tell, don't ask").
```
Contraindicación: no pongas en la entidad lo que requiere I/O (consultar otro módulo, enviar WhatsApp); eso va en el caso de uso.

## 2. Creator
`B` debería crear `A` si `B` contiene/agrega a `A`, lo registra, lo usa de cerca o tiene los datos de inicialización.

```go
func (o *Order) AddService(s ServiceTypeID, price Money) (ServiceItem, error) {
	if o.State.IsClosed() { return ServiceItem{}, ErrOrderClosed }
	it := ServiceItem{ID: NewServiceItemID(), Type: s, Price: price}
	o.Items = append(o.Items, it)
	return it, nil
}
```
Las entidades externas al agregado se referencian **por ID**, no se crean desde dentro.

## 3. Controller
Primer objeto, **sin lógica de negocio**, que recibe una petición del sistema. En SWEVEH:

- HTTP: handler (`adapters/http`) → valida/traduce → llama un caso de uso → traduce la respuesta.
- Webhook de WhatsApp: verifica firma `X-Hub-Signature-256` con comparación en tiempo constante, deduplica por `wamid`, encola; no decide nada.
- Worker de outbox: toma un evento y lo despacha al caso de uso.

```go
func (h *QuoteHandler) Approve(w http.ResponseWriter, r *http.Request) {
	var req ApproveRequest
	if err := decode(r, &req); err != nil { respondError(w, err); return }
	err := h.approve.Handle(r.Context(), app.ApproveQuote{QuoteID: req.ID, Actor: actorFrom(r)})
	respond(w, err)
}
```
Si un handler tiene `if` de negocio, muévelo al dominio o al caso de uso.

## 4. Low Coupling
Reduce dependencias entre tipos/módulos para que un cambio no se propague.

- Dentro del módulo: `domain` no importa nada externo (ni `pgx`, ni `net/http`).
- Entre módulos: solo la **fachada** pública y **eventos**; nunca tablas ajenas ni paquetes internos (esquema Postgres por módulo).
- Interfaces **pequeñas definidas por el consumidor**.
- En TS: no importar de la carpeta interna de otra feature; usar su `index.ts`.

Señal de alerta: un cambio en `vehicles` obliga a editar `orders`.

## 5. High Cohesion
Cada módulo/tipo/función tiene una razón de ser clara.

- Un paquete por contexto (`identity`, `tenancy`, `customers`, `vehicles`, `orders`, `quotes`, `inventory`; el resto, según `docs/06`).
- Funciones que hacen una cosa (ver `clean-code-solid`).
- Evita `utils`, `helpers`, `common` como cajón de sastre; nombra por concepto.

## 6. Polymorphism
Cuando el comportamiento varía por tipo, usa interfaces/uniones, no cadenas de `if/switch` repetidas.

```go
type PlateRule interface{ Validate(string) (Plate, error) }
```
```ts
type Notificacion =
  | { tipo: 'ordenLista'; ordenId: string }
  | { tipo: 'cotizacionLista'; cotizacionId: string };

function plantilla(n: Notificacion): string {
  switch (n.tipo) {
    case 'ordenLista': return 'orden_lista_v1';
    case 'cotizacionLista': return 'cotizacion_lista_v1';
    default: { const _x: never = n; return _x; } // exhaustivo
  }
}
```

## 7. Pure Fabrication
Crea una clase artificial (no del dominio) para mantener alta cohesión y bajo acoplamiento. Es legítimo cuando el "experto" mezclaría persistencia o I/O con reglas de negocio.

Ejemplos aceptados en SWEVEH: repositorios, `UnitOfWork`, `Clock`, `IDGenerator`, `OutboxPublisher`, `AuditLogger`, `ValidadorSalidaIA`. No los multipliques: si no aíslan algo real, no lo crees.

## 8. Indirection
Un intermediario desacopla dos elementos.

- Puertos (`CanalMensajeria`, `LLM`, `Clock`) entre aplicación y mundo exterior.
- Bus de eventos entre módulos.
- Cuidado: demasiadas capas de indirección = *over-engineering*. Cada una debe justificar el cambio que habilita.

## 9. Protected Variations
Identifica los **puntos de inestabilidad** y envuélvelos con una interfaz estable. En SWEVEH los puntos de variación conocidos (ver `docs/02` y `docs/07`):

| Variación | Protección |
|---|---|
| Proveedor de IA | puerto `LLM` + decoradores |
| BSP / Meta directo de WhatsApp | puerto `CanalMensajeria` |
| Fly.io vs Cloud Run (AQ-01) | despliegue en contenedor, config por entorno, sin APIs de plataforma en código |
| Auth propia vs Supabase Auth (AQ-04) | puerto `Identidad`/`TokenIssuer` |
| Región de Supabase (AQ-02) | configuración, sin supuestos de ubicación |
| Reglas por tipo de vehículo | `PlateRule`/`VehicleKindKit` |
| Facturación (Siigo/Alegra, DE previa) | puerto `Facturador`, solo integración |

Las decisiones ⛔ pendientes son precisamente variaciones que debemos proteger hasta que el equipo decida.

## 10. Relación con SOLID y GoF

| GRASP | SOLID / GoF relacionado |
|---|---|
| Information Expert + High Cohesion | SRP, Tell-don't-ask |
| Low Coupling + Indirection | DIP, ISP, Facade, Adapter |
| Polymorphism | OCP, Strategy, State |
| Protected Variations | OCP, DIP, Adapter, Strategy |
| Pure Fabrication | Repository, Unit of Work, Command |
| Controller | Facade, Command (en el borde) |

## 11. Proceso para asignar una responsabilidad nueva

1. Describe la responsabilidad en una frase con verbo (p. ej., "calcular el total de la cotización").
2. ¿Quién tiene los datos? (Expert). Si son datos de varios, ¿quién es el agregado raíz?
3. ¿Necesita I/O? Si sí, caso de uso con un puerto (Pure Fabrication/Indirection).
4. ¿Varía por tipo o proveedor? Polimorfismo/Protected Variations.
5. ¿Quién la dispara? Controller en el borde, nunca el dominio.
6. Revisa acoplamiento y cohesión: ¿se importó algo prohibido? ¿el tipo ahora hace dos cosas?
7. Prueba unitaria sin base de datos ni red para el comportamiento de dominio.
