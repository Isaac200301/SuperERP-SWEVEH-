# Historias de usuario del MVP SWEVEH (v3, 1-nov-2026)

> Fuente de verdad del alcance funcional. Generado desde el backlog del equipo; los cambios se hacen aquí y se registran en `docs/02-reglas-de-decision.md` (regla R-12).

- **101 historias vigentes**, todas **Must** (decisión del equipo, 1-nov-2026) y **1 cancelada** (HU-50).
- 14 épicas, 6 roles, 5 superficies (Panel, Piso, WhatsApp, Motor, Admin). Carros y motos.
- Cada historia guarda su prioridad anterior (Should/Later) solo como dato histórico.
- Cambios del equipo ya aplicados: sin VIN; todo vehículo tiene motor con cilindraje (cc) y turbo (sí/no); HU-74 es del rol Administrador del ERP; HU-50 cancelada.

## Roles

- **Dueño del taller** (`DU`): Hernán en la demo. Decide precios, aprueba lo que propone la IA, mira los números. En talleres de 1 persona también es mecánico y recepción.
- **Mecánico** (`ME`): Jhon en la demo. Trabaja con el celular en la mano: recibe el vehículo, toma fotos, cambia estados, arma la cotización. No ve costos ni KPIs de dinero.
- **Recepción** (`RE`): Yesica en la demo, rol opcional por taller (todas las HU son Must). Registra clientes y vehículos en mostrador y atiende la bandeja si el dueño lo permite.
- **Cliente final** (`CL`): Laura (carro) y Sebastián (moto) en la demo. Solo usa WhatsApp: no instala nada ni entra a ninguna página.
- **Agente y automatizaciones** (`AG`): Actor del sistema que trabaja en nombre del taller: el agente de WhatsApp (AU-04, AU-05) y los avisos automáticos (AU-01 a AU-08). Es el demo de cómo se automatiza un proceso, no el producto completo.
- **Administrador del ERP** (`EQ`): Isaac y Pablo como administradores de SWEVEH (WEVEH Group). No es un rol del taller: ven el consumo de mensajes y el costo de IA de cada taller, porque ese costo va dentro de la suscripción mensual, y preparan, reinician y vigilan la demo.

## Superficies

- **Panel:** web de escritorio para dueño y recepción.
- **Piso:** la misma web en el celular del mecánico, instalable (PWA).
- **WhatsApp:** lo que ve el cliente final (no instala nada).
- **Motor:** backend, automatizaciones y agente (sin pantalla).
- **Admin:** aplicación del Administrador del ERP (Isaac y Pablo).

## Índice por épica

- **E01 · Registro y arranque** (M1): HU-01, HU-02, HU-03, HU-04, HU-05, HU-06, HU-07, HU-08
- **E02 · Cliente, vehículo y placa** (M2): HU-09, HU-10, HU-11, HU-12, HU-13, HU-14, HU-15
- **E03 · Recepción y orden de servicio** (M3): HU-16, HU-17, HU-18, HU-19, HU-20, HU-21, HU-22, HU-23, HU-24, HU-25, HU-26
- **E04 · Cotización y aprobación** (M3): HU-27, HU-28, HU-29, HU-30, HU-31
- **E05 · Inventario básico** (M4): HU-32, HU-33, HU-34, HU-35, HU-36, HU-37, HU-38, HU-39
- **E06 · Catálogo de automatizaciones y avisos** (M5): HU-40, HU-41, HU-42, HU-43, HU-44, HU-45, HU-46, HU-47, HU-48, HU-49, HU-50
- **E07 · Agente de WhatsApp** (M5): HU-51, HU-52, HU-53, HU-54, HU-55, HU-56, HU-57, HU-58, HU-59, HU-60, HU-61
- **E08 · Bandeja "Por revisar"** (M6): HU-62, HU-63, HU-64, HU-65, HU-66, HU-67
- **E09 · Tablero de KPIs y medidor** (M7): HU-68, HU-69, HU-70, HU-71, HU-72, HU-73, HU-74
- **E10 · Asistente que guía al dueño** (M8): HU-75, HU-76, HU-77, HU-78
- **E11 · Usuarios, roles y aislamiento** (M9): HU-79, HU-80, HU-81, HU-82, HU-83, HU-84
- **E12 · Datos personales y trazabilidad** (M10): HU-85, HU-86, HU-87, HU-88, HU-89, HU-90
- **E13 · Migración** (M11): HU-91, HU-92, HU-93, HU-94, HU-95, HU-96
- **E14 · Operación de la demo** (Preparativos): HU-97, HU-98, HU-99, HU-100, HU-101, HU-102

## E01 · Registro y arranque

_Módulo del MVP: M1. Entrar en menos de 5 minutos sin asesor y ver valor la misma tarde (P1)._

### HU-01 · Crear la cuenta con mi celular

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel, Piso, Motor · **Prioridad:** Must · **Origen:** Equipo

**Como** dueño de un taller sin sistema, **quiero** crear mi cuenta solo con mi nombre y mi celular, confirmándolo con un código **para** entrar sin correo, sin contraseña larga y sin llamar a un asesor.

**Criterios de aceptación**

- **Dado** un celular colombiano de 10 dígitos que no está registrado, **cuando** pido el código, **entonces** recibo un código de un solo uso que vence a los 10 minutos y la pantalla me deja escribirlo.
- **Dado** un código vencido o equivocado, **cuando** lo escribo, **entonces** el sistema me dice cuál fue el problema y me ofrece pedir otro; después de 5 intentos fallidos bloquea ese celular 15 minutos.
- **Dado** un celular que ya tiene cuenta, **cuando** intento registrarme, **entonces** el sistema no crea una cuenta nueva y me ofrece entrar con ese celular.
- **Dado** el registro completo, **cuando** lo mide el cronómetro de la demo, **entonces** los tres pasos, hasta la primera orden, toman menos de 5 minutos para alguien que no conoce SWEVEH.

**Notas:** Identidad por celular es decisión del equipo. Los tiempos de vencimiento e intentos son propuesta. Canal del código (SMS o plantilla de autenticación de WhatsApp) sin dato: ver Q-13. Fuentes: [1], [2]

**Trazabilidad:** RF: RF-01 · Escena: 2 · Aceptación: CA-01

### HU-02 · Autorizar el tratamiento de mis propios datos

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must · **Origen:** Respaldo oficial

**Como** dueño que se registra, **quiero** leer y aceptar la política de datos de SWEVEH con una casilla y un enlace **para** saber qué se hace con mis datos y que quede prueba de que acepté.

**Criterios de aceptación**

- **Dado** el paso 1 del registro, **cuando** la pantalla carga, **entonces** la casilla de autorización aparece sin marcar y con enlace a la política publicada.
- **Dado** la casilla sin marcar, **cuando** intento continuar, **entonces** no puedo pasar al paso 2 y el sistema me dice por qué.
- **Dado** la casilla marcada, **cuando** continúo, **entonces** queda guardado el texto aceptado, la versión de la política, el canal, la fecha y la hora.

**Notas:** La Ley 1581 exige autorización previa, expresa e informada (art. 9). El Decreto 1377 de 2013 dice que el silencio nunca es conducta inequívoca (art. 7) y que hay que conservar la prueba (art. 8): por eso la casilla no puede venir marcada. Fuentes: [7], [8]

**Trazabilidad:** RF: RF-01 · Escena: 2 · Depende de: HU-90

### HU-03 · Contar qué tipo de taller tengo

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Equipo · **Aplica a motos:** sí

**Como** dueño en el paso 2 del registro, **quiero** escoger qué servicios hago, si atiendo carros, motos o ambos, en qué ciudad estoy y cuántos trabajamos **para** que SWEVEH me arme todo sin que yo configure nada.

**Criterios de aceptación**

- **Dado** el paso 2, **cuando** la pantalla carga, **entonces** veo selección múltiple de servicios, una pregunta obligatoria "¿qué vehículos atiende?" con Carros, Motos o Ambos, la ciudad y el tamaño (solo yo, 2 a 5, más de 5).
- **Dado** que no escogí tipo de vehículo, **cuando** intento continuar, **entonces** el sistema me lo pide antes de seguir.
- **Dado** que escogí "solo yo", **cuando** termino el registro, **entonces** el sistema no me muestra gestión de usuarios (ver HU-83).

**Carro / moto:** Alcance decidido por el equipo el 31-oct-2026: talleres de motos o de carros. La pregunta de tipo de vehículo cambia la precarga (HU-04), la validación de placa (HU-10) y el checklist (HU-18).

**Notas:** Reemplaza la recomendación D-05 del documento del MVP ("solo carros"), que el equipo ya resolvió. Fuentes: [19]

**Trazabilidad:** RF: RF-02 · Escena: 2

### HU-04 · Recibir el taller ya armado

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño que acaba de decir qué tipo de taller tiene, **quiero** encontrar precargados los servicios, los estados de orden, el checklist de recepción, las plantillas de mensajes, los KPIs y los roles **para** no empezar con una pantalla en blanco.

**Criterios de aceptación**

- **Dado** que escogí Carros, **cuando** termino el paso 2, **entonces** veo la lista de servicios de carro (sin precio), el checklist de carro y las 7 etapas de la orden.
- **Dado** que escogí Motos, **cuando** termino el paso 2, **entonces** veo la lista de servicios de moto (sin precio) y el checklist de moto.
- **Dado** que escogí Ambos, **cuando** termino el paso 2, **entonces** veo las dos listas y los dos checklists; cada servicio dice si es de carro, de moto o de ambos.
- **Dado** cualquier precarga, **cuando** la reviso, **entonces** puedo borrar, renombrar o agregar servicios sin salir del registro.

**Carro / moto:** La lista concreta de servicios de carro sigue sin dato (H8) y la de moto tampoco está investigada (N-09). Ver Q-02.

**Trazabilidad:** RF: RF-02 · Escena: 2 · Depende de: HU-03

### HU-05 · Hacer mi primera orden antes de terminar

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño en el paso 3, **quiero** crear una orden con una placa real o de ejemplo y ver cómo le llegaría al cliente el aviso de "vehículo recibido" **para** ver valor antes de que me pidan NIT, inventario o WhatsApp.

**Criterios de aceptación**

- **Dado** el paso 3, **cuando** escribo una placa, **entonces** el sistema la valida según el tipo (carro o moto) y crea cliente, vehículo y orden.
- **Dado** que no tengo un vehículo a mano, **cuando** pulso "usar ejemplo", **entonces** se crea una orden de ejemplo marcada como tal, con placa de carro o de moto según lo que atiendo.
- **Dado** la orden creada, **cuando** la pantalla avanza, **entonces** veo la vista previa del aviso con el nombre de mi taller, la placa y un espacio para las fotos, sin que se envíe nada.

**Trazabilidad:** RF: RF-03 · Escena: 2 · Aceptación: CA-01 · Depende de: HU-04, HU-10

### HU-06 · Que me pidan los datos solo cuando hagan falta

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño ocupado, **quiero** que el NIT, el logo, el horario y los precios se me pidan en el momento en que se necesitan **para** no abandonar el registro por un formulario largo.

**Criterios de aceptación**

- **Dado** un servicio sin precio, **cuando** lo agrego a una cotización, **entonces** el sistema me pide el precio ahí mismo y lo guarda en la lista.
- **Dado** que no tengo horario cargado, **cuando** enciendo el agente de WhatsApp, **entonces** el sistema me pide el horario antes de encenderlo.

**Trazabilidad:** Depende de: HU-07, HU-08

### HU-07 · Poner el precio y la duración de mis servicios

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ponerle precio de mano de obra y duración a cada servicio, o dejarlo sin precio **para** que la cotización y el agente usen mis precios y nunca inventen uno.

**Criterios de aceptación**

- **Dado** un servicio, **cuando** le pongo precio, **entonces** se guarda en pesos sin decimales y desde ese momento lo usan la cotización y el agente.
- **Dado** un servicio sin precio, **cuando** el agente lo menciona, **entonces** el borrador dice "el taller te confirma el valor" en vez de un número.
- **Dado** un precio cambiado, **cuando** miro una cotización vieja, **entonces** la cotización vieja conserva el precio con que se envió.

**Notas:** Derivada: el documento del MVP la da por hecha (servicio con "precio, puede ser nulo") pero no la escribe como función. Sin ella no hay RF-15. Fuentes: [19]

**Trazabilidad:** RF: RF-09, RF-15, RF-17 · Escena: 4, 6 · Aceptación: CA-02

