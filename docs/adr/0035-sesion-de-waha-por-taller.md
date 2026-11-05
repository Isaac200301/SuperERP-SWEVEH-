# ADR-0035 · Una sesión de WAHA por taller

- **Estado:** Propuesto (demo)
- **Fecha:** 2026-11-05
- **Quién decide / aprueba:** Pablo ("una sesión por cada cliente"); revisión de Isaac en el PR
- **Relacionado con:** ADR-0027 (WAHA), ADR-0034 (código de entrada), ADR-0025 (canal por proveedor), HU-101

## Contexto
El WhatsApp de prueba de WAHA tenía **una sola sesión** (`WAHA_SESSION`, por defecto `default`): un único número para todo el sistema. Con varios talleres, solo uno podía usarlo y los demás no tenían número propio. El sistema ya separa los números por taller (`messaging.canal` guarda el número de cada uno y un webhook se resuelve por ese número); lo único fijo era el nombre de la sesión.

## Decisión
- **Cada taller tiene su propia sesión de WAHA**, nombrada `t-<id del taller>` (`waha.SessionFor`). Su número de WhatsApp se vincula escaneando el QR en Mi taller, como antes, pero el QR es el de su sesión.
- **El canal del taller** guarda `waha:t-<id>` (`messaging.canal.phone_number_id`, único en todo el sistema), así un mensaje que entra por esa sesión se asigna a ese taller y las respuestas salen por esa misma sesión.
- **Compatibilidad:** si el taller ya usa un canal de WAHA (por ejemplo `waha:default` de antes), se respeta su sesión. Un taller nuevo, o uno que vuelve a WAHA después del simulador, usa la suya.
- **El número de la plataforma es aparte:** `WAHA_SESSION` (`default`) es el que manda el **código de entrada** (ADR-0034). No es de ningún taller: sus mensajes entrantes se descartan, y ninguna sesión de taller puede llamarse así.
- Sigue siendo **un solo servidor WAHA**, un solo webhook (`/webhooks/waha`, firmado) y un solo worker. WAHA manda a esa URL los eventos de todas sus sesiones y el campo `session` dice cuál es.

## Consecuencias
- Cada sesión de WAHA consume memoria en el servidor (GOWS es liviano, pero no gratis): en el plan gratis de Render, pocas sesiones.
- **Se pierden al reiniciarse WAHA** si no hay disco persistente (`/app/.sessions`): cada taller vuelve a escanear su QR. Con disco, sobreviven.
- Sigue siendo WhatsApp no oficial, solo para la demo y con números de prueba. Para clientes reales, cada taller conecta su número por Meta (Embedded Signup, HU-101); la parte de enrutar por número ya es la misma.

## Evidencia
- `TestWAHA_SessionPerWorkshop`: dos talleres con su sesión; un mensaje por la sesión de uno llega solo a ese taller; una sesión desconocida o el número de la plataforma no crean conversaciones; cada taller responde por su propia sesión.
- `TestWAHA_DemoNumber` y `TestWhatsApp_VoiceNoteTranscribed` ya trabajan con la sesión del taller.
