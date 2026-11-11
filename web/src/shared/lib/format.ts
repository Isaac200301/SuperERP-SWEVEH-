/** Formatos de docs/08 §4: COP sin decimales, fechas en America/Bogota, celular de 10 dígitos. */

const ZONA = 'America/Bogota';

const cop = new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 });

export function formatoCOP(pesos: number): string {
  return cop.format(Math.round(pesos));
}

const fechaHora = new Intl.DateTimeFormat('es-CO', {
  timeZone: ZONA,
  day: 'numeric',
  month: 'short',
  hour: 'numeric',
  minute: '2-digit',
});

const hora = new Intl.DateTimeFormat('es-CO', { timeZone: ZONA, hour: 'numeric', minute: '2-digit' });

export function formatoFechaHora(iso: string): string {
  return fechaHora.format(new Date(iso));
}

export function formatoHora(iso: string): string {
  return hora.format(new Date(iso));
}

/** Deja solo dígitos y quita el indicativo 57 si viene. */
export function soloCelular(raw: string): string {
  let d = raw.replace(/\D/g, '');
  if (d.length === 12 && d.startsWith('57')) d = d.slice(2);
  return d;
}

export function celularValido(raw: string): boolean {
  return /^3[0-9]{9}$/.test(soloCelular(raw));
}

/**
 * 3001234567 → 300 123 4567. Un chat "…@lid" es uno en que WhatsApp ocultó el número y WAHA no lo pudo traducir
 * (ADR-0027): se dice eso en vez de mostrar el id interno.
 */
export function formatoCelular(raw: string): string {
  if (raw.endsWith('@lid')) return 'Número oculto por WhatsApp';
  const d = soloCelular(raw);
  if (d.length !== 10) return raw;
  return `${d.slice(0, 3)} ${d.slice(3, 6)} ${d.slice(6)}`;
}
