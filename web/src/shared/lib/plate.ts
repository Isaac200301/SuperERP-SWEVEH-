/**
 * Reconocimiento de placa en el navegador, igual que internal/vehicles/domain/plate.go (HU-10).
 * El servidor sigue siendo la autoridad; esto solo da la ayuda inmediata al escribir.
 * Moto y moto antigua son provisionales hasta confirmar con RUNT/Mintransporte (Q-01).
 */
export type FormatoPlaca = 'carro' | 'moto' | 'moto_antigua' | 'especial' | 'invalida' | 'vacia';

export function normalizarPlaca(raw: string): string {
  return raw.toUpperCase().replace(/[\s.-]/g, '');
}

export function reconocerPlaca(raw: string): FormatoPlaca {
  const v = normalizarPlaca(raw);
  if (v === '') return 'vacia';
  if (/^[A-Z]{3}[0-9]{3}$/.test(v)) return 'carro';
  if (/^[A-Z]{3}[0-9]{2}[A-Z]$/.test(v)) return 'moto';
  if (/^[A-Z]{3}[0-9]{2}$/.test(v)) return 'moto_antigua';
  if (/^[A-Z0-9]{5,7}$/.test(v)) return 'especial';
  return 'invalida';
}

export function tipoPorPlaca(f: FormatoPlaca): 'carro' | 'moto' | null {
  if (f === 'carro') return 'carro';
  if (f === 'moto' || f === 'moto_antigua') return 'moto';
  return null;
}

export const ayudaPlaca: Record<FormatoPlaca, string> = {
  vacia: 'Carro ABC123 o moto ABC12D.',
  carro: 'Placa de carro.',
  moto: 'Placa de moto.',
  moto_antigua: 'Placa de moto, formato antiguo.',
  especial: 'No es formato de carro ni de moto. Puedes guardarla como placa especial.',
  invalida: 'Usa solo letras y números, por ejemplo ABC123.',
};
