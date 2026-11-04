# MVP SWEVEH: requisitos funcionales, no funcionales, demo y criterios de aceptación

> Copia de trabajo del documento "MVP SWEVEH" (30-oct-2026) — original: https://claude.ai/artifact/9Cbp2G4mjX6rkVbJyUjCcq
> Conserva las etiquetas del original: `[Equipo]`, `[Propuesta]`, `[Oficial]`, `[Tercero]`, `[Sin dato]`, `[Ficticio]` y las prioridades `[Must]`, `[Should]`, `[Later]`.

## ERRATA VIGENTE: lo que cambió después de este documento (esto MANDA sobre el texto de abajo)

El equipo cambió el rumbo después del 30-oct. Donde el texto de abajo contradiga esta tabla, **gana la tabla**. (Regla de precedencia: `docs/02-reglas-de-decision.md`, sección 1.)

| # | El texto original dice… | Vigente desde | Ahora es… |
|---|---|---|---|
| 1 | Backend **Spring Boot** (sección "Datos") | 1-nov | **Go**; ver `docs/06-arquitectura.md` |
| 2 | Base de datos "PostgreSQL" genérica | 1-nov | **Supabase** (Postgres + Storage), una base, un esquema por módulo |
| 3 | **VIN** opcional en el vehículo (M2 y entidad Vehiculo) | 1-nov | **Sin VIN.** No se guarda |
| 4 | Vehiculo sin motor | 1-nov | Todo vehículo tiene **motor** con `cilindraje_cc` y `turbo` (sí/no); "motor pendiente" hasta pasar a Recibido |
| 5 | **Solo carros**; motos fuera (alcance y D-05) | 31-oct | **Carros y motos.** D-05 queda resuelta. Placa de moto `ABC12D`/`ABC12` (Q-01), checklist de moto (HU-18), escena 5-M opcional |
| 6 | Prioridades Must / Should / Later; "todo Should se corta" | 1-nov | **Todo es Must.** Ya no hay corte automático; el orden lo dan las dependencias (`docs/05`) |
| 7 | Medidor de mensajes y costo es un KPI del dueño (M7, RF-20, escena 9) | 1-nov | El medidor y el costo de IA son del **Administrador del ERP** (HU-74); el dueño ve los 6 KPIs. Q-24 abierta |
| 8 | AU-10 recordatorio de técnico-mecánica (Later) | 1-nov | **HU-50 cancelada.** AU-09 garantía en duda (Q-25) |
| 9 | "6 estados" en la precarga y 7 en la lista | 1-nov | **7 estados** + Cancelada fuera del flujo (Q-04, Q-05) |
| 10 | Catálogo de 10 automatizaciones | 1-nov | **9 tarjetas** (sin AU-10) |
| 11 | 22 funciones Must | 1-nov | Los 22 RF siguen; el backlog son **101 HU** (`docs/04-historias-de-usuario.md`) y cubren los 22 RF y las 10 escenas |
| 12 | Seguridad e infraestructura "propuesta" genérica | 1-nov | Definidas en `docs/06-arquitectura.md` (JWT, RBAC, cookies, RLS, outbox, observabilidad, CI/CD) |

- **Cómo leer el resto:** módulos M1–M11, RF-01 a RF-22, RNF-01 a RNF-14, taller ficticio, guion de 10 escenas, preparativos, riesgos y criterios de aceptación CA-01 a CA-12 **siguen vigentes** salvo las 12 correcciones de arriba. Las decisiones abiertas D-01 a D-13 siguen en `docs/07-preguntas-abiertas.md`.

---


# Propuesta de MVP y requisitos de la demo

Qué construir primero, qué dejar por fuera y qué tiene que pasar, minuto a minuto, para que un dueño de taller vea en una demo de 15 minutos la entrada fácil a un sistema confiable con automatizaciones reales. Nada de esto está construido: es el plano para decidir antes de programar.

`[Equipo]` lo decidió el equipo `[Propuesta]` diseño nuestro, sin dato que lo respalde `[Oficial]` fuente del dueño del dato `[Tercero]` blog, prensa o proveedor `[Sin dato]` hay que averiguarlo `[Ficticio]` inventado solo para la demo 
`[Must]` sin esto no hay demo `[Should]` entra si alcanza; la demo funciona sin él `[Later]` después del MVP 
<!-- sección 1: En una mirada -->

## El MVP cabe en una frase

Un taller pequeño se registra solo en menos de 5 minutos, recibe carros con fotos y checklist, y deja que SWEVEH avise, cotice, conteste "¿ya está mi carro?" y le muestre sus números, todo ligado a la placa.

### Lo que el equipo ya decidió

- Nicho: talleres. MVP enfocado en relación con el cliente y respaldo de todo lo que se le hace al vehículo. `[Equipo]`
- No se vende como ERP. El diferenciador es la entrada fácil a un sistema confiable con automatizaciones reales. `[Equipo]`
- El agente de WhatsApp es el demo de cómo se automatiza un proceso, uno entre muchos. `[Equipo]`
- Solo WhatsApp en el MVP, multi-tenant, identidad por celular, IA con límites y borradores, autorización previa (Ley 1581). `[Equipo]`

### Lo que propone este documento

- 11 módulos, 22 funciones Must, una demo de 10 escenas y 15 minutos. `[Propuesta]`
- WhatsApp con el número de prueba gratuito de Meta, no con WAHA. `[Propuesta]` basada en [1][6]
- Catálogo de 10 automatizaciones con interruptor; 5 encendidas en la demo. `[Propuesta]`
- La escena que Meta no puede hacer: responder "¿ya está mi carro?" con el estado real de la orden. `[Propuesta]` basada en [9]

### Lo que falta saber

- Si WEVEH Group existe como empresa para verificarse con Meta. `[Sin dato]`
- Si la coexistencia de WhatsApp está disponible en Colombia. `[Sin dato]`
- Si los dueños pagarían, y cuánto. Se valida con la encuesta y el concierge. `[Sin dato]`
- 13 decisiones abiertas al final, cada una con recomendación.
<!-- sección 2: Objetivo -->

## Qué tiene que probar el MVP

El MVP no existe para tener muchas funciones. Existe para probar tres cosas frente a dueños de talleres reales, las mismas que quedaron como supuestos en los ejercicios MakersXver+.

