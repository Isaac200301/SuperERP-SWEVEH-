import { useId } from 'react';

type Option<T extends string> = { value: T; label: string; disabled?: boolean };

/** Segmentado accesible con role=radiogroup (DR-14). */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
  hint,
}: {
  label: string;
  value: T;
  options: Option<T>[];
  onChange: (v: T) => void;
  hint?: string | undefined;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <span id={id} className="text-[15px] font-extrabold">
        {label}
      </span>
      <div role="radiogroup" aria-labelledby={id} className="flex flex-wrap gap-2.5">
        {options.map((o) => {
          const on = o.value === value;
          return (
            <button
              key={o.value}
              type="button"
              role="radio"
              aria-checked={on}
              disabled={o.disabled}
              onClick={() => onChange(o.value)}
              className={`min-h-[44px] rounded-full border-[1.5px] px-4 text-[15px] font-bold transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
                on ? 'border-primary bg-primary-tint text-primary' : 'border-[#7F9591] bg-white text-ink'
              }`}
            >
              {o.label}
            </button>
          );
        })}
      </div>
      {hint ? <span className="text-sm text-ink-2">{hint}</span> : null}
    </div>
  );
}
