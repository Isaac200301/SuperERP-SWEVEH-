# ADR-0040 · Cotización por partidas (y lo gastado contra lo cotizado)

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-07 y V-08, 24-nov-2026); Isaac revisa en el PR
- **Fecha:** 2026-11-25 (partidas, R5). La parte de lo gastado (V-08) se agrega en R6.
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Relacionado con:** HU-105 (nueva), HU-106 (R6), HU-27, HU-30, HU-31, docs/06 §8

## Contexto
El taller del piloto hace modificaciones y restauraciones (V-14). Ahí no cotiza tornillo por tornillo: cotiza "Latonería y pintura del capó $900.000" y sabe por dentro qué incluye. Con solo líneas detalladas la cotización se volvía larga y el cliente discutía cada renglón.

## Decisión (partidas, R5)
- **Una partida es una línea** de la cotización con `tipo = 'partida'`: nombre, cantidad 1 y un **valor que pone el taller** (mayor que cero, nunca sugerido, RF-17). Convive con las líneas de servicio y de repuesto, y le aplican el mismo descuento con tope y el mismo IVA (incluido o aparte).
- **Detalle interno opcional** (`quotes.partida_detalle`, migración 00034, RLS forzado): renglones con descripción, cantidad y un valor de referencia opcional, hasta 40. Es informativo: **su suma no cambia el valor de la partida ni el total**.
- **"Mostrar detalle al cliente"** (`quotes.cotizacion.mostrar_detalle`, por cotización): la vista del cliente y el PDF (R7) enseñan el detalle solo si está encendido. El detalle no sale nunca en otro lado.
- Todo se edita solo en borrador: lo que el cliente ya vio no cambia; para más, la cotización adicional (HU-31).
- **Persistencia:** la cotización se guarda entera con su versión (como sus líneas): el detalle se borra y se vuelve a escribir en la misma transacción. `linea_id` no lleva FK porque las líneas también se reescriben en cada guardado; al quitar una línea, el mismo `Save` borra todo el detalle de la cotización y solo vuelve a escribir el de las líneas que quedan, así que no quedan renglones huérfanos.
- **Concurrencia (R-07, hallazgo del revisor):** la cotización expone `version` y las ediciones nuevas (`PUT .../detail`, `PATCH /quote`, y renombrar en `PATCH .../lines/{lineId}`) exigen la versión que tenía la pantalla; si otra persona guardó después, 409 `version_conflicto` y no se cambia nada. Las demás ediciones de la cotización (cantidad, precio, descuento, quitar) la aceptan pero no la exigen todavía: deuda anotada en `docs/07`.
- **API:** `POST /orders/{id}/quote/lines` con `tipo: "partida"`, `PATCH .../lines/{lineId}` con `descripcion` (nombre) y `precioUnitario` (valor), `PUT .../lines/{lineId}/detail` y `PATCH /orders/{id}/quote` `{mostrarDetalle}`. Bitácora `cotizacion.detalle_partida` y `cotizacion.mostrar_detalle`.
- Una partida no es un repuesto: aprobarla no descuenta inventario.

## Consecuencias
- La aprobación ítem por ítem (HU-30) trata la partida como un ítem más.

## Evidencia
- Dominio: `TestPartida_TotalsIgnoreDetail` (IVA aparte e incluido, descuento, el detalle no cambia el total, no descuenta inventario) y `TestPartida_RulesAndDetail`.
- HTTP: `TestPartidas_CRUDDetailAndTotals` (valor obligatorio, detalle, nombre y valor, descuento con tope, mostrar detalle, bitácora, no se edita enviada, aislamiento con dos talleres) `TestPartidas_StaleScreenGets409` (una pantalla con la versión vieja recibe 409 y el detalle de la otra persona queda) y `TestPartidas_ConcurrentDetailEdits` (6 a la vez con la misma versión: gana exactamente una).
- Vitest `partidas.test.tsx`.
