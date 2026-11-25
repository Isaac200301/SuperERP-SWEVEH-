import { expect, test } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * La orden de punta a punta como la usa un taller real (lo que Pablo probó a mano el 3-nov-2026): registro →
 * parámetros → repuesto → orden con ficha → recepción con ítems y fotos → diagnóstico con mano de obra y fotos →
 * cotización con servicio sin precio, repuesto × 2 y horas → aprobar → devolver → esperando repuesto → listo → entrega.
 * Datos Ficticio. Deja capturas en SWEVEH_E2E_OUT para revisarlas.
 */
const uniq = Date.now().toString().slice(-6);
const celular = `310${uniq}1`;
const placa = `E${String.fromCharCode(65 + (Number(uniq) % 26))}T${uniq.slice(-3)}`;
const jpg = { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 1, 0]) };

test('orden de punta a punta en un taller real', async ({ page }) => {
  // 1. Registro y arranque.
  await registerWorkshop(page, celular);
  await shot(page, '01-arranque');

  // 2. Parámetros: Esperando repuesto, IVA aparte y tarifa por hora.
  await page.goto('/panel/taller');
  await page.getByLabel(/Usar el estado Esperando repuesto/).check();
  await page.getByLabel(/Mis precios ya incluyen IVA/).uncheck();
  await page.getByLabel('Tarifa por hora de mano de obra (COP)').fill('80000');
  await page.getByRole('button', { name: 'Guardar parámetros' }).click();
  await expect(page.getByText('Parámetros guardados.')).toBeVisible();
  await shot(page, '02-parametros');

  // 3. Un repuesto con existencia.
  await page.goto('/panel/repuestos');
  await page.getByLabel('Referencia', { exact: true }).fill(`PF-${uniq}`);
  await page.getByLabel('Nombre', { exact: true }).fill('Pastillas delanteras');
  await page.getByLabel('Precio de venta (COP)').fill('65000');
  await page.getByLabel('Existencia', { exact: true }).fill('5');
  await page.getByRole('button', { name: 'Agregar repuesto' }).click();
  await expect(page.getByText('5 / mín. 0')).toBeVisible();

  // 4. Orden con la ficha del vehículo completa.
  await page.goto('/panel/ordenes/nueva');
  await page.getByLabel('Placa').fill(placa);
  await page.getByLabel('Celular del cliente').fill(`300${uniq}2`);
  await page.getByLabel('Nombre', { exact: true }).fill('Laura Cliente');
  await page.getByLabel('Combustible', { exact: true }).selectOption('gasolina');
  await page.getByLabel('Caja', { exact: true }).selectOption('manual');
  await page.getByLabel('Cilindraje', { exact: true }).fill('1600');
  await page.getByRole('radio', { name: 'Atmosférico' }).click();
  await expect(page.getByText('Ficha completa')).toBeVisible();
  await page.getByLabel('Lo que dice el cliente').fill('Chilla al frenar');
  await page.getByRole('button', { name: 'Abrir orden' }).click();
  await expect(page.getByRole('heading', { name: /OS-/ })).toBeVisible();

  // 5. Recepción: ítems, combustible, kilometraje, 4 fotos y una nota con 2 fotos.
  await page.getByLabel('Kilometraje', { exact: true }).fill('84500');
  await page.getByLabel('Combustible', { exact: true }).selectOption('1/2');
  await page.getByRole('radiogroup', { name: 'Luces' }).getByRole('radio', { name: 'Mal' }).click();
  await page.getByLabel('Qué tiene Luces').fill('Stop derecho fundido');
  await page.getByRole('button', { name: 'Guardar checklist' }).click();
  const lados = page.locator('section[aria-labelledby="recepcion"] label:has-text("Tomar foto") input[type=file]');
  for (let falta = 4; falta > 0; falta--) {
    await lados.first().setInputFiles(jpg);
    await expect(lados).toHaveCount(falta - 1); // cada lado pasa a "Cambiar foto" cuando la subida termina
  }
  await page.getByLabel('Nota (lo que encontraste o cambiaste)').fill('Rayón en la puerta trasera');
  await page.getByLabel('Fotos de la nota').setInputFiles([jpg, { ...jpg, name: 'b.jpg' }]);
  await page.getByRole('button', { name: 'Guardar nota' }).click();
  await expect(page.getByText('Rayón en la puerta trasera')).toBeVisible();
  await shot(page, '03-recepcion');
  await page.getByRole('button', { name: 'Pasar a Recibido' }).click();
  await page.getByRole('button', { name: 'Pasar a En diagnóstico' }).click();

  // 6. Diagnóstico con mano de obra estimada; "Cotizar esto" la pasa a la cotización.
  await page.getByLabel('Sistema', { exact: true }).selectOption('frenos');
  await page.getByLabel('Qué encontraste').fill('Pastillas delanteras al 10 %');
  await page.getByLabel('Mano de obra estimada (COP)').fill('60000');
  await page.getByRole('radio', { name: 'Urgente' }).click();
  await page.getByRole('button', { name: 'Agregar hallazgo' }).click();
  await page.getByRole('button', { name: 'Cotizar esto' }).click();
  await expect(page.getByText(/Mano de obra · Frenos/)).toBeVisible();
  await shot(page, '04-diagnostico');

  // 7. Cotización: repuesto × 2, mano de obra por horas, enviar y aprobar.
  await page.getByLabel('Buscar servicio o repuesto').fill('Pastillas');
  await page.getByLabel('Cantidad de Repuesto · Pastillas delanteras').fill('2');
  await page.locator('li', { hasText: 'Repuesto · Pastillas delanteras' }).getByRole('button', { name: 'Agregar' }).click();
  await page.getByLabel('Mano de obra u otro ítem').fill('Rectificar discos');
  await page.getByLabel('Horas', { exact: true }).fill('1,5');
  await page.getByRole('button', { name: 'Agregar ítem' }).click();
  await expect(page.getByText('(1.5 h)')).toBeVisible();
  await expect(page.getByText('IVA 19 %')).toBeVisible();
  await shot(page, '05-cotizacion');
  await page.getByRole('button', { name: 'Enviar cotización al cliente' }).click();

  // 7b. V-10: la cotización en pantalla completa para el cliente; autoriza sus datos (HU-86) y aprueba en el mostrador.
  const orderId = /ordenes\/([0-9a-f-]{36})/.exec(page.url())?.[1] ?? '';
  await page.getByRole('link', { name: 'Mostrar al cliente' }).click();
  await expect(page).toHaveURL(/\/cliente$/);
  await expect(page.getByRole('navigation')).toHaveCount(0); // sin el menú del panel
  await expect(page.getByText('Rectificar discos')).toBeVisible();
  await shot(page, '05b-vista-cliente');
  // CA-11: en un celular de 390 px la pantalla del cliente no se desborda de lado y los totales quedan antes de las acciones.
  const grande = page.viewportSize();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByText(/^Total/)).toBeVisible();
  const { ancho, vista } = await page.evaluate(() => ({ ancho: document.documentElement.scrollWidth, vista: window.innerWidth }));
  expect(ancho, 'la pantalla del cliente no debe desplazarse de lado').toBeLessThanOrEqual(vista);
  await shot(page, '05c-vista-cliente-celular');
  if (grande) await page.setViewportSize(grande);
  const pdf = await page.request.get(`/api/v1/orders/${orderId}/quote.pdf`);
  expect(pdf.ok()).toBe(true);
  expect(pdf.headers()['content-type']).toBe('application/pdf');
  expect((await pdf.body()).subarray(0, 5).toString()).toBe('%PDF-');
  await page.getByRole('button', { name: 'El cliente aprobó' }).click();
  await page.getByRole('button', { name: 'El cliente autoriza y aprueba' }).click();
  await expect(page.getByText('Aprobada. El trabajo puede empezar.')).toBeVisible();
  // Para volver al panel hay que mantener presionado "Volver": un toque corto no sale.
  const volver = page.getByRole('button', { name: 'Volver', exact: true });
  await volver.click();
  await expect(page.getByText('Mantén presionado «Volver» para salir.')).toBeVisible();
  await volver.hover();
  await page.mouse.down();
  await page.waitForTimeout(900);
  await page.mouse.up();
  await expect(page).toHaveURL(new RegExp(`/panel/ordenes/${orderId}$`));
  await expect(page.locator('header').getByText('En reparación')).toBeVisible();
  await expect(page.getByText(/Descontado 2 · Devuelto 0/)).toBeVisible();

  // 8. Devolver una unidad: se ve qué pasó.
  await page.getByRole('button', { name: 'Devolver 1 al inventario' }).click();
  await expect(page.getByText(/Se devolvió 1 de Pastillas delanteras/)).toBeVisible();
  await expect(page.getByText(/Descontado 2 · Devuelto 1/)).toBeVisible();

  // 9. Esperando repuesto con motivo, vuelve a reparación y queda Listo.
  await page.getByLabel('¿Qué repuesto falta?').fill('Disco trasero, llega el jueves');
  await page.getByRole('button', { name: 'Poner en Esperando repuesto' }).click();
  await page.getByRole('button', { name: 'Pasar a En reparación' }).click();
  await page.getByRole('button', { name: 'Pasar a Listo' }).click();
  await expect(page.locator('header').getByText('Listo')).toBeVisible();
  await shot(page, '06-listo');

  // 10. Entrega (HU-25): quién recoge y confirmar que revisó lo que dejó (va en las notas); la orden se cierra.
  const entregar = page.getByRole('button', { name: /Pasar a Entregado/ });
  await expect(entregar).toBeDisabled();
  await page.getByLabel('¿Quién recoge?').fill('Laura Cliente');
  await page.getByLabel(/Revisé con el cliente lo que dejó/).check();
  await entregar.click();
  await expect(page.locator('header').getByText('Entregado')).toBeVisible();
  await expect(page.getByText('Mover la orden')).toHaveCount(0);
  await shot(page, '07-entregado');
});