### HU-08 · Cargar el horario del taller

**Épica:** E01 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** decir qué días y horas atiendo **para** que el agente ofrezca citas reales y los avisos digan cuándo recoger.

**Criterios de aceptación**

- **Dado** el horario cargado, **cuando** el agente redacta una cita, **entonces** solo propone horas dentro del horario.
- **Dado** un aviso de vehículo listo, **cuando** se envía, **entonces** incluye el horario de entrega del día.

**Notas:** Derivada de RF-15 ("usando solo precios y horarios del taller").

**Trazabilidad:** RF: RF-15 · Automatización: AU-03, AU-04 · Escena: 4, 8

## E02 · Cliente, vehículo y placa

_Módulo del MVP: M2. Un solo registro por celular y por placa, sea carro o moto, con todo su historial._

### HU-09 · Registrar al cliente por su celular

**Épica:** E02 · **Roles:** Recepción, Mecánico, Dueño del taller · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Equipo

**Como** persona que recibe al cliente, **quiero** buscar o crear al cliente por su celular de 10 dígitos con nombre y correo opcional **para** no tener el mismo cliente dos veces.

**Criterios de aceptación**

- **Dado** un celular que ya existe en mi taller, **cuando** lo escribo, **entonces** el sistema me muestra ese cliente en vez de crear otro.
- **Dado** un celular con menos o más de 10 dígitos, **cuando** intento guardar, **entonces** el sistema lo rechaza y me dice el formato.
- **Dado** un cliente nuevo, **cuando** lo guardo, **entonces** queda con estado de autorización "pendiente" hasta que la dé (HU-85 o HU-86).

**Trazabilidad:** RF: RF-04 · Escena: 4, 5

### HU-10 · Registrar el vehículo por su placa, sea carro o moto

**Épica:** E02 · **Roles:** Recepción, Mecánico, Dueño del taller · **Superficies:** Panel, Piso, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** persona que recibe el vehículo, **quiero** escribir la placa y que el sistema reconozca si es carro o moto y cree el vehículo si no existe **para** que todo lo que se le haga quede ligado a esa placa.

**Criterios de aceptación**

- **Dado** la placa KLM482 (3 letras y 3 números), **cuando** la escribo, **entonces** el sistema la toma como carro.
- **Dado** la placa XVN43F (3 letras, 2 números y 1 letra), **cuando** la escribo, **entonces** el sistema la toma como moto.
- **Dado** una placa de moto antigua de 3 letras y 2 números, **cuando** la escribo, **entonces** el sistema la acepta como moto y la marca "formato antiguo".
- **Dado** una placa que no encaja en ningún formato, **cuando** la escribo, **entonces** el sistema me deja guardarla como "placa especial" después de confirmar, para no bloquear la recepción.
- **Dado** una placa que ya existe en mi taller, **cuando** la escribo, **entonces** el sistema abre ese vehículo; nunca hay dos con la misma placa en un taller.
- **Dado** minúsculas, espacios o guiones (klm-482), **cuando** la escribo, **entonces** el sistema la guarda normalizada como KLM482.

**Carro / moto:** Carro: placa única nacional de 3 letras y 3 números (Res. 3800 de 1995, oficial). Moto: 3 letras, 2 números y 1 letra, con un formato antiguo sin la letra final; solo lo dice una fuente de terceros (Wikipedia) y no se encontró la resolución oficial. Ver Q-01.

**Notas:** RF-05 hoy solo valida carros: hay que ampliarlo a motos. Fuentes: [12], [13]

**Trazabilidad:** RF: RF-05 · Escena: 4, 5

### HU-11 · Guardar los datos del vehículo y de su motor

**Épica:** E02 · **Roles:** Recepción, Mecánico · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Equipo · **Aplica a motos:** sí

**Como** mecánico, **quiero** guardar marca, línea, modelo (año), color, kilometraje y la ficha del vehículo (combustible, caja, cilindraje, turbo o atmosférico y, según el tipo, tracción o 2/4 tiempos) **para** cotizar el repuesto correcto y saber qué aceite, repuestos y procedimiento lleva.

**Criterios de aceptación**

- **Dado** un vehículo nuevo, sea carro o moto, **cuando** lo guardo, **entonces** puedo indicar combustible, caja, cilindraje (en cc) y aspiración (atmosférico, turbo o supercargado), además de marca, línea, año, color, kilometraje y medida de llanta; no hay campo VIN.
- **Dado** un carro de 1.2 L, **cuando** escribo el cilindraje, **entonces** acepto 1.2 L o 1200 cc y el sistema lo guarda siempre en cc, para poder comparar y filtrar.
- **Dado** una moto, **cuando** abro su ficha, **entonces** veo 2 o 4 tiempos y refrigeración por aire o líquido en vez de tracción y carrocería; viene atmosférica por defecto y el combustible es gasolina o eléctrica.
- **Dado** un vehículo eléctrico, **cuando** lo guardo, **entonces** no se piden cilindraje ni aspiración.
- **Dado** un vehículo con la ficha incompleta (falta combustible o caja; en combustión, cilindraje o aspiración; en moto, los tiempos), **cuando** abro su ficha, **entonces** queda marcado "ficha pendiente" y no bloquea la orden hasta que intento pasarla a Recibido (ver HU-20).
- **Dado** un dato de la ficha ya guardado, **cuando** lo cambio, **entonces** queda en la bitácora quién, cuándo y el valor anterior.
- **Dado** un kilometraje menor al de la última orden, **cuando** lo escribo, **entonces** el sistema me advierte y me pide confirmar (puede ser cambio de tablero).

**Carro / moto:** La ficha la decidió Pablo el 3-nov-2026 (DE-17) y reemplaza "cilindraje y turbo" (1-nov). Tracción y carrocería solo aplican a carros; tiempos y refrigeración, solo a motos. Las listas de valores son propuesta. El VIN sigue fuera del MVP por decisión del equipo.

**Notas:** Cambio del 1-nov: antes decía "VIN opcional" y cilindraje solo en motos. Cambio del 3-nov: Q-23 queda resuelta con la ficha.

**Trazabilidad:** RF: RF-05 · Depende de: HU-10

### HU-12 · Un cliente con varios vehículos y vehículos que cambian de dueño

**Épica:** E02 · **Roles:** Dueño del taller, Recepción · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** que un cliente tenga varios vehículos y que un vehículo pase a otro cliente sin perder su historial **para** que el historial siga a la placa y no a la persona.

**Criterios de aceptación**

- **Dado** un cliente con un carro y una moto, **cuando** abro su ficha, **entonces** veo los dos vehículos con su tipo.
- **Dado** un vehículo vendido, **cuando** lo asigno a un nuevo cliente, **entonces** el historial anterior se conserva y queda la fecha del cambio; el dueño nuevo no ve las conversaciones del anterior.

**Notas:** Lo que ve el dueño nuevo es propuesta, para no exponer datos del anterior (Ley 1581). Fuentes: [7]

**Trazabilidad:** Depende de: HU-09, HU-10

### HU-13 · Ver todo lo que se le ha hecho a un vehículo

**Épica:** E02 · **Roles:** Dueño del taller, Mecánico · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño o mecánico, **quiero** abrir la ficha de un vehículo y ver sus órdenes, fotos, conversaciones y repuestos usados en orden de fecha **para** responder con respaldo cuando el cliente pregunta qué se le hizo.

**Criterios de aceptación**

- **Dado** el Kia Picanto FQD264 con 2 órdenes entregadas, **cuando** abro su ficha, **entonces** veo OS-0031 y OS-0038 con sus fotos, ítems y mensajes.
- **Dado** el rol mecánico, **cuando** abro la ficha, **entonces** veo trabajos, fotos y repuestos, pero no costos ni totales de dinero.
- **Dado** cualquier vehículo, **cuando** abro la ficha, **entonces** veo arriba marca, línea, año, cilindraje y si tiene turbo.

**Notas:** Es lo que el agente de Meta dentro de la app no hace: no se conecta a órdenes ni a historial. Fuentes: [6]

**Trazabilidad:** RF: RF-06 · Aceptación: CA-07

### HU-14 · Buscar desde cualquier pantalla

**Épica:** E02 · **Roles:** Dueño del taller, Mecánico, Recepción · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** cualquiera del taller, **quiero** buscar por placa, celular o nombre desde una barra siempre visible **para** encontrar el vehículo con el cliente al frente.

**Criterios de aceptación**

- **Dado** parte de una placa (KLM), **cuando** la escribo, **entonces** veo las coincidencias con tipo de vehículo, cliente y orden abierta.
- **Dado** un celular, **cuando** lo escribo, **entonces** veo el cliente y todos sus vehículos.

### HU-15 · Unir clientes o vehículos repetidos

**Épica:** E02 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** que el sistema me avise de posibles duplicados y me deje unirlos con confirmación **para** limpiar lo que entró por migración o a la carrera.

**Criterios de aceptación**

- **Dado** dos clientes con el mismo celular después de una importación, **cuando** abro el aviso, **entonces** veo los dos lado a lado y al unir se conservan todas las órdenes.
- **Dado** una unión hecha, **cuando** reviso la bitácora, **entonces** queda quién la hizo y qué registros se unieron.

**Trazabilidad:** Depende de: HU-88

## E03 · Recepción y orden de servicio

_Módulo del MVP: M3. El respaldo: checklist, fotos, estados y constancia de recibo._

### HU-16 · Abrir una orden desde la placa

**Épica:** E03 · **Roles:** Mecánico, Recepción, Dueño del taller · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** abrir una orden escribiendo la placa y lo que dice el cliente que tiene **para** empezar el respaldo desde el primer minuto.

**Criterios de aceptación**

- **Dado** una placa, **cuando** abro una orden, **entonces** se crea con número consecutivo del taller (OS-0045), estado Recibido o Por recibir, el síntoma en palabras del cliente y la hora.
- **Dado** un vehículo que ya tiene una orden abierta, **cuando** intento abrir otra, **entonces** el sistema me avisa y me deja abrirla solo si confirmo.

**Trazabilidad:** RF: RF-07 · Escena: 5 · Depende de: HU-10

### HU-17 · Hacer el checklist de recepción en el celular (carro)

**Épica:** E03 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** mecánico que recibe un carro, **quiero** marcar kilometraje y combustible, tomar al menos 4 fotos (los 4 lados), anotar golpes y objetos dentro **para** que quede constancia del estado en que llegó.

**Criterios de aceptación**

- **Dado** una orden de carro en Por recibir, **cuando** abro el checklist en un celular de 400 px, **entonces** puedo hacerlo con una mano y la cámara abre directo.
- **Dado** menos de 4 fotos, **cuando** intento pasar a Recibido, **entonces** el sistema me pide las que faltan.
- **Dado** un rayón en la puerta trasera, **cuando** lo anoto, **entonces** queda como salvedad con foto y hora.
- **Dado** el checklist completo, **cuando** paso a Recibido, **entonces** se guarda quién lo hizo y cuándo y sale el aviso AU-01 si está encendido.

**Carro / moto:** Checklist de carro: los 4 lados, tablero, combustible, objetos dentro.

**Notas:** La Ley 1480 art. 18 presume que el bien se entregó en buen estado si no se hizo salvedad: las fotos y las salvedades protegen al taller. Fuentes: [10], [11]

**Trazabilidad:** RF: RF-08 · Automatización: AU-01 · Escena: 5 · Aceptación: CA-11 · Depende de: HU-16

### HU-18 · Hacer el checklist de recepción de una moto

**Épica:** E03 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** mecánico que recibe una moto, **quiero** un checklist pensado para moto **para** no llenar campos de carro que no aplican y registrar lo que sí importa en una moto.

**Criterios de aceptación**

- **Dado** una orden de moto, **cuando** abro el checklist, **entonces** veo fotos de lado izquierdo, lado derecho, frente, trasera y tablero (mínimo 4), kilometraje, nivel de combustible si la moto tiene indicador, y accesorios entregados (casco, baúl, maletas, documentos).
- **Dado** una moto sin indicador de combustible, **cuando** llego a ese campo, **entonces** puedo marcar "no aplica".
- **Dado** un casco dejado en el taller, **cuando** lo marco, **entonces** aparece en la constancia de recibo y en el aviso de entrega para devolverlo.

**Carro / moto:** Todo el contenido del checklist de moto es propuesta: no se ha visitado ningún taller de motos (N-09). Validarlo en la encuesta y con 2 visitas antes de construir (Q-02).

