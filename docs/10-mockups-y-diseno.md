# 10 · Mockups y reglas de diseño de la interfaz

> Fuente visual del MVP para el agente constructor. Versión 1 (2026-11-01).
> **Origen:** `Propuesta` (diseño de Claude a pedido de Isaac y Pablo). La paleta y el logo vienen de los artefactos de WEVEH aprobados por el equipo. Los patrones de tablero (sidebar en píldoras, tarjetas pastel, acento lima) salen de las 3 imágenes de referencia que el equipo compartió (`docs/mockups/referencias/`).
> **Canvas vivo, con las 11 pantallas lado a lado:** https://claude.ai/artifact/Yc493iPY4yenjM9B3JYzSe
> **Precedencia (R-01 de `docs/02`):** si una pantalla contradice una HU, **manda la HU**. El mockup muestra la intención visual; no agrega alcance. Si encuentras una contradicción, regístrala en `docs/07`.

---

## 1. Cómo usar este documento

1. Antes de construir la UI de una HU, busca su pantalla en la **tabla de trazabilidad** (sección 6) y abre el `.dc.html` correspondiente en `docs/mockups/`.
2. Copia **tokens y patrones**, no el HTML. Los mockups usan estilos en línea para que se puedan leer solos; en el código van como tokens de Tailwind (sección 2.4) y componentes de React.
3. Los textos de los mockups son **textos de referencia en español colombiano**: úsalos como primera versión del copy (van a `i18n/es-CO`), salvo los marcados `[ENTRE CORCHETES]`, que son pendientes (sección 8).
4. Los datos (Laura, Andrés, KLM482, $128.000…) son del **taller ficticio "Servitaller El Pistón"** de `docs/03`. Las filas de "Taller ejemplo B/C/D" del Admin son relleno visual, no datos.

### Cómo abrir los archivos

- Cada `*.dc.html` es un HTML estático. Se abre directo en el navegador; los estilos y el contenido se ven aunque falte `support.js` (ese script solo existe dentro del canvas publicado).
- `canvas.json` dice el tamaño y el orden de cada tablero.
- Si tienes acceso web, abre el canvas: es la versión de referencia.

---

## 2. Sistema visual

### 2.1 Tokens de color (tema claro: panel, piso, landing)

| Token | Hex | Uso |
|---|---|---|
| `primary` | `#005F54` | Botón principal, enlaces, estado seleccionado, ícono de la app |
| `primary-pressed` | `#004A42` | Hover/pressed del primario |
| `primary-tint` | `#E2F1EE` | Fondo de selección, chips activos, tarjeta resaltada |
| `text` | `#0F1F1D` | Texto principal |
| `text-2` | `#4C5F5C` | Texto secundario |
| `text-3` | `#667774` | Ayudas, metadatos (no para texto pequeño sobre gris) |
| `surface` | `#F4F7F6` | Tarjetas y fondos de grupo |
| `surface-2` | `#E8EEEC` | Fondo de app del panel (shell) |
| `line` | `#D3DCDA` | Bordes y divisores |
| `ok` / `ok-tint` / `ok-ink` | `#1E7A46` / `#E5F4EA` / `#17693A` | Estado correcto (✓ + palabra) |
| `warn` / `warn-tint` / `warn-ink` | `#F9E03F` / `#FFF5C2` / `#6F5200` | Atención (triángulo + palabra) |
| `err` / `err-tint` / `err-ink` | `#C9302C` / `#FCE9E7` / `#A5231F` | Error / bloqueo (✕ o ! + palabra) |
| `accent-nav` | `#DDF47B` | **Solo** ítem activo de navegación y el valor resaltado de una gráfica |
| `kpi-peach` / `kpi-lilac` / `kpi-aqua` | `#FFF1E2` / `#EEE9FD` / `#DDF5F1` | Fondos de tarjetas KPI (decorativos, sin significado) |
| `plate` | `#F9E03F` con borde `#0F1F1D` | Placa del vehículo, tipografía mono |
| `brand-amber` | `#FFB000` | **Solo** el punto del logo |

