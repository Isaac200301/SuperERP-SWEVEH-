import type { ReactNode } from 'react';

import { IconStop, IconWarn } from './icons';

/** Aviso con ícono + palabra (DR-01). role=alert para que lo anuncie el lector de pantalla. */
export function Alert({ kind = 'error', title, children }: { kind?: 'error' | 'warn'; title?: string; children: ReactNode }) {
  const err = kind === 'error';
  return (
    <div
      role="alert"
      className={`flex gap-3 rounded-2xl p-4 text-[15px] ${err ? 'bg-err-tint text-err-ink' : 'bg-warn-tint text-warn-ink'}`}
    >
      <span className="mt-0.5 flex-none">{err ? <IconStop size={18} /> : <IconWarn size={18} />}</span>
      <div className="flex flex-col gap-1">
        {title ? <b>{title}</b> : null}
        <div>{children}</div>
      </div>
    </div>
  );
}