**Trazabilidad:** RF: RF-08 · Automatización: AU-01 · Escena: 5-M · Aceptación: CA-11 · Depende de: HU-16, HU-04

### HU-19 · Recibir la constancia con fotos y confirmarla

**Épica:** E03 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** cliente que dejó su carro o su moto, **quiero** recibir por WhatsApp el resumen de cómo lo entregué, con las fotos, y poder decir si estoy de acuerdo **para** quedarme tranquilo y tener prueba de cómo lo dejé.

**Criterios de aceptación**

- **Dado** que el mecánico pasa la orden a Recibido y AU-01 está encendida, **cuando** pasa menos de 1 minuto, **entonces** recibo el mensaje con placa, kilometraje, salvedades, fotos y dos botones: "Está bien" y "Algo no cuadra".
- **Dado** que pulso "Está bien", **cuando** llega la respuesta, **entonces** la orden guarda mi aceptación con fecha y hora.
- **Dado** que pulso "Algo no cuadra", **cuando** llega la respuesta, **entonces** el dueño recibe una alerta en la bandeja y la orden queda marcada.

**Carro / moto:** El texto dice "tu carro" o "tu moto" según el tipo del vehículo.

**Notas:** Los botones de respuesta de WhatsApp admiten hasta 3 y 20 caracteres por título (oficial). Fuera de la ventana de 24 h el aviso necesita plantilla aprobada (ver HU-49). La Ley 1480 art. 18 pide además dirección del cliente, término de garantía y abonos en el recibo: ver Q-06. Fuentes: [3], [10], [2]

**Trazabilidad:** RF: RF-14 · Automatización: AU-01 · Escena: 5 · Aceptación: CA-05 · Depende de: HU-17, HU-18, HU-48, HU-85

### HU-20 · Mover la orden por sus estados

**Épica:** E03 · **Roles:** Mecánico, Dueño del taller · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** cambiar el estado de la orden con un toque **para** que el cliente y el dueño sepan en qué va sin preguntarme.

**Criterios de aceptación**

- **Dado** una orden, **cuando** cambio de estado, **entonces** solo puedo ir por el orden Por recibir → Recibido → En diagnóstico → Esperando aprobación → En reparación → Listo → Entregado, o devolverme un paso.
- **Dado** una orden en Esperando aprobación, **cuando** intento pasar a En reparación sin aprobación del cliente, **entonces** el sistema no me deja; el dueño sí puede, escribiendo el motivo (por ejemplo "aprobó por teléfono").
- **Dado** cualquier cambio, **cuando** se guarda, **entonces** queda el estado anterior, el nuevo, el usuario y la hora (EventoOrden).
- **Dado** una orden cuyo vehículo tiene "motor pendiente", **cuando** intento pasarla a Recibido, **entonces** el sistema me pide cilindraje y turbo en la misma pantalla (2 toques) antes de dejarme avanzar.

**Notas:** El documento del MVP dice "6 estados" en la precarga y lista 7. Aquí se toman los 7 (Q-04).

**Trazabilidad:** RF: RF-07 · Escena: 5, 6, 8 · Depende de: HU-16

### HU-21 · Agregar fotos y notas en cualquier momento

**Épica:** E03 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** tomar fotos y escribir notas durante el diagnóstico y la reparación **para** mostrar al cliente lo que encontré y lo que cambié.

**Criterios de aceptación**

- **Dado** una orden abierta, **cuando** tomo una foto, **entonces** queda con hora, autor y etapa en que se tomó.
- **Dado** una foto marcada "mostrar al cliente", **cuando** el cliente pregunta por su vehículo, **entonces** AU-05 usa la última foto marcada.

**Notas:** Estudio J.D. Power 2024 en EE. UU.: con fotos o video el 51 % aprueba la reparación frente al 24 % sin ellos. Fuentes: [16]

**Trazabilidad:** RF: RF-08 · Automatización: AU-05 · Escena: 7

### HU-22 · Asignar mecánico y fecha prometida

**Épica:** E03 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** poner a quién le toca la orden y para cuándo la prometí **para** que "¿ya está mi carro?" tenga una fecha real y que cada mecánico vea lo suyo.

**Criterios de aceptación**

- **Dado** una orden con fecha prometida, **cuando** el cliente pregunta por su vehículo, **entonces** la respuesta incluye la fecha y hora prometida.
- **Dado** una orden sin fecha prometida, **cuando** el cliente pregunta, **entonces** la respuesta da el estado y dice "te avisamos apenas esté listo", sin inventar fecha.

**Notas:** Está como Should en el MVP pero AU-05 (Must) la usa. Recomendación: subirla a Must o aceptar que AU-05 salga sin fecha.

**Trazabilidad:** RF: RF-16 · Automatización: AU-05 · Escena: 7

### HU-23 · Ver mis órdenes en el celular

**Épica:** E03 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** una lista de mis órdenes abiertas con placa, tipo de vehículo y estado **para** saber qué sigue sin preguntarle al dueño.

**Criterios de aceptación**

- **Dado** órdenes asignadas a mí, **cuando** abro la app, **entonces** veo primero las mías, ordenadas por fecha prometida.
- **Dado** un taller que no asigna mecánico, **cuando** abro la app, **entonces** veo todas las órdenes abiertas del taller.

**Notas:** Qué ve el mecánico sin asignación es propuesta (Q-14).

**Trazabilidad:** RF: RF-21 · Escena: 5 · Aceptación: CA-11 · Depende de: HU-81

### HU-24 · Ver la conversación de WhatsApp dentro de la orden

**Épica:** E03 · **Roles:** Dueño del taller, Mecánico · **Superficies:** Panel, Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ver en la orden todos los mensajes que entraron y salieron con ese cliente **para** tener respaldo de lo que se habló y se aprobó.

**Criterios de aceptación**

- **Dado** una orden creada por el agente, **cuando** la abro, **entonces** veo la conversación desde el primer mensaje, con hora y quién envió cada respuesta (IA aprobada, dueño o automático).
- **Dado** un mensaje nuevo del cliente con orden abierta, **cuando** llega, **entonces** se adjunta a esa orden.

**Notas:** Huecos H-03 y R-05 del análisis de competencia. Fuentes: [22]

**Trazabilidad:** RF: RF-11 · Escena: 4

### HU-25 · Entregar el vehículo

**Épica:** E03 · **Roles:** Dueño del taller, Recepción, Mecánico · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** quien entrega, **quiero** marcar Entregado registrando quién lo recogió y los accesorios devueltos **para** cerrar la orden con constancia.

**Criterios de aceptación**

- **Dado** una orden en Listo, **cuando** la marco Entregado, **entonces** pido nombre de quien recoge y confirmo accesorios del checklist (casco, documentos, objetos).
- **Dado** la orden entregada, **cuando** se cierra, **entonces** sale de órdenes abiertas y entra al cálculo de ticket y tiempo promedio.

**Carro / moto:** En motos es común dejar el casco; el checklist de entrega lo recuerda (propuesta).

**Trazabilidad:** RF: RF-07 · Escena: 8 · Depende de: HU-20

### HU-26 · Cancelar una orden que no siguió

**Épica:** E03 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** cancelar una orden con un motivo (no llegó, no aprobó, se lo llevó sin reparar) **para** que no se quede abierta para siempre ni dañe los KPIs.

**Criterios de aceptación**

- **Dado** una orden en Por recibir o Esperando aprobación, **cuando** la cancelo, **entonces** pido el motivo, la orden sale de las abiertas y los repuestos reservados vuelven al inventario.
- **Dado** una orden cancelada, **cuando** miro el historial del vehículo, **entonces** aparece tachada con el motivo.

**Notas:** Nueva: el flujo de 7 estados no tiene salida para órdenes que no siguen (Q-05).

**Trazabilidad:** Depende de: HU-38

## E04 · Cotización y aprobación

_Módulo del MVP: M3. Que el cliente apruebe por WhatsApp y quede registrado._

### HU-27 · Armar la cotización con servicios y repuestos

**Épica:** E04 · **Roles:** Mecánico, Dueño del taller · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** agregar servicios de la lista y repuestos del inventario y ver el total **para** enviar la cotización en un minuto sin calculadora.

**Criterios de aceptación**

- **Dado** una orden en diagnóstico, **cuando** agrego "cambio de pastillas" y PF-SPK, **entonces** veo cada ítem con cantidad, precio de venta y el total en pesos sin decimales.
- **Dado** el rol mecánico, **cuando** armo la cotización, **entonces** veo precios de venta pero nunca costos.
- **Dado** un repuesto sin existencias, **cuando** lo agrego, **entonces** el sistema me avisa "sin stock" pero me deja cotizarlo (Q-11).
- **Dado** la cotización lista, **cuando** paso a Esperando aprobación, **entonces** sale AU-02 si está encendida.

**Notas:** Quién da el último visto bueno a la cotización (mecánico o dueño) está abierto: Q-10.

**Trazabilidad:** RF: RF-09 · Automatización: AU-02 · Escena: 6 · Depende de: HU-07, HU-34

### HU-28 · Aprobar la cotización desde WhatsApp

**Épica:** E04 · **Roles:** Cliente final · **Superficies:** WhatsApp · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** cliente, **quiero** recibir la cotización con el detalle y aprobarla o decir que tengo dudas con un botón **para** no tener que ir al taller ni llamar.

**Criterios de aceptación**

- **Dado** una orden en Esperando aprobación y AU-02 encendida, **cuando** pasa menos de 1 minuto, **entonces** recibo ítems, total y los botones "Aprobar" y "Tengo dudas".
- **Dado** que pulso Aprobar, **cuando** llega la respuesta, **entonces** recibo "Listo, empezamos" y la orden pasa a En reparación.
- **Dado** que respondo con texto en vez de botón ("sí, hágale"), **cuando** llega la respuesta, **entonces** el sistema no lo toma como aprobación: la manda a la bandeja para que el dueño la confirme.

**Carro / moto:** Mismo flujo para carro y moto.

**Notas:** La Ley 1480 art. 18 exige que la aceptación del valor sea expresa y quede documentada: por eso un "sí" en texto pasa por humano (propuesta). Botones: máximo 3, 20 caracteres. Fuentes: [10], [3]

**Trazabilidad:** RF: RF-10, RF-14 · Automatización: AU-02 · Escena: 6 · Aceptación: CA-05 · Depende de: HU-27, HU-48, HU-85

### HU-29 · Registrar la respuesta del cliente y mover la orden

**Épica:** E04 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** automatización de cotización, **quiero** guardar la respuesta del cliente con el id del mensaje y cambiar el estado al aprobar **para** que la aprobación quede como prueba y el trabajo arranque sin que nadie toque nada.

**Criterios de aceptación**

- **Dado** un toque en Aprobar, **cuando** llega por webhook, **entonces** se guarda texto del botón, id del mensaje de WhatsApp, hora, y la orden pasa a En reparación registrando "cliente por WhatsApp" como autor.
- **Dado** un toque en Tengo dudas, **cuando** llega, **entonces** la orden sigue en Esperando aprobación y el dueño recibe una tarjeta en la bandeja.
- **Dado** una aprobación que llega dos veces (reintento de Meta), **cuando** se procesa, **entonces** no se duplica ni se descuenta dos veces el inventario.

**Trazabilidad:** RF: RF-10, RF-12 · Escena: 6 · Aceptación: CA-06 · Depende de: HU-28

### HU-30 · Aprobar ítem por ítem

**Épica:** E04 · **Roles:** Cliente final · **Superficies:** WhatsApp · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** cliente, **quiero** aprobar algunos ítems de la cotización y otros no **para** hacer solo lo urgente si no me alcanza.

**Criterios de aceptación**

- **Dado** una cotización de 3 ítems, **cuando** apruebo 2, **entonces** la orden sigue solo con esos 2 y el total se recalcula.

**Notas:** D-11 recomienda cotización completa en el MVP e ítem por ítem después. Con 3 botones como máximo por mensaje, esta historia necesita otro diseño (lista o enlace). Fuentes: [3]

**Trazabilidad:** Depende de: HU-28

### HU-31 · Cotización adicional durante la reparación

**Épica:** E04 · **Roles:** Mecánico, Dueño del taller, Cliente final · **Superficies:** Piso, WhatsApp · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** mecánico que encuentra otro daño, **quiero** agregar una cotización adicional y pedir aprobación de nuevo **para** no hacer trabajos que el cliente no aprobó.

**Criterios de aceptación**

