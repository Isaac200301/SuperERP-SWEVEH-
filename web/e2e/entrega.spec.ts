import { expect, test, type Page } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * R12 (docs/plan-entrega-viernes.md): el recorrido de la entrega del 27-nov de punta a punta, con la IA falsa de
 * scripts/fake-ai.mjs y sin WhatsApp. Registro con PIN → servicios sin precio → el asistente abre la orden con fotos →
 * Confirmar (con la autorización del cliente, R11) → Recibido → cotización con 2 partidas y 1 línea → Mostrar al
 * cliente → El cliente aprobó → anotar un gasto → Listo → Entregar. Corre en escritorio y en Pixel 7. Datos Ficticio.
 */
test.skip(process.env.E2E_WHATSAPP === 'true', 'la entrega del 27-nov corre sin WhatsApp');

/** En el celular la orden va en pestañas (Figma 05); en escritorio todo está a la vista (D2). */
async function pestana(page: Page, nombre: string) {
  const tab = page.getByRole('tab', { name: nombre, exact: true });
  if (await tab.isVisible()) await tab.click();
}

const jpg = { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 1, 0]) };

test('la entrega del viernes de punta a punta', async ({ page }, info) => {
  test.setTimeout(180_000);
  const uniq = (Date.now() + info.workerIndex * 7).toString().slice(-6);
  const placa = `E${String.fromCodePoint(65 + (Number(uniq) % 26))}N${uniq.slice(-3)}`;

  // 1. Registro con PIN; el inventario queda en otro programa (V-06) y los servicios llegan sin precio (nunca sugeridos).
  await registerWorkshop(page, `300${uniq}9`, 'Taller Entrega E2E', false);
  await page.goto('/panel/servicios');
  await expect(page.getByText(/Sin precio · ponlo/).first()).toBeVisible();

  // 2. El asistente: la placa abre una orden nueva en la ficha; nada se guarda todavía.
  await page.goto('/panel/asistente');
  const mensaje = page.getByLabel('Mensaje para el asistente');
  const enviar = page.getByRole('button', { name: 'Enviar' });
  await mensaje.fill(`Llegó el Mazda ${placa} de Carlos Gómez, celular 3001112233, le suenan los frenos`);
  await enviar.click();
  const ficha = page.getByRole('region', { name: 'Ficha en vivo' });
  await expect(ficha.getByText('Carlos Gómez')).toBeVisible();

  // 3. Fotos y kilometraje, y la ficha del motor dicha por la persona.
  await page.getByLabel('Fotos', { exact: true }).setInputFiles([jpg, { ...jpg, name: 'b.jpg' }, { ...jpg, name: 'c.jpg' }, { ...jpg, name: 'd.jpg' }]);
  await mensaje.fill('Trae 45000 km y medio tanque');
  await enviar.click();
  await expect(ficha.getByText('45000 km')).toBeVisible();
  await expect(ficha.getByText('Frente · Atrás · Lado izq. · Lado der.')).toBeVisible(); // las 4 fotos, cada una con su lado
  await mensaje.fill('Es gasolina, caja manual, 1600 cc, atmosférico');
  await enviar.click();
  await expect(ficha.getByText('1600', { exact: false }).first()).toBeVisible();

  // 4. Confirmar: primero la autorización de datos del cliente nuevo (HU-86).
  await ficha.getByRole('radio', { name: 'Autoriza', exact: true }).click();
  await ficha.getByRole('button', { name: 'Confirmar' }).click();
  await expect(page.getByText(/Quedó guardado: .*la orden OS-\d{4}/)).toBeVisible();
  await shot(page, `entrega-01-asistente-${info.project.name}`);
  await page.getByRole('link', { name: /Seguimos en la orden OS-\d{4}/ }).click();
  const orderId = /ordenes\/([0-9a-f-]{36})/.exec(page.url())?.[1] ?? '';
  expect(orderId).not.toBe('');

  // 5. Recibido y diagnóstico.
  await page.getByRole('button', { name: 'Pasar a Recibido' }).click();
  await page.getByRole('button', { name: 'Pasar a En diagnóstico' }).click();

  // 6. Cotización con 2 partidas y 1 línea, con valores que pone el taller.
  await pestana(page, 'Cotización');
  await page.getByLabel('Nombre de la partida').fill('Frenos completos');
  await page.getByLabel('Valor de la partida (COP)').fill('480000');
  await page.getByRole('button', { name: '+ Partida' }).click();
  await expect(page.getByText('Frenos completos').first()).toBeVisible();
  await page.getByLabel('Nombre de la partida').fill('Pintura del bómper');
  await page.getByLabel('Valor de la partida (COP)').fill('350000');
  await page.getByRole('button', { name: '+ Partida' }).click();
  await expect(page.getByText('Pintura del bómper').first()).toBeVisible();
  await page.getByLabel('Mano de obra u otro ítem').fill('Alineación');
  await page.getByLabel('Valor (COP)', { exact: true }).fill('60000');
  await page.getByRole('button', { name: 'Agregar ítem' }).click();
  await expect(page.getByText('Alineación').first()).toBeVisible();
  await page.getByRole('button', { name: 'Enviar cotización al cliente' }).click();

  // 7. La cotización en pantalla para el cliente, que aprueba en el mostrador (ya autorizó sus datos al abrir la orden).
  await page.getByRole('link', { name: 'Mostrar al cliente' }).click();
  await expect(page).toHaveURL(/\/cliente$/);
  await expect(page.getByText('Frenos completos')).toBeVisible();
  await shot(page, `entrega-02-cliente-${info.project.name}`);
  await page.getByRole('button', { name: 'El cliente aprobó' }).click();
  await expect(page.getByText('Aprobada. El trabajo puede empezar.')).toBeVisible();
  await page.goto(`/panel/ordenes/${orderId}`);
  await expect(page.locator('header').getByText('En reparación')).toBeVisible(); // el estado de la orden, no la barra de pasos
  await pestana(page, 'Cotización');

  // 8. Un gasto de la obra (interno), Listo y Entregar.
  await page.getByRole('button', { name: /Anotar un gasto/ }).click();
  await page.getByLabel('¿Qué fue?').fill('Pastillas en el almacén');
  await page.getByLabel('Valor del gasto (COP)').fill('95000');
  await page.getByRole('button', { name: 'Guardar el gasto' }).click();
  await expect(page.getByText('Pastillas en el almacén').first()).toBeVisible();
  await page.getByRole('button', { name: 'Pasar a Listo' }).click();
  const entregar = page.getByRole('button', { name: /Pasar a Entregado/ });
  await expect(entregar).toBeVisible();
  const recoge = page.getByLabel('¿Quién recoge?');
  if (!(await recoge.isVisible())) await entregar.click(); // celular: la entrega se abre desde la acción de abajo
  await recoge.fill('Carlos Gómez');
  await page.getByLabel(/Revisé con el cliente lo que dejó/).check();
  await page.getByRole('button', { name: /Pasar a Entregado/ }).last().click();
  await expect(page.locator('header').getByText('Entregado')).toBeVisible(); // el estado de la orden, no la barra de pasos
  await shot(page, `entrega-03-entregado-${info.project.name}`);

  // CA-11: sin desborde de lado.
  const { ancho, vista } = await page.evaluate(() => ({ ancho: document.documentElement.scrollWidth, vista: window.innerWidth }));
  expect(ancho).toBeLessThanOrEqual(vista);
});