| # | Lo que debe probar | Cómo lo prueba la demo | Respaldo |
|---|---|---|---|
| P1 | Que entrar es fácil: el dueño ve valor la misma tarde, sin asesor. | Registro en vivo con cronómetro; primera orden con placa antes de 5 minutos. | `[Propuesta]` requisito R-01 del artifact de huecos. Supuesto S2 (la barrera es el esfuerzo, no solo el precio) sigue sin dato: Innocar reporta costo 29 %/28 % como primera barrera.[20] |
| P2 | Que las automatizaciones ahorran tiempo de WhatsApp. | El cliente escribe, el borrador sale solo, el aviso de "listo" sale solo, "¿ya está mi carro?" se contesta solo. | `[Sin dato]` supuesto S5: no hay línea base de minutos que pierde un taller en WhatsApp. En Metro Frenos el aviso es manual (visita del equipo). |
| P3 | Que el respaldo por placa da confianza al cliente del taller. | Fotos y checklist llegan al cliente al recibir el carro; la orden guarda la conversación. | `[Estudio]` J.D. Power EE. UU.: con fotos o video, 51 % aprueba la reparación frente a 24 % sin ellos.[11] Constancia de recibo exigida por art. 18 Ley 1480.[10] |

Lo que el MVP no intenta probar: que la IA diagnostica, que el taller deja de usar WhatsApp, ni que reemplaza a su contador. Pitz ya ocupa el diagnóstico con IA para el mecánico.[19]

<!-- sección 3: Alcance -->

## Qué entra y qué queda fuera

### Dentro del MVP

- Talleres de carros en una ciudad (hipótesis de Medellín, 1 a 5 personas).
- Registro en 3 pasos con precarga por tipo de taller.
- Núcleo de datos: cliente por celular, vehículo por placa, historial.
- Órdenes con estados, checklist de recepción, fotos y aprobación de cotización.
- Inventario básico de repuestos que se descuenta al usarse en una orden.
- Catálogo de automatizaciones con interruptor, incluido el agente de WhatsApp.
- Bandeja "Por revisar": todo lo que propone la IA pasa por el dueño.
- Tablero con 6 KPIs y medidor de mensajes y costo.
- Asistente que explica los números al dueño (versión mínima).
- Roles dueño y mecánico; recepción opcional.
- Autorización de datos (Ley 1581) y origen visible de cada dato.
- Migración: captura al operar y plantilla Excel/CSV.

### Fuera del MVP

- Factura electrónica DIAN y contabilidad (se integrará Siigo o Alegra después, como decidió el equipo). `[Equipo]`
- Instagram, Facebook y otros canales. `[Equipo]`
- Diagnóstico con IA, marketplace de repuestos, voz del mecánico.
- Motos: no investigadas todavía (N-09) ni formato de placa verificado.
- Constructor visual de automatizaciones (solo interruptores).
- Lectura de cuadernos completos y chats viejos con IA (falta medir precisión, H6).
- Campañas de marketing y mensajes a clientes migrados sin autorización (H7).
- Pagos en línea, nómina, multi-sede, app nativa en tiendas.
- Agenda con calendario completo (solo fecha deseada en la orden).
- Conexión del número real del taller: llega con el primer cliente real, no en la demo.
4 · Módulos y funciones

## Once módulos, priorizados

Cada fila es una función. La prioridad es `[Propuesta]` salvo donde dice otra cosa. La columna "Por qué" dice qué hueco o decisión justifica la función: H-xx y R-xx son los del artifact de huecos de la competencia; N-xx los de huecos en talleres.

### M1 · Registro y arranque

entrada fácil

| Función | Prioridad | Por qué |
|---|---|---|
| Paso 1: nombre, celular con WhatsApp verificado por código, casilla de autorización de datos con enlace a la política. | `[Must]` | Identidad por celular `[Equipo]`; Ley 1581[12] |
| Paso 2: nombre del taller, ciudad, qué hace (selección múltiple), cuántos trabajan. | `[Must]` | Alimenta la precarga; R-01 |
| Precarga según el tipo: servicios sin precio, 6 estados de orden, checklist de recepción, plantillas de mensajes, KPIs, roles. | `[Must]` | Evita la pantalla en blanco. La lista concreta de servicios está `[Sin dato]` (H8): sacarla de visitas y encuesta. |
| Paso 3: primera orden con una placa real o de ejemplo, y vista previa del aviso "vehículo recibido". | `[Must]` | Valor antes de pedir NIT, inventario o WhatsApp |
| Pedir NIT, logo, horario y precios solo cuando hagan falta (al facturar, al cotizar, al conectar WhatsApp). | `[Should]` | Artifact Arranque SWEVEH |

### M2 · Núcleo de datos por placa

cliente · vehículo · historial

| Función | Prioridad | Por qué |
|---|---|---|
| Cliente único por celular (10 dígitos), con nombre, correo opcional y estado de autorización. | `[Must]` | `[Equipo]` |
| Vehículo identificado por placa (3 letras y 3 números para carros), marca, línea, modelo, kilometraje; VIN opcional. Un cliente puede tener varios vehículos y un vehículo puede cambiar de dueño. | `[Must]` | N-03; formato de placa de la Res. 3800 de 1995[15] |
| Ficha del vehículo con historial: órdenes, fotos, conversaciones y repuestos usados. | `[Must]` | Lo que el agente de Meta no tiene en la app[9] |
| Búsqueda por placa, celular o nombre desde cualquier pantalla. | `[Must]` | Uso en el piso del taller |
| Fusión de duplicados (mismo celular o misma placa) con confirmación. | `[Should]` | Migración y captura al operar |

### M3 · Órdenes de servicio

el respaldo

| Función | Prioridad | Por qué |
|---|---|---|
| Estados: Por recibir → Recibido → En diagnóstico → Esperando aprobación → En reparación → Listo → Entregado. Cada cambio guarda quién y cuándo. | `[Must]` | Flujo v0 y visitas (aviso manual en Metro Frenos) |
| Checklist de recepción desde el celular: kilometraje, combustible, fotos de los 4 lados, golpes, objetos dentro, aceptación del cliente. | `[Must]` | Constancia de recibo, art. 18 Ley 1480[10]; R-03 |
| Fotos y notas en cualquier momento de la orden, con hora. | `[Must]` | N-02; 51 % vs 24 %[11] |
| Cotización con ítems: servicios de la lista y repuestos del inventario, con total. | `[Must]` | R-03 |
| Aprobación del cliente de la cotización completa (aprobar o tengo dudas) con registro de la respuesta. | `[Must]` | Piso que ya tienen Kreski y Tuulapp[16][18] |
| Aprobación ítem por ítem. | `[Should]` | Kreski lo ofrece; pide más diseño de mensajes |
| Conversación de WhatsApp adjunta a la orden como respaldo. | `[Must]` | H-03, R-05 |
| Fecha prometida de entrega y mecánico asignado. | `[Should]` | Alimenta "¿ya está mi carro?" y KPIs |
| Garantía por orden (días) para el seguimiento posventa. | `[Later]` | H-05 |

### M4 · Inventario básico

repuestos

| Función | Prioridad | Por qué |
|---|---|---|
| Crear, editar y desactivar repuestos: nombre, referencia, costo, precio de venta, cantidad, mínimo. | `[Must]` | MVP original del equipo (gestión básica de inventario) `[Equipo]` |
| Descontar automáticamente cuando el repuesto se usa en una orden aprobada; movimiento con referencia a la orden. | `[Must]` | Un mismo registro (R-05) |
| Entrada de mercancía manual. | `[Should]` | Operación diaria |
| Importar inventario desde Excel/CSV. | `[Should]` | Migración vía A |
| Proveedores, órdenes de compra, varias bodegas. | `[Later]` | No aporta a la demo |