- **Dado** una orden En reparación, **cuando** agrego un ítem nuevo, **entonces** la orden vuelve a Esperando aprobación solo por el adicional y el cliente recibe el detalle de lo nuevo.
- **Dado** el adicional aprobado, **cuando** vuelve a En reparación, **entonces** el total de la orden suma las dos aprobaciones.

**Notas:** Nueva, no estaba en el MVP. Responde a la Ley 1480 art. 18 (aceptación expresa del valor). Fuentes: [10]

**Trazabilidad:** Depende de: HU-28

## E05 · Inventario básico

_Módulo del MVP: M4. Repuestos que se descuentan solos al aprobarse la orden._

### HU-32 · Crear, editar y desactivar repuestos

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Equipo · **Aplica a motos:** sí

**Como** dueño, **quiero** manejar mis repuestos con nombre, referencia, costo, precio de venta, cantidad y mínimo **para** saber qué tengo y a cuánto lo vendo.

**Criterios de aceptación**

- **Dado** un repuesto nuevo, **cuando** lo guardo, **entonces** la referencia no se repite en mi taller.
- **Dado** un repuesto usado en órdenes viejas, **cuando** lo desactivo, **entonces** deja de salir al cotizar pero el historial lo conserva.
- **Dado** un taller de Ambos, **cuando** creo un repuesto, **entonces** puedo marcar si es de carro, de moto o de ambos para filtrarlo al cotizar (Should).

**Notas:** Gestión básica de inventario era parte del MVP original del equipo.

**Trazabilidad:** RF: RF-12

### HU-33 · Descontar el repuesto al aprobarse la orden

**Épica:** E05 · **Roles:** Agente y automatizaciones, Dueño del taller · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** que los repuestos se descuenten solos cuando el cliente aprueba **para** no llevar el inventario a mano.

**Criterios de aceptación**

- **Dado** PF-SPK con 4 unidades en la cotización aprobada de OS-0045, **cuando** el cliente aprueba, **entonces** queda en 3 y hay un movimiento que apunta a OS-0045.
- **Dado** una cotización no aprobada, **cuando** pasa el tiempo, **entonces** no se descuenta nada.

**Trazabilidad:** RF: RF-12 · Escena: 6 · Aceptación: CA-06 · Depende de: HU-29, HU-32

### HU-34 · Buscar repuestos al cotizar

**Épica:** E05 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** buscar repuestos por nombre o referencia y ver si hay **para** no cotizar lo que no tenemos sin saberlo.

**Criterios de aceptación**

- **Dado** "pastillas", **cuando** busco, **entonces** veo las coincidencias con existencia y precio de venta, nunca el costo.

**Trazabilidad:** RF: RF-09 · Escena: 6 · Depende de: HU-32

### HU-35 · Registrar entrada de mercancía

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** sumar unidades cuando compro **para** que la existencia sea real.

**Criterios de aceptación**

- **Dado** 10 cuartos de aceite comprados, **cuando** registro la entrada, **entonces** la cantidad sube y queda un movimiento de entrada con fecha y usuario.

### HU-36 · Recibir alerta de repuesto bajo el mínimo

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** ver dentro de la app cuándo un repuesto baja del mínimo **para** comprar antes de quedarme sin él.

**Criterios de aceptación**

- **Dado** FA-SPK con 3 unidades y mínimo 3, y BH-H4 agotado, **cuando** abro el tablero, **entonces** aparecen en "Hoy tienes pendiente".
- **Dado** la alerta, **cuando** se genera, **entonces** no cuesta mensajes de WhatsApp.

**Trazabilidad:** Automatización: AU-08 · Depende de: HU-76

### HU-37 · Importar inventario desde Excel o CSV

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño con una hoja de repuestos, **quiero** subirla con una plantilla **para** no digitar todo.

**Criterios de aceptación**

- **Dado** la plantilla llena, **cuando** la subo, **entonces** veo cada fila validada; las que tienen error se marcan y no se importan hasta corregirlas.

**Trazabilidad:** Depende de: HU-92

### HU-38 · Devolver al inventario lo que no se usó

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Motor, Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** que si quito un repuesto de una orden aprobada o la cancelo, la cantidad vuelva **para** que el inventario no quede descuadrado.

**Criterios de aceptación**

- **Dado** un repuesto descontado, **cuando** lo quito de la orden, **entonces** vuelve la unidad y queda un movimiento de devolución que apunta a la orden.

**Notas:** Derivada: el MVP solo describe el descuento.

**Trazabilidad:** Depende de: HU-33

### HU-39 · Proveedores, órdenes de compra y bodegas

**Épica:** E05 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** dueño, **quiero** guardar mis proveedores, pedirles repuestos con una orden de compra y, si tengo más de un lugar de almacenamiento, mover repuestos entre ellos **para** comprar a tiempo y saber dónde está cada repuesto.

**Criterios de aceptación**

- **Dado** un proveedor nuevo, **cuando** lo creo, **entonces** guardo nombre, celular y qué repuestos me vende.
- **Dado** un repuesto bajo el mínimo, **cuando** pido una orden de compra, **entonces** la orden queda con proveedor, ítems y cantidades, en estado Pedida.
- **Dado** una orden de compra Pedida, **cuando** llega la mercancía y la marco Recibida, **entonces** las cantidades suman al inventario y cada movimiento apunta a esa orden de compra.
- **Dado** un taller con dos bodegas, **cuando** muevo un repuesto de una a otra, **entonces** queda un traslado con origen, destino, cantidad y usuario, y la existencia total no cambia.

**Notas:** Estaba como Later y sin criterios. Los criterios son nuevos y propuestos; ningún taller visitado ha pedido compras ni bodegas (sin dato). Es casi un módulo entero (proveedores, compras, bodegas) y no aparece en ninguna escena de la demo: conviene dimensionarla antes de comprometerla.

**Trazabilidad:** Depende de: HU-32, HU-35

## E06 · Catálogo de automatizaciones y avisos

_Módulo del MVP: M5. Cada automatización se enciende con un interruptor; los avisos salen solos._

### HU-40 · Ver el catálogo de automatizaciones

**Épica:** E06 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ver tarjetas que digan qué hace cada automatización, cuándo se dispara, qué mensaje envía y cuánto cuesta en mensajes **para** entender qué me ahorra sin saber de tecnología.

**Criterios de aceptación**

- **Dado** el catálogo, **cuando** lo abro, **entonces** veo 9 tarjetas: 5 encendidas, 3 apagadas y 1 "próximamente" (AU-09) sin fecha prometida.
- **Dado** una tarjeta, **cuando** la leo, **entonces** dice el disparador en palabras del taller ("cuando la orden pasa a Listo").

**Notas:** Sin constructor visual (H-02, R-04). Fuentes: [22]

**Trazabilidad:** RF: RF-13 · Escena: 10

### HU-41 · Encender y apagar una automatización

**Épica:** E06 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** un interruptor por automatización **para** decidir qué sale solo.

**Criterios de aceptación**

- **Dado** AU-03 apagada, **cuando** paso una orden a Listo, **entonces** no sale ningún mensaje.
- **Dado** AU-03 encendida, **cuando** paso una orden a Listo, **entonces** el mensaje sale en menos de 1 minuto.
- **Dado** un cambio de interruptor, **cuando** se guarda, **entonces** queda en la bitácora quién y cuándo.

**Trazabilidad:** RF: RF-13, RF-14 · Escena: 10 · Aceptación: CA-05

### HU-42 · Editar el texto de un aviso

**Épica:** E06 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** cambiar el texto con variables como {nombre}, {placa}, {vehiculo}, {total} **para** que suene como mi taller.

**Criterios de aceptación**

- **Dado** un texto con {vehiculo}, **cuando** se envía para una moto, **entonces** dice "tu moto"; para un carro dice "tu carro".
- **Dado** una variable mal escrita, **cuando** guardo, **entonces** el sistema me la marca antes de guardar.
- **Dado** un aviso que sale fuera de la ventana de 24 h, **cuando** edito el texto, **entonces** el sistema me avisa que el texto nuevo requiere aprobación de Meta antes de usarse (en producción).

**Notas:** En producción, los mensajes que salen fuera de la ventana usan plantillas aprobadas por Meta: cambiar el texto implica una plantilla nueva (Q-17). Fuentes: [2]

**Trazabilidad:** RF: RF-13

### HU-43 · Saber que mi vehículo está listo

**Épica:** E06 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** cliente, **quiero** un mensaje cuando mi carro o mi moto esté listo, con el total y el horario para recogerlo **para** no tener que preguntar.

**Criterios de aceptación**

- **Dado** AU-03 encendida, **cuando** la orden pasa a Listo, **entonces** recibo total, horario del día y, si dejé accesorios, el recordatorio de recogerlos.
- **Dado** el horario de hoy ya pasó, **cuando** llega el aviso, **entonces** dice el horario del siguiente día hábil.

**Notas:** Hoy en Metro Frenos este aviso se hace a mano (visita del equipo).

**Trazabilidad:** RF: RF-14 · Automatización: AU-03 · Escena: 8 · Aceptación: CA-05 · Depende de: HU-08, HU-48

### HU-44 · Recordatorio de la cita

**Épica:** E06 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** cliente con cita, **quiero** un recordatorio la mañana del día **para** no olvidar llevar el vehículo.

**Criterios de aceptación**

- **Dado** una orden Por recibir con fecha de hoy y AU-06 encendida, **cuando** son las 7:00 a. m., **entonces** recibo el recordatorio con la hora y la dirección.

**Notas:** Probablemente fuera de la ventana de 24 h: necesita plantilla de utilidad (cobrable). Fuentes: [2]

**Trazabilidad:** Automatización: AU-06

### HU-45 · Recordatorio de vehículo sin recoger

**Épica:** E06 · **Roles:** Cliente final, Dueño del taller · **Superficies:** WhatsApp, Motor, Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** que al cliente le llegue un recordatorio si su vehículo lleva 3 días listo (configurable) **para** liberar espacio y cobrar.

**Criterios de aceptación**

- **Dado** la orden de Marta en Listo hace 4 días, **cuando** corre la revisión diaria, **entonces** Marta recibe el recordatorio una sola vez y el panel marca "4 días sin recoger".

**Notas:** El Decreto 1413 de 2018 sobre bienes abandonados es dudoso para vehículos (memoria del pitch).

**Trazabilidad:** Automatización: AU-07 · Escena: 8

### HU-46 · Registrar cada ejecución de una automatización

**Épica:** E06 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** automatización, **quiero** guardar cada disparo con resultado, mensaje enviado, categoría y costo **para** que el dueño y el equipo vean qué salió y cuánto costó.

**Criterios de aceptación**

- **Dado** un aviso enviado, **cuando** Meta confirma la entrega, **entonces** la ejecución queda como entregada con el id de WhatsApp.
- **Dado** un envío que falla, **cuando** Meta responde error, **entonces** la ejecución queda fallida, el dueño ve una alerta y la orden sigue funcionando.

**Notas:** RNF-13: si WhatsApp falla, la orden y el historial siguen funcionando.

**Trazabilidad:** RF: RF-14, RF-20 · Aceptación: CA-05, CA-08

### HU-47 · Respetar la ventana de 24 horas

**Épica:** E06 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** automatización, **quiero** saber si el cliente escribió en las últimas 24 horas antes de enviar **para** no mandar texto libre donde Meta exige plantilla.

**Criterios de aceptación**

- **Dado** un cliente que escribió hace 3 horas, **cuando** sale un aviso, **entonces** se envía como mensaje de servicio.
- **Dado** un cliente que no ha escrito en 24 h y no hay plantilla aprobada, **cuando** toca enviar, **entonces** no se envía; el dueño ve "este cliente tiene que escribir primero" y la opción de avisarle por otro medio.

**Notas:** En la demo todos los clientes escriben primero para quedar dentro de la ventana. Desde el 1-oct-2026 los primeros 1.000 mensajes de servicio al mes por número son gratis. Fuentes: [1], [2]

**Trazabilidad:** RF: RF-14

### HU-48 · Enviar el aviso correcto al cambiar el estado

**Épica:** E06 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** automatización, **quiero** escuchar los cambios de estado y disparar AU-01, AU-02 o AU-03 si están encendidas **para** que los avisos salgan sin que nadie los escriba.

**Criterios de aceptación**

