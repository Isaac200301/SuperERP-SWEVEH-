# ADR-0038 · Un solo panel: el mecánico ve precios y dinero de la orden

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-05, 24-nov-2026); **Isaac debe revisarlo en el PR**
- **Fecha:** 2026-11-25
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Cambia:** RF-21 ("limitar al mecánico a sus funciones") y HU-81 ("sin costos ni KPIs de dinero"), errata 13 de `docs/03`
- **Relacionado con:** HU-81, HU-82, HU-75, docs/06 §6.6, `internal/identity/domain/roles.go`

## Contexto
El piloto del 27-nov es un taller chico donde todos hacen de todo: el mecánico recibe, diagnostica, cotiza y le muestra la cotización al cliente en el celular (V-10, V-11). Con RF-21 el mecánico no veía precios ni podía cotizar con descuento ni cancelar, y tenía que llamar al dueño. Pablo pidió un solo panel para los 3 roles.

## Decisión
- **Los 3 roles (dueño, recepción, mecánico) son universales** y cada taller decide quién tiene cuál. El código sigue preguntando por el permiso, nunca por el rol (R-05).
- **Mecánico:** todo lo del trabajo diario, más `money:read`, `quote:edit`, `quote:discount`, `order:cancel`, `customer:read_all`, `order:assign`, `vehicle:change_owner`, `quote:register_response` y la bandeja (`inbox:manage`, `conversation:read`, apagada con WhatsApp). **No tiene** `kpi:read`, `user:manage`, `workshop:manage`, `service:manage` (la lista de precios), `data_request:manage`, `customer:merge`, `import:run` ni `audit:read`.
- **Recepción:** lo mismo que el mecánico, más `kpi:read` (los números del taller).
- **Dueño:** todo. Solo él tiene Equipo, Mi taller (escribir sus datos y parámetros) y la lista de servicios y precios. `part:cost` y `part:manage` siguen del dueño (con el inventario apagado, V-06, casi no aplican).
- **Descuentos:** todos los que tienen `quote:discount` descuentan hasta el tope de Mi taller (`MaxDiscountPct`). El parámetro "recepción puede dar descuentos" deja de usarse; el dato queda en `tenancy.parametros` y la casilla sale de Mi taller.
- **Cancelar:** todos, con motivo obligatorio (ya lo era).
- **"Hoy tienes pendiente":** la misma lista para todo el equipo. Ninguna entrada lleva montos (placas, códigos y horas), así que quien no tiene `kpi:read` no ve dinero ahí. Los borradores solo para quien tiene `inbox:manage`. Se decide en el servidor.
- **Leer** `/workshop` y sus parámetros sigue abierto a todo el equipo (los usan la recepción y la cotización); **escribirlos** es del dueño.

## Alternativas descartadas
- **Rol nuevo "mecánico de confianza":** más combinaciones que probar y que explicar en la entrega.
- **Permisos por persona:** es lo que hará falta en talleres grandes, pero no cabe en la entrega.

## Consecuencias
- RF-21 queda reducido a "aislar los datos de cada taller y que solo el dueño configure"; HU-81 cambia su criterio 1 (errata 13 de `docs/03`).
- En talleres con mecánicos que no deben ver dinero, hará falta volver a separar (pendiente para después del piloto).

## Evidencia
- `TestHasPermission_DenyByDefault`: el mecánico ve clientes y dinero, no los 8 permisos de configuración; recepción ve KPI pero no Mi taller.
- `TestMechanic_SeesOwnOrdersWithoutMoneyOrSettings`: permisos del mecánico; 403 en `/users`, `PUT /workshop/settings`, onboarding, horario, `POST` y `PATCH /services`, `POST /parts` y `/dashboard`; cancela con motivo.
- `TestReception_Permissions`: recepción ve `/dashboard`, no `/users` ni automatizaciones.
- `TestSettings_RealQuoteWithVATHoursDiscountValidity`: recepción y mecánico descuentan hasta el mismo tope.
- `TestMechanic_PendingTodayWithoutAmounts`: el mecánico recibe el mismo pendiente que el dueño y sin montos.
