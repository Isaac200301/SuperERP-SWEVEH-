import { useEffect, useState, type ReactNode } from 'react';

import { Brand } from '../../shared/ui/Logo';
import { IconCheck } from '../../shared/ui/icons';

/** Cronómetro visible: la promesa es la primera orden en menos de 5 minutos (HU-01, criterio 4). */
function Stopwatch() {
  const [start] = useState(() => Date.now());
  const [now, setNow] = useState(start);
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  const s = Math.floor((now - start) / 1000);
  const mm = String(Math.floor(s / 60)).padStart(2, '0');
  const ss = String(s % 60).padStart(2, '0');
  return (
    <span className="font-mono text-[34px] font-bold text-night-accent" aria-label={`Llevas ${mm} minutos y ${ss} segundos`}>
      {mm}:{ss}
    </span>
  );
}

export type Step = { title: string; detail: string };

/** Marco del registro según el mockup Registro.dc.html: columna oscura con pasos y cronómetro. */
export function AuthLayout({ steps, current, children, timer = true }: { steps: Step[]; current: number; children: ReactNode; timer?: boolean }) {
  return (
    <div className="mx-auto flex min-h-screen max-w-[1280px] flex-col gap-5 p-4 md:flex-row md:p-5">
      <aside className="flex flex-col gap-7 rounded-[28px] bg-night p-7 text-night-text md:w-[300px] md:flex-none">
        <Brand />
        {timer ? (
          <div className="flex flex-col gap-1 rounded-[18px] border border-night-line bg-night-card p-4">
            <span className="text-[13px] text-night-text-2">Llevas</span>
            <Stopwatch />
            <span className="text-[13px] text-night-text-2">La meta es tu primera orden antes de 5 minutos.</span>
          </div>
        ) : null}
        <ol className="flex flex-col gap-[18px]">
          {steps.map((s, i) => {
            const done = i < current;
            const on = i === current;
            return (
              <li key={s.title} className="flex items-start gap-3" aria-current={on ? 'step' : undefined}>
                <span
                  className={`grid h-[22px] w-[22px] flex-none place-items-center rounded-full text-xs font-extrabold ${
                    done ? 'bg-ok text-white' : on ? 'bg-nav text-ink' : 'border-2 border-[#43685F] text-night-text-2'
                  }`}
                >
                  {done ? <IconCheck size={14} strokeWidth={3} /> : i + 1}
                </span>
                <span>
                  <b className={on || done ? '' : 'text-night-text-2'}>{s.title}</b>
                  <br />
                  <span className="text-[13px] text-night-text-2">{s.detail}</span>
                </span>
              </li>
            );
          })}
        </ol>
        <p className="mt-auto text-[13px] text-[#7F9490]">NIT, logo, horario y precios se piden cuando hagan falta, no ahora.</p>
      </aside>
      <main className="flex min-w-0 flex-1 flex-col gap-6 rounded-[28px] bg-white px-6 py-8 md:px-10">{children}</main>
    </div>
  );
}
