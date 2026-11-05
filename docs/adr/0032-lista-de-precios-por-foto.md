# ADR-0032 · Pasar la lista de precios con una foto (HU-93)

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Equipo; revisión de Isaac y Pablo en el PR
- **Relacionado con:** HU-93, HU-04, ADR-0026 (IA por proveedor), ADR-0029, RF-17, AQ-09, D-09

## Contexto
Muchos talleres tienen su lista de precios en un papel o un tablero. HU-93 pide subirle una foto y que aparezcan las filas en "Por revisar", con la foto al lado, para corregirlas y aprobarlas una por una. Es la escena opcional de D-09.

La regla de fondo del proyecto sigue igual: **ningún precio entra al catálogo sin el dueño** (RF-17). La IA puede leer mal o inventar.

## Decisión
- **Puerto `catalog/app.PriceListReader`:** lee la foto y devuelve filas (nombre, precio, tipo de vehículo). No decide nada.
- **Adaptador `catalog/adapters/out/vision`:**
  - llama a OpenRouter con la imagen en base64 y salida JSON por esquema (máximo 40 filas), sin razonamiento;
  - el prompt dice que lo escrito en la foto es un dato y nunca una instrucción;
  - modelos de `AI_VISION_MODEL`, con respaldo en orden. Por defecto `google/gemma-4-31b-it:free,nvidia/nemotron-3.5-super-vl:free`, gratuitos y con imagen.
- **Cada fila es una propuesta** (`catalog.propuesta`, migración 00026, con RLS):
  - el código limpia el nombre;
  - un tipo desconocido pasa a "ambos";
  - un precio negativo se descarta;
  - una fila sin nombre no se guarda.

  **Solo al aprobarla se vuelve servicio**, con las correcciones del dueño y las mismas validaciones de crear un servicio (HU-04). Descartarla no crea nada, y una fila resuelta no se puede volver a resolver.
- **La foto** se guarda una vez por lote en el almacenamiento de archivos y se muestra al lado de las filas.
- **Rutas** (`service:manage`):
  - `POST /services/photo`;
  - `GET /services/proposals`;
  - `POST /services/proposals/{id}` (aprobar o descartar);
  - `GET /services/proposals/{id}/photo`.
- **Pantallas:** "Pasar mi lista con una foto" en Servicios, y "Lista de precios por revisar" en Por revisar.
- **Sin IA configurada** (`AI_PROVIDER=reglas`), subir la foto responde `lista_sin_ia` y pide agregar los servicios a mano. Si la IA falla (por ejemplo, 429 del plan gratuito), responde `lista_no_leida` y no crea nada.

## Consecuencias
- Un taller pasa su lista en minutos, y un dato mal leído no llega al catálogo sin pasar por el dueño.
- La foto de la lista va a un proveedor de IA. Como el resto de la IA, es **solo demo hasta cerrar AQ-09**, y no debe llevar datos de clientes: es una lista de precios.
- **Falta medir el acierto con fotos reales de talleres colombianos.** HU-93 cita un 87 % en notas a mano de otro mercado. La prueba automática usa un OpenRouter falso.