### 2.2 Tema oscuro (Admin del ERP y franjas oscuras de la landing)

| Token | Hex |
|---|---|
| `bg` | `#061211` |
| `card` | `#0E1E1C`, borde `#22403C` |
| `primary` | `#3CCFB6` (texto sobre él: `#03211D`) |
| `text` / `text-2` | `#EAF3F1` / `#A3B8B4` |
| `link` | `#5FDCC8` |

### 2.3 Tipografía y forma

- **Manrope** (500–800) para todo el texto; **JetBrains Mono** para placas, números de orden (OS-0043), cifras de KPI, horas y montos en tablas.
- Radios: tarjetas 20–24 px; botones 12–14 px; chips, pills y navegación 999 px.
- Logo: cuadro redondeado con la W `M17.5 27 L24.75 46 L32 33 L39.25 46 L46.5 27` (trazo 6, puntas redondas) y el punto ámbar en (32, 18), r 5, en un viewBox de 64.

### 2.4 Propuesta de `tailwind.config` (extracto)

```ts
// tailwind.config.ts: extiende, no reemplaces la paleta base
export default {
  theme: {
    extend: {
      colors: {
        primary: { DEFAULT: '#005F54', pressed: '#004A42', tint: '#E2F1EE' },
        ink: { DEFAULT: '#0F1F1D', 2: '#4C5F5C', 3: '#667774' },
        surface: { DEFAULT: '#F4F7F6', 2: '#E8EEEC' },
        line: '#D3DCDA',
        ok: { DEFAULT: '#1E7A46', tint: '#E5F4EA', ink: '#17693A' },
        warn: { DEFAULT: '#F9E03F', tint: '#FFF5C2', ink: '#6F5200' },
        err: { DEFAULT: '#C9302C', tint: '#FCE9E7', ink: '#A5231F' },
        nav: '#DDF47B',
        kpi: { peach: '#FFF1E2', lilac: '#EEE9FD', aqua: '#DDF5F1' },
        plate: '#F9E03F',
        amber: '#FFB000',
      },
      fontFamily: { sans: ['Manrope', 'system-ui', 'sans-serif'], mono: ['"JetBrains Mono"', 'monospace'] },
      borderRadius: { card: '24px', btn: '14px' },
    },
  },
};
```

---

## 3. Reglas de diseño (obligatorias)

| # | Regla | Por qué |
|---|---|---|
| DR-01 | **Todo estado lleva ícono + palabra**, nunca solo color (✓ Normal, ▲ Esperando aprobación, ✕ Cancelada). | WCAG 1.4.1: el color no puede ser el único medio. Además, en el taller hay pantallas al sol y gente daltónica. |
| DR-02 | **Lima (`accent-nav`) ≠ limón (`warn`).** El lima solo marca dónde estás; el limón siempre significa "atención" y va con su triángulo. | En las referencias el lima es acento de marca. En WEVEH el amarillo es un estado. Mezclarlos haría que el dueño lea "alerta" donde no la hay. |
| DR-03 | **Ámbar solo en el punto del logo.** | Es la firma de WEVEH Group. Si se usa en la UI compite con `warn`. |
| DR-04 | **Áreas táctiles ≥ 44 px** en el panel y **≥ 52 px** en el piso (manos con grasa o guantes, el celular en una mano). | WCAG 2.5.8 pide un mínimo de 24 px y Apple HIG recomienda 44 pt. En el piso se sube a 52 a propósito. |
| DR-05 | **Una acción principal por pantalla** (un botón `primary` relleno); el resto, secundarios o texto. | Ley de Hick: menos opciones con el mismo peso, decisión más rápida. |
| DR-06 | **Lo deshabilitado explica qué falta** (ej. "Falta 1 foto y el motor"), en vez de un botón gris mudo. | NN/g: visibilidad del estado del sistema y prevención de errores. |
| DR-07 | **La IA nunca envía sola.** Todo borrador muestra una etiqueta de origen, de dónde salió cada dato (chips de fuente, HU-64) y botones Enviar / Editar / Responder yo / Descartar. | RF-17/RF-18 y la regla 9 de `CLAUDE.md`. Esto se ve en `PorRevisar`. |
| DR-08 | **Botones de WhatsApp: máximo 3, máximo 20 caracteres** cada uno. | Límite de Meta para los *interactive reply buttons*. |
| DR-09 | **La placa siempre en estilo placa** (amarillo, borde negro, mono) y sirve para buscar en cualquier pantalla (HU-14). | Así el taller identifica el vehículo, y es el mismo patrón que se aprobó en WEVEH. |
| DR-10 | **El costo de IA no aparece en pantallas del taller** (solo en Admin, rol EQ). | HU-74 es del Administrador del ERP (Q-24, DE-18). |
| DR-11 | **Cada KPI trae "¿Qué significa?"** con explicación en lenguaje del taller (HU-75). | La barrera de entrada es entender los números, no solo verlos. |
| DR-12 | **Contraste ≥ 4,5:1** para texto normal. `text-3` no va sobre `surface-2` en tamaños < 14 px. | WCAG 1.4.3. |
| DR-13 | **Sin emojis en la UI.** Los íconos son SVG con trazo de 2 px, `aria-hidden`, y el texto va al lado. | Se ven igual en cualquier dispositivo y funcionan con lector de pantalla. |
| DR-14 | Interruptores con `role="switch"` y `aria-checked`; segmentados con `radiogroup`; navegación con `aria-current="page"`. | Accesibilidad básica, y los tests de React Testing Library pueden buscar los elementos por rol. |

