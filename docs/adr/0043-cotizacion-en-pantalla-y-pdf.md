# ADR-0043 · La cotización en pantalla y en PDF

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-10, 24-nov-2026); Isaac revisa en el PR
- **Fecha:** 2026-11-25 (R7)
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Relacionado con:** HU-109 (nueva), HU-29, HU-86, HU-105, HU-106, ADR-0040, docs/06 §6 y §8

## Contexto
Sin WhatsApp (V-01) la cotización ya no le llega al cliente por un botón. El cliente está en el taller o al teléfono: el dueño le **muestra la cotización en el celular o la tablet** y toca "El cliente aprobó" o "No aprobó", o le **comparte un PDF** por la app que quiera. No hay integración con WhatsApp ni enlace público (V-10).

## Decisión

### Qué ve el cliente (lista blanca)
- **`GET /orders/{id}/quote/client-view`** devuelve `documento` (taller, orden, vehículo, placa, nombre del cliente y la cotización) y, aparte, `autorizacion` (para quien atiende, no para el cliente).
- La salida se arma con un tipo propio del dominio, `ClientView`, no con la cotización completa: lo que no está en ese tipo **no puede salir**. No lleva la referencia al catálogo, si el precio lo puso la persona, las existencias, las horas de mano de obra, la versión, los gastos (HU-106) ni las notas de la orden.
- **El detalle de las partidas solo va si la cotización tiene "Mostrar detalle al cliente"**, y **nunca con su valor de referencia** (`valorRef` es interno, ADR-0040): el detalle sale con descripción y cantidad.
- Sirve en cualquier estado. Un **borrador** se puede mostrar y compartir (el PDF dice "BORRADOR"), pero solo una **enviada** (o con dudas) se aprueba: el servidor ya lo exige en `POST /quote/response`. Una cotización **vencida** se ve, avisa que venció y no ofrece aprobar (el servidor tampoco la deja aprobar, HU-27).
- **Permiso:** `quote:read`, que tienen los 3 roles (V-05). La orden se busca con el `taller_id` de la sesión: de otro taller, 404 (CA-10).

### Aprobar o no aprobar
- **Se reutiliza `POST /orders/{id}/quote/response` con `canal: "mostrador"`** (HU-29). La ficha del plan lo llamaba `/quote/decision`; esa ruta no existe y no hacía falta crearla. Es idempotente por estado: la segunda respuesta responde 422 `cotizacion_no_enviada`.
- **"No aprobó" lleva un motivo opcional** (hasta 200 letras). Va en la **bitácora** (`cotizacion.respuesta`, campo `motivo`) y no en la cotización: no hace falta migración y no vuelve a aparecer en pantallas del cliente. Es texto libre del taller; si un cliente pide el borrado de sus datos (HU-87), la bitácora es inmutable y puede conservarlo (límite ya anotado para AQ-10).
- **Autorización de datos en el mostrador (HU-86):** `autorizacion` trae el estado del cliente y el texto que se guarda como prueba. Si está **pendiente**, antes de registrar la aprobación la pantalla pide "El cliente autoriza y aprueba" o "No autoriza, aprueba igual", llama a `POST /customers/{id}/consent` y solo si eso salió bien registra la respuesta. No autorizar **no impide** aprobar (la ley pide autorización para tratar datos, no para comprar), pero queda registrado y el cliente no recibe avisos automáticos (HU-95). El texto lo pone la raíz de composición con la misma constante que usa el servidor al guardar la prueba, para que lo mostrado y lo guardado no se separen.

### El PDF
- **`GET /orders/{id}/quote.pdf`** responde `application/pdf`, `Content-Disposition: attachment; filename="cotizacion-OS-0044.pdf"` y `Cache-Control: no-store` (lleva el nombre del cliente). Tamaño **carta**, español, pesos sin decimales (`$ 1.234.000`), fecha en `America/Bogota`. Lleva el nombre del taller, la orden y la cotización, el vehículo y la placa, el cliente, las partidas (con detalle solo si está encendido), el descuento por línea, el IVA, el total, la vigencia y un espacio para la firma y la fecha. Numera las hojas y no parte un ítem entre dos.
- **Sin logo:** el sistema todavía no tiene dónde subirlo; el encabezado lleva el nombre del taller (la ficha decía "logo o el nombre"). Cuando exista el logo (después de la entrega), entra en el mismo `Renderer`.
- **Decisión: sin librería de PDF.** El generador es propio y mínimo (`internal/platform/pdfx`, unos 300 renglones con pruebas): texto, líneas y recuadros con las fuentes estándar Helvetica y Helvetica-Bold (que el lector trae, sin incrustar) codificadas en WinAnsi, que cubre el español. La maqueta de la cotización vive en `internal/quotes/adapters/out/pdf`, detrás del puerto `Renderer` de `quotes/app`. Un carácter que no existe en WinAnsi (por ejemplo un emoji en el nombre de un cliente) sale como `?`.

