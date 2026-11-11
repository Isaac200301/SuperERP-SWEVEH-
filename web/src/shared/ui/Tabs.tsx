import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react';

export type Tab<T extends string> = { value: T; label: string };

/** Pestañas accesibles (DR-14): role=tablist/tab/tabpanel, flechas para moverse y el panel ligado a su pestaña. */
export function Tabs<T extends string>({
  label,
  tabs,
  value,
  onChange,
  children,
}: {
  label: string;
  tabs: Tab<T>[];
  value: T;
  onChange: (v: T) => void;
  children: ReactNode;
}) {
  const id = useId();
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKey = (e: KeyboardEvent, i: number) => {
    const step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const next = (i + step + tabs.length) % tabs.length;
    const t = tabs[next];
    if (!t) return;
    onChange(t.value);
    refs.current[next]?.focus();
  };
  return (
    <>
      <div role="tablist" aria-label={label} className="flex gap-1 rounded-2xl bg-surface p-1">
        {tabs.map((t, i) => {
          const on = t.value === value;
          return (
            <button
              key={t.value}
              ref={(el) => {
                refs.current[i] = el;
              }}
              type="button"
              role="tab"
              id={`${id}-tab-${t.value}`}
              aria-selected={on}
              aria-controls={`${id}-panel`}
              tabIndex={on ? 0 : -1}
              onClick={() => onChange(t.value)}
              onKeyDown={(e) => onKey(e, i)}
              className={`min-h-[44px] flex-1 rounded-xl px-2 text-[14px] font-extrabold ${
                on ? 'bg-white text-primary shadow-[0_2px_6px_rgba(15,31,29,.1)]' : 'text-ink-2'
              }`}
            >
              {t.label}
            </button>
          );
        })}
      </div>
      <div role="tabpanel" id={`${id}-panel`} aria-labelledby={`${id}-tab-${value}`} className="flex flex-col gap-4">
        {children}
      </div>
    </>
  );
}
