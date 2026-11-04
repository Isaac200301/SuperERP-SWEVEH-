import { expect, test } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * WhatsApp con el canal de prueba (ADR-0025), con api y worker reales: un cliente nuevo escribe por el simulador,
 * recibe la autorización de datos (HU-85), acepta y su pregunta llega a "Por revisar" como borrador (HU-62).
 */
const uniq = Date.now().toString().slice(-6);

test('cliente nuevo por WhatsApp: autorización y borrador para el dueño', async ({ page }) => {
  await registerWorkshop(page, `311${uniq}2`);

  await page.goto('/panel/taller');
  await page.getByRole('link', { name: 'Probar WhatsApp' }).click();
  await expect(page.getByRole('heading', { name: 'Probar WhatsApp' })).toBeVisible();
  await page.getByLabel('Celular del cliente').fill(`300${uniq}5`);
  await page.getByLabel('Nombre en WhatsApp').fill('Laura Cliente');
  await page.getByLabel('Mensaje').fill('Hola, ¿a cómo el cambio de aceite?');
  await page.getByRole('button', { name: 'Enviar' }).click();

  const phone = page.getByRole('region', { name: 'Teléfono del cliente' });
  await expect(phone.getByText(/aceptas el tratamiento de tus datos/)).toBeVisible({ timeout: 15_000 });
  await phone.getByRole('button', { name: 'Acepto', exact: true }).click();
  await expect(phone.getByText(/Ya quedó registrada tu autorización/)).toBeVisible({ timeout: 15_000 });
  await shot(page, 'w1-simulador');

  await page.goto('/panel/por-revisar');
  await expect(page.getByText('Hola, ¿a cómo el cambio de aceite?')).toBeVisible({ timeout: 15_000 });
  await shot(page, 'w2-por-revisar');
});