### M5 · Catálogo de automatizaciones

el diferenciador

Una pantalla con tarjetas. Cada tarjeta dice qué hace, cuándo se dispara, qué mensaje envía y cuánto cuesta en mensajes; el dueño la enciende con un interruptor y puede editar el texto. Sin constructor visual (H-02, R-04).

| ID | Automatización | Se dispara cuando | Prioridad | Hueco |
|---|---|---|---|---|
| AU-01 | Aviso "vehículo recibido" con resumen del checklist y fotos. | La orden pasa a Recibido. | `[Must]` | N-01, N-02 |
| AU-02 | Cotización para aprobar por WhatsApp con botones. | La orden pasa a Esperando aprobación. | `[Must]` | R-03 |
| AU-03 | Aviso "vehículo listo" con total y horario de entrega. | La orden pasa a Listo. | `[Must]` | Metro Frenos lo hace a mano |
| AU-04 | **Agente de WhatsApp**: lee el mensaje, extrae vehículo, síntoma, servicio y fecha, redacta borrador con los precios del taller y crea la orden "Por recibir" al aprobar. | Llega un mensaje de alguien que no tiene orden abierta. | `[Must]` | H-03; flujo v0 |
| AU-05 | Respuesta a "¿ya está mi carro?" con el estado real, la fecha prometida y la última foto. | Llega un mensaje de un cliente con orden abierta y la intención es "estado". | `[Must]` | H-04, R-06 |
| AU-06 | Recordatorio de cita la mañana del día. | Orden "Por recibir" con fecha de hoy. | `[Should]` | Flujo v0 |
| AU-07 | Vehículo listo sin recoger. | Orden en Listo hace 3 días (configurable). | `[Should]` | H-05, N-05 |
| AU-08 | Alerta interna de repuesto bajo el mínimo (al dueño, dentro de la app; no cuesta mensajes). | El inventario baja del mínimo. | `[Should]` | M4 |
| AU-09 | Seguimiento de garantía. | N días después de Entregado. | `[Later]` | H-05 |
| AU-10 | Recordatorio de técnico-mecánica o mantenimiento por fecha o kilometraje. | Fecha calculada por vehículo. | `[Later]` | N-04; necesita resolver H7 (autorización para mensajes no transaccionales) |

En la demo se muestran AU-09 y AU-10 como tarjetas "próximamente" para que el dueño vea que el catálogo crece, sin prometer fechas.

### M6 · Bandeja "Por revisar"

IA propone, el dueño decide

| Función | Prioridad | Por qué |
|---|---|---|
| Tarjeta por cada propuesta de la IA: mensaje original, datos extraídos, borrador. Botones Enviar, Editar, Lo contesto yo. | `[Must]` | IA con borradores `[Equipo]`; flujo v0 |
| Registro de cada edición del dueño (qué cambió) para medir si el asistente sirve. | `[Must]` | Métrica del flujo v0: % de borradores enviados sin editar |
| Origen visible en cada dato que vino de la IA (mensaje del 30-oct 7:42, foto 2, fila 12). | `[Must]` | Auditoría; artifact Arranque |
| Filas de migración (plantilla o foto de lista de precios) para aprobar, corregir o descartar. | `[Should]` | Migración vía C |
| Auto-envío opcional, solo para horario y precio exacto de lista, si el dueño lo activa. | `[Later]` | Decisión abierta D-07 |

### M7 · Tablero de KPIs

6 números y un medidor

| KPI | Cómo se calcula | Prioridad |
|---|---|---|
| Órdenes abiertas por estado | Conteo de órdenes que no están en Entregado | `[Must]` |
| Aprobaciones pendientes | Órdenes en Esperando aprobación y horas desde que se envió la cotización | `[Must]` |
| Tiempo promedio por orden | Promedio de Recibido → Listo, últimos 30 días | `[Must]` |
| Ticket promedio | Total de órdenes entregadas ÷ número de órdenes, mes en curso | `[Must]` |
| Clientes que vuelven | % de vehículos con más de una orden en 12 meses | `[Should]` |
| Minutos hasta responder un WhatsApp | Mediana entre mensaje entrante y primera respuesta enviada | `[Must]` (H-07) |
| Medidor de mensajes y costo | Mensajes enviados en el mes por categoría, cuántos de los 1.000 de servicio gratis van usados, y costo de IA por conversación | `[Must]` (H-06, R-08)[4] |

### M8 · Asistente que guía al dueño

versión mínima

| Función | Prioridad | Por qué |
|---|---|---|
| Botón "¿qué significa?" en cada KPI que explica el número con los datos del propio taller en lenguaje simple. | `[Should]` | El equipo definió un asistente que guía al dueño `[Equipo]`; qué hace primero está por decidir (D-06). H-08 `[Sin dato]` sobre competidores. |
| "Hoy tienes pendiente": 3 acciones del día (aprobaciones sin respuesta, carros listos sin recoger, borradores en la bandeja). | `[Should]` | Se arma con reglas y consultas; no necesita IA |
| Preguntas libres al asistente sobre el negocio ("¿cuánto facturé en frenos este mes?"). | `[Later]` | Riesgo de respuestas inventadas; requiere consultas validadas |

### M9 · Usuarios y roles

multi-tenant

| Función | Prioridad | Por qué |
|---|---|---|
| Cada taller es un inquilino aislado; ningún dato cruza entre talleres. | `[Must]` | Multi-tenant `[Equipo]` |
| Roles dueño (todo) y mecánico (sus órdenes, estados, fotos; sin precios de costo ni KPIs de dinero). | `[Must]` | Si marcó "solo yo", se oculta la gestión de usuarios |
| Rol recepción. | `[Should]` | Talleres de más de 2 personas |
| Invitar usuarios por celular. | `[Should]` | Suscripción por usuarios `[Equipo]` |

### M10 · Datos personales y trazabilidad

Ley 1581

| Función | Prioridad | Por qué |
|---|---|---|
| Primer mensaje a un número nuevo pide la autorización de tratamiento de datos antes de seguir; se guarda fecha, hora y texto aceptado. | `[Must]` | Ley 1581[12]; flujo v0 |
| Taller como responsable y SWEVEH como encargado, con contrato de transmisión. | `[Must]` (documento, no código) | Concepto SIC 23-394496 `[Tercero]`[13] |
| Exportar y borrar los datos de un cliente a pedido. | `[Should]` | Derechos del titular |
| Bitácora de acciones (quién cambió qué). | `[Must]` | Sistema confiable |

### M11 · Migración

traer lo que ya existe

