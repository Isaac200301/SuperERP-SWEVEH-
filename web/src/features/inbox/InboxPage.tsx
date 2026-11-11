import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { usePermission } from '../auth/api';
import { PriceListReview } from '../catalog/PriceList';
import { audioURL } from '../messaging/Thread';
import { formatoCelular, formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { useIsMobile } from '../../shared/ui/useIsMobile';

type Borrador = Schemas['Borrador'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const key = ['bandeja'] as const;
const campos: Record<string, string> = {
  precio: 'Precio',
  horario: 'Horario',
  otro: 'Pidió algo que no está en tu lista',
  atiende: 'Atiendes',
};

/**
 * HU-62, HU-63 y HU-64: lo que queda aquí sale solo cuando el dueño pulsa Enviar; si edita, sale su texto.
 * "Lo contesto yo" pasa la conversación a una persona (HU-59).
 */
function DraftCard({ d, mobile = false }: { d: Borrador; mobile?: boolean }) {
  const qc = useQueryClient();
  const [texto, setTexto] = useState(d.propuesto);
  const resolver = useMutation({
    mutationFn: async (decision: 'enviar' | 'lo_contesto_yo' | 'descartar') =>
      unwrap(
        await api.POST('/inbox/{id}', {
          params: { path: { id: d.id } },
          body: { decision, ...(decision === 'enviar' && texto !== d.propuesto ? { texto } : {}) },
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });
  if (mobile) return <DraftMobile d={d} texto={texto} setTexto={setTexto} resolver={resolver} />;
  return (
    <li className={card}>
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-ink-2">
        <span className="font-mono">{formatoCelular(d.celular)}</span>
        <span>
          {d.intencion} · {formatoFechaHora(d.creadoEn)}
        </span>
      </div>
      <div className="flex flex-col gap-2 rounded-2xl bg-surface p-3">
        {d.notaDeVoz && d.mensajeId ? (
          <>
            <b>El cliente mandó una nota de voz{d.mensajeCliente ? '. Dice:' : '.'}</b>
            <audio controls preload="none" src={audioURL(d.conversacionId, d.mensajeId)} aria-label="Escuchar la nota de voz" />
            {d.mensajeCliente ? <p>{d.mensajeCliente}</p> : null}
          </>
        ) : (
          <p>
            <b>El cliente escribió:</b> {d.mensajeCliente}
          </p>
        )}
      </div>
      <label htmlFor={`b-${d.id}`} className="text-[15px] font-extrabold">
        Respuesta propuesta (puedes editarla)
      </label>
      <textarea
        id={`b-${d.id}`}
        rows={3}
        className="rounded-btn border-[1.5px] border-line p-3 text-base"
        value={texto}
        onChange={e => setTexto(e.target.value)}
      />
      {d.fuentes && d.fuentes.length > 0 ? (
        <ul className="text-sm text-ink-2" aria-label="De dónde salió cada dato">
          {d.fuentes.map(f => (
            <li key={`${f.campo}-${f.origen}`}>
              {campos[f.campo] ?? f.campo}: {f.valor} · {f.origen}
            </li>
          ))}
        </ul>
      ) : null}
      {resolver.isError ? <Alert>{resolver.error.message}</Alert> : null}
      <div className="flex flex-wrap gap-3">
        <Button
          busy={resolver.isPending}
          missing={texto.trim() ? undefined : 'Escribe la respuesta.'}
          onClick={() => resolver.mutate('enviar')}
        >
          Enviar
        </Button>
        <Button variant="secondary" disabled={resolver.isPending} onClick={() => resolver.mutate('lo_contesto_yo')}>
          Lo contesto yo
        </Button>
        <Button variant="ghost" disabled={resolver.isPending} onClick={() => resolver.mutate('descartar')}>
          Descartar
        </Button>
      </div>
    </li>
  );
}

type Resolver = { mutate: (d: 'enviar' | 'lo_contesto_yo' | 'descartar') => void; isPending: boolean; isError: boolean; error: Error | null };

/**
 * El borrador en el celular (mockup PorRevisar): 1 · lo que escribió, 2 · lo que entendió y de dónde lo sacó (HU-64),
 * 3 · la respuesta editable (HU-63). Enviar es la acción principal y queda fija abajo (DR-05, DR-07).
 */
function DraftMobile({ d, texto, setTexto, resolver }: { d: Borrador; texto: string; setTexto: (t: string) => void; resolver: Resolver }) {
  const paso = 'text-xs font-extrabold tracking-[.06em] text-primary';
  const conFuentes = (d.fuentes?.length ?? 0) > 0;
  return (
    <article className="flex flex-col gap-3" aria-label={`Borrador para ${formatoCelular(d.celular)}`}>
      <div className="flex flex-col rounded-[22px] bg-white p-3.5">
        <b className="font-mono">{formatoCelular(d.celular)}</b>
        <span className="text-[13px] text-ink-2">
          {d.intencion} · {formatoFechaHora(d.creadoEn)}
        </span>
      </div>
      <span className={paso}>1 · LO QUE ESCRIBIÓ</span>
      <div className="flex flex-col gap-2 self-start rounded-[18px] rounded-tl-md bg-white p-3.5 text-[15px]">
        {d.notaDeVoz && d.mensajeId ? (
          <>
            <b>Nota de voz{d.mensajeCliente ? '. Dice:' : '.'}</b>
            <audio controls preload="none" src={audioURL(d.conversacionId, d.mensajeId)} aria-label="Escuchar la nota de voz" className="max-w-full" />
          </>
        ) : null}
        {d.mensajeCliente ? <p>{d.mensajeCliente}</p> : null}
      </div>
      {conFuentes ? (
        <details open className="rounded-[18px] bg-white p-3.5">
          <summary className={`${paso} min-h-[32px] cursor-pointer`}>2 · LO QUE ENTENDIÓ Y DE DÓNDE LO SACÓ</summary>
          <ul className="mt-2 flex flex-col gap-2 text-sm" aria-label="De dónde salió cada dato">
            {(d.fuentes ?? []).map((f) => (
              <li key={`${f.campo}-${f.origen}`} className="flex justify-between gap-3 border-t border-surface pt-2">
                <span className="text-ink-2">{campos[f.campo] ?? f.campo}</span>
                <span className="text-right">
                  {f.valor}
                  <br />
                  <span className="rounded-full bg-primary-tint px-2 text-xs font-bold text-primary">{f.origen}</span>
                </span>
              </li>
            ))}
          </ul>
        </details>
      ) : null}
      <label htmlFor={`m-${d.id}`} className={paso}>
        {conFuentes ? '3' : '2'} · BORRADOR DE RESPUESTA
      </label>
      <textarea
        id={`m-${d.id}`}
        rows={5}
        className="rounded-[18px] border-[1.5px] border-line bg-white p-3.5 text-base"
        value={texto}
        onChange={(e) => setTexto(e.target.value)}
      />
      <p className="text-xs text-ink-2">El asistente solo usa tus precios y horarios. Nada sale hasta que pulses Enviar.</p>
      {resolver.isError ? <Alert>{resolver.error?.message}</Alert> : null}
      <div className="sticky bottom-[calc(88px+env(safe-area-inset-bottom))] flex flex-col gap-2 rounded-[22px] bg-white p-3 shadow-[0_-4px_20px_rgba(15,31,29,.08)]">
        <Button busy={resolver.isPending} missing={texto.trim() ? undefined : 'Escribe la respuesta.'} onClick={() => resolver.mutate('enviar')}>
          Enviar
        </Button>
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            disabled={resolver.isPending}
            onClick={() => resolver.mutate('lo_contesto_yo')}
            className="min-h-[44px] rounded-btn bg-surface text-sm font-extrabold text-primary"
          >
            Lo contesto yo
          </button>
          <button
            type="button"
            disabled={resolver.isPending}
            onClick={() => resolver.mutate('descartar')}
            className="min-h-[44px] rounded-btn bg-surface text-sm font-extrabold text-ink-2"
          >
            Descartar
          </button>
        </div>
      </div>
    </article>
  );
}

/** Por revisar en el celular: se elige un borrador arriba y se trabaja uno a la vez. */
function InboxMobile() {
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/inbox')).borradores, refetchInterval: 10_000 });
  const [sel, setSel] = useState<string | null>(null);
  const lista = q.data ?? [];
  const actual = lista.find((d) => d.id === sel) ?? lista[0];
  return (
    <>
      <header className="-mx-4 -mt-4 flex flex-col gap-3 rounded-b-[28px] bg-primary px-[18px] pt-5 pb-4 text-white">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-[-.02em]">Por revisar</h1>
          <p className="text-[13px] text-[#CFE9E4]">Lo que el asistente propone. Nada sale hasta que tú lo envíes.</p>
        </div>
        {lista.length > 1 ? (
          <div role="radiogroup" aria-label="Elegir borrador" className="-mx-[18px] flex gap-2 overflow-x-auto px-[18px] [scrollbar-width:none]">
            {lista.map((d) => {
              const on = d.id === actual?.id;
              return (
                <button
                  key={d.id}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  onClick={() => setSel(d.id)}
                  className={`flex min-h-[52px] flex-none flex-col items-start justify-center rounded-2xl px-3 text-left ${on ? 'bg-white text-ink' : 'bg-white/15 text-white'}`}
                >
                  <b className="font-mono text-sm">{formatoCelular(d.celular)}</b>
                  <span className="text-xs opacity-80">{formatoFechaHora(d.creadoEn)}</span>
                </button>
              );
            })}
          </div>
        ) : null}
      </header>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.isSuccess && lista.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-ink-2">No hay nada por revisar.</p> : null}
      {actual ? <DraftCard key={actual.id} d={actual} mobile /> : null}
      <PriceListReview />
      <AutoReply />
    </>
  );
}

/**
 * ADR-0028 / HU-67: el asistente responde solo (encendido de fábrica). Se detiene si el cliente pide una persona o
 * el taller toma la conversación. Solo el dueño lo apaga; el resto lo ve.
 */
function AutoReply() {
  const qc = useQueryClient();
  const puede = usePermission('automation:manage');
  const s = useQuery({ queryKey: [...key, 'auto'], queryFn: async () => unwrap(await api.GET('/assistant/settings')) });
  const recientes = useQuery({
    queryKey: [...key, 'todos'],
    enabled: s.data?.autoEnvio === true,
    refetchInterval: 10_000,
    queryFn: async () =>
      unwrap(await api.GET('/inbox', { params: { query: { todos: '1' } } }))
        .borradores.filter(d => d.estado === 'auto_enviado')
        .slice(0, 5),
  });
  const guardar = useMutation({
    mutationFn: async (autoEnvio: boolean) => unwrap(await api.PUT('/assistant/settings', { body: { autoEnvio } })),
    onSuccess: d => qc.setQueryData([...key, 'auto'], d),
  });
  if (!s.data) return null;
  const on = s.data.autoEnvio;
  return (
    <section className={card} aria-labelledby="auto">
      <label className="flex min-h-[44px] items-center gap-2 font-bold">
        <input
          type="checkbox"
          role="switch"
          aria-checked={on}
          checked={on}
          disabled={!puede || guardar.isPending}
          onChange={e => guardar.mutate(e.target.checked)}
        />
        <span id="auto">El asistente responde solo</span>
      </label>
      <p className="text-ink-2">
        {on
          ? 'Las respuestas salen sin tu revisión, con los precios y horarios de tu lista. Si un cliente pide una persona o tomas la conversación, el asistente se detiene.'
          : 'Apagado: cada respuesta espera aquí hasta que la envíes.'}
        {puede ? '' : ' Solo el dueño lo cambia.'}
      </p>
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
      {on && recientes.data && recientes.data.length > 0 ? (
        <details>
          <summary className="cursor-pointer font-bold text-primary">Últimas respuestas automáticas</summary>
          <ul className="mt-2 flex flex-col gap-2">
            {recientes.data.map(d => (
              <li key={d.id} className="rounded-2xl bg-surface p-3 text-sm">
                <span className="font-mono">{formatoCelular(d.celular)}</span> · {formatoFechaHora(d.creadoEn)}
                <br />
                <b>Escribió:</b> {d.mensajeCliente}
                <br />
                <b>Automática:</b> {d.final ?? d.propuesto}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </section>
  );
}

export function InboxPage() {
  if (useIsMobile()) return <InboxMobile />;
  return <InboxDesktop />;
}

function InboxDesktop() {
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/inbox')).borradores, refetchInterval: 10_000 });
  const stats = useQuery({ queryKey: [...key, 'stats'], queryFn: async () => unwrap(await api.GET('/inbox/stats')) });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Por revisar</h1>
        <p className="text-ink-2">
          Lo que el asistente no puede responder solo espera aquí.
          {stats.data && stats.data.enviados > 0
            ? ` Editaste ${Math.round(stats.data.porcentajeEditados)} % de los ${stats.data.enviados} enviados.`
            : ''}
        </p>
      </header>
      <AutoReply />
      <PriceListReview />
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data?.length === 0 ? <p className="rounded-card bg-white p-5 text-ink-2">No hay nada por revisar.</p> : null}
      <ul className="flex flex-col gap-3">
        {(q.data ?? []).map(d => (
          <DraftCard key={d.id} d={d} />
        ))}
      </ul>
      <p className="text-sm text-ink-2">
        ¿Quieres ver cómo llega un mensaje?{' '}
        <Link to="/panel/whatsapp-prueba" className="font-bold text-primary">
          Probar WhatsApp como cliente
        </Link>
      </p>
    </>
  );
}
