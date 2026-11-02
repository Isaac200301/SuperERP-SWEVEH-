---
name: sweveh-frontend-react
description: Frontend de SWEVEH con React, TypeScript estricto, Vite y Tailwind: estructura por features, cliente generado desde OpenAPI, TanStack Query, formularios RHF+Zod, manejo de 401/403/409, PWA "Piso" móvil, accesibilidad, UI en español de Colombia y seguridad en el navegador. Úsala al crear o modificar cualquier pantalla o componente.
---

# Frontend React + TypeScript + Vite + Tailwind

Stack del equipo (ADR-03/04): React, TypeScript, Vite y Tailwind. Librerías de apoyo (TanStack Query, React Hook Form + Zod, React Router, Vitest, Playwright) son `Propuesta`. Referencias: https://react.dev · https://vite.dev · https://tailwindcss.com/docs · https://tanstack.com/query/latest · https://www.w3.org/WAI/WCAG22/quickref/

Superficies (`docs/04`): **panel** (escritorio, dueño/recepción/administración), **piso** (PWA móvil para el mecánico, sin costos ni KPIs de dinero — RF-21), **WhatsApp** (cliente) y **motor** (automatizaciones). Esta skill cubre panel y piso.

## 1. Estructura por features

```
web/src/
├── app/            # rutas, providers, layout, guardas
├── features/
│   ├── orders/     { api/, components/, hooks/, pages/, schemas/, index.ts }
│   ├── quotes/  vehicles/  customers/  inventory/  auth/
├── shared/
│   ├── api/        # cliente generado de OpenAPI + fetch wrapper
│   ├── ui/         # componentes base (Button, Field, Dialog…)
│   ├── lib/        # formatters (COP, fechas), utilidades puras
│   └── a11y/
└── main.tsx
```
- Una feature solo importa de otra por su `index.ts` (Low Coupling, `grasp`). Regla de ESLint (`import/no-restricted-paths`) la hace cumplir.
- Componentes presentacionales puros; la lógica de datos en hooks (`useOrder`, `useTransitionOrder`).

## 2. TypeScript estricto

`tsconfig`: `"strict": true`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`. Sin `any` (ESLint `no-explicit-any` en error); `unknown` + validación en bordes. Uniones discriminadas para estados de UI y errores; `switch` exhaustivo con `never`.

```ts
type OrderState = 'porRecibir' | 'recibido' | 'enDiagnostico' | 'esperandoAprobacion'
                | 'enReparacion' | 'listo' | 'entregado' | 'cancelada';
```
(Valores de ejemplo; la fuente son los 7 estados + Cancelada de `docs/02` §10 y el cliente generado.)

## 3. API: cliente generado, nunca tipos a mano

- `make gen` genera el cliente y tipos desde `api/openapi.yaml`. No dupliques DTO a mano.
- Un wrapper de `fetch` con `credentials: 'include'`, encabezado anti-CSRF personalizado, manejo uniforme de `application/problem+json` y propagación de `ETag`.
- **Cookies HttpOnly:** el JS no ve el token. No guardes nada sensible en `localStorage`.
- 401 → un único intento de refresh (el servidor rota el refresh) y reintento; si falla, ir a login conservando la ruta destino.
- 403 → pantalla/estado "sin permiso" (el servidor decide; el front solo oculta controles).
- 404 en recurso → "no encontrado" (no revela si es de otro taller).
- **409 (versión):** mostrar aviso claro ("Otra persona cambió esta orden"), recargar datos y permitir reaplicar. Usa `If-Match` con el `ETag` recibido.
- 422/400 → errores de campo mapeados al formulario.

## 4. Datos con TanStack Query

```ts
export function useOrder(id: string) {
  return useQuery({ queryKey: ['orders', id], queryFn: () => api.orders.get(id), staleTime: 15_000 });
}

