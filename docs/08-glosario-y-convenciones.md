# Glosario y convenciones

## 1. Glosario del dominio

| Término | Significado en SWEVEH |
|---|---|
| **Taller** | El cliente que paga la suscripción (inquilino o *tenant*). Atiende carros, motos o ambos |
| **Dueño** (`DU`) | Persona que administra el taller; decide precios y aprueba lo que propone la IA |
| **Mecánico** (`ME`) | Trabaja con el celular en la mano; no ve costos ni KPIs de dinero |
| **Recepción** (`RE`) | Rol opcional que registra clientes y vehículos en mostrador |
| **Cliente final** (`CL`) | Dueño del vehículo; solo usa WhatsApp, no instala nada |
| **Agente / automatizaciones** (`AG`) | Actor del sistema que trabaja en nombre del taller |
| **Administrador del ERP** (`EQ`) | Isaac y Pablo; ven consumo de mensajes y costo de IA por taller. No es un rol del taller |
| **Orden de servicio** (OS-0043) | Registro de lo que se le hace a un vehículo: estados, checklist, fotos, cotización, conversación |
| **Placa** | Identificador del vehículo (único por taller). Carro `ABC123`; moto `ABC12D` o antigua `ABC12` |
| **Motor** | Entidad del vehículo con `cilindraje_cc` y `turbo` |
| **Checklist de recepción** | Lista que el mecánico llena al recibir el vehículo (km, combustible, fotos, golpes, objetos); distinta para carro y moto (HU-17, HU-18) |
| **Constancia de recibo** | Resumen con fotos que recibe el cliente y confirma (HU-19, Ley 1480 art. 18) |
| **Cotización** | Servicios + repuestos con total; el cliente aprueba o "tiene dudas" por botón de WhatsApp |
| **Panel** | Superficie web de escritorio (dueño, recepción) |
| **Piso** | La misma web en el celular del mecánico, instalable (PWA) |
| **Motor (superficie)** | Backend, automatizaciones y agente; no tiene pantalla. No confundir con el motor del vehículo |
| **AU-01…AU-09** | Automatizaciones del catálogo (AU-10 cancelada). AU-04 = agente de WhatsApp; AU-05 = "¿ya está mi carro?" |
| **Catálogo de automatizaciones** | Pantalla de tarjetas con interruptor, texto editable y costo estimado en mensajes |
| **Por revisar** | Bandeja donde la IA propone y el dueño decide (Enviar / Editar / Lo contesto yo) |
| **Medidor** | Mensajes y costo de IA por taller; lo ve el Administrador del ERP (HU-74) |
| **Ventana de 24 h** | Periodo desde el último mensaje del cliente en el que se puede responder sin plantilla |
| **Plantilla (WhatsApp)** | Mensaje preaprobado por Meta; necesario fuera de la ventana de 24 h |
| **`wamid`** | Identificador del mensaje de WhatsApp; clave de idempotencia |
| **Tramo (T0…T7)** | Grupo de HU en orden de construcción (`docs/05`) |
| **Rebanada vertical** | Una HU construida de punta a punta (migración → UI → pruebas) |
| **RF / RNF / CA** | Requisito funcional / no funcional / criterio de aceptación de la demo |
| **Q / D / AQ** | Preguntas abiertas: backlog / MVP / arquitectura |
| **ADR** | Registro de decisión de arquitectura |
| **Outbox** | Tabla de eventos escrita en la misma transacción que el cambio de estado; la lee el worker |
| **RLS** | *Row Level Security* de Postgres: aislamiento por `taller_id` en la base |
| **RBAC** | Control de acceso por rol → permisos `recurso:acción` |
| **BOLA / IDOR** | Acceder al objeto de otro usuario cambiando un id; se previene con verificación de propiedad |
| **SCA / SAST** | Análisis de dependencias / de código propio (Snyk, SonarQube) |

## 2. Etiquetas

`Equipo`, `Propuesta`, `Oficial`, `Tercero`, `Inferencia`, `Sin dato`, `Ficticio` (ver `docs/02`, sección 2). Prioridades `Must`/`Should`/`Later` (hoy todas Must).

## 3. Nombres

