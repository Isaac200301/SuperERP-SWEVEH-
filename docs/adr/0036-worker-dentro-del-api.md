# ADR-0036 · El worker dentro del api para la demo (`RUN_WORKER`)

- **Estado:** Propuesto (demo)
- **Fecha:** 2026-11-05
- **Quién decide / aprueba:** Pablo (opción "gratis" para Render); revisión de Isaac en el PR
- **Relacionado con:** ADR-0022 (cómputo), ADR-0027 (WAHA), ADR-0034, ADR-0035, docs/06 §4.5

## Contexto
En Render el api recibe los webhooks de WhatsApp y los guarda en la cola, pero los procesa el **worker**, que es otro proceso. Render cobra los Background Workers. Sin worker, el 5 de noviembre había 27 webhooks sin procesar: los mensajes no aparecían en la app y el agente no contestaba.

## Decisión
- **Variable `RUN_WORKER`** (por defecto `false`). Con `true`, el api arranca el worker en el mismo proceso: outbox (avisos, agente y envíos), webhooks crudos y tareas diarias. El código es uno solo: `wire.StartWorker` lo usan `cmd/worker` y `cmd/api`.
- **Rol de base propio:** el worker embebido abre un segundo pool con `WORKER_DATABASE_URL` (rol `sweveh_worker`, máx. 4 conexiones), porque el despacho necesita permisos que el api no tiene. Los roles y el aislamiento por taller quedan como siempre.
- **Mismo almacenamiento** que el api (`STORAGE_DIR`): las fotos que se mandan por WhatsApp salen del disco donde se subieron.
- **Apagado ordenado:** al detenerse el api, se cancela el worker, se espera que termine y se cierra su pool.
- **Dos procesos a la vez no duplican envíos:** el outbox y los webhooks usan `FOR UPDATE SKIP LOCKED`, y las tareas diarias un candado consultivo.

## Consecuencias
- Un solo servicio web gratis en Render alcanza para la demo. Comparten la máquina: el worker casi no gasta CPU (espera respuestas de OpenRouter, Groq y WAHA) y suma unos 30 a 60 MB de memoria.
- **Si el servicio se duerme** (plan gratis, 15 minutos sin tráfico), el worker también. Un mensaje de WhatsApp lo despierta (la primera respuesta tarda 30 a 60 s), pero las tareas diarias (recordatorio de las 7:00 a. m.) no corren mientras duerme. Un ping cada 10 minutos a `/healthz` lo mantiene despierto.
- **Producción:** api y worker separados (ADR-0022), sin `RUN_WORKER`: escalan distinto y una tanda grande de mensajes no debe competir con las pantallas.

## Evidencia
- `TestStartWorker_ProcessesInboundAlone`: el worker arrancado por `StartWorker` procesa solo un mensaje entrante (el cliente recibe la autorización) y se detiene al cancelarlo.
- `TestRunWorkerNeedsWorkerDatabase`: `RUN_WORKER=true` exige `WORKER_DATABASE_URL`; sin la variable, nada cambia.
