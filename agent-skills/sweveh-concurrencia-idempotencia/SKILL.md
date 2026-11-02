---
name: sweveh-concurrencia-idempotencia
description: Control de concurrencia, idempotencia, outbox, reintentos y resiliencia en SWEVEH (bloqueo optimista con version/ETag, descuento atómico de inventario, FOR UPDATE SKIP LOCKED, 40001/40P01, circuit breaker, backoff con jitter). Úsala al tocar inventario, órdenes, cotizaciones, webhooks, colas o llamadas externas.
---

# Concurrencia, idempotencia y resiliencia

Fuente: `docs/02` R-07 (tabla de mecanismos) y `docs/06` §8–9. Toda HU que toque estos casos exige su **prueba de concurrencia** antes de darse por terminada.

## 1. Elegir el mecanismo

| Caso | Mecanismo | Detalle |
|---|---|---|
| Editar orden, cotización, artículo | **Optimista** | columna `version`; `If-Match`/`ETag`; 409 si no coincide |
| Descontar inventario | **Atómico en SQL** | `UPDATE … SET cantidad = cantidad - $n WHERE cantidad >= $n` + movimiento en el libro, misma tx |
| Número de orden por taller | **Contador con `FOR UPDATE`** | una fila de contador por taller |
| Tocar varias filas | **Orden ascendente de ID** | evita interbloqueos |
| Entrada externa (webhook, POST con efectos) | **Idempotencia** | `wamid` único, `Idempotency-Key` |
| Cola / worker | **`FOR UPDATE SKIP LOCKED`** | + estado del mensaje |
| Tarea programada | **`pg_try_advisory_lock`** | una sola ejecución entre réplicas |
| Invariantes entre filas | **`SERIALIZABLE`** + reintento | ante `40001` y `40P01`, máx. 3, con jitter |

## 2. Bloqueo optimista (lo más frecuente)

```sql
-- db/queries/orders.sql
-- name: UpdateOrderState :one
UPDATE orders.ordenes
   SET estado = $2, version = version + 1, actualizado_en = now()
 WHERE id = $1 AND version = $3
RETURNING version;
```
```go
v, err := q.UpdateOrderState(ctx, id, to, expectedVersion)
if errors.Is(err, pgx.ErrNoRows) { return ErrVersionConflict } // HTTP 409 con problem+json
```
- La API devuelve `ETag: "<version>"`; el cliente manda `If-Match`. Sin `If-Match` en un PUT/PATCH de recurso editable → 428.
- El front, ante 409, recarga y muestra el cambio ajeno (ver `sweveh-frontend-react`).

## 3. Inventario sin sobreventa

```sql
-- name: ConsumeStock :execrows
UPDATE inventory.articulos
   SET cantidad = cantidad - $2, version = version + 1
 WHERE id = $1 AND cantidad >= $2;
```
```go
n, err := q.ConsumeStock(ctx, itemID, qty)
if err != nil { return err }
if n == 0 { return ErrInsufficientStock }
if err := q.InsertMovement(ctx, movement); err != nil { return err } // libro: quién, cuánto, por qué, orden asociada
```
Mismo commit. Nada de "leer, restar en Go, escribir".

## 4. Idempotencia

**Webhook de WhatsApp** (entrega "al menos una vez"):
```sql
INSERT INTO messaging.mensajes_entrantes (wamid, taller_id, cuerpo, recibido_en)
VALUES ($1, $2, $3, now())
ON CONFLICT (wamid) DO NOTHING;
-- 0 filas afectadas = duplicado: responde 200 y no proceses de nuevo
```
**POST con efectos** (crear orden, aprobar): encabezado `Idempotency-Key`; tabla `idempotency_keys(taller_id, clave, huella_solicitud, respuesta, estado, creado_en)` con `UNIQUE (taller_id, clave)`. Misma clave + misma huella → devuelve la respuesta guardada; misma clave + distinta huella → 422. Expira (p. ej. 24 h; valor `Propuesta`).

