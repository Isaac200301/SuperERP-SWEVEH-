# ADR-0027 · WAHA como WhatsApp de prueba para la demo (no oficial)

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Pablo (pidió WAHA para la demo, sabiendo que no es oficial); revisión de Isaac en el PR. D-01 sigue abierta para producción
- **Relacionado con:** ADR-0025 (canal por proveedor), ADR-0026 (IA por proveedor), D-01, AQ-10, HU-85, HU-28, HU-19, HU-57, HU-101, R-05, R-09

## Contexto
La demo necesita un WhatsApp de verdad: un cliente escribe desde su celular y el taller responde. El número de prueba de Meta exige cuenta de negocio verificada y una URL pública para el webhook, y todavía no los tenemos. Pablo eligió **WAHA** (https://waha.devlike.pro), que vincula un número normal escaneando un QR, como WhatsApp Web, y expone una API HTTP. Desde la versión 2026.6.1 la edición gratuita incluye varias sesiones y multimedia.

La documentación de WAHA, consultada el 2026-11-04, dice que **los botones no funcionan** ("DEPRECATED - Buttons do not work at the moment") y que las listas "pueden dejar de funcionar". El flujo usa botones para tres cosas: la autorización de datos (HU-85), aprobar la cotización (HU-28) y la constancia de recibo (HU-19).

## Decisión
- **Un paquete más detrás de los mismos puertos** (`messaging/adapters/out/whatsapp/waha`). Implementa `app.Channel` para enviar texto e imagen en base64 (`SendTemplate` responde `ErrUnsupported`) y `app.InboundParser` para leer los eventos `message` y `message.ack`. Los casos de uso, el outbox, las automatizaciones y el agente no cambian.
- **Menú con números en vez de botones.** `SendButtons` agrega al texto "Responde con el número: 1. Acepto 2. No acepto" (`domain.Menu`). Al guardar un mensaje entrante (`messaging/app.menuReply`), si el último mensaje de la conversación fue nuestro con opciones y el cliente responde con el número o el título ("1", "acepto", "Aprobar"), se guarda como `respuesta_boton` con el id del botón (`domain.MatchReply`). Un texto que no es una opción sigue como texto normal. La regla vale para cualquier canal: con botones reales, escribir la opción también funciona.
- **Número del taller `waha:<sesión>`.** El enrutador manda esos números a WAHA, los `local-…` al simulador y el resto a Meta. Un taller cambia entre el simulador y WAHA con `PUT /whatsapp/channel` (HU-101). Mientras usa WAHA, el simulador no funciona, para que una respuesta no le llegue a un celular real por error.
- **Webhook propio con firma.** `POST /webhooks/waha` valida HMAC-SHA512 del cuerpo crudo (`X-Webhook-Hmac`, clave `WAHA_WEBHOOK_SECRET` = `WHATSAPP_HOOK_HMAC_KEY` en WAHA). Guarda el cuerpo en la misma cola que Meta, y el parser de `wire` reconoce el formato: WAHA trae `event` y `session`. La prueba usa el ejemplo de firma de la documentación oficial.
- **Motor GOWS**, sin navegador por dentro: imagen `devlikeapro/waha:gows-2026.10.1` en el perfil `waha` de `docker-compose.yml`, que solo arranca con `make waha`. El QR se escanea desde Mi taller → WhatsApp de prueba; el api se lo pide a WAHA y la clave nunca llega al navegador.
- **Solo demo.** Con `WAHA_URL` en producción la configuración no arranca. Sin `WAHA_API_KEY` y `WAHA_WEBHOOK_SECRET`, tampoco.

## Consecuencias
- La demo funciona con un celular real sin cuenta de Meta. Pasar a Meta después es configuración y `PUT` del canal; el código del flujo es el mismo.
- **Riesgo de bloqueo:** WhatsApp puede bloquear números que usan clientes no oficiales, y limita a quien escribe a muchos contactos nuevos ("Reachout Timelock"). Por eso se usa un **número de prueba**, nunca el del taller, y nunca con datos reales (AQ-10).
- Sin plantillas ni ventana oficial de 24 h en WAHA. Se mantiene la regla de la ventana (HU-47) para que la demo se comporte como producción.
- **Chats `@lid` (pasó en la prueba real, 2026-11-10).** WhatsApp entregó remitentes como `…@lid`, un id que oculta el número. El agente fallaba con `celular_invalido` antes de enviar la autorización, y el outbox reintentaba esos eventos sin fin (más de 600 intentos en los logs de Render) sin responder nada. Se corrigió así:
  - **Traducción del id:** al guardar el mensaje, `messaging/app.storeInbound` traduce el `@lid` al celular con el puerto opcional `PhoneResolver`. WAHA lo resuelve con `GET /api/{sesión}/lids/{lid}`, con memoria por sesión (https://waha.devlike.pro/docs/how-to/contacts/, consultada el 2026-11-10).
  - **Si no se puede traducir:** si WAHA no conoce el número (`pn: null`) o el chat no tiene un celular colombiano, el agente no procesa el mensaje. La conversación pasa a una persona (modo `humano`, `pide_humano`) y no queda nada reintentándose (Q-34).
  - Prueba: `TestWAHA_LidChats`.
  - **Traducción tardía (2026-11-11, Q-36):** si al llegar no se supo el número, al listar o abrir la conversación se vuelve a pedir (máximo una vez cada 5 min). Si WAHA ya lo sabe, la conversación queda con el celular. La pantalla nunca muestra el `@lid`: muestra el nombre que mandó WhatsApp con "Número oculto por WhatsApp".
- **WhatsApp puede cerrar la sesión (pasó el 2026-11-11).** Los logs de WAHA mostraron `device removed` → `Logged out` (401), y después cada `sendText` quedaba en `request aborted`. Una sesión `FAILED` no vuelve con `/start`, porque sus credenciales ya no sirven, así que "Conectar número de prueba" la borra (`DELETE /api/sessions/{s}`) y la crea limpia para que haya un QR nuevo. El panel muestra "WhatsApp desconectado" en todas las pantallas mientras la sesión del taller no esté `WORKING`. Cambiar de motor (NOWEB/Baileys) no evita esto: todos se hacen pasar por WhatsApp Web.
- El id de los mensajes que devuelve `sendText` no está documentado. El adaptador acepta texto u objeto con `_serialized`, y se confirma con el número real que coincide con el de `message.ack`.