---

## 4. Patrones por superficie

### 4.1 Landing (`Main.dc.html`)
Orden: hero (promesa + CTA "Empezar prueba gratis" + vista previa del panel y de WhatsApp) → 3 dolores vistos en los talleres (Metro Frenos, Energiteca, taller del amigo) → "Listo en 3 pasos" → 6 funciones → "El asistente propone. Tú apruebas" → plan con precio `[PRECIO]` → preguntas frecuentes → CTA final → pie con línea de Ley 1581. **No dice "ERP"** (DE de posicionamiento): dice "sistema para tu taller".

### 4.2 Registro (`Registro.dc.html`)
Tres pasos con barra de progreso y un **cronómetro visible** (promesa de < 5 min, HU-01..06). El paso 2 usa chips (Carros / Motos / ambos, especialidades, tamaño del equipo). La columna derecha muestra "Esto te dejamos listo", que es lo que HU-04 arma automáticamente. Solo pide lo indispensable (HU-06).

### 4.3 Panel del dueño (`Panel`, `Orden`, `PorRevisar`, `Automatizaciones`)
- Shell gris `surface-2`. Sidebar de píldoras, con el ítem activo en lima. Búsqueda global arriba (HU-14).
- Inicio = "Hoy tienes pendiente" (HU-76) + KPIs pastel + barras por estado (HU-68) + tabla de órdenes recientes.
- La orden muestra un **stepper de los 7 estados** con hora de cada paso, y debajo la conversación de WhatsApp de esa orden (HU-24) y la auditoría (HU-88).
- Automatizaciones: tarjeta con disparador, plantilla, interruptor y "ejecutada N veces esta semana" (HU-46). AU-09 aparece como "Próximamente", sin interruptor.

### 4.4 Piso (PWA del mecánico, 390 × 844)
- Barra inferior: Órdenes / Recibir / Asistente / Yo.
- La tarjeta de la siguiente acción va resaltada (borde primario).
- El checklist de recepción alterna **Carro / Moto** (HU-17/18). Km en mono grande, combustible en segmentado, bloque "**Motor pendiente**" en `warn-tint` con cilindraje y turbo (sin VIN), fotos de los 4 lados con contador y notas que se pueden dictar.

