import { expect, test, type Page } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * El panel en el celular (mockups móviles v1): barra inferior, hoja "Nueva orden", orden en pestañas y "Más".
 * CA-11: ninguna pantalla se desborda de lado. Datos Ficticio. Deja capturas en SWEVEH_E2E_OUT.
 */
const uniq = Date.now().toString().slice(-6);
const celular = `311${uniq}1`;
const placa = `M${String.fromCharCode(65 + (Number(uniq) % 26))}V${uniq.slice(-3)}`;

async function sinDesborde(page: Page) {
  const { ancho, vista } = await page.evaluate(() => ({ ancho: document.documentElement.scrollWidth, vista: window.innerWidth }));
  expect(ancho, 'la pantalla no debe desplazarse de lado').toBeLessThanOrEqual(vista);
}

test('el dueño maneja el taller desde el celular', async ({ page }) => {
  await registerWorkshop(page, celular);
  const barra = page.getByRole('navigation', { name: 'Secciones' });

  // Hoy, con la barra inferior del dueño.
  await barra.getByRole('link', { name: 'Hoy' }).click();
  await expect(page.getByRole('heading', { name: /Buen día|Buenas tardes|Buenas noches/ })).toBeVisible();
  for (const tab of ['Hoy', 'Órdenes', 'Taller']) await expect(barra.getByRole('link', { name: tab })).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-01-hoy');

  // Nueva orden desde la hoja: la placa no ha venido, así que va a la pantalla completa.
  await barra.getByRole('button', { name: 'Nueva orden' }).click();
  const hoja = page.getByRole('dialog', { name: 'Nueva orden' });
  await hoja.getByLabel('Placa').fill(placa);
  await hoja.getByRole('button', { name: 'Registrar vehículo nuevo' }).click();
  await expect(page.getByLabel('Placa')).toHaveValue(placa);
  await page.getByLabel('Celular del cliente').fill(`300${uniq}3`);
  await page.getByLabel('Nombre', { exact: true }).fill('Laura Cliente');
  await page.getByLabel('Combustible', { exact: true }).selectOption('gasolina');
  await page.getByLabel('Caja', { exact: true }).selectOption('manual');
  await page.getByLabel('Cilindraje', { exact: true }).fill('1600');
  await page.getByRole('radio', { name: 'Atmosférico' }).click();
  await page.getByLabel('Lo que dice el cliente').fill('Chilla al frenar');
  await sinDesborde(page);
  await page.getByRole('button', { name: 'Abrir orden' }).click();

  // La orden en pestañas, sin barra inferior y con la acción principal abajo.
  await expect(page.getByRole('tab', { name: 'Estado' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('region', { name: 'Mover la orden' })).toBeVisible();
  await expect(barra).toHaveCount(0);
  await sinDesborde(page);
  await shot(page, 'movil-02-orden-estado');
  for (const [tab, name] of [['Cotización', 'cotizacion'], ['WhatsApp', 'whatsapp'], ['Historial', 'historial']] as const) {
    await page.getByRole('tab', { name: tab }).click();
    await sinDesborde(page);
    await shot(page, `movil-03-orden-${name}`);
  }

  // Órdenes en tarjetas con chips.
  await page.getByRole('link', { name: 'Volver a órdenes' }).click();
  await expect(page.getByRole('heading', { name: 'Órdenes' })).toBeVisible();
  await expect(page.getByText(placa).first()).toBeVisible();
  await expect(page.getByRole('table')).toHaveCount(0);
  await sinDesborde(page);
  await shot(page, 'movil-04-ordenes');

  // La misma placa en la hoja ya tiene orden abierta: no deja duplicarla.
  await barra.getByRole('button', { name: 'Nueva orden' }).click();
  await page.getByRole('dialog', { name: 'Nueva orden' }).getByLabel('Placa').fill(placa);
  await expect(page.getByText(/Ya tiene abierta la OS-/)).toBeVisible();
  await shot(page, 'movil-05-hoja-nueva-orden');
  await page.keyboard.press('Escape');

  // Taller / Más: las demás secciones y cerrar sesión.
  await barra.getByRole('link', { name: 'Taller' }).click();
  await expect(page.getByRole('link', { name: 'Mi taller' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cerrar sesión' })).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-06-mas');
});
