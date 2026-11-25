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

/** Lee pesos escritos por la persona ("480.000", "$ 480000"): solo los dígitos. Vacío = undefined. */
export function pesos(s: string): number | undefined {
  return s.trim() === '' ? undefined : Number(s.replace(/\D/g, ''));
}

const DIAS = [
  ['lun', 'lun'],
  ['mar', 'mar'],
  ['mie', 'mié'],
  ['jue', 'jue'],
  ['vie', 'vie'],
  ['sab', 'sáb'],
  ['dom', 'dom'],
] as const;

/** "08:30" → "8:30 a. m." (docs/08 §4). */
function hora12(hhmm: string): string {
  const [h = 0, m = 0] = hhmm.split(':').map(Number);
  return `${h % 12 === 0 ? 12 : h % 12}:${String(m).padStart(2, '0')} ${h < 12 ? 'a. m.' : 'p. m.'}`;
}

/**
 * El horario semanal en una línea: los días seguidos con las mismas horas se juntan ("Lun a vie 7:30 a. m. – 6:00 p. m. ·
 * sáb 8:00 a. m. – 1:00 p. m."). Un día sin horas está cerrado y no sale.
 */
export function resumenHorario(h: Readonly<Record<string, { abre: string; cierra: string } | null | undefined>>): string {
  const grupos: { ini: number; fin: number; abre: string; cierra: string }[] = [];
  DIAS.forEach(([clave], i) => {
    const s = h[clave];
    if (!s) return;
    const ult = grupos.at(-1);
    if (ult && ult.fin === i - 1 && ult.abre === s.abre && ult.cierra === s.cierra) ult.fin = i;
    else grupos.push({ ini: i, fin: i, abre: s.abre, cierra: s.cierra });
  });
  if (grupos.length === 0) return 'Sin horario cargado';
  const nombre = (i: number) => DIAS[i]?.[1] ?? '';
  const rango = (g: { ini: number; fin: number }) => {
    if (g.ini === g.fin) return nombre(g.ini);
    return g.fin === g.ini + 1 ? `${nombre(g.ini)} y ${nombre(g.fin)}` : `${nombre(g.ini)} a ${nombre(g.fin)}`;
  };
  const linea = grupos.map((g) => `${rango(g)} ${hora12(g.abre)} – ${hora12(g.cierra)}`).join(' · ');
  return linea.charAt(0).toUpperCase() + linea.slice(1);
}
