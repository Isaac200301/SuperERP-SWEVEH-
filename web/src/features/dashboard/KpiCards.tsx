import { useState } from 'react';

import type { Schemas } from '../../shared/api/client';
import { BottomSheet } from '../../shared/ui/BottomSheet';
import { Button } from '../../shared/ui/Button';

type Kpi = Schemas['Kpi'];
const fondos = ['bg-kpi-peach', 'bg-kpi-lilac', 'bg-kpi-aqua'];

/**
 * Los números del taller en tarjetas pastel; cada uno abre "¿Qué significa?" en una hoja inferior (HU-75, DR-11).
 * En fila desplazable, en el celular (Hoy, "Tus números").
 */
export function KpiCards({ kpis }: Readonly<{ kpis: Kpi[] }>) {
  const [kpi, setKpi] = useState<Kpi | null>(null);
  return (
    <>
      <div className="-mx-4 flex gap-2.5 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
        {kpis.map((k, i) => (
          <div
            key={k.codigo}
            className={`flex w-[150px] flex-none flex-col gap-1 rounded-[20px] p-3.5 ${fondos[i % fondos.length]}`}
          >
            <span className={`font-mono font-bold break-words ${k.texto.length > 9 ? 'text-[17px]' : 'text-[22px]'}`}>{k.texto}</span>
            <span className="text-[13px] font-bold text-[#33443F]">
              {k.titulo}
              {k.ejemplo ? <span className="ml-1 rounded-full bg-warn-tint px-1.5 text-[11px] text-warn-ink">ejemplo</span> : null}
            </span>
            <button
              type="button"
              onClick={() => setKpi(k)}
              className="mt-auto min-h-[32px] self-start rounded-full bg-white/75 px-2.5 text-xs font-bold text-primary"
            >
              ¿Qué significa?
            </button>
          </div>
        ))}
      </div>
      <BottomSheet open={kpi !== null} onClose={() => setKpi(null)} title={kpi?.titulo ?? ''} eyebrow="¿QUÉ SIGNIFICA?">
        {kpi ? (
          <>
            <div className="flex flex-col gap-0.5 rounded-[20px] bg-kpi-aqua p-3.5">
              <span className="font-mono text-[30px] font-bold break-words">{kpi.texto}</span>
              <span className="text-sm">{kpi.detalle}</span>
            </div>
            <p className="text-[15px] leading-normal">{kpi.queSignifica}</p>
            <Button onClick={() => setKpi(null)}>Entendido</Button>
          </>
        ) : null}
      </BottomSheet>
    </>
  );
}
