import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { expect, test, type Page } from '@playwright/test';

/**
 * La orden de punta a punta como la usa un taller real (lo que Pablo probó a mano el 3-nov-2026): registro →
 * parámetros → repuesto → orden con ficha → recepción con ítems y fotos → diagnóstico con mano de obra y fotos →
 * cotización con servicio sin precio, repuesto × 2 y horas → aprobar → devolver → esperando repuesto → listo.
 * Datos Ficticio. Deja capturas en SWEVEH_E2E_OUT para revisarlas.
 */
const out = process.env.SWEVEH_E2E_OUT ?? 'test-results';
const apiLog = process.env.SWEVEH_API_LOG ?? '';
const uniq = Date.now().toString().slice(-6);
const celular = `310${uniq}1`;
const placa = `E${String.fromCharCode(65 + (Number(uniq) % 26))}T${uniq.slice(-3)}`;
const jpg = { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 1, 0]) };

async function shot(page: Page, name: string) {
  await page.screenshot({ path: join(out, `${name}.png`), fullPage: true });
}

/** El api local escribe el código en consola (OTP_SENDER=consola): se toma el último para ese celular. */
async function lastCode(): Promise<string> {
  const tail = celular.slice(-4);
  for (let i = 0; i < 40; i++) {
    const matches = [...readFileSync(apiLog, 'utf8').matchAll(/Código SWEVEH para (\S+[^:]*): (\d{6})/g)].filter((m) => (m[1] ?? '').includes(tail));
    const last = matches.at(-1);
    if (last?.[2]) return last[2];
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error('No apareció el código en el log del api: ¿SWEVEH_API_LOG apunta al log de `go run ./cmd/api`?');
}

test('orden de punta a punta en un taller real', async ({ page }) => {
  // Sin el log no hay código de entrada: falla en vez de saltarse, para que el CI no pase sin haber corrido.
  if (!apiLog) throw new Error('Define SWEVEH_API_LOG con la ruta del log del api (o corre `make e2e`).');

  // 1. Registro y arranque.
  await page.goto('/registro');
  await page.getByLabel('Tu nombre').fill('Pablo Prueba');
  await page.getByLabel('Tu celular').fill(celular);
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Recibir código' }).click();
  await page.getByLabel('Código').fill(await lastCode());
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page.getByText('PASO 2 DE 3')).toBeVisible();
  await page.getByRole('radio', { name: 'Carros' }).click();
  await page.getByLabel(/Frenos y suspensión/).check();
  await page.getByLabel(/Acepto el contrato/).check();
  await page.getByRole('button', { name: 'Armar mi taller' }).click();
  await expect(page.getByText('Tu taller ya está armado')).toBeVisible();
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
  await page.getByRole('button', { name: 'Aprobó en el mostrador' }).click();
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

  // 10. Entrega (HU-25): quién recoge y confirmar que no dejó accesorios; la orden se cierra.
  const entregar = page.getByRole('button', { name: /Pasar a Entregado/ });
  await expect(entregar).toBeDisabled();
  await page.getByLabel('¿Quién recoge?').fill('Laura Cliente');
  await page.getByLabel('No dejó accesorios').check();
  await entregar.click();
  await expect(page.locator('header').getByText('Entregado')).toBeVisible();
  await expect(page.getByText('Mover la orden')).toHaveCount(0);
  await shot(page, '07-entregado');
});