### 4.5 WhatsApp (cliente)
- Burbuja con una etiqueta pequeña en mono de qué automatización la envió (AU-01, AU-02…). Esa etiqueta es **solo del mockup** para trazabilidad; **no va en el mensaje real**.
- La autorización de datos llega **antes** de procesar cualquier cosa (HU-85).
- Los mensajes redactados por IA solo salen después de que el dueño los revisa.

### 4.6 Admin del ERP (tema oscuro)
Mensajes enviados por categoría, costo de IA del mes, costo por conversación y % de salidas descartadas. Debajo: medidor por taller (servicio usado de 1.000 gratis), últimas llamadas a la IA con proveedor, tokens, latencia y resultado (HU-99), número de WhatsApp (HU-101), y Reiniciar demo / Agregar la moto (HU-97/98). **Proveedor y modelo de IA están `[por definir]`.**

### 4.7 Celular (mockups móviles v1, 2026-11-10)
Mockups en `docs/mockups/movil/` (390 × 844). Canvas: https://claude.ai/artifact/KdDbBumybZZvt7Rixtcb7V
- **Responsive, no otra app.** Por debajo de 768 px (`useIsMobile`, `shared/ui/useIsMobile.ts`) el panel usa `MobileShell` con barra inferior; por encima, el panel de escritorio sin cambios. Se elige con JS para que solo una navegación exista en el DOM.
- **Barra inferior por rol** (`features/panel/TabBar.tsx`):
  - **Dueño:** Hoy · Órdenes · + Nueva · Revisar · Taller.
  - **Recepción** (sin `kpi:read`): Órdenes · + Nueva · Revisar · Más.
  - **Mecánico:** Órdenes (Piso) · Yo.

  El activo va en lima con `aria-current` (DR-02, DR-14), y Revisar lleva el contador de HU-66.
- **Patrones:**
  - `BottomSheet`: "Nueva orden" y "¿Qué significa?".
  - `Tabs`: la orden en Estado / Cotización / WhatsApp / Historial.
  - `Chips`: filtros con conteo y desplazamiento horizontal.
  - Acción principal de la orden fija abajo (DR-05), con las demás en "Más acciones".
  - La orden y la recepción ocupan toda la pantalla, sin barra.
- **"Taller" / "Más" / "Yo"** (`/panel/mas`) lista las demás secciones del rol, con la misma lista del menú de escritorio (`features/panel/sections.ts`), y cerrar sesión.
  - **Dueño:** encima de la lista, Números (KPI con "¿Qué significa?" y barras por estado que abren Órdenes filtrada con `?estado=`) e Inventario (existencia contra el mínimo con ícono y palabra, HU-36).
- **Por revisar:** un borrador a la vez, en tres pasos (lo que escribió · de dónde sacó cada dato · respuesta editable), con Enviar fijo abajo. Mismo endpoint y misma lógica que en escritorio (`DraftCard`).
- **Clientes:** tarjetas por cliente y la ficha en hoja inferior (autorización, vehículos con historial, Nueva orden, copia y borrado HU-87). No lleva los chips Carros/Motos del mockup: `GET /customers` no trae los vehículos.
- **No se tomó del mockup** (manda la HU o la decisión del equipo):
  - "Accesorios que deja" en la recepción de moto: Pablo los quitó el 3-nov; van como nota con foto.
  - La pestaña "Asistente" del Piso es HU-77 (T7).
  - "Pregúntale al asistente" en Taller es HU-78 (T7).
  - "Reasignar / cancelar" en el menú de la orden: hoy no hay pantalla para cancelar (HU-26 está en el API).
  - El nombre del mecánico en las tarjetas: la orden solo trae `mecanicoId`.
- **Prueba:** `web/e2e/movil.spec.ts` (Pixel 7) recorre Hoy → Nueva orden → orden en pestañas → Órdenes → Más y verifica que ninguna pantalla se desborde de lado (CA-11).

---

## 5. Inventario de mockups

