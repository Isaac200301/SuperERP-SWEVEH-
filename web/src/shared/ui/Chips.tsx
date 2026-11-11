export type Chip<T extends string> = { value: T; label: string; count?: number };

/**
 * Filtros en píldoras con desplazamiento horizontal (mockups móviles: estados de las órdenes, tipo de vehículo).
 * Es un radiogroup (DR-14); el conteo va al lado de la palabra.
 */
export function Chips<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: Chip<T>[];
  onChange: (v: T) => void;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
      {options.map((o) => {
        const on = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={on}
            onClick={() => onChange(o.value)}
            className={`inline-flex min-h-[40px] flex-none items-center gap-2 whitespace-nowrap rounded-full px-4 text-[14px] font-bold ${
              on ? 'bg-primary text-white' : 'bg-surface text-[#33443F]'
            }`}
          >
            {o.label}
            {o.count !== undefined ? (
              <span className={`rounded-full px-[7px] font-mono text-xs ${on ? 'bg-white/20' : 'bg-white'}`}>{o.count}</span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
