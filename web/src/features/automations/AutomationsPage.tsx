import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, ApiError, unwrap, type Schemas } from '../../shared/api/client';
import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Chips } from '../../shared/ui/Chips';
import { IconBack } from '../../shared/ui/icons';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { usePermission } from '../auth/api';
import { Schedule } from '../workshop/WorkshopPage';

type Automatizacion = Schemas['Automatizacion'];
const key = ['automatizaciones'] as const;

/** HU-40, HU-41 y HU-42: catálogo con interruptor; los avisos editables muestran sus datos de plantilla. */
function Card({ a, canManage }: { a: Automatizacion; canManage: boolean }) {
  const qc = useQueryClient();
  // En el celular el editor va plegado: la tarjeta muestra el aviso y se edita al abrirlo (mockup Automatizaciones).
  const mobile = useIsMobile();
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
      {a.enviaMensaje && a.editable && canManage && mobile ? (
        <details className="rounded-2xl bg-surface p-3">
          <summary className="min-h-[32px] cursor-pointer list-none text-sm [&::-webkit-details-marker]:hidden">
            <span className="line-clamp-2">{a.texto}</span>
            <span className="font-bold text-primary">Editar texto del aviso</span>
          </summary>
          <div className="mt-3 flex flex-col gap-3">
            <TextEditor a={a} texto={texto} setTexto={setTexto} busy={guardar.isPending} onSave={() => guardar.mutate({ texto })} />
          </div>
        </details>
      ) : a.enviaMensaje && a.editable && canManage ? (
        <>
          <label htmlFor={`t-${a.codigo}`} className="text-[15px] font-extrabold">
            Texto del aviso
          </label>
          <textarea id={`t-${a.codigo}`} rows={2} className="rounded-btn border-[1.5px] border-line p-3" value={texto} onChange={(e) => setTexto(e.target.value)} />
          {a.datos && a.datos.length > 0 ? <p className="text-sm text-ink-2">Datos que se llenan solos: {a.datos.join(', ')}</p> : null}
          {a.fueraDeVentana ? (
            <p role="note" className="rounded-2xl bg-warn-tint p-3 text-sm text-warn-ink">
              Este aviso suele salir cuando el cliente no ha escrito en 24 horas. En producción WhatsApp exige que el texto sea una plantilla
              aprobada por Meta: si lo cambias, el texto nuevo necesita esa aprobación antes de usarse.
            </p>
          ) : null}
          <div>
            <Button variant="secondary" disabled={texto === a.texto} busy={guardar.isPending} onClick={() => guardar.mutate({ texto })}>
              Guardar texto
            </Button>
          </div>
        </>
      ) : a.enviaMensaje ? (
        <p className="rounded-2xl bg-surface p-3 text-sm">{a.texto}</p>
      ) : null}
      {guardar.error instanceof ApiError && guardar.error.code === 'horario_requerido' ? (
        <AskSchedule message={guardar.error.message} onSaved={() => guardar.mutate({ activa: true })} />
      ) : guardar.isError ? (
        <Alert>{guardar.error.message}</Alert>
      ) : null}
    </li>
  );
}

/** El editor del texto del aviso en el celular (HU-42), con sus datos y el aviso de plantilla de Meta. */
function TextEditor({ a, texto, setTexto, busy, onSave }: { a: Automatizacion; texto: string; setTexto: (t: string) => void; busy: boolean; onSave: () => void }) {
  return (
    <>
      <label htmlFor={`tm-${a.codigo}`} className="text-[15px] font-extrabold">
        Texto del aviso
      </label>
      <textarea id={`tm-${a.codigo}`} rows={4} className="rounded-btn border-[1.5px] border-line bg-white p-3" value={texto} onChange={(e) => setTexto(e.target.value)} />
      {a.datos && a.datos.length > 0 ? <p className="text-sm text-ink-2">Datos que se llenan solos: {a.datos.join(', ')}</p> : null}
      {a.fueraDeVentana ? (
        <p role="note" className="rounded-2xl bg-warn-tint p-3 text-sm text-warn-ink">
          Si lo cambias, en producción el texto nuevo necesita la aprobación de Meta antes de usarse.
        </p>
      ) : null}
      <Button variant="secondary" disabled={texto === a.texto} busy={busy} onClick={onSave}>
        Guardar texto
      </Button>
    </>
  );
}

