# ADR-0039 · Inventario en otro programa (interruptor)

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-06, 24-nov-2026); Isaac revisa en el PR
- **Fecha:** 2026-11-25
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Relacionado con:** HU-110 (nueva), HU-32 a HU-38, HU-26, HU-27, V-16, Q-41

## Contexto
El taller del piloto ya lleva su inventario en otro programa. Llevarlo también en SWEVEH es trabajo doble y, si no se hace, las existencias quedan mal y la cotización avisa "sin existencia" de algo que sí hay. Pablo pidió apagar el inventario sin borrarlo.

## Decisión
- **Parámetro del taller** `tenancy.parametros.inventario_externo` (migración 00033), `false` por defecto: los talleres que ya existen no cambian. Se cambia en Mi taller → Parámetros → **Avanzado** ("Llevo el inventario en otro programa") y queda en la bitácora con el resto de los parámetros. En el paso 2 del registro la casilla viene **marcada** (Q-41).
- **Con el interruptor encendido:**
  - una línea de repuesto se escribe a mano: descripción, cantidad y un valor mayor que cero que pone el taller (nunca se sugiere, RF-17). Un repuesto del inventario (`refId`) responde 409 `inventario_externo`;
  - al aprobar no se descuentan existencias, ni siquiera de líneas viejas con referencia; cancelar no devuelve repuestos (HU-26) y "Devolver al inventario" (HU-38) no aparece;
  - `/parts*` responde 409 `inventario_externo` al escribir (crear, editar, entrada, devolución, importación, foto); la lectura sigue por si hay datos viejos;
  - "Hoy tienes pendiente" no trae la alerta de stock bajo (HU-36).
- **Web:** Repuestos sale del menú, de "Más" y de los accesos rápidos; la pestaña Inventario de Taller no aparece; la cotización muestra "Repuesto escrito a mano" en vez de los repuestos del inventario.
- Las líneas con referencia que ya existían siguen funcionando.

## Consecuencias
- No hay control de existencias en SWEVEH para esos talleres; el gasto real se lleva con "lo gastado" (V-08, R6).
- Volver a encender el interruptor no reconstruye existencias: el taller las ajusta con una entrada.

## Evidencia
- `TestExternalInventory_ManualPartsNoStockMoves`: 409 en `/parts` al escribir y al cotizar un repuesto con referencia; el repuesto a mano sin valor o en cero falla; aprobar no mueve la existencia (5 sigue en 5) y cancelar no deja movimientos; la bitácora guarda el cambio.
- `TestExternalInventory_PerWorkshopAndOnboarding`: el interruptor es de cada taller (R-06) y el registro con la casilla marcada lo enciende.
- Vitest `externalInventory.test.tsx`: Repuestos sale del menú y la cotización pide el repuesto a mano sin sugerir el valor.