| Función | Prioridad | Por qué |
|---|---|---|
| Captura al operar: cada orden nueva crea cliente y vehículo. | `[Must]` | Vía E, el único camino para quien no tiene nada |
| Plantilla Excel/CSV de clientes, vehículos, servicios y repuestos con validación por fila. | `[Should]` | Vía A; "migrar desde CSV" `[Equipo]` |
| Foto de la lista de precios leída por IA hacia "Por revisar". | `[Should]` | Vía C. Precisión en talleres `[Sin dato]`; un tercero reporta 87 % en notas a mano y 1–3 % de datos inventados[14] |
| Excel propio con mapeo de columnas asistido. | `[Later]` | Vía B |
| Cuadernos completos y chats viejos. | `[Later]` | H6 y H7 sin resolver |

<!-- sección 5: Requisitos funcionales -->

## Lo que el sistema debe hacer, verificable

Versión condensada de las funciones Must, escrita para poder marcarla como cumplida o no. Todos son `[Propuesta]`.

| ID | El sistema debe… | Módulo |
|---|---|---|
| RF-01 | Permitir crear una cuenta con celular verificado por código y registrar la autorización de datos del dueño. | M1 |
| RF-02 | Precargar servicios, estados, checklist, plantillas, KPIs y roles según el tipo de taller elegido. | M1 |
| RF-03 | Llevar al dueño a crear su primera orden con placa como último paso del registro. | M1 |
| RF-04 | Identificar al cliente por celular de 10 dígitos y rechazar duplicados exactos. | M2 |
| RF-05 | Validar placa de carro con 3 letras y 3 números y crear el vehículo si no existe. | M2 |
| RF-06 | Mostrar el historial completo de un vehículo: órdenes, fotos, mensajes y repuestos. | M2 |
| RF-07 | Mover una orden por los 7 estados registrando usuario y hora de cada cambio. | M3 |
| RF-08 | Capturar el checklist de recepción y al menos 4 fotos desde un celular. | M3 |
| RF-09 | Armar una cotización con servicios y repuestos y calcular el total. | M3 |
| RF-10 | Registrar la aprobación o duda del cliente recibida por WhatsApp y cambiar el estado al aprobar. | M3 |
| RF-11 | Adjuntar a la orden cada mensaje entrante y saliente del cliente. | M3 |
| RF-12 | Gestionar repuestos (crear, editar, desactivar) y descontarlos al aprobarse la orden que los usa. | M4 |
| RF-13 | Mostrar el catálogo de automatizaciones con interruptor, texto editable y costo estimado en mensajes. | M5 |
| RF-14 | Enviar AU-01, AU-02 y AU-03 cuando la orden cambia al estado correspondiente, si están encendidas. | M5 |
| RF-15 | Ante un mensaje de cliente sin orden abierta, generar extracción y borrador (AU-04) usando solo precios y horarios del taller. | M5 |
| RF-16 | Ante "¿ya está mi carro?" de un cliente con orden abierta, responder con el estado real de la base de datos (AU-05). | M5 |
| RF-17 | Descartar cualquier salida de la IA que no cumpla el esquema o cite un precio que no está en la lista, y mandarla al humano. | M5, M6 |
| RF-18 | No enviar nada redactado por la IA sin que el dueño pulse Enviar. Las plantillas con datos del taller (AU-05 y las respuestas del agente, que solo interpreta) salen solas con el envío automático encendido (ADR-0028). | M6 |
| RF-19 | Guardar el origen de cada dato propuesto por la IA y cada edición del dueño. | M6 |
| RF-20 | Calcular los KPIs Must y el medidor de mensajes y costo a partir de los datos registrados. | M7 |
| RF-21 | Aislar los datos de cada taller y limitar al mecánico a sus funciones. | M9 |
| RF-22 | Pedir y registrar la autorización de datos antes de procesar el primer mensaje de un número nuevo. | M10 |

Nota sobre RF-16: la respuesta de estado se arma con una plantilla y datos de la base (estado, fecha prometida, foto), no con texto libre de la IA. La IA solo clasifica la intención. Así no puede inventar un estado.

<!-- sección 6: Requisitos no funcionales -->

## Cómo debe comportarse

| ID | Requisito | Meta propuesta | Respaldo |
|---|---|---|---|
| RNF-01 | Tiempo de registro | Menos de 5 minutos para una persona que no lo ha visto | `[Propuesta]` R-01 |
| RNF-02 | Celular primero | Todas las pantallas del mecánico usables a 400 px de ancho, con una mano | `[Propuesta]`; Tuulapp se usa desde móvil (ficha) |
| RNF-03 | Velocidad del borrador | Tarjeta en "Por revisar" en menos de 30 s desde que llega el mensaje | `[Propuesta]`; no hay línea base |
| RNF-04 | Velocidad de avisos | Aviso de cambio de estado enviado en menos de 1 minuto | `[Propuesta]` |
| RNF-05 | IA acotada | Una llamada al modelo por mensaje; salida JSON validada por código; cero precios fuera de lista | `[Equipo]` reglas en código; flujo v0 |
| RNF-06 | IA intercambiable | Proveedor detrás de una interfaz propia; costo y tokens registrados por conversación | `[Equipo]` capa multi-proveedor |
| RNF-07 | Canal intercambiable | WhatsApp detrás de una interfaz `CanalMensajeria` para pasar del número de prueba al número del taller sin tocar el resto | `[Propuesta]` del artifact Arranque |
| RNF-08 | Uso permitido por Meta | El agente responde solo sobre el taller; nunca como asistente general | `[Tercero]` política desde 15-ene-2026[7] |
| RNF-09 | Aislamiento | Identificador de taller en cada tabla y filtro obligatorio en cada consulta; prueba automática que intenta leer datos de otro taller | `[Propuesta]` |
| RNF-10 | Seguridad | HTTPS, contraseñas o códigos con expiración, fotos en almacenamiento privado con enlaces temporales, token de Meta solo en el servidor | `[Propuesta]` |
| RNF-11 | Datos personales | Autorización previa guardada; servidores o IA fuera de Colombia tratados como transmisión | `[Tercero]`[13]; Ley 1581[12] |
| RNF-12 | Idioma | Español de Colombia en interfaz, plantillas y borradores; pesos colombianos sin decimales | `[Propuesta]` |
| RNF-13 | Confiabilidad de la demo | Si la IA o WhatsApp fallan, la orden y el historial siguen funcionando; el borrador pasa a "Lo contesto yo" | `[Propuesta]`; flujo v0 |
| RNF-14 | Costo | Costo de IA y mensajes por taller visible y medible desde el día uno | `[Equipo]` medición de costo; H-06 |

<!-- sección 7: Datos de ejemplo -->

## El taller ficticio de la demo

`[Ficticio]` Todo en esta sección es inventado para la demo. Los precios no son referencia de mercado y las placas se escogieron al azar: en la demo no se consultan registros reales. Los celulares de los clientes son los 5 números verificados del equipo.