- **Dado** un cambio a Recibido, Esperando aprobación o Listo, **cuando** la automatización está encendida, **entonces** el mensaje sale en menos de 1 minuto.
- **Dado** un estado que se devuelve y vuelve a avanzar, **cuando** pasa dos veces por Listo, **entonces** el aviso no se repite salvo que el dueño lo reenvíe.

**Trazabilidad:** RF: RF-14 · Automatización: AU-01, AU-02, AU-03 · Escena: 5, 6, 8 · Aceptación: CA-05 · Depende de: HU-20, HU-41

### HU-49 · Seguimiento de garantía y garantía por orden

**Épica:** E06 · **Roles:** Cliente final, Dueño del taller · **Superficies:** WhatsApp, Motor, Panel · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** cliente, **quiero** que unos días después de recoger mi vehículo el taller me pregunte cómo sigue y poder avisar si tuve un problema **para** sentir respaldo.

**Criterios de aceptación**

- **Dado** una orden que se entrega, **cuando** la marco Entregado, **entonces** queda guardada la garantía de esa orden en días, con el valor por defecto del taller, y aparece en el recibo.
- **Dado** AU-09 encendida y una orden entregada hace N días (N lo configura el dueño), **cuando** corre la revisión diaria, **entonces** el cliente recibe una pregunta con botones "Todo bien" y "Tuve un problema".
- **Dado** que el cliente pulsa "Tuve un problema", **cuando** llega la respuesta, **entonces** el dueño recibe una tarjeta en la bandeja con la orden y la garantía vigente, y el cliente recibe "ya te contactamos".
- **Dado** una orden con la garantía vencida, **cuando** corre la revisión, **entonces** no se envía el seguimiento.

**Notas:** Estaba como Later y sin criterios; los criterios son nuevos. Valor de N y garantía por defecto: sin dato (la Ley 1480 art. 8 habla de garantía de 3 meses en bienes reparados, memoria del pitch). Falta resolver si este mensaje posventa cuenta como servicio para Meta y para la Ley 1581 (H7). Q-25: el equipo canceló HU-50; falta confirmar si esta sigue. Fuentes: [2]

**Trazabilidad:** Automatización: AU-09 · Depende de: HU-25, HU-41, HU-48

### HU-50 · Recordatorio de técnico-mecánica por tipo de vehículo

**Épica:** E06 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Cancelada · **Origen:** Propuesta

> **CANCELADA.** Cancelada por el equipo el 1-nov-2026: el ERP no tiene por qué recordar este tipo de servicios (técnico-mecánica ni mantenimiento por fecha o kilometraje). Sale del catálogo: AU-10 ya no existe y no se muestra como "próximamente".

**Como** cliente, **quiero** que me recuerden la revisión técnico-mecánica **para** no quedar multado.

**Criterios de aceptación**

- Cancelada. No se construye. Se conserva el texto solo para trazabilidad.

**Carro / moto:** Lo que se había propuesto: motos nuevas la primera revisión a los 2 años de matriculadas y carros particulares a los 5; después, cada año (RUNT, oficial).

**Notas:** Efecto: el hueco N-04 del análisis de competencia (recordatorio de técnico-mecánica, abierto en Colombia) queda sin cubrir por decisión del equipo, y la automatización AU-10 sale del catálogo, que pasa de 10 a 9 tarjetas. Ninguna historia dependía de esta. Fuentes: [14]

## E07 · Agente de WhatsApp

_Módulo del MVP: M5. El demo: atiende al cliente nuevo y contesta "¿ya está mi carro / mi moto?" con datos reales._

### HU-51 · Escribirle al taller en mis palabras

**Épica:** E07 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** cliente nuevo, **quiero** escribir por WhatsApp como hablo ("mi Spark está chillando cuando freno") y recibir respuesta rápida con precios reales **para** no esperar a que alguien deje la llave para contestar.

**Criterios de aceptación**

- **Dado** que escribo por primera vez, **cuando** mando el mensaje, **entonces** primero recibo la solicitud de autorización (HU-85).
- **Dado** que acepté, **cuando** pasa menos de 30 segundos, **entonces** el dueño tiene un borrador listo; cuando lo envía recibo precios de la lista del taller y una hora de cita dentro del horario.
- **Dado** que escribo de una moto ("mi NMAX hace ruido en la cadena"), **cuando** llega el borrador, **entonces** usa servicios y precios de moto.

**Trazabilidad:** RF: RF-15 · Automatización: AU-04 · Escena: 4 · Aceptación: CA-03 · Depende de: HU-85, HU-52, HU-53, HU-62

### HU-52 · Entender qué pide el cliente

**Épica:** E07 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** agente de WhatsApp, **quiero** clasificar la intención (cotizar, agendar, estado, otra) y sacar tipo de vehículo, marca, línea, año, placa, síntoma, servicios y fecha **para** que el dueño no tenga que leer y digitar.

**Criterios de aceptación**

- **Dado** el mensaje de Laura, **cuando** lo proceso, **entonces** saco Chevrolet Spark GT 2015, carro, "chilla al frenar", revisión de frenos y cambio de pastillas, mañana, y marco que falta la placa.
- **Dado** un mensaje con "moto", "la nave" o un modelo de moto, **cuando** lo proceso, **entonces** marco tipo moto.
- **Dado** una moto que escribe a un taller que solo atiende carros, **cuando** lo proceso, **entonces** el borrador dice amablemente que el taller no atiende motos; el dueño decide si enviarlo.
- **Dado** un servicio que no está en la lista del taller, **cuando** lo proceso, **entonces** no lo invento: lo dejo como "otro" para el dueño.

**Notas:** Una sola llamada al modelo por mensaje y salida JSON validada (RNF-05). El juego de 30 mensajes de prueba debe incluir mensajes de moto (propuesta: 10 de 30).

**Trazabilidad:** RF: RF-15, RF-17 · Automatización: AU-04 · Escena: 4 · Aceptación: CA-02

### HU-53 · Redactar el borrador solo con datos del taller

**Épica:** E07 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** agente, **quiero** redactar el borrador usando solo precios, servicios y horario del taller **para** no inventar nunca un precio.

**Criterios de aceptación**

- **Dado** un servicio con precio, **cuando** redacto, **entonces** uso exactamente ese precio.
- **Dado** un servicio sin precio (cambio de amortiguadores), **cuando** redacto, **entonces** escribo "el taller te confirma el valor".
- **Dado** un borrador con un número que no está en la lista, **cuando** pasa la validación, **entonces** se descarta y la tarjeta queda en "Lo contesto yo".

**Trazabilidad:** RF: RF-15, RF-17 · Aceptación: CA-02 · Depende de: HU-07, HU-08

### HU-54 · Validar todo lo que sale de la IA

**Épica:** E07 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** sistema, **quiero** validar el JSON contra el esquema y los precios contra la lista antes de mostrar el borrador **para** que una respuesta mala nunca llegue al cliente.

**Criterios de aceptación**

- **Dado** una salida que no cumple el esquema, **cuando** llega, **entonces** se descarta, se registra el error y la tarjeta queda como "Lo contesto yo".
- **Dado** que la IA no responde en 30 segundos, **cuando** vence el tiempo, **entonces** pasa lo mismo y la orden y el historial siguen funcionando.

**Notas:** RNF-05 y RNF-13.

**Trazabilidad:** RF: RF-17 · Aceptación: CA-02, CA-03

### HU-55 · Crear la orden cuando llega la placa

**Épica:** E07 · **Roles:** Agente y automatizaciones, Dueño del taller · **Superficies:** Motor, Panel · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño, **quiero** que al enviar el borrador y llegar la placa se cree sola la orden Por recibir con la conversación **para** no digitar nada de lo que el cliente ya escribió.

**Criterios de aceptación**

- **Dado** el borrador enviado y Laura responde KLM482, **cuando** llega la placa, **entonces** se crean cliente, vehículo (carro) y orden OS-0045 Por recibir con fecha deseada y conversación adjunta.
- **Dado** una placa de moto, **cuando** llega, **entonces** el vehículo se crea como moto.
- **Dado** la placa y nada más sobre el vehículo, **cuando** se crea el vehículo, **entonces** queda con "motor pendiente" para completarlo en la recepción.

**Trazabilidad:** RF: RF-15, RF-11 · Automatización: AU-04 · Escena: 4 · Depende de: HU-10, HU-24, HU-62

### HU-56 · Preguntar "¿ya está mi carro?" o "¿ya está mi moto?"

**Épica:** E07 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** cliente con vehículo en el taller, **quiero** preguntar como sea y recibir el estado real al instante **para** no tener que llamar ni esperar.

**Criterios de aceptación**

- **Dado** Andrés con OS-0043 En reparación prometida a las 5:00 p. m., **cuando** escribe "ya está mi carro?", **entonces** recibe en menos de 1 minuto el estado, la hora prometida y la última foto marcada para el cliente.
- **Dado** 10 formas distintas de preguntar, incluidas de moto ("cómo va la moto", "ya puedo ir por la nave"), **cuando** se prueban, **entonces** las 10 reciben el estado correcto.
- **Dado** la respuesta, **cuando** se arma, **entonces** es una plantilla llenada con datos de la base; la IA solo clasificó la intención.

**Notas:** Escena clave frente al agente de Meta: según su documentación no se conecta a órdenes ni a historial. Fuentes: [6]

**Trazabilidad:** RF: RF-16 · Automatización: AU-05 · Escena: 7 · Aceptación: CA-04 · Depende de: HU-21, HU-22

### HU-57 · Preguntar cuál vehículo si hay más de uno

**Épica:** E07 · **Roles:** Agente y automatizaciones, Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** agente, **quiero** preguntar con botones de placa cuando el cliente tiene dos o más órdenes abiertas **para** no responder sobre el vehículo equivocado.

**Criterios de aceptación**

- **Dado** un cliente con su carro y su moto en el taller, **cuando** pregunta "¿ya está?", **entonces** recibe botones con las dos placas y después el estado de la que escoja.
- **Dado** más de 3 vehículos abiertos, **cuando** pregunta, **entonces** se usa una lista en vez de botones.

**Notas:** Derivada: caso real en talleres de Ambos. Botones máximo 3. Fuentes: [3]

**Trazabilidad:** RF: RF-16 · Automatización: AU-05 · Depende de: HU-56

### HU-58 · Hablar solo del taller

**Épica:** E07 · **Roles:** Agente y automatizaciones · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** agente, **quiero** responder solo sobre el taller y redirigir lo demás **para** cumplir la política de Meta y no parecer un asistente general.

**Criterios de aceptación**

- **Dado** una pregunta ajena ("escríbeme un poema"), **cuando** llega, **entonces** el borrador dice que solo puede ayudar con servicios del taller.
- **Dado** una pregunta de diagnóstico ("¿qué tiene mi carro?"), **cuando** llega, **entonces** el agente no diagnostica: ofrece revisión con precio de lista.

**Notas:** Desde el 15-ene-2026 Meta prohíbe chatbots de propósito general en la API (RNF-08). Fuentes: [5]

**Trazabilidad:** RF: RF-17

### HU-59 · Pedir hablar con una persona

**Épica:** E07 · **Roles:** Cliente final, Dueño del taller · **Superficies:** WhatsApp, Panel, Motor · **Prioridad:** Must · **Origen:** Equipo

**Como** cliente, **quiero** escribir "quiero hablar con alguien" y que me atienda una persona **para** no quedar atrapado con un robot.

**Criterios de aceptación**

- **Dado** el pedido de humano, **cuando** llega, **entonces** el agente deja de proponer borradores para esa conversación, el dueño recibe una alerta y el cliente un mensaje de "ya te atiende alguien".
- **Dado** el dueño, **cuando** cierra la atención, **entonces** el agente vuelve a quedar activo.

**Notas:** Salida a humano es decisión del equipo.

**Trazabilidad:** RF: RF-18

### HU-60 · Mandar notas de voz

**Épica:** E07 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** cliente, **quiero** mandar un audio en vez de escribir **para** explicar el ruido sin teclear.

**Criterios de aceptación**

- **Dado** un audio, **cuando** llega, **entonces** se transcribe, la transcripción se muestra en la tarjeta junto al audio y sigue el mismo flujo de texto.

**Notas:** El MVP menciona audios transcritos en el juego de prueba pero no los define como función. Sin dato de cuántos clientes de talleres usan audio (Q-12).

### HU-61 · Respuesta fuera de horario

**Épica:** E07 · **Roles:** Cliente final · **Superficies:** WhatsApp, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** cliente que escribe de noche, **quiero** saber que me leyeron y cuándo me responden **para** no pensar que me ignoraron.

