import { expect, test, type Page } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * El panel en el celular (mockups móviles v1; barra de la entrega del 27-nov): Hoy · Órdenes · Asistente · Taller,
 * hoja "Nueva orden" desde Órdenes, orden en pestañas y "Más". Sin WhatsApp (V-01) a menos que E2E_WHATSAPP=true.
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
  await expect(page.getByRole('heading', { name: 'Tus números' })).toBeVisible(); // Figma 04
  for (const tab of ['Hoy', 'Órdenes', 'Asistente', 'Taller']) await expect(barra.getByRole('link', { name: tab })).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-01-hoy');

  // Asistente: por ahora el chat vacío (R8).
  await barra.getByRole('link', { name: 'Asistente' }).click();
  await expect(page.getByRole('heading', { name: 'Asistente' })).toBeVisible();
  await sinDesborde(page);

  // Nueva orden desde Órdenes: la placa no ha venido, así que va a la pantalla completa.
  await barra.getByRole('link', { name: 'Órdenes' }).click();
  await page.getByRole('button', { name: 'Nueva orden' }).click();
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
  await expect(page.getByRole('tab', { name: 'WhatsApp' })).toHaveCount(0); // V-01
  for (const [tab, name] of [['Cotización', 'cotizacion'], ['Historial', 'historial']] as const) {
    await page.getByRole('tab', { name: tab }).click();
    await sinDesborde(page);
    await shot(page, `movil-03-orden-${name}`);
  }

  // Órdenes en tarjetas con chips.
  await page.getByRole('link', { name: 'Volver a órdenes' }).click();
  await expect(page.getByRole('heading', { name: 'Órdenes' })).toBeVisible();
  await expect(page.getByText(placa).first()).toBeVisible();
  await expect(page.getByText('Sigue: Recibir el vehículo')).toBeVisible(); // Figma 05
  await expect(page.getByRole('table')).toHaveCount(0);
  await sinDesborde(page);
  await shot(page, 'movil-04-ordenes');

  // La misma placa en la hoja ya tiene orden abierta: no deja duplicarla.
  await page.getByRole('button', { name: 'Nueva orden' }).click();
  await page.getByRole('dialog', { name: 'Nueva orden' }).getByLabel('Placa').fill(placa);
  await expect(page.getByText(/Ya tiene abierta la OS-/)).toBeVisible();
  await shot(page, 'movil-05-hoja-nueva-orden');
  await page.keyboard.press('Escape');

  // Por revisar es de WhatsApp (V-01): sin la bandera no está en la barra.
  await expect(barra.getByRole('link', { name: 'Revisar' })).toHaveCount(0);

  // Taller (Figma 12): las secciones con su subtítulo y el resumen de Mi taller con el interruptor del inventario.
  await barra.getByRole('link', { name: 'Taller' }).click();
  await expect(page.getByRole('link', { name: /^Servicios/ })).toContainText('Tu lista de precios');
  await expect(page.getByRole('heading', { name: 'Mi taller' })).toBeVisible();
  await expect(page.getByRole('switch', { name: /Llevo el inventario en otro programa/ })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Avanzado' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cerrar sesión' })).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-07-taller');

  // Servicios en el celular: la tabla se vuelve lista y no se desborda (CA-11).
  await page.getByRole('link', { name: /^Servicios/ }).click();
  await expect(page.getByRole('heading', { name: 'Servicios', exact: true }).first()).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-06-servicios');
  await barra.getByRole('link', { name: 'Taller' }).click();

  // Clientes: la ficha en una hoja inferior.
  await page.getByRole('link', { name: 'Clientes' }).click();
  await page.getByRole('button', { name: /Laura Cliente/ }).click();
  const ficha = page.getByRole('dialog', { name: 'Laura Cliente' });
  await expect(ficha.getByText(placa)).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-08-cliente');
  await page.keyboard.press('Escape');

  // Automatizaciones son de WhatsApp (V-01): no salen en Taller.
  await barra.getByRole('link', { name: 'Taller' }).click();
  await expect(page.getByRole('link', { name: 'Automatizaciones' })).toHaveCount(0);

  // Piso: la orden por recibir va arriba y se recibe a pantalla completa.
  await page.getByRole('link', { name: 'Piso' }).click();
  await expect(page.getByText('LO SIGUIENTE')).toBeVisible();
  await sinDesborde(page);
  await shot(page, 'movil-10-piso');
  await page.getByRole('link', { name: 'Recibir vehículo' }).click();
  await expect(page.getByRole('heading', { name: 'Recibir' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Mover la orden' })).toContainText('Pasar a Recibido');
  await sinDesborde(page);
  await shot(page, 'movil-11-recibir');
});
