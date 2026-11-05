# ADR-0030 · Administrador del ERP y medidor de consumo por taller

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Equipo (1-nov-2026: el medidor es del administrador del ERP); revisión de Isaac y Pablo en el PR
- **Relacionado con:** HU-74, HU-46, HU-99, Q-24, D-04, R-05, docs/06 §6 (aislamiento por taller)

## Contexto
HU-74 pide que el **administrador del ERP** vea, por taller, el consumo de mensajes y el costo de IA del mes. El ejemplo del criterio dice "34 de 1.000" e "IA $1.200". También pide que un usuario de taller (dueño, mecánico o recepción) no lo encuentre ni en pantalla ni por URL ni por API: debe recibir "sin permiso".

Hasta ahora todo el sistema era por taller. Los roles (dueño, recepción, mecánico) y los permisos viven dentro de un taller, y la base aísla cada taller con RLS (`tenancy.taller_actual()`). El api no puede listar los talleres; solo el worker tiene una política para hacerlo (00009).

## Decisión
- **Quién es administrador:** los celulares de `PLATFORM_ADMINS` (lista separada por comas, guardada en E.164). Es configuración del servidor y no un rol del taller, así que un dueño no puede dárselo ni quitárselo desde la app. La regla se decide en **un solo lugar**, `server.Modules.PlatformAdmin`, que compara el celular del usuario de la sesión. `/me` responde `adminPlataforma` para que la pantalla muestre el enlace.
- **Endpoint `GET /api/v1/platform/usage`:**
  - Si quien llama no es administrador, responde **403 `sin_permiso`**.
  - Si es administrador, devuelve por taller los mensajes enviados del mes (enviados, entregados o leídos; los fallidos no cuentan), los incluidos, el costo de IA en micro USD y en pesos, y si se pasó. El que más cuesta va primero.
- **Excepción controlada al aislamiento** (migración 00025):
  - La política `taller_admin_lista` deja al api **listar solo los talleres (id y nombre)**, y únicamente dentro de una transacción en la que el código fijó `app.admin_plataforma = 'on'` después de comprobar que el usuario es administrador.
  - Las cifras de cada taller se leen después **dentro de ese taller** (`app.taller_id`), con el aislamiento normal.
  - Sin `SECURITY DEFINER` ni `BYPASSRLS`, y el valor no sale de nada que mande el usuario.
- **Valores provisionales**, mientras D-04 y Q-24 siguen abiertas:

  | Variable | Para qué | Por defecto |
  |---|---|---|
  | `PLAN_MENSAJES_INCLUIDOS` | Mensajes incluidos por taller | 1.000 |
  | `TRM_COP_POR_USD` | Pasar a pesos el costo de IA, que se registra en USD | 4.000 |

  Si alguna está mal configurada, el api no arranca.
- **La escena 9** sigue con los 6 KPIs del taller, y el medidor se muestra aparte, en la vista del administrador ("Plataforma"). Es la recomendación de Q-24.

## Consecuencias
- El equipo detecta al taller que se pasa de mensajes o de IA sin entrar a cada taller y sin una puerta trasera en la base.
- El dueño no ve costos de IA ni el medidor (Q-24, HU-74 criterio 4).
- **Riesgo:** quien controle la configuración del servidor puede nombrar administradores. Por eso va en el gestor de secretos y no en la base. Cada consulta al medidor deja su traza en el log de solicitudes con el usuario.
- **Cuando se decida el plan (D-04)**, los incluidos pueden pasar a ser por taller. La vista ya devuelve `incluidos` en cada fila.