**Criterios de aceptación**

- **Dado** un mensaje fuera del horario cargado, **cuando** llega, **entonces** sale un acuse fijo (sin IA) con el horario; el borrador queda en la bandeja para la mañana.

**Notas:** Nueva; no estaba en el MVP. Es plantilla con datos, no texto de IA, así que no choca con RF-18.

**Trazabilidad:** Depende de: HU-08

## E08 · Bandeja "Por revisar"

_Módulo del MVP: M6. La IA propone, el dueño decide, y todo queda medido._

### HU-62 · Revisar y enviar el borrador

**Épica:** E08 · **Roles:** Dueño del taller · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Equipo

**Como** dueño, **quiero** ver cada propuesta de la IA con el mensaje original, los datos extraídos y el borrador, y decidir con un botón **para** que nada salga sin mí.

**Criterios de aceptación**

- **Dado** un mensaje entrante, **cuando** pasan menos de 30 s, **entonces** aparece la tarjeta en Por revisar (9 de 10 pruebas).
- **Dado** la tarjeta, **cuando** pulso Enviar, **entonces** sale el mensaje y queda registrado que lo aprobé yo.
- **Dado** la tarjeta, **cuando** pulso Lo contesto yo, **entonces** se cierra la tarjeta sin enviar y escribo yo desde la conversación.
- **Dado** cualquier texto redactado por la IA, **cuando** intenta salir sin aprobación, **entonces** no sale nunca.

**Trazabilidad:** RF: RF-18 · Escena: 4 · Aceptación: CA-03, CA-09

### HU-63 · Editar el borrador antes de enviarlo

**Épica:** E08 · **Roles:** Dueño del taller · **Superficies:** Panel, Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** corregir el borrador y enviarlo **para** decirlo a mi manera.

**Criterios de aceptación**

- **Dado** un borrador, **cuando** lo edito y envío, **entonces** queda guardado el texto original, el editado y la diferencia.

**Trazabilidad:** RF: RF-19 · Aceptación: CA-09

### HU-64 · Saber de dónde salió cada dato

**Épica:** E08 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ver el origen de cada dato propuesto por la IA (mensaje del 30-oct 7:42, foto 2, fila 12) **para** confiar o corregir.

**Criterios de aceptación**

- **Dado** un dato extraído, **cuando** lo toco, **entonces** veo el mensaje, foto o fila de donde salió.

**Trazabilidad:** RF: RF-19

### HU-65 · Ver qué tan seguido edito los borradores

**Épica:** E08 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ver el porcentaje de borradores que envío sin editar **para** decidir algún día si el agente puede enviar solo.

**Criterios de aceptación**

- **Dado** 20 borradores enviados, 15 sin cambios, **cuando** abro la bandeja, **entonces** veo 75 % sin editar.

**Notas:** Métrica del flujo v0; base para D-07 (auto-envío). Fuentes: [21]

**Trazabilidad:** RF: RF-19

### HU-66 · Enterarme de que hay algo por revisar

**Épica:** E08 · **Roles:** Dueño del taller, Recepción · **Superficies:** Panel, Piso · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño ocupado en el piso, **quiero** un aviso cuando llega una tarjeta nueva **para** no dejar al cliente esperando.

**Criterios de aceptación**

- **Dado** la app abierta en el celular, **cuando** llega una tarjeta, **entonces** veo un contador y un sonido.
- **Dado** el rol recepción con permiso de bandeja, **cuando** llega una tarjeta, **entonces** también la ve y puede enviarla.

**Notas:** Avisar al dueño por WhatsApp gasta mensajes; notificación del navegador es lo más barato. Sin decidir (Q-18).

### HU-67 · Auto-envío para respuestas exactas

**Épica:** E08 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** dueño, **quiero** dejar que el sistema responda solo cuando el cliente pregunta únicamente el horario o el precio exacto de un servicio de mi lista **para** ahorrar más tiempo.

**Criterios de aceptación**

- **Dado** el auto-envío, **cuando** es la primera vez que entro a la configuración, **entonces** está apagado y todo pasa por "Por revisar".
- **Dado** el auto-envío encendido y un mensaje que pide solo el horario, **cuando** llega, **entonces** la respuesta sale sin tarjeta, con el horario cargado tal cual, y queda registrada como "automática".
- **Dado** un mensaje que mezcla horario con un síntoma o un diagnóstico, **cuando** llega, **entonces** no sale solo: va a "Por revisar".
- **Dado** el dueño, **cuando** apaga el auto-envío, **entonces** desde ese momento todo vuelve a pasar por revisión.

**Notas:** Estaba como Later; los criterios son nuevos. CHOQUE: contradice RF-18, CA-09 y D-07 ("nada redactado por la IA sale sin una aprobación registrada"). Solo es coherente si la respuesta automática no la redacta la IA sino que llena una plantilla con datos del taller, como AU-05. Q-27 pide decidirlo.

**Trazabilidad:** Depende de: HU-62, HU-65

## E09 · Tablero de KPIs y medidor

_Módulo del MVP: M7. Seis números para el dueño. El medidor de mensajes y costo de IA es del administrador del ERP, no del taller._

### HU-68 · Ver órdenes abiertas por estado

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño, **quiero** contar cuántas órdenes tengo en cada estado **para** saber cómo está el taller hoy.

**Criterios de aceptación**

- **Dado** los datos de ejemplo, **cuando** abro el tablero, **entonces** veo el conteo por estado igual a un conteo manual.
- **Dado** un taller de Ambos, **cuando** filtro por tipo, **entonces** veo carros y motos por separado (Should).

**Trazabilidad:** RF: RF-20 · Escena: 9 · Aceptación: CA-07

### HU-69 · Ver aprobaciones pendientes y cuánto llevan

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** ver las órdenes esperando aprobación y las horas desde que envié la cotización **para** llamar a quien no responde.

**Criterios de aceptación**

- **Dado** OS-0044 esperando hace 26 h, **cuando** abro el tablero, **entonces** veo "1 · 26 h".

**Trazabilidad:** RF: RF-20 · Escena: 9 · Aceptación: CA-07

### HU-70 · Ver el tiempo promedio por orden

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño, **quiero** el promedio de Recibido a Listo en los últimos 30 días **para** saber si estoy demorando.

**Criterios de aceptación**

- **Dado** órdenes entregadas en 30 días, **cuando** abro el tablero, **entonces** veo el promedio en horas y días hábiles.
- **Dado** un taller de Ambos, **cuando** filtro, **entonces** veo el promedio de motos y de carros por separado, porque mezclar los dos falsea el número (Should).

**Carro / moto:** Que las órdenes de moto sean más cortas o baratas es una suposición sin dato; el filtro sirve justo para comprobarlo.

**Trazabilidad:** RF: RF-20 · Escena: 9 · Aceptación: CA-07

### HU-71 · Ver el ticket promedio

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño, **quiero** el total de órdenes entregadas dividido entre su número en el mes **para** saber cuánto deja cada carro o moto.

**Criterios de aceptación**

- **Dado** el mes en curso, **cuando** abro el tablero, **entonces** veo el ticket en pesos sin decimales.
- **Dado** datos de ejemplo, **cuando** se muestran, **entonces** llevan la marca "ejemplo".

**Trazabilidad:** RF: RF-20 · Escena: 9 · Aceptación: CA-07

### HU-72 · Ver cuántos clientes vuelven

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** el porcentaje de vehículos con más de una orden en 12 meses **para** medir confianza.

**Criterios de aceptación**

- **Dado** Carlos con 2 visitas, **cuando** se calcula, **entonces** cuenta como vehículo que volvió.

**Trazabilidad:** RF: RF-20

### HU-73 · Ver cuánto me demoro en contestar WhatsApp

**Épica:** E09 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** la mediana de minutos entre un mensaje entrante y la primera respuesta **para** ver el efecto del agente.

**Criterios de aceptación**

- **Dado** mensajes del mes, **cuando** abro el tablero, **entonces** veo la mediana; las respuestas automáticas de AU-05 cuentan aparte.

**Notas:** H-07. Es la línea base que hoy no existe para el supuesto S5. Fuentes: [22], [21]

**Trazabilidad:** RF: RF-20 · Escena: 9

### HU-74 · Ver el consumo de mensajes y el costo de IA de cada taller

**Épica:** E09 · **Roles:** Administrador del ERP · **Superficies:** Admin, Motor · **Prioridad:** Must · **Origen:** Equipo

**Como** administrador del ERP, **quiero** un medidor por taller con los mensajes enviados por categoría, cuántos de los 1.000 de servicio gratis van y cuánto costó la IA en el mes **para** saber si la suscripción mensual cubre los costos de cada taller y ajustar el precio del plan (D-04).

**Criterios de aceptación**

- **Dado** el Servitaller El Pistón con 34 mensajes de servicio y $1.200 de IA en el mes (ejemplo), **cuando** entro como administrador, **entonces** veo "34 de 1.000" y "IA $1.200" en la fila de ese taller.
- **Dado** varios talleres, **cuando** abro la vista, **entonces** los veo en una lista con su costo del mes, para detectar al que se pasa.
- **Dado** el medidor, **cuando** lo comparo con la base, **entonces** coincide con los mensajes y las llamadas de IA registrados.
- **Dado** un usuario de taller (dueño, mecánico o recepción), **cuando** busca el medidor en pantalla o por URL o API, **entonces** no existe en su panel ni responde: recibe "sin permiso".

**Notas:** Cambio del 1-nov: antes era del dueño del taller. El costo de IA va incluido en la suscripción mensual, así que el taller no lo ve. Efectos por decidir: la escena 9 de la demo y lo que el dueño ve del consumo de mensajes (Q-24). Precios de Meta por categoría y país en su página oficial. Fuentes: [2]

**Trazabilidad:** RF: RF-20 · Escena: 9 · Aceptación: CA-08 · Depende de: HU-46, HU-99

## E10 · Asistente que guía al dueño

_Módulo del MVP: M8. Explicar los números y listar los pendientes del día._

### HU-75 · Preguntar qué significa un número

**Épica:** E10 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Equipo

**Como** dueño que no es de números, **quiero** un botón "¿qué significa?" en cada KPI que lo explique con datos de mi taller **para** entender mi negocio.

**Criterios de aceptación**

- **Dado** el ticket promedio, **cuando** toco "¿qué significa?", **entonces** leo en 2 o 3 frases simples qué es, cómo va frente al mes pasado y qué órdenes lo mueven.

**Notas:** El equipo definió un asistente que guía al dueño; qué hace primero es D-06.

**Trazabilidad:** Escena: 9

### HU-76 · Ver qué tengo pendiente hoy

**Épica:** E10 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** 3 acciones del día: aprobaciones sin respuesta, vehículos listos sin recoger, borradores en la bandeja **para** arrancar el día sabiendo qué hacer.

**Criterios de aceptación**

- **Dado** los datos de ejemplo, **cuando** abro el tablero, **entonces** veo OS-0044, la orden de Marta y los borradores pendientes, cada uno con un botón que lleva a resolverlo.

**Notas:** Se arma con reglas y consultas, sin IA.

**Trazabilidad:** Escena: 9

### HU-77 · Que el asistente también guíe al mecánico

**Épica:** E10 · **Roles:** Mecánico · **Superficies:** Piso · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** mecánico, **quiero** ver en cada orden qué me falta (fotos, estado, cotización) **para** no dejar huecos en el respaldo.

**Criterios de aceptación**

- **Dado** una orden recibida con menos de 4 fotos, **cuando** la abro, **entonces** veo "te faltan N fotos" con un botón que abre la cámara.
- **Dado** una orden en diagnóstico sin cotización, **cuando** la abro, **entonces** veo "falta la cotización" con un botón a armarla.
- **Dado** una orden con todo completo, **cuando** la abro, **entonces** no veo avisos.
- **Dado** el rol mecánico, **cuando** veo los avisos, **entonces** no incluyen costos ni totales de dinero.

**Notas:** Estaba como Later; los criterios son nuevos. El equipo dijo el 29-oct que el asistente sirve a dueño y mecánico. Se arma con reglas y consultas, sin IA, como HU-76 ("Hoy tienes pendiente"). Q-15 sigue abierta: qué hace primero el asistente.

**Trazabilidad:** Depende de: HU-17, HU-21, HU-27, HU-76

### HU-78 · Hacerle preguntas libres al asistente

**Épica:** E10 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** dueño, **quiero** preguntar con mis palabras, por ejemplo "¿cuánto facturé en frenos este mes?" **para** no buscar en pantallas.