| # | Archivo | Tamaño | Superficie | Rol |
|---|---|---|---|---|
| 1 | `mockups/Main.dc.html` | 1440 × ~3700 | Landing | Visitante |
| 2 | `mockups/Registro.dc.html` | 1280 × 860 | Panel | DU |
| 3 | `mockups/Panel.dc.html` | 1440 × 1080 | Panel | DU |
| 4 | `mockups/Orden.dc.html` | 1440 × 1080 | Panel | DU / RE |
| 5 | `mockups/PorRevisar.dc.html` | 1440 × 960 | Panel | DU / RE |
| 6 | `mockups/Automatizaciones.dc.html` | 1440 × 960 | Panel | DU |
| 7 | `mockups/PisoOrdenes.dc.html` | 390 × 844 | Piso | ME |
| 8 | `mockups/PisoChecklist.dc.html` | 390 × 844 | Piso | ME / RE |
| 9 | `mockups/WhatsLaura.dc.html` | 390 × 844 | WhatsApp | CL |
| 10 | `mockups/WhatsAndres.dc.html` | 390 × 844 | WhatsApp | CL |
| 11 | `mockups/Admin.dc.html` | 1440 × 960 | Operación | EQ |
| — | `mockups/canvas.json` | — | Disposición del canvas | — |
| — | `mockups/referencias/ref-1..3.png` | — | Imágenes de referencia del equipo (estilo tablero) | — |

---

## 6. Trazabilidad HU → pantalla

| HU | Título (resumen) | Pantalla(s) |
|---|---|---|
| HU-01, 02, 03, 04, 05, 06 | Registro con celular, autorización, tipo de taller, taller armado, primera orden, datos mínimos | 2 Registro (y CTA de 1 Landing) |
| HU-10, 11 | Vehículo por placa, carro o moto; motor con cilindraje y turbo | 4 Orden, 8 Checklist |
| HU-13 | Historial del vehículo | 4 Orden |
| HU-14 | Buscar desde cualquier pantalla | 3 Panel, 7 Piso |
| HU-16, 17, 18 | Orden desde la placa; checklist carro / moto | 7 Piso, 8 Checklist |
| HU-19 | Constancia con fotos y confirmación | 9 WhatsApp Laura |
| HU-20 | Mover la orden por estados | 4 Orden (stepper), 7 Piso |
| HU-21 | Fotos y notas en cualquier momento | 7 Piso (botón cámara), 8 Checklist |
| HU-22 | Mecánico y fecha prometida | 4 Orden |
| HU-23, 81 | Mis órdenes en el celular; solo lo que me toca | 7 Piso |
| HU-24 | Conversación de WhatsApp dentro de la orden | 4 Orden |
| HU-25, 26 | Entregar / cancelar | 4 Orden (botones) |
| HU-27, 28 | Cotización; aprobación por WhatsApp | 4 Orden, 10 WhatsApp Andrés |
| HU-40, 41, 42 | Catálogo, interruptor, editar texto | 6 Automatizaciones |
| HU-43 | Aviso de vehículo listo (AU-03) | 10 WhatsApp Andrés |
| HU-45 | Vehículo sin recoger (Marta, 4 días) | 3 Panel ("Hoy tienes pendiente") |
| HU-46 | Registro de ejecuciones | 6 Automatizaciones (conteo semanal) |
| HU-51, 53 | Escribirle al taller en sus palabras; borrador con datos del taller | 9 WhatsApp Laura, 5 Por revisar |
| HU-55 | Crear la orden cuando llega la placa | 5 Por revisar (casilla "crear orden Por recibir") |
| HU-56 | "¿Ya está mi carro?" con estado real (AU-05) | 10 WhatsApp Andrés |
| HU-59 | Pedir una persona | 10 WhatsApp Andrés ("Hablar con alguien"), 5 Por revisar ("Responder yo") |
| HU-62, 63, 64, 65, 66 | Revisar/editar borrador, fuente de cada dato, tasa de edición, aviso de pendiente | 5 Por revisar |
| HU-68, 69, 70, 71, 72, 73 | KPIs | 3 Panel |
| HU-74 | Medidor de mensajes y costo de IA | 11 Admin |
| HU-75 | ¿Qué significa un número? | 3 Panel |
| HU-76 | Pendientes de hoy | 3 Panel |
| HU-85 | Autorización antes de procesar el mensaje | 9 WhatsApp Laura |
| HU-88 | Quién cambió qué | 4 Orden (auditoría) |
| HU-97, 98, 99, 101 | Reiniciar demo, agregar moto, llamadas a la IA, cambio de número | 11 Admin |