### Servitaller El Pistón

- Medellín, barrio Belén. Mecánica general, frenos y suspensión, diagnóstico con escáner.
- Solo carros. 3 personas: Hernán (dueño), Jhon (mecánico), Yesica (recepción, opcional).
- Horario: lunes a viernes 7:30 a. m. a 5:30 p. m.; sábado hasta la 1:00 p. m.
- Perfil: el "taller del amigo" de las visitas, sin sistema; avisa por WhatsApp a mano.

### Quién hace qué en la demo

- **Presentador** (Isaac o Pablo): maneja el panel del dueño en el portátil.
- **Celular A**: hace de Laura, cliente nueva.
- **Celular B**: hace de Andrés, cliente con carro en reparación.
- **Celular C**: el del mecánico, con la app web abierta.
- **Celular D**: el del dueño de taller invitado, si quiere probar como cliente.

### Lista de servicios

| Servicio | Precio mano de obra | Duración estimada |
|---|---|---|
| Revisión de frenos | $40.000 | 45 min |
| Cambio de pastillas delanteras | $60.000 | 1 h |
| Cambio de aceite y filtro | $25.000 | 30 min |
| Diagnóstico con escáner | $70.000 | 1 h |
| Alineación y balanceo | $80.000 | 1 h |
| Revisión antes de técnico-mecánica | $90.000 | 1 h 30 min |
| Cambio de amortiguadores (par) | sin precio cargado | 2 h |

El último servicio no tiene precio a propósito: sirve para mostrar que el borrador dice "el taller te confirma el valor" en vez de inventar un número.

### Inventario

| Ref. | Repuesto | Costo | Venta | Cantidad | Mínimo |
|---|---|---|---|---|---|
| PF-SPK | Pastillas delanteras Chevrolet Spark | $55.000 | $85.000 | 4 | 2 |
| AC-2050 | Aceite 20W-50 (cuarto) | $22.000 | $32.000 | 12 | 6 |
| FA-SPK | Filtro de aceite Spark / Aveo | $14.000 | $22.000 | 3 | 3 |
| LF-DOT4 | Líquido de frenos DOT 4 | $18.000 | $28.000 | 6 | 2 |
| BH-H4 | Bombillo H4 | $9.000 | $15.000 | 0 | 2 |

El filtro está justo en el mínimo y el bombillo agotado: alimentan la alerta AU-08 y el "Hoy tienes pendiente".

### Clientes, vehículos y órdenes precargadas

| Cliente | Vehículo | Placa | Orden | Estado al empezar la demo | Para qué sirve |
|---|---|---|---|---|---|
| Laura Gómez | Chevrolet Spark GT 2015 | KLM482 | — | No existe todavía | Escribe en vivo (escena 4) |
| Andrés Restrepo | Renault Logan 2018 | HTR315 | OS-0043 | En reparación, prometido hoy 5:00 p. m. | Pregunta "¿ya está mi carro?" (escena 7) |
| Diego Zapata | Nissan March 2019 | GBW118 | OS-0044 | Esperando aprobación hace 26 h | KPI de aprobaciones pendientes |
| Marta Ospina | Mazda 2 2020 | JNX907 | OS-0040 | Listo hace 4 días | AU-07, vehículo sin recoger |
| Carlos Mejía | Kia Picanto 2017 | FQD264 | OS-0031, OS-0038 | Entregado (2 visitas) | Historial y "clientes que vuelven" |

Además se cargan unas 20 órdenes entregadas de los últimos 30 días para que los KPIs no salgan en cero. Su ticket y tiempos se generan con valores ficticios y se marcan como datos de ejemplo en la base.

<!-- sección 8: Guion -->

## La demo, escena por escena

Unos 15 minutos con un dueño de taller sentado al lado. Cada escena dice qué se ve, quién actúa, qué prueba y qué hueco ataca. El orden importa: primero el dueño ve que entrar es fácil, después ve el WhatsApp trabajando solo, y al final ve su negocio en números. `[Propuesta]`

---
**Escena 1 · 1 min**

### Lo que pasa hoy

- **Se ve:** Una sola pantalla: el celular de un taller lleno de chats sin responder y un cuaderno.
- **Se dice:** "Usted hoy avisa a mano cuando el carro está listo, y si el cliente pregunta, alguien deja la llave para contestar." Basado en lo visto en Metro Frenos y el taller del amigo.
- **Prueba:** Nada todavía; ubica el dolor con palabras del taller.

---
**Escena 2 · 3 min**

### Registro en vivo con cronómetro

- **Se ve:** Cronómetro visible. Paso 1 celular y código; paso 2 "mecánica general, frenos, carros, 2 a 5 personas"; aparece todo precargado; paso 3 placa de ejemplo y vista previa del aviso "vehículo recibido".
- **Actúa:** El dueño invitado, si quiere; si no, el presentador.
- **Prueba:** P1. RF-01 a RF-03. Hueco H-01, R-01.
- **Ojo:** Este registro crea un taller nuevo vacío; el resto de la demo sigue en el taller ficticio ya cargado. Decirlo en voz alta.

---
**Escena 3 · 1 min**

### La lista de precios por foto `[Should]`

- **Se ve:** Se sube la foto de una lista de precios escrita a mano. Aparecen 7 filas en "Por revisar" con la foto al lado; el presentador corrige una y aprueba.
- **Prueba:** Que la IA propone y el dueño decide. Si la escena no está lista, se salta sin afectar el resto.

---
**Escena 4 · 3 min**

### Un cliente nuevo escribe por WhatsApp

> (entra) Buenas, mi Spark 2015 está chillando cuando freno, cuánto me cobran por revisar y cambiar pastillas? puedo llevarlo mañana temprano?Laura · celular A
> (sale) Hola Laura, te escribe Servitaller El Pistón. Antes de seguir, ¿aceptas el tratamiento de tus datos para atender tu servicio? [Acepto] [Ver política]Automático (RF-22)
> (entra) Acepto
> (sale) ¡Hola Laura! La revisión de frenos cuesta $40.000 y el cambio de pastillas delanteras $60.000 de mano de obra más $85.000 de pastillas. Mañana tenemos cupo a las 8:00 a. m. ¿Me regalas la placa?Borrador de la IA, enviado por Hernán con un toque
> (entra) KLM482
- **Se ve:** En el portátil aparece la tarjeta en "Por revisar": mensaje original, datos extraídos (Chevrolet Spark 2015, "chilla al frenar", revisión de frenos, mañana) y el borrador. Hernán pulsa Enviar. Al llegar la placa se crea la orden OS-0045 "Por recibir" con la conversación adjunta.
- **Prueba:** P2. RF-15, RF-17, RF-18. AU-04. Hueco H-03.
- **Ojo:** Como Laura escribió primero, la ventana de 24 h está abierta y los mensajes no necesitan plantilla aprobada.[4]

---
**Escena 5 · 2 min**