### Compartir PDF en el celular
- El navegador baja el PDF (`GET quote.pdf`) como `Blob` y, si `navigator.canShare({ files })` lo permite, llama a **`navigator.share({ files: [pdf] })`**: se abre el menú de compartir del sistema y la persona escoge la app. **No es una integración con WhatsApp.** Si no hay soporte (escritorio, navegadores viejos), se **descarga** el archivo y la pantalla lo dice. Cerrar el menú sin elegir no es un error.
- La página que muestra la cotización es la ruta `/panel/ordenes/:id/cliente`, **fuera del menú del panel** (pantalla completa, letra grande), pero con la misma sesión: sigue exigiendo entrar.
- **Para volver al panel hay que mantener presionado "Volver"** (0,7 s; también con Enter o espacio sostenidos). Un toque corto solo avisa cómo se sale. Así el cliente no navega el panel por error con el celular en la mano.

## Alternativas descartadas
- **`github.com/go-pdf/fpdf` (MIT), que proponía el plan:** resuelve más cosas (fuentes incrustadas, imágenes) de las que hoy hacen falta. Es una dependencia de **runtime** nueva (`docs/02` §9) y no se verificó su mantenimiento actual (`Sin dato`). Con solo texto y líneas, el generador propio es más chico que justificar la librería y no suma superficie para Snyk ni govulncheck.
- **Imprimir a PDF desde el navegador:** no sirve para compartir un archivo desde el celular sin pasos manuales, y el resultado cambia con el navegador.
- **Enlace público para el cliente:** queda para después (V-10); pide tokens de un solo uso, vencimiento y una vista sin sesión, que son más seguridad que revisar antes de la entrega.
- **Guardar el motivo en la cotización:** exigía una migración y mostrar el campo en otras pantallas; la bitácora ya es inmutable y por orden.

## Consecuencias
- Cualquier campo nuevo de la cotización es **interno por defecto**: para que el cliente lo vea hay que agregarlo a propósito a `ClientView`, al DTO y al PDF (con su prueba).
- El PDF no tiene logo ni tipografía de marca (fuentes estándar). Los nombres con caracteres fuera de cp1252 se degradan a `?`.
- Los anchos de las fuentes salen de una tabla AFM estándar (95 valores por fuente): las líneas se parten bien para texto normal, no es una medición tipográfica exacta.
- La aprobación en el mostrador no verifica quién es el cliente: queda `canal = mostrador`, la hora y el usuario del taller que tocó el botón (igual que "Aprobó en el mostrador" de antes).

## Evidencia
- Dominio: `TestClientView_DetailOnlyWhenShown` (el detalle solo con "Mostrar detalle", sin valor de referencia), `TestClientView_AmountsAndState` y `TestClientView_ItemByItem`.
- HTTP (`internal/server/clientview_test.go`): lo que ve el cliente y lo que no (ni gastos, ni notas, ni referencias, ni versión, en JSON y en PDF), PDF con cabeceras y total, borrador, motivo en la bitácora, autorización en el mostrador, 404 entre talleres, 401 sin sesión y el mecánico que sí la ve.
- PDF: `pdfx_test.go` (estructura del archivo, tabla de referencias con los desplazamientos reales, WinAnsi, anchos) y `render_test.go` (contenido, detalle oculto, paginación, borrador y vencida). Revisado además con `pdftotext` sobre un PDF de 4 hojas.
- Vitest `clientQuote.test.tsx` (pantalla, autorización, motivo, `canShare` con y sin soporte, cancelar el menú, "Volver" sostenido) y e2e `orden.spec.ts` (mostrar → autorizar y aprobar → "Volver" sostenido → la orden pasa a En reparación).
