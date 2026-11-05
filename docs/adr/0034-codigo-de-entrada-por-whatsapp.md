# ADR-0034 · Código de entrada por el WhatsApp de prueba (WAHA)

- **Estado:** Propuesto (demo)
- **Fecha:** 2026-11-05
- **Quién decide / aprueba:** Equipo; revisión de Isaac y Pablo en el PR
- **Relacionado con:** Q-13, AQ-03, ADR-0027 (WAHA), ADR-0022, docs/08 §6

## Contexto
El código de 6 números para entrar solo tenía el adaptador `consola`, que lo escribe en el log del api y la configuración rechaza fuera de local (el código nunca va a logs). Sin un envío real, el api no arranca con `APP_ENV=staging` y no se puede probar en la nube. El canal definitivo (plantilla de autenticación de Meta o SMS) sigue abierto (Q-13/AQ-03) y su costo es `Sin dato`.

## Decisión
- `OTP_SENDER=waha`: el código sale por el WhatsApp de prueba de WAHA, con el texto "Tu código de SWEVEH es *123456*. Vence en 10 minutos. No lo compartas con nadie."
- **Vive en `wire`** (`wire.WAHAOTP`), no en `identity`: junta el puerto `OTPSender` de identity con el cliente de messaging, y las reglas de arquitectura no dejan que un módulo importe adaptadores de otro.
- **Número de la plataforma:** el código sale por la sesión `WAHA_SESSION` (por defecto `default`), que no es de ningún taller; los talleres vinculan cada uno su sesión (ADR-0035).
- **Configuración:** `OTP_SENDER=waha` exige `WAHA_URL`, `WAHA_API_KEY` y `WAHA_WEBHOOK_SECRET`. Como WAHA ya está vedado en producción (ADR-0027), `waha` tampoco llega a producción; allí falta el canal oficial.
- **El código nunca sale en logs ni en errores:** el cliente de WAHA devuelve solo el estado HTTP.
- `WHATSAPP_PROVIDER` se queda en `local`: WAHA se activa solo con `WAHA_URL`; `waha` no es un valor válido de ese proveedor.

## Consecuencias
- El número de prueba de WAHA debe estar vinculado (sesión `WAHA_SESSION`) para que llegue el código; si WAHA está caído, pedir el código falla y el usuario no puede entrar.
- El celular que entra debe tener WhatsApp y puede ser el mismo del número vinculado: WhatsApp no entrega mensajes a uno mismo de forma fiable, así que hay que entrar con otro número.
- **Pendiente para producción:** plantilla de autenticación de Meta (categoría "autenticación") o SMS, con su costo (Q-13).

## Evidencia
- `TestWAHAOTP` (el envío a la sesión y al chat correctos y el error sin el código) y `TestOTPSenderWAHA` (reglas de configuración por entorno).