### Recepción con fotos

- **Se ve:** En el celular C, Jhon abre OS-0045, toma 4 fotos, marca kilometraje y combustible, anota un rayón en la puerta trasera. Cambia a Recibido. En el celular A llega el aviso con el resumen y las fotos.
- **Prueba:** P3. RF-08, RF-14. AU-01. N-01, N-02.

---
**Escena 6 · 1 min**

### Cotización y aprobación

- **Se ve:** Jhon agrega "cambio de pastillas" y el repuesto PF-SPK. Pasa a Esperando aprobación. Laura recibe la cotización con botones y pulsa Aprobar. La orden pasa a En reparación y el inventario de PF-SPK baja de 4 a 3.
- **Prueba:** RF-09, RF-10, RF-12. AU-02. Un mismo registro (R-05).

---
**Escena 7 · 1 min**

### "¿Ya está mi carro?"

> (entra) Hola, ya está mi carro?Andrés · celular B
> (sale) Hola Andrés. Tu Renault Logan HTR315 está en reparación. Te lo tenemos prometido para hoy a las 5:00 p. m. Esta es la última foto del trabajo. [foto]Automático con datos de la orden OS-0043 (AU-05)
- **Se dice:** "Esto lo contestó el sistema solo, con el estado real de la orden. Nadie dejó la llave."
- **Prueba:** RF-16. H-04, R-06. Es lo que el agente de Meta en la app no documenta: no se conecta a órdenes ni historial.[9]

---
**Escena 8 · 1 min**

### Listo y sin recoger

- **Se ve:** Jhon pasa OS-0043 a Listo y a Andrés le llega el aviso con el total. En el panel, la orden de Marta aparece marcada "4 días sin recoger" con el recordatorio AU-07 enviado.
- **Prueba:** AU-03 y AU-07. H-05.

---
**Escena 9 · 2 min**

### El negocio en números

Órdenes abiertas4
Aprobaciones pendientes1 · 26 h
Minutos hasta responder2 min

Valores `[Ficticio]` que saldrían de los datos de ejemplo.

- **Se ve:** Los 6 KPIs, el medidor "34 de 1.000 mensajes de servicio gratis usados este mes · IA $1.200 en el mes" y el bloque "Hoy tienes pendiente". El dueño toca "¿qué significa?" sobre el ticket promedio.
- **Prueba:** RF-20. H-06, H-07, H-08 (M8 si entra).

---
**Escena 10 · 1 min**

### El catálogo de automatizaciones y la pregunta

- **Se ve:** Las 10 tarjetas: 5 encendidas, 3 apagadas, 2 "próximamente". El dueño apaga y prende una.
- **Se dice:** "WhatsApp es solo una de las automatizaciones. Cada una se enciende con un botón y viene incluida."
- **Cierre:** Pedir compromiso concreto, como en el experimento concierge: ¿lo usaría 10 días hábiles con nosotros acompañando? ¿pagaría la preventa? Criterio propuesto: 2 o más que pagan = confirma.
9 · Requisitos técnicos e integración

## WhatsApp, IA y datos

### WhatsApp

| Tema | Requisito para la demo | Respaldo |
|---|---|---|
| Número | Número de prueba que Meta crea gratis al agregar WhatsApp a una app de desarrollador. Escribe hasta a 5 números verificados y trae plantillas preaprobadas. | `[Oficial]`[1] |
| Por qué no WAHA | WAHA conecta un número normal como WhatsApp Web; su propia documentación advierte que WhatsApp no permite clientes no oficiales y que el número puede ser bloqueado. Además el código no sirve para producción (sin plantillas ni ventana). Solo como plan B con número desechable. | `[Oficial de WAHA]`[6] |
| Recepción | Webhook HTTPS público que recibe mensajes y estados de entrega; servidor desplegado (no el portátil) para que no falle en vivo. | `[Propuesta]` |
| Ventana de 24 h | Toda la demo se diseña para que el cliente escriba primero, así las respuestas son mensajes de servicio dentro de la ventana. Los 1.000 primeros mensajes de servicio al mes por número son gratis desde el 1-oct-2026. | `[Oficial]`[4] |
| Plantillas propias | AU-01, AU-03 y AU-07 en producción necesitan plantillas de utilidad aprobadas porque pueden salir fuera de la ventana. Si se pueden crear y aprobar plantillas propias en la cuenta de prueba, y en cuánto tiempo, no lo dice la guía de inicio. | `[Sin dato]` probarlo al montar la cuenta |
| Fotos y botones | Envío de imágenes y botones de respuesta en los mensajes. Asumido por la documentación general de la Cloud API; no verificado en la guía consultada. | `[Sin dato]` verificar en el montaje |
| Paso a producción | Antes del primer cliente real, del número propio del taller, de cobrar o de guardar datos reales: SWEVEH como Tech Provider directo y Embedded Signup con coexistencia. Antes de verificar: 10 talleres nuevos por semana; después: 200. | `[Oficial]`[2][3][5] |

### IA

1. **sin IA:** Llega el mensaje por webhook y se guarda tal cual.
2. **sin IA:** Cliente por celular, autorización, ventana, horario.
3. **sin IA:** Contexto desde la base: servicios, precios, horario, historial.
4. **IA:** Una llamada: clasificar intención, extraer datos, redactar borrador.
5. **sin IA:** Validar JSON y precios contra la lista.
6. **humano:** El dueño envía, edita o contesta él.
7. **sin IA:** Envío, orden "Por recibir", avisos programados.

```
{
  "intencion": "cotizar_y_agendar",   // cotizar | agendar | estado | otra
  "vehiculo": { "marca": "Chevrolet", "linea": "Spark GT", "anio": 2015, "placa": null },
  "sintoma_cliente": "chilla cuando frena",
  "servicios": ["revision_frenos", "cambio_pastillas_del"],   // deben existir en la lista
  "fecha_deseada": "2026-10-31T08:00",
  "faltan_datos": ["placa"],
  "borrador_respuesta": "…",
  "confianza": "alta"
}
```

- Proveedor de IA detrás de una interfaz propia; se registra modelo, tokens, costo y latencia por llamada. `[Equipo]`
- Instrucciones del sistema fijas: responder solo sobre el taller, nunca diagnosticar, nunca dar un precio que no esté en el contexto. `[Propuesta]`
- Juego de 30 mensajes de prueba escritos como los escriben los clientes (con errores, audios transcritos, emojis) para medir extracción antes de la demo. `[Propuesta]`
- Qué modelo usar queda abierto (D-08). No se compara aquí porque no hay medición propia todavía.

### Datos

Propuesta de entidades mínimas. Toda tabla lleva `taller_id`. Stack `[Propuesta]` coherente con lo que el equipo ya maneja: React (web responsive), Spring Boot, PostgreSQL y almacenamiento de objetos privado para fotos.