- **Idioma:** la **interfaz, los mensajes de usuario, los errores visibles y los textos de WhatsApp** van en **español de Colombia**. Documentación y comentarios en español.
- **Código Go:** identificadores en **inglés** técnico y de patrón (`ApproveQuote`, `QuoteRepository`, `Outbox`), como en la arquitectura. Términos del dominio sin traducción natural pueden quedarse en español (`taller`, `placa`).
- **Base de datos:** esquemas en inglés por módulo (`orders`, `quotes`, `inventory`, `identity`…) y tablas/columnas del dominio en **español `snake_case` sin tildes** (`orders.orden`, `taller_id`, `cilindraje_cc`), tal como el modelo de la arquitectura (sección 5.5). Siempre `taller_id`, `created_at`, `updated_at` y `version` donde haya edición concurrente.
- **Permisos RBAC:** `recurso:accion` en inglés (`quote:approve`, `order:update_state`).
- **Rutas HTTP:** `/api/v1/...` en inglés plural (`/orders`, `/quotes`); por superficie: `/api/v1/panel/*`, `/api/v1/piso/*`, `/api/v1/admin/*`, `/webhooks/*`.
- **Ramas y commits:** ver R-14 de `docs/02`.

> Si una convención aquí choca con `docs/06-arquitectura.md`, gana la arquitectura y se corrige este archivo.

## 4. Formatos

| Dato | Formato |
|---|---|
| Moneda | COP **sin decimales**; en la base como entero (pesos); en pantalla `$1.250.000` (punto de miles) |
| Fechas y horas | Se guardan en **UTC** (`timestamptz`); se muestran en `America/Bogota`; en API, ISO 8601 |
| Celular | Se pide de **10 dígitos** (Colombia, empieza por 3); se guarda **E.164** (`+573…`); único por taller en `customers` |
| Placa carro | `^[A-Z]{3}[0-9]{3}$` (Res. 3800 de 1995), mayúsculas sin espacios |
| Placa moto | `^[A-Z]{3}[0-9]{2}[A-Z]$` y antigua `^[A-Z]{3}[0-9]{2}$`; **provisional** hasta confirmar con RUNT/Mintransporte (Q-01) |
| Placa especial | **`Sin dato`**: aceptar provisionalmente alfanumérico de 5 a 7 caracteres marcado como "especial" (Q-01) |
| Cilindraje | Entero en **cc** (un carro de 1,6 L se guarda 1600) |
| Turbo | Booleano `turbo` (sí/no) |
| Identificadores | UUID (UUIDv7 recomendado); número de orden legible por taller (`OS-0043`) con contador |
| Errores de API | RFC 9457 (*Problem Details*) con `request_id` |
| Paginación | Por cursor, no `OFFSET` |
| Dinero y cantidades | Nunca `float` |

## 5. Pruebas

- Backend: `*_test.go` junto al código; nombres `TestApproveQuote_ConflictOnStaleVersion`; integración con Postgres real (testcontainers); concurrencia con N goroutines y `-race`.
- Front: Vitest + Testing Library (`*.test.tsx`), MSW para contrato, Playwright para E2E (escenas de la demo), `axe-core` para accesibilidad.
- Datos de prueba: **sintéticos** (taller ficticio Servitaller El Pistón, placas y celulares inventados; ver `docs/03-mvp-requisitos.md`, sección 7). Nunca datos reales.
- Cada tabla nueva: prueba de aislamiento con **dos talleres**.

## 6. Logs y métricas (resumen)

JSON con `slog`; campos fijos `request_id`, `trace_id`, `taller_id`, `user_id` opaco, `module`, `route`, `status`, `duration_ms`. **Prohibido** registrar tokens, cookies, OTP, cuerpo completo de mensajes y celulares completos (se enmascaran). `taller_id` es etiqueta válida en métricas; `user_id`, `placa` y `wamid` no (cardinalidad). Detalle en arquitectura, sección 10.

## 7. Seguridad en una línea (para recordar)

Servidor decide, cliente solo muestra; `taller_id` sale de la sesión; sin tokens en `localStorage`; sin secretos en el repo; la IA propone y el humano aprueba.
