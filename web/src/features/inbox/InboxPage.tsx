import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { usePermission } from '../auth/api';
import { formatoCelular, formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';

type Borrador = Schemas['Borrador'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const key = ['bandeja'] as const;
const campos: Record<string, string> = { precio: 'Precio', horario: 'Horario', otro: 'Pidió algo que no está en tu lista', atiende: 'Atiendes' };

/**
 * HU-62, HU-63 y HU-64: lo que queda aquí sale solo cuando el dueño pulsa Enviar; si edita, sale su texto.
 * "Lo contesto yo" pasa la conversación a una persona (HU-59).
 */
function DraftCard({ d }: { d: Borrador }) {
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
  return (
    <li className={card}>
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-ink-2">
        <span className="font-mono">{formatoCelular(d.celular)}</span>
        <span>
          {d.intencion} · {formatoFechaHora(d.creadoEn)}
        </span>
      </div>
      <p className="rounded-2xl bg-surface p-3">
        <b>El cliente escribió:</b> {d.mensajeCliente}
      </p>
      <label htmlFor={`b-${d.id}`} className="text-[15px] font-extrabold">
        Respuesta propuesta (puedes editarla)
      </label>
      <textarea
        id={`b-${d.id}`}
        rows={3}
        className="rounded-btn border-[1.5px] border-line p-3 text-base"
        value={texto}
        onChange={(e) => setTexto(e.target.value)}
      />
      {d.fuentes && d.fuentes.length > 0 ? (
        <ul className="text-sm text-ink-2" aria-label="De dónde salió cada dato">
          {d.fuentes.map((f) => (
            <li key={`${f.campo}-${f.origen}`}>
              {campos[f.campo] ?? f.campo}: {f.valor} · {f.origen}
            </li>
          ))}
        </ul>
      ) : null}
      {resolver.isError ? <Alert>{resolver.error.message}</Alert> : null}
      <div className="flex flex-wrap gap-3">
        <Button busy={resolver.isPending} missing={texto.trim() ? undefined : 'Escribe la respuesta.'} onClick={() => resolver.mutate('enviar')}>
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
        .borradores.filter((d) => d.estado === 'auto_enviado')
        .slice(0, 5),
  });
  const guardar = useMutation({
    mutationFn: async (autoEnvio: boolean) => unwrap(await api.PUT('/assistant/settings', { body: { autoEnvio } })),
    onSuccess: (d) => qc.setQueryData([...key, 'auto'], d),
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
          onChange={(e) => guardar.mutate(e.target.checked)}
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
            {recientes.data.map((d) => (
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
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/inbox')).borradores, refetchInterval: 10_000 });
  const stats = useQuery({ queryKey: [...key, 'stats'], queryFn: async () => unwrap(await api.GET('/inbox/stats')) });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Por revisar</h1>
        <p className="text-ink-2">
          Lo que el asistente no puede responder solo espera aquí.
          {stats.data && stats.data.enviados > 0 ? ` Editaste ${Math.round(stats.data.porcentajeEditados)} % de los ${stats.data.enviados} enviados.` : ''}
        </p>
      </header>
      <AutoReply />
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data?.length === 0 ? <p className="rounded-card bg-white p-5 text-ink-2">No hay nada por revisar.</p> : null}
      <ul className="flex flex-col gap-3">
        {(q.data ?? []).map((d) => (
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