export function useTransitionOrder(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { to: OrderState; etag: string }) => api.orders.transition(id, v.to, { ifMatch: v.etag }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['orders'] }),
  });
}
```
- Claves jerárquicas; invalidar por prefijo. Actualizaciones optimistas solo donde el riesgo de conflicto sea bajo (no para dinero ni inventario).
- Estados explícitos: `cargando`, `error` (con reintento), `vacío` (con acción sugerida), `éxito`.

## 5. Formularios: React Hook Form + Zod

Un esquema Zod por formulario, **alineado con el contrato** (misma regla que el servidor; el servidor sigue siendo la autoridad).

```ts
export const vehicleSchema = z.object({
  tipo: z.enum(['carro', 'moto']),
  placa: z.string().trim().toUpperCase(),
  cilindrajeCc: z.coerce.number().int().positive('Cilindraje en cc'),
  turbo: z.boolean(),
}).superRefine((v, ctx) => {
  const re = v.tipo === 'carro' ? /^[A-Z]{3}\d{3}$/ : /^[A-Z]{3}\d{2}[A-Z]?$/; // moto: ABC12D y formato antiguo ABC12 (Q-01 por confirmar con RUNT)
  if (!re.test(v.placa)) ctx.addIssue({ code: 'custom', path: ['placa'], message: 'Placa inválida' });
});
```
- Sin VIN. Motor con `cilindraje_cc` y `turbo` obligatorios (puede quedar "pendiente" hasta Recibido, Q-23).
- Mensajes de error en español claro, junto al campo, con `aria-describedby`.
- Celular colombiano de 10 dígitos; enviar al servidor en E.164 (`+57…`).

## 6. Idioma, formato y localización (docs/08)

- UI en **español de Colombia**; tuteo/ustedeo consistente (definir en glosario).
- Moneda: COP **sin decimales** — `new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 })`.
- Fechas: mostrar en `America/Bogota` con `Intl.DateTimeFormat('es-CO', { timeZone: 'America/Bogota' })`; el servidor entrega UTC.
- Dinero como entero de pesos (nunca `float`).
- Textos en un diccionario (`shared/i18n/es-CO.ts`) aunque haya un solo idioma; evita cadenas dispersas.

## 7. Tailwind y diseño

- Tokens del tema en `tailwind.config` (colores, espaciado, tipografías); no valores mágicos repetidos.
- Componentes base en `shared/ui` con variantes (`cva` o similar); clases ordenadas con el plugin oficial de Prettier.
- Mobile-first. **Piso:** objetivo táctil ≥ 44×44 px, contraste alto, texto legible con luz de taller, formularios cortos, acciones principales al alcance del pulgar, funciona con mala conexión.
- Modo oscuro solo si el equipo lo pide (no está en el alcance).

## 8. PWA "Piso" (propuesta)

Instalable (manifest + service worker con Workbox/`vite-plugin-pwa`), cacheo de la app shell, **sin cachear respuestas con datos sensibles** ni tokens, cola de acciones sin conexión **solo si el equipo la aprueba** (afecta concurrencia y conflictos; hoy `Sin dato`/pregunta abierta). Cámara para fotos del vehículo: pedir permiso en el momento, comprimir antes de subir, mostrar progreso y reintentos.

## 9. Accesibilidad (WCAG 2.2 AA como objetivo `Propuesta`)

HTML semántico, etiquetas asociadas, foco visible y gestionado en diálogos, orden de tabulación lógico, `aria-live` para avisos, no depender solo del color, contraste ≥ 4,5:1, textos alternativos de fotos, pruebas con axe y recorrido por teclado.

## 10. Seguridad en el navegador (`sweveh-seguridad-auth`)

Sin `dangerouslySetInnerHTML` (si es inevitable, sanitizar con DOMPurify y justificar); URLs de usuario validadas; sin `eval`/scripts en línea (CSP estricta); dependencias auditadas (`npm audit`/Snyk); no exponer secretos en variables `VITE_*` (se incrustan en el paquete: solo valores públicos); enlaces externos con `rel="noopener noreferrer"`.

## 11. Rendimiento

División de código por ruta (`React.lazy`), listas largas virtualizadas, imágenes redimensionadas, evitar re-renderizados (estado local, selectores, `memo` solo con medición). Presupuesto de bundle inicial como `Propuesta` (medir con `vite-bundle-visualizer`).

## 12. Checklist

- [ ] Tipos del cliente generado; sin `any`.
- [ ] Estados de carga/error/vacío/éxito.
- [ ] 401/403/404/409/422 manejados.
- [ ] Formulario con Zod alineado al contrato y mensajes en español.
- [ ] COP sin decimales, fechas en America/Bogota.
- [ ] Accesible por teclado y con etiquetas; piso con objetivos táctiles grandes.
- [ ] Pruebas Vitest + (si es parte de la demo) Playwright.
