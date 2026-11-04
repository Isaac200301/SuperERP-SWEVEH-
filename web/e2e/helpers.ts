import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { expect, type Page } from '@playwright/test';

/** Ayudantes de los recorridos en navegador. Datos Ficticio. */
export const out = process.env.SWEVEH_E2E_OUT ?? 'test-results';
const apiLog = process.env.SWEVEH_API_LOG ?? '';

export async function shot(page: Page, name: string) {
  await page.screenshot({ path: join(out, `${name}.png`), fullPage: true });
}

/** El api local escribe el código en consola (OTP_SENDER=consola): se toma el último para ese celular. */
async function lastCode(celular: string): Promise<string> {
  // Sin el log no hay código de entrada: falla en vez de saltarse, para que el CI no pase sin haber corrido.
  if (!apiLog) throw new Error('Define SWEVEH_API_LOG con la ruta del log del api (o corre `make e2e`).');
  const tail = celular.slice(-4);
  for (let i = 0; i < 40; i++) {
    const matches = [...readFileSync(apiLog, 'utf8').matchAll(/Código SWEVEH para (\S+[^:]*): (\d{6})/g)].filter((m) => (m[1] ?? '').includes(tail));
    const last = matches.at(-1);
    if (last?.[2]) return last[2];
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error('No apareció el código en el log del api: ¿SWEVEH_API_LOG apunta al log de `go run ./cmd/api`?');
}

/** Registra un taller de carros nuevo y lo deja armado (pasos 1 a 3 del alta). */
export async function registerWorkshop(page: Page, celular: string, nombre = 'Pablo Prueba') {
  await page.goto('/registro');
  await page.getByLabel('Tu nombre').fill(nombre);
  await page.getByLabel('Tu celular').fill(celular);
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Recibir código' }).click();
  await page.getByLabel('Código').fill(await lastCode(celular));
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page.getByText('PASO 2 DE 3')).toBeVisible();
  await page.getByRole('radio', { name: 'Carros' }).click();
  await page.getByLabel(/Frenos y suspensión/).check();
  await page.getByLabel(/Acepto el contrato/).check();
  await page.getByRole('button', { name: 'Armar mi taller' }).click();
  await expect(page.getByText('Tu taller ya está armado')).toBeVisible();
}