## 5. Outbox y worker

```go
// Productor: dentro de la misma transacción que el cambio
tx.Outbox().Add(OutboxEvent{Tipo: "order.ready", Agregado: o.ID, Payload: payload})

// Consumidor
rows, _ := tx.Query(ctx, `
  SELECT id, tipo, payload FROM outbox
   WHERE estado = 'pendiente' AND disponible_en <= now()
   ORDER BY id
   LIMIT 20
   FOR UPDATE SKIP LOCKED`)
```
- Entrega **al menos una vez** → los consumidores son **idempotentes** (clave natural del evento).
- Éxito: `estado='enviado'`. Fallo: `intentos+1`, `disponible_en = now() + backoff`, tras N intentos → `muerto` (DLQ) + alerta.
- Los efectos externos (enviar WhatsApp) ocurren **después del commit**, desde el worker, nunca dentro de la tx del caso de uso.

## 6. Reintentos de transacción

```go
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); err == nil { return nil }
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || (pg.Code != "40001" && pg.Code != "40P01") { return err }
		time.Sleep(backoffWithJitter(attempt))
	}
	return err
}
```
Solo se reintenta lo **idempotente por diseño** y solo esos códigos. Nunca reintentes ciegamente una llamada con efectos externos.

## 7. Llamadas salientes (Meta, proveedor de IA, Siigo/Alegra a futuro)

- Timeout explícito por llamada (`context.WithTimeout`).
- Reintento con **backoff exponencial + jitter**, máximo acotado, solo ante 429/5xx/red; respeta `Retry-After`.
- **Circuit breaker** por dependencia (decorador, ver `patrones-gof` §2.4): al abrir, degradar con elegancia.
- **Bulkhead:** límite de concurrencia por dependencia (semáforo) para que la IA lenta no agote el pool.
- **Degradación (RNF-13):** si falla la IA o WhatsApp, la orden y el historial siguen funcionando; el borrador pasa a "Lo contesto yo"; los mensajes pendientes quedan en el outbox.
- Los umbrales numéricos (timeouts, reintentos, apertura del breaker) son `Propuesta`: ajústalos con medición y regístralos en configuración.

## 8. Pruebas de concurrencia (obligatorias)

```go
func TestConsumeStock_NoOversell(t *testing.T) {
	db := testdb.New(t); tenant := db.NewTenant(t)
	item := tenant.CreateItem(t, qty: 10)

	var wg sync.WaitGroup
	var ok, fail atomic.Int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tenant.Consume(t, item, 1); err == nil { ok.Add(1) } else { fail.Add(1) }
		}()
	}
	wg.Wait()
	if ok.Load() != 10 || fail.Load() != 40 { t.Fatalf("ok=%d fail=%d", ok.Load(), fail.Load()) }
	if tenant.Stock(t, item) != 0 { t.Fatal("stock debe ser 0") }
}
```
Otras pruebas típicas: dos editores simultáneos de una orden (uno recibe 409); mismo `wamid` enviado 20 veces (una sola fila); dos workers sobre el mismo outbox (cada evento procesado una vez). Ejecuta con `go test -race`.

## 9. Anti-patrones

- Leer-modificar-escribir sin `version` ni `WHERE` condicional.
- Bloqueos largos (`SELECT … FOR UPDATE` mientras se llama a la IA o a Meta).
- Reintentar `INSERT` no idempotentes.
- Variables globales/maps sin mutex como "caché" compartida.
- Goroutines sin `ctx`, sin límite o sin manejo de error (`errgroup`).
- Asumir exactamente-una-vez en el webhook.

## 10. Checklist

- [ ] Mecanismo elegido según la tabla y justificado en el PR.
- [ ] `version` / `ETag` o `WHERE` condicional presente.
- [ ] Idempotencia en entradas externas.
- [ ] Efectos externos fuera de la transacción (outbox).
- [ ] Prueba de concurrencia con `-race` en el PR.
