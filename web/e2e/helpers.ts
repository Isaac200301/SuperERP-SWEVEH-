import { join } from 'node:path';

import { expect, type Page } from '@playwright/test';

/** Ayudantes de los recorridos en navegador. Datos Ficticio. */
export const out = process.env.SWEVEH_E2E_OUT ?? 'test-results';

export async function shot(page: Page, name: string) {
  await page.screenshot({ path: join(out, `${name}.png`), fullPage: true });
}

/** PIN de las pruebas en navegador (Ficticio, no es uno de los fáciles que rechaza el registro). */
export const PIN_E2E = '2580';

/** Escribe un PIN en las 4 casillas (HU-104). */
export async function fillPin(page: Page, label: string, pin: string) {
  await page.getByLabel(`${label}: número 1 de 4`).click();
  await page.keyboard.type(pin);
}

/**
 * Registra un taller de carros nuevo con celular y PIN y lo deja armado (paso 1 y 2 del alta, V-04). Ya no lee el
 * código del log del api: con AUTH_MODE=pin no hay código.
 */
export async function registerWorkshop(page: Page, celular: string, taller = 'Taller Prueba E2E', inventarioPropio = true) {
  await page.goto('/registro');
  await page.getByLabel('Tu celular').fill(celular);
  await page.getByLabel('Nombre del taller').fill(taller);
  await fillPin(page, 'Crea tu PIN', PIN_E2E);
  await fillPin(page, 'Repite el PIN', PIN_E2E);
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Seguir' }).click();
  await expect(page.getByText('PASO 2 DE 2')).toBeVisible();
  await page.getByRole('radio', { name: 'Carros' }).click();
  await page.getByLabel(/Frenos y suspensión/).check();
  // Estos recorridos usan el inventario de SWEVEH: se desmarca "Llevo el inventario en otro programa", que en el
  // registro viene marcado (V-06, Q-41). El recorrido de la entrega (R12) lo deja marcado.
  if (inventarioPropio) await page.getByLabel(/Llevo el inventario en otro programa/).uncheck();
  await page.getByLabel(/Acepto el contrato/).check();
  await page.getByRole('button', { name: 'Armar mi taller' }).click();
  await expect(page.getByText('Te dejamos listos estos servicios')).toBeVisible();
}
