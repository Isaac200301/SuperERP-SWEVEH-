# ADR-0028 · El agente responde solo (envío automático, encendido por defecto)

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Pablo (lo pidió al probar el simulador); revisión de Isaac en el PR
- **Relacionado con:** D-07, Q-27, HU-67, HU-59, HU-54, HU-58, RF-17, RF-18, CA-09, RNF-05, ADR-0026

## Contexto
Hasta ahora todo lo que el agente proponía quedaba en "Por revisar" hasta que una persona pulsaba Enviar (D-07: "siempre con aprobación en el MVP"). Al probarlo, Pablo vio que el taller tenía que estar pendiente de cada mensaje. Su regla es otra: **el agente responde siempre, a menos que el cliente o el taller digan lo contrario.**

D-07 y RF-18 nacieron para que un texto **redactado por la IA** no le llegara al cliente sin revisión. Desde RNF-05 y HU-53 la IA ya no redacta: solo devuelve un JSON validado con la intención, los servicios y la placa (`domain.ParseUnderstanding`). El texto lo arma el código con plantillas y con la lista de precios del taller (`domain.BuildDraft`), y `domain.Validate` rechaza un precio que no esté en la lista (RF-17). Es el caso que la recomendación de Q-27 permite enviar solo.

## Decisión
- **Interruptor por taller, encendido de fábrica.** Lo guarda `assistant.ajuste.auto_envio` y vale `true` si el taller no tiene fila, así que los talleres que ya existen también arrancan encendidos. Se lee con `GET /api/v1/assistant/settings` (`inbox:manage`) y solo el dueño lo cambia con `PUT` (`automation:manage`). Cada cambio queda en auditoría (`asistente.auto_envio`).
- **Encendido, toda respuesta al cliente sale sola:** precios, diagnóstico, fuera de tema, placa y respuestas sin precio. El borrador queda `auto_enviado`, sin persona que lo resolvió, y la auditoría registra `borrador.auto_enviado` hecho por el agente. El texto se vuelve a validar antes de salir, y si trae una placa nueva se crea la orden (HU-55), igual que cuando el dueño pulsa Enviar.
- **Lo que siempre espera en "Por revisar":**
  - las notas para el dueño que no tienen nada que decirle al cliente: "No pudimos armar una respuesta segura. Responde tú." (la IA falló o la validación no pasó, HU-54) y la nota de voz (HU-60);
  - la respuesta de estado cuando el dueño apagó AU-05;
  - una respuesta que no pasa la validación de precios o que llega con la ventana de 24 h cerrada.
- **El agente se detiene** cuando la conversación pasa a una persona:
  - el cliente pide hablar con alguien (HU-59);
  - el cliente no acepta la autorización;
  - el cliente tiene dudas sobre la cotización;
  - el taller pulsa "Lo contesto yo" o toma la conversación.

  Vuelve con "Devolver al asistente". El interruptor apagado devuelve todo a "Por revisar" (HU-67).
- **Sin envíos repetidos.** `InsertDraft` dice si el borrador ya existía; si el evento llega otra vez, no se envía de nuevo.

## Consecuencias
- El taller no tiene que estar pendiente de cada mensaje. En "Por revisar" ve las últimas respuestas automáticas.
- **Cambian D-07, RF-18 y CA-09:** lo que sale sin aprobación son plantillas con datos del taller, no texto de la IA. Si en el futuro la IA redacta texto libre (una mejora a HU-53), ese texto **no** puede salir solo sin revisar esta decisión.
- HU-67 decía "apagado por defecto" y "lo que mezcla un síntoma va a revisión". Pablo eligió "solo siempre": el diagnóstico también sale solo, porque es plantilla ("no te podemos decir qué tiene sin revisarlo; el diagnóstico cuesta…"). Si se quiere excluir algún tipo, se agrega la excepción en `assistant/app.answer` y no en el prompt.
- El riesgo es que la IA entienda mal y se cotice otro servicio. Lo acotan la lista cerrada del taller y la validación de precios; la medición de HU-102 lleva 28 de 30. Hay que seguir midiendo con mensajes reales antes de producción (D-08).
