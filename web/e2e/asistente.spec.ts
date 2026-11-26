import { expect, test } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * R8 (V-11, HU-107, ADR-0041): recibir un carro hablándole al asistente del piso, con la IA falsa de scripts/fake-ai.mjs
 * (guion fijo, nunca un modelo real). Nada se guarda hasta Confirmar; Confirmar abre la orden con su recepción y las
 * fotos. Corre sin WhatsApp (la entrega del 27-nov). Datos Ficticio.
 */
test.skip(process.env.E2E_WHATSAPP === 'true', 'el asistente del piso se prueba con la IA falsa, sin WhatsApp');

const uniq = Date.now().toString().slice(-6);
const celular = `313${uniq}2`;
const placa = `A${String.fromCodePoint(65 + (Number(uniq) % 26))}C${uniq.slice(-3)}`;
const jpg = { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 1, 0]) };

test('recibir un carro hablándole al asistente y confirmar', async ({ page }) => {
  await registerWorkshop(page, celular);
  await page.goto('/panel/asistente');
  await expect(page.getByRole('heading', { name: 'Asistente', level: 1 })).toBeVisible();

  await page.getByLabel('Mensaje para el asistente').fill(`Llegó el Mazda ${placa} de Carlos Gómez, celular 3001112233, le suenan los frenos`);
  await page.getByRole('button', { name: 'Enviar' }).click();
  const ficha = page.getByRole('region', { name: 'Ficha en vivo' });
  await expect(ficha).toBeVisible();
  await expect(ficha.getByText('Carlos Gómez')).toBeVisible();
  await expect(ficha.getByText('le suenan los frenos')).toBeVisible();
  await expect(ficha.getByText(/falta el kilometraje/)).toBeVisible();

  // Las 4 fotos con el kilometraje: la ficha queda completa.
  await page.getByLabel('Fotos', { exact: true }).setInputFiles([jpg, { ...jpg, name: 'b.jpg' }, { ...jpg, name: 'c.jpg' }, { ...jpg, name: 'd.jpg' }]);
  await page.getByLabel('Mensaje para el asistente').fill('Trae 45000 km y medio tanque');
  await page.getByRole('button', { name: 'Enviar' }).click();
  await expect(ficha.getByText(/\d+ de \d+ datos$/)).toBeVisible();
  await expect(ficha.getByText('45000 km')).toBeVisible();
  await shot(page, 'asistente-ficha-en-vivo');

  await ficha.getByRole('button', { name: 'Confirmar' }).click();
  await expect(page.getByText(/Quedó guardado: .*la orden OS-\d{4}/)).toBeVisible();
  await page.getByRole('link', { name: /Seguimos en la orden OS-\d{4}/ }).click();
  await expect(page.getByText(placa).first()).toBeVisible();
  // La orden quedó "Por recibir" con su recepción y la conversación sigue en la orden (D2, a la derecha).
  await expect(page.getByRole('heading', { name: 'Asistente de esta orden' })).toBeVisible();
  await shot(page, 'asistente-orden-d2');
});
