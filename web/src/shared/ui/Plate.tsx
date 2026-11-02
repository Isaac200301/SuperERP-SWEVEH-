/** DR-09: la placa siempre en estilo placa (amarillo, borde negro, mono). */
export function Plate({ value, size = 'sm' }: { value: string; size?: 'sm' | 'lg' }) {
  const s = size === 'lg' ? 'text-xl px-3 py-1 rounded-lg' : 'text-[13px] px-2 py-px rounded-md';
  return (
    <span className={`inline-block border-[1.5px] border-ink bg-plate font-mono font-bold tracking-[.05em] text-ink ${s}`}>
      {value}
    </span>
  );
}