/** HU-06: el horario se pide justo al encender el agente; al guardarlo, el agente se enciende. */
function AskSchedule({ message, onSaved }: { message: string; onSaved: () => void }) {
  const t = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  return (
    <div className="flex flex-col gap-3">
      <p role="alert" className="rounded-2xl bg-warn-tint p-3 text-sm font-bold text-warn-ink">{message}</p>
      {t.data ? <Schedule t={t.data} onSaved={onSaved} /> : null}
    </div>
  );
}

/**
 * Encabezado del celular (mockup Automatizaciones): volver a Taller, las cifras de la semana y el filtro.
 * Los borradores son los que esperan en Por revisar.
 */
function MobileHeader({ lista, filtro, setFiltro }: { lista: Automatizacion[]; filtro: Filtro; setFiltro: (f: Filtro) => void }) {
  const stats = useQuery({ queryKey: ['bandeja', 'stats'], queryFn: async () => unwrap(await api.GET('/inbox/stats')) });
  const disponibles = lista.filter((a) => !a.proximamente);
  const encendidas = disponibles.filter((a) => a.activa).length;
  const envios = lista.reduce((n, a) => n + a.ejecucionesSemana, 0);
  return (
    <>
      <header className="-mx-4 -mt-4 flex flex-col gap-4 rounded-b-[28px] bg-primary px-[18px] pt-4 pb-5 text-white">
        <div className="flex items-center gap-3">
          <Link to="/panel/mas" aria-label="Volver a Taller" className="grid h-11 w-11 flex-none place-items-center rounded-[14px] bg-white/15">
            <IconBack size={20} />
          </Link>
          <div>
            <h1 className="text-[22px] font-extrabold tracking-[-.02em]">Automatizaciones</h1>
            <p className="text-[13px] text-[#CFE9E4]">Avisos y tareas que trabajan solos</p>
          </div>
        </div>
        <dl className="grid grid-cols-3 gap-2 text-center">
          {[
            [String(encendidas), 'encendidas'],
            [String(envios), 'envíos esta semana'],
            [stats.data ? String(stats.data.pendientes) : '—', 'borradores por revisar'],
          ].map(([n, t]) => (
            <div key={t} className="flex flex-col-reverse rounded-2xl bg-white/12 px-2 py-2.5">
              <dt className="text-[11px] leading-tight text-[#CFE9E4]">{t}</dt>
              <dd className="font-mono text-[22px] font-bold">{n}</dd>
            </div>
          ))}
        </dl>
      </header>
      <Chips
        label="Filtrar"
        value={filtro}
        onChange={setFiltro}
        options={[
          { value: 'todas', label: 'Todas', count: lista.length },
          { value: 'encendidas', label: 'Encendidas', count: encendidas },
          { value: 'apagadas', label: 'Apagadas', count: disponibles.length - encendidas },
        ]}
      />
    </>
  );
}

type Filtro = 'todas' | 'encendidas' | 'apagadas';

export function AutomationsPage() {
  const canManage = usePermission('automation:manage');
  const mobile = useIsMobile();
  const [filtro, setFiltro] = useState<Filtro>('todas');
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/automations')).automatizaciones });
  const visibles = (q.data ?? []).filter(
    (a) => !mobile || filtro === 'todas' || (filtro === 'encendidas' ? a.activa && !a.proximamente : !a.activa || a.proximamente),
  );
  const runs = useQuery({
    queryKey: [...key, 'ejecuciones'],
    enabled: canManage,
    queryFn: async () => unwrap(await api.GET('/automations/runs')).ejecuciones,
  });
  return (
    <>
      {mobile ? (
        <MobileHeader lista={q.data ?? []} filtro={filtro} setFiltro={setFiltro} />
      ) : (
        <header>
          <h1 className="text-[28px] font-extrabold">Automatizaciones</h1>
          <p className="text-ink-2">Enciende lo que quieras que pase solo. Cada envío por WhatsApp cuenta como un mensaje.</p>
        </header>
      )}
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      <ul className="grid gap-3 lg:grid-cols-2">
        {visibles.map((a) => (
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