**Criterios de aceptación**

- **Dado** una pregunta sobre cifras del taller que corresponde a una consulta validada, **cuando** la escribo, **entonces** el asistente responde con ese número y muestra de qué órdenes sale.
- **Dado** una pregunta que no corresponde a ninguna consulta validada, **cuando** la escribo, **entonces** responde "todavía no sé responder eso" en vez de inventar una cifra.
- **Dado** una respuesta con cifras, **cuando** la comparo con un conteo manual, **entonces** coincide.
- **Dado** el rol mecánico, **cuando** pregunta por facturación o costos, **entonces** no recibe cifras de dinero.

**Notas:** Estaba como Later; los criterios son nuevos. Riesgo: respuestas inventadas. Por eso solo responde con consultas validadas, y hay que definir cuáles (sin dato: ningún dueño las ha pedido). El costo de IA de cada pregunta entra en lo que ve el administrador (HU-74).

**Trazabilidad:** Depende de: HU-75, HU-79

## E11 · Usuarios, roles y aislamiento

_Módulo del MVP: M9. Cada taller aislado; cada rol ve lo suyo._

### HU-79 · Que mis datos no los vea otro taller

**Épica:** E11 · **Roles:** Dueño del taller · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Equipo

**Como** dueño, **quiero** que ningún dato de mi taller se cruce con otro **para** confiar en el sistema.

**Criterios de aceptación**

- **Dado** un usuario del taller B, **cuando** intenta abrir por URL o API una orden del taller A, **entonces** recibe "no encontrado" y el intento queda registrado.
- **Dado** cada consulta a la base, **cuando** se ejecuta, **entonces** lleva el filtro de taller; una prueba automática lo verifica.

**Notas:** RNF-09.

**Trazabilidad:** RF: RF-21 · Aceptación: CA-10

### HU-80 · Agregar a mi mecánico

**Épica:** E11 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** agregar un mecánico con su celular **para** que use la app desde su teléfono.

**Criterios de aceptación**

- **Dado** el celular del mecánico, **cuando** lo agrego, **entonces** el mecánico entra con código a su celular y solo ve lo de su rol.
- **Dado** un celular que ya es usuario en otro taller, **cuando** lo agrego, **entonces** puede pertenecer a los dos talleres y escoge en cuál trabaja.

**Notas:** El MVP pone "invitar por celular" como Should pero el rol mecánico como Must: alguna forma de crearlo es Must. La invitación pulida (enlace, recordatorio) queda Should.

**Trazabilidad:** RF: RF-21 · Escena: 5

### HU-81 · Ver solo lo que me toca como mecánico

**Épica:** E11 · **Roles:** Mecánico · **Superficies:** Piso, Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** mecánico, **quiero** ver mis órdenes, estados, fotos y precios de venta, sin costos ni KPIs de dinero **para** trabajar sin ruido.

**Criterios de aceptación**

- **Dado** el rol mecánico, **cuando** abro el tablero o el inventario, **entonces** no veo costos, ticket ni medidor de costo.
- **Dado** el rol mecánico, **cuando** intento entrar por URL a la configuración, **entonces** recibo "sin permiso".

**Trazabilidad:** RF: RF-21 · Aceptación: CA-10

### HU-82 · Trabajar como recepción

**Épica:** E11 · **Roles:** Recepción · **Superficies:** Panel, Piso · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** recepcionista, **quiero** registrar clientes, vehículos y órdenes y atender la bandeja si el dueño me deja **para** descargar al dueño.

**Criterios de aceptación**

- **Dado** el rol recepción, **cuando** entro, **entonces** veo clientes, vehículos, órdenes y bandeja; no veo costos ni configuración de automatizaciones.

### HU-83 · Ocultar la gestión de usuarios si trabajo solo

**Épica:** E11 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño que trabaja solo, **quiero** no ver opciones de equipo **para** que la app sea más simple.

**Criterios de aceptación**

- **Dado** "solo yo" en el registro, **cuando** uso la app, **entonces** no aparece Usuarios; puedo activarlo después en configuración.

### HU-84 · Desactivar a un usuario que se va

**Épica:** E11 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño, **quiero** quitarle el acceso a quien ya no trabaja conmigo **para** proteger mis datos.

**Criterios de aceptación**

- **Dado** un mecánico desactivado, **cuando** intenta entrar, **entonces** no puede; sus fotos, cambios y órdenes siguen con su nombre.

**Notas:** Derivada.

## E12 · Datos personales y trazabilidad

_Módulo del MVP: M10. Autorización previa (Ley 1581) y bitácora de todo._

### HU-85 · Dar mi autorización antes de que procesen mi mensaje

**Épica:** E12 · **Roles:** Cliente final, Agente y automatizaciones · **Superficies:** WhatsApp, Motor · **Prioridad:** Must · **Origen:** Respaldo oficial

**Como** cliente que escribe por primera vez, **quiero** que me pidan autorización con botones Acepto / Ver política antes de seguir **para** saber qué hacen con mis datos.

**Criterios de aceptación**

- **Dado** un número nuevo para ese taller, **cuando** escribe, **entonces** antes de cualquier proceso con IA recibe el mensaje de autorización con el nombre del taller como responsable.
- **Dado** que pulsa Acepto, **cuando** llega, **entonces** se guarda texto, versión de política, canal, id del mensaje, fecha y hora, y entonces se procesa el mensaje original.
- **Dado** que no responde o rechaza, **cuando** pasa el tiempo, **entonces** su mensaje no pasa por la IA; queda para que el dueño lo atienda a mano (Q-09).

**Notas:** Ley 1581 art. 9 (autorización previa) y Decreto 1377 de 2013 art. 7: se aceptan mecanismos técnicos que permitan la manifestación automatizada; el silencio nunca es conducta inequívoca. Si un toque en un botón de WhatsApp basta como prueba ante la SIC no se encontró: Q-08. Fuentes: [7], [8], [18]

**Trazabilidad:** RF: RF-22 · Escena: 4

### HU-86 · Pedir la autorización en el mostrador

**Épica:** E12 · **Roles:** Recepción, Mecánico, Cliente final · **Superficies:** Piso, WhatsApp · **Prioridad:** Must · **Origen:** Propuesta

**Como** persona que recibe a un cliente que llegó sin escribir antes, **quiero** pedirle la autorización en el checklist de recepción **para** poder enviarle los avisos sin violar la ley.

**Criterios de aceptación**

- **Dado** un cliente que llega en persona, **cuando** hago el checklist, **entonces** aparece la casilla de autorización con el texto, que el cliente marca en mi celular.
- **Dado** un cliente que no autoriza, **cuando** termino el checklist, **entonces** la orden se crea igual, pero no le sale ningún aviso automático y el dueño lo ve marcado.

**Notas:** Nueva y necesaria: RF-22 solo cubre a quien escribe primero, pero AU-01 le escribe a clientes recibidos en mostrador. Además ese primer aviso queda fuera de la ventana de 24 h y necesita plantilla (Q-07). Fuentes: [7], [2]

**Trazabilidad:** RF: RF-22 · Depende de: HU-17

### HU-87 · Pedir mis datos o que los borren

**Épica:** E12 · **Roles:** Cliente final, Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** cliente, **quiero** pedir una copia de mis datos o que los borren **para** ejercer mis derechos como titular.

**Criterios de aceptación**

- **Dado** una solicitud del cliente, **cuando** el dueño la atiende, **entonces** exporta sus datos o los anonimiza conservando órdenes y montos sin identificarlo.

### HU-88 · Ver quién cambió qué

**Épica:** E12 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** una bitácora de acciones **para** tener un sistema confiable y resolver reclamos.

**Criterios de aceptación**

- **Dado** cualquier cambio en órdenes, precios, inventario, usuarios o automatizaciones, **cuando** ocurre, **entonces** queda quién, qué, valor anterior, valor nuevo y cuándo.
- **Dado** la bitácora, **cuando** la abro, **entonces** puedo filtrar por placa, usuario o fecha.

**Trazabilidad:** RF: RF-07, RF-19

### HU-89 · Aceptar el contrato de transmisión con SWEVEH

**Épica:** E12 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** aceptar en el registro que mi taller es responsable de los datos y SWEVEH es encargado **para** cumplir con la ley desde el día uno.

**Criterios de aceptación**

- **Dado** el registro, **cuando** acepto, **entonces** queda guardado el contrato aceptado con versión y fecha.

**Notas:** Documento, no código. Concepto SIC 23-394496 (resumen de terceros). Servidores o IA fuera de Colombia se tratan como transmisión (RNF-11). Fuentes: [9]

### HU-90 · Publicar la política de tratamiento de datos

**Épica:** E12 · **Roles:** Administrador del ERP · **Superficies:** Admin, Panel · **Prioridad:** Must · **Origen:** Propuesta

**Como** equipo, **quiero** tener la política en una URL pública **para** enlazarla en las autorizaciones y en la revisión de Meta.

**Criterios de aceptación**

- **Dado** la URL, **cuando** se abre, **entonces** muestra la versión vigente y su fecha; las autorizaciones guardan esa versión.

## E13 · Migración

_Módulo del MVP: M11. Traer lo que ya existe sin bloquear el arranque._

### HU-91 · Llenar el sistema mientras trabajo

**Épica:** E13 · **Roles:** Dueño del taller, Mecánico · **Superficies:** Motor, Piso · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño que no tiene nada digital, **quiero** que cada orden nueva cree cliente y vehículo **para** no tener que migrar nada para empezar.

**Criterios de aceptación**

- **Dado** una orden nueva con placa y celular desconocidos, **cuando** la creo, **entonces** se crean cliente y vehículo sin pantallas extra.

**Notas:** Vía E: el único camino para quien no tiene nada. Fuentes: [20]

**Trazabilidad:** Depende de: HU-16

### HU-92 · Subir mis clientes y vehículos con una plantilla

**Épica:** E13 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Should) · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** dueño con Excel, **quiero** subir clientes, vehículos, servicios y repuestos con una plantilla **para** no empezar de cero.

**Criterios de aceptación**

- **Dado** la plantilla con columna tipo de vehículo, **cuando** la subo, **entonces** cada placa se valida según sea carro o moto y las filas con error se marcan.
- **Dado** clientes importados, **cuando** quedan en la base, **entonces** su autorización queda "pendiente" (HU-95).
- **Dado** una fila con placa pero sin cilindraje o turbo, **cuando** la importo, **entonces** el vehículo entra con "motor pendiente" y no se rechaza la fila.

### HU-93 · Pasar mi lista de precios con una foto

**Épica:** E13 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** dueño con lista escrita a mano, **quiero** tomarle foto y que la IA proponga las filas **para** cargar precios en un minuto.

**Criterios de aceptación**

- **Dado** una foto de 7 servicios, **cuando** la subo, **entonces** aparecen 7 filas en Por revisar con la foto al lado; corrijo y apruebo cada una.

**Notas:** Escena opcional (D-09). Un tercero reporta 87 % de acierto en notas a mano y 1–3 % de datos inventados; en talleres colombianos sin dato. Fuentes: [17]

**Trazabilidad:** Escena: 3

### HU-94 · Subir mi propio Excel con mapeo de columnas

**Épica:** E13 · **Roles:** Dueño del taller · **Superficies:** Panel · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** dueño, **quiero** subir mi Excel tal como lo tengo y que el sistema proponga a qué dato corresponde cada columna **para** no copiar a una plantilla.

**Criterios de aceptación**

- **Dado** un Excel con columnas propias ("Cliente", "Cel", "Vehiculo"), **cuando** lo subo, **entonces** veo una propuesta de a qué campo corresponde cada columna y la confirmo o corrijo en "Por revisar".
- **Dado** una columna que no se sabe mapear, **cuando** la veo, **entonces** queda sin mapear y no se inventa nada.
- **Dado** el mapeo confirmado, **cuando** importo, **entonces** se aplican las mismas validaciones de la plantilla (celular, placa, motor pendiente).
- **Dado** clientes importados, **cuando** entran, **entonces** con autorización pendiente y sin avisos automáticos (HU-93).

**Notas:** Estaba como Later (vía B); los criterios son nuevos. Precisión de la IA en hojas de talleres colombianos: sin dato.

**Trazabilidad:** Depende de: HU-92, HU-62

### HU-95 · No escribirle a clientes importados sin autorización

