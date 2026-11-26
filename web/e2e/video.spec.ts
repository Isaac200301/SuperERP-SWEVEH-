import { expect, test } from '@playwright/test';

import { registerWorkshop, shot } from './helpers';

/**
 * R9 (V-12, HU-108, ADR-0042): recibir con un video en Chromium real, con su cámara y micrófono falsos (patrón de
 * prueba) y la IA falsa de scripts/fake-ai.mjs. Se graba de verdad con MediaRecorder; solo salen el audio y los
 * cuadros, la guía marca lo que el modelo "vio" y nada se guarda hasta Confirmar. Datos Ficticio.
 */
test.skip(process.env.E2E_WHATSAPP === 'true', 'el asistente del piso se prueba con la IA falsa, sin WhatsApp');

test.use({
  launchOptions: { args: ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] },
  permissions: ['camera', 'microphone'],
});

const uniq = Date.now().toString().slice(-6);
const celular = `314${uniq}3`;
const placa = `V${String.fromCodePoint(65 + (Number(uniq) % 26))}D${uniq.slice(-3)}`;

test('recibir un carro con un video y la guía de lados', async ({ page }) => {
  test.setTimeout(150_000);
  await registerWorkshop(page, celular);

  // La orden se abre con el asistente (R8) y queda "Por recibir".
  await page.goto('/panel/asistente');
  await page.getByLabel('Mensaje para el asistente').fill(`Llegó el Mazda ${placa} de Carlos Gómez, celular 3001112233, le suenan los frenos`);
  await page.getByRole('button', { name: 'Enviar' }).click();
  const general = page.getByRole('region', { name: 'Ficha en vivo' });
  await general.getByRole('button', { name: 'Confirmar' }).click();
  await page.getByRole('link', { name: /Seguimos en la orden OS-\d{4}/ }).click();

  const video = page.getByRole('region', { name: 'Recibir con un video' });
  await expect(video).toBeVisible();
  const guia = video.getByRole('list', { name: 'Guía de lados' });
  await expect(guia.getByRole('listitem').filter({ hasText: 'Frente' })).toContainText('falta');

  await video.getByRole('button', { name: 'Grabar recepción' }).click();
  await expect(video.getByRole('timer')).toContainText('/ 1:30');
  await expect(video.getByRole('timer')).toContainText('0:12', { timeout: 20_000 });
  await shot(page, 'video-grabando');
  await video.getByRole('button', { name: 'Terminar' }).click();

  // La IA falsa marca un lado por cuadro: frente, trasera, izquierdo. El código dice lo que no vio.
  await expect(page.getByText(/No vi la placa, el tablero ni el lado derecho/)).toBeVisible({ timeout: 30_000 });
  // El audio sí salió (el e2e corre sin Groq): sin Permissions-Policy microphone=(self) no había pista de audio.
  await expect(page.getByText(/No pude escuchar el audio del video/)).toBeVisible();
  await expect(guia.getByRole('listitem').filter({ hasText: 'Frente' })).toContainText('visto');
  await expect(guia.getByRole('listitem').filter({ hasText: 'Lado der.' })).toContainText('falta');
  await expect(page.getByText(/Video de recepción · 0:1\d · 3 cuadros/)).toBeVisible();
  await expect(video.getByRole('button', { name: 'Grabar otro video' })).toBeVisible();
  const ficha = page.getByRole('region', { name: 'Ficha en vivo' });
  await expect(ficha.getByText(/Frente · Atrás · Lado izq\./)).toBeVisible();
  await shot(page, 'video-guia');

  // Confirmar guarda los cuadros como fotos de la orden.
  await ficha.getByRole('button', { name: 'Confirmar' }).click();
  await expect(page.getByText(/Quedó guardado: .*3 fotos/)).toBeVisible();
});