| Entidad | Campos clave | Relación |
|---|---|---|
| Taller | nombre, ciudad, tipos de servicio, tamaño, horario, plan | Inquilino raíz |
| Usuario | celular, nombre, rol | Pertenece a un taller |
| Cliente | celular (único por taller), nombre, correo | Tiene vehículos y autorizaciones |
| Autorizacion | texto aceptado, versión de política, canal, fecha y hora | Del cliente |
| Vehiculo | placa (única por taller), marca, línea, modelo, km, VIN | De uno o más clientes en el tiempo |
| Orden | número, estado, fecha prometida, mecánico, total | De un vehículo y un cliente |
| EventoOrden | estado anterior, nuevo, usuario, hora | Historial de la orden |
| ItemOrden | tipo (servicio o repuesto), cantidad, precio, aprobado | De la orden |
| Checklist y Foto | km, combustible, observaciones; ruta privada, hora, autor | De la orden |
| Servicio | nombre, precio (puede ser nulo), duración | Catálogo del taller |
| Repuesto y MovimientoInventario | referencia, costo, venta, cantidad, mínimo; tipo, cantidad, orden | Inventario del taller |
| Conversacion y Mensaje | dirección, texto, adjuntos, id de WhatsApp, categoría, costo | Del cliente, ligada a la orden |
| Automatizacion y Ejecucion | tipo AU-xx, encendida, texto; disparo, resultado, mensaje enviado | Del taller |
| ItemPorRevisar | origen, datos propuestos, borrador, decisión, edición | Bandeja M6 |
| LlamadaIA | proveedor, modelo, tokens, costo, latencia | De un mensaje o una migración |

<!-- sección 10: Preparativos -->

## Lista para el día de la demo

### Cuentas y canal

- Cuenta de Meta for Developers y app con WhatsApp; número de prueba creado.[1]
- 5 números verificados: celulares A, B, C, D y uno de reserva.
- Token de acceso de larga duración en el servidor; revisar en la guía de Meta cuánto dura el temporal antes de confiar en él. `[Sin dato]`
- Webhook desplegado y probado el día anterior.
- Empezar en paralelo el trámite de Tech Provider (no bloquea la demo).[3]

### Datos y respaldo

- Taller ficticio cargado con un script que se puede reiniciar entre demos.
- Foto de lista de precios de ejemplo impresa (escena 3).
- Los 30 mensajes de prueba pasados por el agente con resultados revisados.
- Video grabado de la demo completa como plan B si falla la red o Meta.
- Datos móviles de respaldo para el portátil.
- Política de datos publicada en una URL (la pide la autorización y la App Review de Meta).[3]
<!-- sección 11: Riesgos -->

## Qué puede salir mal y qué hacer

| # | Riesgo | Mitigación propuesta | Respaldo |
|---|---|---|---|
| RG-01 | El dueño piensa "esto ya me lo da WhatsApp gratis" por el agente de Meta en la app. | Mostrar en la escena 7 lo que Meta no hace: estado real de la orden, historial por placa, fotos. No vender "IA que contesta". | `[Oficial]`[8][9] |
| RG-02 | El taller no registra nada y el sistema queda vacío. | Captura al operar, celular primero, el agente crea la orden por ellos; medir en el concierge si el mecánico cambia estados. | `[DANE]` 68,2 % de micronegocios sin registro contable[21] |
| RG-03 | Precio que no cierra frente a Ingo ($152.000) o Tuulapp, más costo de mensajes. | Medidor desde el día uno; precio a definir con encuesta (D-04). | `[Vendedor]`[17] |
| RG-04 | WEVEH Group no está constituida o la verificación de Meta tarda. | Demo con número de prueba (no depende de eso); iniciar trámite ya (D-02). | `[Sin dato]` plazos no publicados[3] |
| RG-05 | La IA inventa un precio o un dato. | Validación en código, borrador siempre revisado, prueba con 30 mensajes. | `[Tercero]` 1–3 % de datos inventados en lectura de documentos[14] |
| RG-06 | Uso de datos personales sin autorización. | Autorización antes del primer proceso; datos ficticios en la demo; nada de clientes reales con el número de prueba. | `[Ley 1581]`[12] |
| RG-07 | Falla en vivo (red, webhook, Meta). | Servidor desplegado, datos móviles, video de respaldo, escena 3 omitible. | `[Propuesta]` |
| RG-08 | La coexistencia no está disponible en Colombia y el taller tendría que cambiar de número. | Probar con el número de uno del equipo en Embedded Signup antes del primer piloto (H3). | `[Sin dato]`[5] |
| RG-09 | Pitz llega a Colombia a inicios de 2027 con IA incluida. | Tener talleres de referencia antes; diferenciarse en el lado del cliente final (R-10). | `[Prensa]`[19] |
| RG-10 | El alcance crece y el MVP no sale. | Solo las 22 funciones Must; todo Should se corta sin discusión si atrasa. | `[Propuesta]` |
| RG-11 | La conexión del número a la API quita funciones de la app (catálogo, difusión, respuestas rápidas, etiquetas). | Contárselo al taller antes de conectar; no aplica en la demo. | `[Oficial]`[5] |

12 · Criterios de aceptación

## Cuándo la demo está lista

Dos grupos. Los técnicos dicen si la demo funciona; los de validación dicen si sirvió. Todos son `[Propuesta]`, y los umbrales se pueden ajustar.

### Técnicos

| ID | Dado / cuando / entonces |
|---|---|
| CA-01 | Dada una persona que no conoce SWEVEH, cuando se registra sola, termina con su primera orden en menos de 5 minutos (3 de 3 intentos). |
| CA-02 | Dados los 30 mensajes de prueba, cuando pasan por el agente, ningún borrador contiene un precio que no esté en la lista y al menos 25 tienen el servicio bien extraído. |
| CA-03 | Dado un mensaje entrante, la tarjeta aparece en "Por revisar" en menos de 30 s en 9 de 10 pruebas. |
| CA-04 | Dado un cliente con orden abierta, cuando pregunta por su carro de 10 formas distintas, recibe el estado correcto en 10 de 10. |
| CA-05 | Cuando una orden cambia a Recibido, Esperando aprobación o Listo con la automatización encendida, el mensaje llega en menos de 1 minuto; con la automatización apagada, no sale nada. |
| CA-06 | Cuando el cliente aprueba, el inventario baja exactamente lo usado y el movimiento apunta a la orden. |
| CA-07 | Los KPIs coinciden con un conteo manual sobre los datos de ejemplo. |
| CA-08 | El medidor de mensajes coincide con el número de mensajes registrados en la base. |
| CA-09 | Ningún texto redactado por la IA sale sin una aprobación registrada. Lo que sale solo es plantilla con datos y queda registrado como `auto_enviado` (ADR-0028). |
| CA-10 | Un usuario del taller B no puede ver ni por URL ni por API datos del taller A. |
| CA-11 | Las pantallas del mecánico funcionan en un celular de gama media a 400 px. |
| CA-12 | Las 10 escenas corren de principio a fin en menos de 17 minutos, dos veces seguidas, sin tocar código ni la base. |

