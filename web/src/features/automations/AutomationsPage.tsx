import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { usePermission } from '../auth/api';

type Automatizacion = Schemas['Automatizacion'];
const key = ['automatizaciones'] as const;

/** HU-40, HU-41 y HU-42: catálogo con interruptor; los avisos editables muestran sus datos de plantilla. */
function Card({ a, canManage }: { a: Automatizacion; canManage: boolean }) {
  const qc = useQueryClient();
  const [texto, setTexto] = useState(a.texto);
  const guardar = useMutation({
    mutationFn: async (body: { activa?: boolean; texto?: string }) =>
      unwrap(await api.PATCH('/automations/{code}', { params: { path: { code: a.codigo } }, body })),
    onSuccess: (data) => qc.setQueryData(key, data.automatizaciones),
  });
  return (
    <li className={`flex flex-col gap-3 rounded-card bg-white p-5 ${a.proximamente ? 'opacity-70' : ''}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-[17px] font-extrabold">{a.nombre}</h2>
          <p className="text-sm text-ink-2">
            {a.disparador} · {a.costo} · {a.ejecucionesSemana} esta semana
          </p>
        </div>
        {a.proximamente ? (
          <span className="rounded-full bg-surface px-3 py-1 text-sm font-bold">Próximamente</span>
        ) : (
          <label className="flex min-h-[44px] items-center gap-2 font-bold">
            <input
              type="checkbox"
              role="switch"
              aria-checked={a.activa}
              checked={a.activa}
              disabled={!canManage || guardar.isPending}
              onChange={(e) => guardar.mutate({ activa: e.target.checked })}
            />
            {a.activa ? 'Encendida' : 'Apagada'}
          </label>
        )}
      </div>
      {a.enviaMensaje && a.editable && canManage ? (
        <>
          <label htmlFor={`t-${a.codigo}`} className="text-[15px] font-extrabold">
            Texto del aviso
          </label>
          <textarea id={`t-${a.codigo}`} rows={2} className="rounded-btn border-[1.5px] border-line p-3" value={texto} onChange={(e) => setTexto(e.target.value)} />
          {a.datos && a.datos.length > 0 ? <p className="text-sm text-ink-2">Datos que se llenan solos: {a.datos.join(', ')}</p> : null}
          <div>
            <Button variant="secondary" disabled={texto === a.texto} busy={guardar.isPending} onClick={() => guardar.mutate({ texto })}>
              Guardar texto
            </Button>
          </div>
        </>
      ) : a.enviaMensaje ? (
        <p className="rounded-2xl bg-surface p-3 text-sm">{a.texto}</p>
      ) : null}
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
    </li>
  );
}

export function AutomationsPage() {
  const canManage = usePermission('automation:manage');
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/automations')).automatizaciones });
  const runs = useQuery({
    queryKey: [...key, 'ejecuciones'],
    enabled: canManage,
    queryFn: async () => unwrap(await api.GET('/automations/runs')).ejecuciones,
  });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Automatizaciones</h1>
        <p className="text-ink-2">Enciende lo que quieras que pase solo. Cada envío por WhatsApp cuenta como un mensaje.</p>
      </header>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      <ul className="grid gap-3 lg:grid-cols-2">
        {(q.data ?? []).map((a) => (
          <Card key={a.codigo} a={a} canManage={canManage} />
        ))}
      </ul>
      {canManage && runs.data && runs.data.length > 0 ? (
        <section className="flex flex-col gap-2 rounded-card bg-white p-5" aria-labelledby="ejecuciones">
          <h2 id="ejecuciones" className="text-[17px] font-extrabold">
            Últimas ejecuciones
          </h2>
          <ul className="flex flex-col gap-1 text-sm">
            {runs.data.slice(0, 20).map((r, i) => (
              <li key={`${r.codigo}-${i}`} className="flex flex-wrap gap-3">
                <span className="font-mono text-ink-2">{formatoFechaHora(r.creadoEn)}</span>
                <b>{r.codigo}</b>
                <span>{r.resultado}</span>
                <span className="text-ink-2">{r.detalle}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </>
  );
}