**Épica:** E13 · **Roles:** Agente y automatizaciones, Dueño del taller · **Superficies:** Motor · **Prioridad:** Must · **Origen:** Propuesta

**Como** dueño, **quiero** que los clientes importados no reciban avisos automáticos hasta autorizar **para** no violar la Ley 1581.

**Criterios de aceptación**

- **Dado** un cliente importado con autorización pendiente, **cuando** su orden pasa a Recibido, **entonces** no sale AU-01 y el dueño ve por qué.

**Notas:** Hueco H7 del arranque. Derivada. Fuentes: [7], [20]

**Trazabilidad:** RF: RF-22

### HU-96 · Traer cuadernos completos y chats viejos

**Épica:** E13 · **Roles:** Dueño del taller · **Superficies:** Panel, Motor · **Prioridad:** Must (antes Later) · **Origen:** Propuesta

**Como** dueño, **quiero** subir fotos de las páginas de mi cuaderno o un chat de WhatsApp exportado y que la IA proponga qué hay ahí **para** tener todo el historial.

**Criterios de aceptación**

- **Dado** la foto de una página de cuaderno con varias líneas de servicios, **cuando** la subo, **entonces** veo filas propuestas (fecha, placa o nombre, servicio, valor) en "Por revisar", con la foto al lado, y apruebo, corrijo o descarto cada una.
- **Dado** una línea ilegible, **cuando** la veo, **entonces** queda marcada "no se pudo leer" y no se inventa un dato.
- **Dado** un chat exportado de WhatsApp, **cuando** lo subo, **entonces** se proponen clientes, vehículos y servicios, y no se envía ningún mensaje a esos clientes.
- **Dado** clientes sacados de cuadernos o chats, **cuando** entran, **entonces** con autorización pendiente y sin avisos automáticos (HU-93).
- **Dado** un lote de prueba de 100 líneas, **cuando** se procesa, **entonces** queda registrado el porcentaje aprobado sin cambios, para decidir si la función se ofrece.

**Notas:** Estaba como Later; los criterios son nuevos. H6 y H7 sin resolver: no hay dato de precisión con cuadernos en español (un tercero reporta 87 % en notas a mano y 1 a 3 % de datos inventados) y un chat exportado contiene datos de terceros, que la Ley 1581 pide autorizar antes de usarlos. Fuentes: [7], [17]

**Trazabilidad:** Depende de: HU-91, HU-93, HU-62

## E14 · Operación de la demo

_Módulo del MVP: Preparativos. Que la demo corra dos veces seguidas sin tocar código._

### HU-97 · Reiniciar el taller ficticio entre demos

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Admin · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** equipo, **quiero** un script que deje Servitaller El Pistón en el estado inicial **para** correr la demo dos veces seguidas.

**Criterios de aceptación**

- **Dado** una demo terminada, **cuando** corro el script, **entonces** quedan los 5 clientes, la moto de Sebastián, OS-0040 a OS-0044, el inventario inicial y unas 20 órdenes de ejemplo marcadas.
- **Dado** los vehículos de ejemplo, **cuando** se cargan, **entonces** todos tienen cilindraje y turbo (valores ficticios), menos uno que queda con "motor pendiente" para mostrar el caso.
- **Dado** las 10 escenas, **cuando** las corro dos veces seguidas, **entonces** cada vez tardan menos de 17 minutos sin tocar código ni la base.

**Trazabilidad:** Escena: 1-10 · Aceptación: CA-12

### HU-98 · Agregar una moto al taller ficticio

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Admin · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** equipo, **quiero** que la demo muestre una moto **para** que un dueño de taller de motos se vea reflejado.

**Criterios de aceptación**

- **Dado** el taller ficticio pasa a atender carros y motos, **cuando** se carga, **entonces** existe Sebastián Arango con una Honda CB 190R 2021 placa XVN43F (inventada).
- **Dado** la escena 5-M (opcional, 1 min), **cuando** Jhon recibe la moto, **entonces** se ve el checklist de moto, el cilindraje y turbo de la moto, y el aviso llega al celular D.

**Carro / moto:** Propuesta para cumplir la decisión del 31-oct. Usa el celular D (de 5 números verificados del número de prueba). Ficticio.

**Trazabilidad:** Escena: 5-M

### HU-99 · Ver costo, tokens y latencia de cada llamada a la IA

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Motor, Admin · **Prioridad:** Must · **Origen:** Equipo

**Como** equipo, **quiero** registrar proveedor, modelo, tokens, costo y latencia por llamada **para** comparar modelos (D-08) y cuidar el costo.

**Criterios de aceptación**

- **Dado** una llamada a la IA, **cuando** termina, **entonces** queda en LlamadaIA ligada al mensaje o a la migración.

**Notas:** RNF-06 y RNF-14.

### HU-100 · Ver el registro técnico de mensajes

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Admin · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** equipo, **quiero** ver lo que entra y sale por el webhook con sus estados **para** diagnosticar una falla en vivo.

**Criterios de aceptación**

- **Dado** un mensaje que no llegó, **cuando** busco por número, **entonces** veo si Meta lo recibió, el error y la hora.

### HU-101 · Cambiar del número de prueba al número del taller

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Admin, Motor · **Prioridad:** Must (antes Should) · **Origen:** Propuesta

**Como** equipo, **quiero** que WhatsApp esté detrás de una interfaz de canal **para** pasar a producción sin tocar el resto.

**Criterios de aceptación**

- **Dado** el número de prueba, **cuando** se cambia por el número del taller, **entonces** solo cambia la configuración del canal.

**Notas:** RNF-07. Producción requiere Tech Provider y Embedded Signup; coexistencia en Colombia sin dato. Fuentes: [23], [4]

### HU-102 · Probar el agente con 30 mensajes reales de estilo

**Épica:** E14 · **Roles:** Administrador del ERP · **Superficies:** Admin · **Prioridad:** Must · **Origen:** Propuesta · **Aplica a motos:** sí

**Como** equipo, **quiero** un juego de 30 mensajes escritos como los escriben los clientes, con 10 de moto **para** medir antes de la demo.

**Criterios de aceptación**

- **Dado** los 30 mensajes, **cuando** pasan por el agente, **entonces** ningún borrador tiene precios fuera de lista y al menos 25 tienen el servicio bien extraído.

**Notas:** Los 10 de moto son propuesta nueva.

**Trazabilidad:** Aceptación: CA-02

## Referencias citadas en las historias

1. Meta for Developers, "Get started", WhatsApp Cloud API (número de prueba, 5 destinatarios, plantillas preaprobadas). <https://developers.facebook.com/docs/whatsapp/cloud-api/get-started/> (Oficial)
2. Meta for Developers, "Pricing on the WhatsApp Business Platform" (ventana de 24 h, categorías, 1.000 de servicio gratis desde 1-oct-2026). <https://developers.facebook.com/documentation/business-messaging/whatsapp/pricing> (Oficial)
3. Meta for Developers, "Interactive reply buttons messages" (hasta 3 botones, 20 caracteres por título, encabezado con imagen). Consultado 1-nov-2026. <https://developers.facebook.com/documentation/business-messaging/whatsapp/messages/interactive-reply-buttons-messages> (Oficial)
4. Meta for Developers, "Onboarding WhatsApp Business app users" (coexistencia). <https://developers.facebook.com/documentation/business-messaging/whatsapp/embedded-signup/onboarding-business-app-users> (Oficial)
5. TechCrunch, "WhatsApp changes its terms to bar general-purpose chatbots from its platform", 18-oct-2025. <https://techcrunch.com/2025/10/18/whatssapp-changes-its-terms-to-bar-general-purpose-chatbots-from-its-platform> (Prensa)
6. Meta for Developers, "Meta Business Agent: capabilities". <https://developers.facebook.com/documentation/meta-business-agent/capabilities> (Oficial)
7. Ley Estatutaria 1581 de 2012, Función Pública (art. 9 autorización previa; art. 8 derechos del titular). <https://www.funcionpublica.gov.co/eva/gestornormativo/norma.php?i=49981> (Norma)
8. Decreto 1377 de 2013, Alcaldía de Bogotá, Sisjur (art. 7 modo de obtener la autorización y silencio; art. 8 prueba). Consultado 1-nov-2026. <https://www.alcaldiabogota.gov.co/sisjur/normas/Norma1.jsp?i=53646> (Norma)
9. G&D Consulting, resumen del concepto SIC 23-394496 (datos en la nube, responsable y encargado). <https://www.gydconsulting.com/como-gestionar-datos-en-la-nube-concepto-sic-n-23-394496/> (Tercero)
10. Ley 1480 de 2011, art. 18 (recibo del bien: fecha, datos de quien entrega, identificación del bien, servicio, abonos, garantía; aceptación expresa del valor; presunción de buen estado sin salvedad). Compendio SDV, consultado 1-nov-2026. <https://sdv.com.mx/compendio/ley-1480-2011/articulo-18/> (Tercero (texto de norma))
11. SIC, concepto 15-102150 de 2015 sobre el art. 18 de la Ley 1480 (constancia de recibo). <https://ambitojuridico.com/sites/default/files/BancoMedios/Archivos/cpto-15102150-15.pdf> (Autoridad)
12. Resolución 3800 de 1995, Mintransporte (placa única nacional de tres letras y tres números). <https://www.alcaldiabogota.gov.co/sisjur/normas/Norma1.jsp?i=4803> (Norma)
13. Wikipedia, "Vehicle registration plates of Colombia" (motos: ABC·12F actual y ABC·12 antiguo; sin norma citada). Consultado 1-nov-2026. <https://en.wikipedia.org/wiki/Vehicle_registration_plates_of_Colombia> (Tercero)
14. RUNT, "ABC Renovación RTM" (primera revisión: motos a los 2 años de matriculadas, particulares a los 5; luego anual). <https://www.runt.gov.co/sites/default/files/documentos/ABC%20Renovaci%C3%B3n%20RTM.pdf> (Oficial)
15. RUNT, Boletín de prensa 001 de 2026, 27-feb-2026 (21.345.925 vehículos activos al cierre de 2025, 13.528.164 motos, 63 %). <https://www.runt.gov.co/sites/default/files/Bolet%C3%ADn%20001%20de%202026.pdf> (Oficial)
16. J.D. Power 2024 U.S. Aftermarket Service Index, Business Wire, 30-abr-2024. <https://www.businesswire.com/news/home/20240430464654/en/Customers-Appreciate-Aftermarket-Service-Providers-for-Their-Convenience-J.D.-Power-Finds> (Estudio)
17. RaftLabs, "OCR vs LLM for invoice processing". <https://raftlabs.com/blog/ocr-vs-llm-invoice-processing> (Tercero)
18. Asuntos Legales, "Conductas inequívocas en el tratamiento de datos", 4-ago-2026. <https://www.asuntoslegales.com.co/analisis/andrea-usme-castillo-2538483/conductas-inequivocas-en-el-tratamiento-de-datos-2719929> (Prensa / opinión)
19. MVP SWEVEH (artifact del proyecto, 30-oct-2026): módulos, RF, RNF, guion, D-01 a D-13. <https://claude.ai/artifact/9Cbp2G4mjX6rkVbJyUjCcq> (Proyecto)
20. Arranque SWEVEH (artifact del proyecto): registro, migración, H1 a H8. <https://claude.ai/artifact/Hg5y3kJVHUstHMuTrz5kdv> (Proyecto)
21. Ejercicios MakersXver+ (artifact del proyecto): S2, S5, flujo v0. <https://claude.ai/artifact/6MieucEHoZRqSMmkkSj3ZX> (Proyecto)
22. Huecos de la competencia SWEVEH (artifact del proyecto): H-01 a H-08, R-01 a R-10. <https://claude.ai/artifact/VeGd6DRH1ykSTWeRhJZcAz> (Proyecto)
23. Meta for Developers, "Get started for Tech Providers". <https://developers.secure.facebook.com/documentation/business-messaging/whatsapp/solution-providers/get-started-for-tech-providers/> (Oficial)
24. MDN, "Making PWAs installable" (manifest, HTTPS, iconos 192 y 512 px; service worker no obligatorio; iOS 16.4 o posterior instala desde el menú Compartir). Consultado 1-nov-2026. <https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_Installable> (Oficial)
25. MDN, "HTML attribute: capture" (input type=file con capture=environment abre la cámara trasera; funciona mejor en móvil; no es Baseline). Consultado 1-nov-2026. <https://developer.mozilla.org/en-US/docs/Web/HTML/Attributes/capture> (Oficial)