**HU sin pantalla todavía (el agente diseña siguiendo estas reglas y deja captura en el PR):** HU-07, 08, 09, 12, 15, 29–39, 44, 47–49, 52, 54, 57, 58, 60, 61, 67, 77–80, 82–84, 86, 87, 89–96, 100, 102. **HU-50 está cancelada.** Las HU de superficie `B` (motor) no tienen UI propia. Si una de estas necesita una decisión visual nueva (un componente que no existe aquí), es una decisión `Propuesta`: aplícala y menciónala en el resumen de la sesión.

---

## 7. Justificación UX (resumen)

1. **El dueño entra a "Hoy", no a un tablero de números.** El perfil objetivo ("taller del amigo", sin sistema) necesita saber qué hacer ahora. Los KPIs van debajo y traen su explicación. Así se aplica la heurística de NN/g de *match between system and the real world*.
2. **El piso está pensado para el pulgar.** La acción principal va abajo, los objetivos son grandes, se teclea poco (chips, segmentados, cámara, dictado) y la placa funciona como buscador.
3. **WhatsApp es la interfaz del cliente.** El cliente no instala nada. Los botones respetan los límites de Meta, y cada mensaje automático tiene una sola intención con máximo 3 respuestas.
4. **Confianza visible.** Fotos con hora, auditoría, origen de cada dato y aprobación humana de la IA. Esto ataca el problema de confianza entre taller y cliente que el equipo validó en campo, sin venderlo como "IA mágica".
5. **Los datos del negocio y el costo de la plataforma están separados.** El taller ve sus métricas; el costo de IA lo ve solo WEVEH Group en Admin.

---

## 8. Pendientes visibles en los mockups

| Marcador | Dónde | Depende de |
|---|---|---|
| `[PRECIO]`, `[N DÍAS]`, `[N]` | Landing, plan | D-04 (precio y prueba gratis), `Sin dato` |
| `[proveedor A/B] · [modelo]` | Admin | Capa multi-proveedor de IA, sin proveedor elegido |
| Cifras del Admin distintas de 34 mensajes / IA $1.200 | Admin | Son de ejemplo |
| Visibilidad del medidor para el dueño | Admin | Q-24 (hoy: solo EQ) |
| Texto de la política de datos ("Ver política") | WhatsApp Laura | HU-90, Q-25 |
| Logo SWEVEH definitivo | Todas | Se usa la W de WEVEH como provisional |

---

## 9. Fuentes

- W3C, *Understanding SC 2.5.8 Target Size (Minimum)*, WCAG 2.2: https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html
- W3C, *Understanding SC 1.4.1 Use of Color*: https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html
- W3C, *Understanding SC 1.4.3 Contrast (Minimum)*: https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
- Apple, *Human Interface Guidelines: Accessibility* (objetivos de 44 × 44 pt): https://developer.apple.com/design/human-interface-guidelines/accessibility
- Meta for Developers, *Interactive reply buttons messages* (máx. 3 botones, 20 caracteres): https://developers.facebook.com/documentation/business-messaging/whatsapp/messages/interactive-reply-buttons-messages
- Nielsen Norman Group, *10 Usability Heuristics for User Interface Design*: https://www.nngroup.com/articles/ten-usability-heuristics/
- Laws of UX, *Hick's Law*: https://lawsofux.com/hicks-law/
- WAI-ARIA Authoring Practices, *Switch pattern*: https://www.w3.org/WAI/ARIA/apg/patterns/switch/
- Artefactos de WEVEH usados como base (paleta, portada, garaje, prueba de íconos): ver lista en `docs/01-producto-y-contexto.md`, sección 12, y el canvas.