### De validación con talleres

| ID | Criterio | Respaldo |
|---|---|---|
| CV-01 | La demo se muestra a 5 o más dueños de talleres del perfil objetivo. | Experimento concierge con 3–5 talleres (MakersXver+) |
| CV-02 | Al menos 3 de 5 aceptan usarlo 10 días hábiles con acompañamiento. | `[Propuesta]` |
| CV-03 | Al menos 2 pagan la preventa. | Umbral del experimento concierge |
| CV-04 | Se anota qué escena genera más preguntas o interés, para ordenar el catálogo de automatizaciones. | `[Propuesta]` |

<!-- sección 13: Decisiones abiertas -->

## Lo que tienen que decidir ustedes

Cada una trae una recomendación `[Propuesta]`. Ninguna cambia lo que el equipo ya decidió.

| # | Pregunta | Recomendación |
|---|---|---|
| D-01 | ¿WhatsApp de la demo con número de prueba de Meta o con WAHA? | Número de prueba de Meta: oficial, gratis, el código sirve para producción. |
| D-02 | ¿Constituir WEVEH Group (o usar RUT de persona natural) y empezar ya el trámite de Tech Provider? | Empezar ya, en paralelo; no bloquea la demo pero sí el primer piloto. |
| D-03 | ¿Implementación acompañada obligatoria u opcional? | Opcional en el producto, obligatoria en los pilotos (concierge). |
| D-04 | ¿Precio del plan y tamaño de la bolsa de conversaciones? | No fijarlo antes de la encuesta; mostrar en la demo solo el medidor. |
| D-05 | ¿Carros solamente o también motos? | Solo carros en el MVP; motos no están investigadas. |
| D-06 | ¿Qué hace primero el asistente que guía al dueño? | Explicar KPIs y listar pendientes del día, sin preguntas libres. |
| D-07 | ¿El agente envía solo o siempre con aprobación? | Siempre con aprobación en el MVP; medir % sin editar antes de soltar. |
| D-08 | ¿Qué proveedor y modelo de IA para empezar? | Elegir con los 30 mensajes de prueba midiendo acierto y costo. |
| D-09 | ¿Foto de lista de precios dentro de la demo? | Sí como escena opcional; se salta si no está estable. |
| D-10 | ¿Cómo ve el taller la marca: SWEVEH, WEVEH Group o ambas? | "SWEVEH" al frente y "by WEVEH Group" en el pie (pendiente D-15 anterior). |
| D-11 | ¿Aprobación de cotización completa o ítem por ítem? | Completa en el MVP, ítem por ítem después. |
| D-12 | ¿Facturación electrónica dentro del MVP? | Fuera; integrar Siigo/Alegra después como ya decidieron, aunque Ingo y Kreski la incluyan. |
| D-13 | ¿Con qué taller real se hace la primera demo? | El taller del amigo (perfil objetivo, sin sistema). |

Fuentes

## Referencias

1. Meta for Developers, "Get started, WhatsApp Cloud API" (número de prueba, 5 destinatarios, plantillas preaprobadas). [developers.facebook.com] · consultado 30-oct-2026. Oficial.
2. Meta for Developers, "Embedded Signup overview" (pasos, límites 10/200). [developers.facebook.com]. Oficial.
3. Meta for Developers, "Get started for Tech Providers" (verificación, App Review, política de privacidad). [developers.secure.facebook.com]. Oficial.
4. Meta for Developers, "Pricing on the WhatsApp Business Platform" (cobro por mensaje, 1.000 de servicio gratis desde 1-oct-2026, COP). [developers.facebook.com]. Oficial.
5. Meta for Developers, "Onboarding WhatsApp Business app users" (coexistencia y funciones que se pierden). [developers.facebook.com]. Oficial.
6. WAHA, "Introduction" (advertencia de bloqueo y clientes no oficiales). [waha.devlike.pro]. Oficial de WAHA.
7. TechCrunch, "WhatsApp changes its terms to bar general-purpose chatbots", 18-oct-2025. [techcrunch.com]. Prensa.
8. WhatsApp Business, agente de IA en la app. [whatsappbusiness.com]. Oficial.
9. Meta for Developers, "Meta Business Agent: capabilities". [developers.facebook.com]. Oficial.
10. SIC, concepto 15-102150 de 2015 sobre el art. 18 de la Ley 1480 (constancia de recibo). [ambitojuridico.com]. Autoridad.
11. J.D. Power 2024 U.S. Aftermarket Service Index, Business Wire, 30-abr-2024. [businesswire.com]. Estudio.
12. Ley Estatutaria 1581 de 2012, Función Pública. [funcionpublica.gov.co]. Norma.
13. G&D Consulting, resumen del concepto SIC 23-394496 (datos en la nube, responsable y encargado). [gydconsulting.com]. Tercero.
14. RaftLabs, "OCR vs LLM for invoice processing". [raftlabs.com]. Tercero, metodología no publicada.
15. Ministerio de Transporte, Resolución 3800 de 1995 (placa única nacional de tres letras y tres números). [alcaldiabogota.gov.co] · consultado 30-oct-2026. Norma.
16. Kreski, software para talleres y precios. [kreski.co/industrias/talleres] · [kreski.co/precios]. Vendedor.
17. Ingo Talleres, página oficial. [ingotaller.com]. Vendedor.
18. Tuulapp, página oficial y términos vigentes. [tuulapp.com] · [legal.tuulapp.com]. Vendedor.
19. Valora Analitik, Pitz llega a Colombia, 18-may-2026. [valoraanalitik.com]. Prensa.
20. La República, uso de tecnología en talleres (encuesta Innocar y Roshfrans), 8-jun-2024. [larepublica.co]. Prensa; Innocar es casa matriz de Tuulapp.
21. DANE, boletín EMICRON 2024. [dane.gov.co]. Oficial.

### Trabajo previo del proyecto

- [Arranque SWEVEH]: registro, migración, número de prueba de Meta, WAHA, Tech Provider.
- [Ejercicios MakersXver+]: supuestos S2, S5, S6; flujo v0; experimento concierge.
- [Huecos de la competencia SWEVEH]: H-01 a H-08 y R-01 a R-10.
- [Huecos en talleres]: N-01 a N-10.
- [SWEVEH by WEVEH Group]: ficha, Tuulapp a fondo, decisiones previas.
- Visitas del equipo a Metro Frenos, Energiteca y un taller sin sistema, 28 y 29 oct 2026. Relato, sin URL.
WEVEH Group · Isaac Cano Orozco y Pablo Luis Rodríguez Burgos · Fase de ideación: SWEVEH no existe todavía y todo lo dicho sobre su funcionamiento es propuesta. El taller, sus clientes, placas y precios son ficticios.
