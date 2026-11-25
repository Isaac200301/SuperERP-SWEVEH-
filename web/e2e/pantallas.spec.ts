import { expect, test, type Page } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * R10 (docs/plan-entrega-viernes.md, V-18): las pantallas de escritorio con diseño propio (Figma D1 Hoy, D2 la orden con su
 * cotización y D3 Servicios y Mi taller) y la columna centrada de las demás, con el menú lateral. CA-11: ninguna se
 * desborda de lado, ni a 1440 ni a 1024 de ancho. Datos Ficticio. Deja capturas en SWEVEH_E2E_OUT.
 */
const uniq = Date.now().toString().slice(-6);
const celular = `312${uniq}1`;
const placa = `P${String.fromCodePoint(65 + (Number(uniq) % 26))}N${uniq.slice(-3)}`;

async function sinDesborde(page: Page, donde: string) {
  const { ancho, vista } = await page.evaluate(() => ({ ancho: document.documentElement.scrollWidth, vista: window.innerWidth }));
  expect(ancho, `${donde} no debe desplazarse de lado`).toBeLessThanOrEqual(vista);
}

/** Abre la ruta a 1440 y a 1024 de ancho, espera lo suyo y revisa que no se desborde; deja una captura a 1440. */
async function enAmbosAnchos(page: Page, ruta: string, listo: () => Promise<void>, captura: string) {
  for (const [width, height] of [
    [1440, 900],
    [1024, 768],
  ] as const) {
    await page.setViewportSize({ width, height });
    await page.goto(ruta);
    await listo();
    await sinDesborde(page, `${ruta} a ${width} px`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(ruta);
  await listo();
  await shot(page, captura);
}

test('las pantallas de escritorio siguen a Figma y no se desbordan', async ({ page }) => {
  await registerWorkshop(page, celular);
  const menu = page.getByRole('navigation', { name: 'Secciones del panel' });

  // D1 · Hoy: saludo, buscador, "Abrir asistente", pendientes y órdenes por estado.
  await page.goto('/panel/tablero');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Buenos días|Buenas tardes|Buenas noches/);
  await expect(page.getByRole('link', { name: 'Abrir asistente' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Hoy tienes pendiente' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Órdenes por estado' })).toBeVisible();
  // El menú de Figma: Hoy, Órdenes, Asistente, Clientes, Servicios y Mi taller (sin Piso ni WhatsApp). El taller del e2e es de
  // "solo yo", así que Equipo no sale (HU-83).
  for (const nombre of ['Hoy', 'Órdenes', 'Asistente', 'Clientes', 'Servicios', 'Mi taller']) {
    await expect(menu.getByRole('link', { name: nombre, exact: true })).toBeVisible();
  }
  await expect(menu.getByRole('link', { name: 'Equipo' })).toHaveCount(0);
  await expect(menu.getByRole('link', { name: 'Piso' })).toHaveCount(0);
  await expect(menu.getByRole('link', { name: 'Por revisar' })).toHaveCount(0); // V-01
  await enAmbosAnchos(page, '/panel/tablero', () => expect(page.getByRole('heading', { name: 'Órdenes por estado' })).toBeVisible(), 'escritorio-d1-hoy');

  // D3 · Servicios y Mi taller: la tabla con "Sin precio · ponlo" y el resumen de Mi taller al lado.
  await page.goto('/panel/servicios');
  await expect(page.getByRole('heading', { name: 'Servicios y Mi taller' })).toBeVisible();
  await expect(page.getByRole('table', { name: 'Servicios' })).toBeVisible();
  await expect(page.getByText('Pasar mi lista con una foto')).toHaveCount(0); // V-01: la lista por foto es de WhatsApp
  await expect(page.getByRole('switch', { name: /Llevo el inventario en otro programa/ })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Avanzado' })).toBeVisible();
  await page.getByRole('button', { name: '+ Agregar servicio' }).click();
  await page.getByLabel('Nombre', { exact: true }).fill('Frenos');
  await page.getByRole('button', { name: 'Agregar servicio', exact: true }).click();
  await page.getByRole('button', { name: 'Poner el precio de Frenos' }).click();
  await page.getByLabel('Precio de Frenos').fill('180000');
  await page.getByRole('button', { name: 'Guardar', exact: true }).click();
  await expect(page.getByText('$ 180.000')).toBeVisible();
  await enAmbosAnchos(page, '/panel/servicios', () => expect(page.getByRole('table', { name: 'Servicios' })).toBeVisible(), 'escritorio-d3-servicios');

  // Las demás van como la columna centrada, con el mismo menú.
  const demas: [string, string][] = [
    ['/panel/ordenes', 'Órdenes'],
    ['/panel/clientes', 'Clientes'],
    ['/panel/equipo', 'Equipo'],
    ['/panel/taller', 'Mi taller'],
    ['/panel/asistente', 'Asistente'],
  ];
  for (const [ruta, titulo] of demas) {
    await enAmbosAnchos(
      page,
      ruta,
      () => expect(page.getByRole('heading', { name: titulo, exact: true }).first()).toBeVisible(),
      `escritorio-${titulo.toLowerCase().replace(' ', '-')}`,
    );
  }

  // D2 · La orden: encabezado, fichas de estados, la cotización a la izquierda y lo gastado y el asistente a la derecha.
  await page.goto('/panel/ordenes/nueva');
  await page.getByLabel('Placa').fill(placa);
  await page.getByLabel('Celular del cliente').fill(`300${uniq}2`);
  await page.getByLabel('Nombre', { exact: true }).fill('Laura Cliente');
  await page.getByLabel('Combustible', { exact: true }).selectOption('gasolina');
  await page.getByLabel('Caja', { exact: true }).selectOption('manual');
  await page.getByLabel('Cilindraje', { exact: true }).fill('1600');
  await page.getByRole('radio', { name: 'Atmosférico' }).click();
  await page.getByLabel('Lo que dice el cliente').fill('Chilla al frenar');
  await page.getByRole('button', { name: 'Abrir orden' }).click();
  await expect(page.getByRole('heading', { name: /OS-/ })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Volver a órdenes' })).toBeVisible();
  await expect(page.locator('[aria-current="step"]')).toContainText('Por recibir');
  await expect(page.getByRole('heading', { name: 'Asistente de esta orden' })).toBeVisible();
  await enAmbosAnchos(
    page,
    new URL(page.url()).pathname,
    () => expect(page.getByRole('heading', { name: 'Asistente de esta orden' })).toBeVisible(),
    'escritorio-d2-orden',
  );
});
