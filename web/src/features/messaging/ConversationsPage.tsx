import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { formatoCelular, formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { Thread } from './Thread';

type Conversacion = Schemas['Conversacion'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';

/** Primero las que esperan a una persona (HU-59), después las más recientes. */
function orden(a: Conversacion, b: Conversacion): number {
  if (a.pideHumano !== b.pideHumano) return a.pideHumano ? -1 : 1;
  return (b.ultimoEn ?? '').localeCompare(a.ultimoEn ?? '');
}

/** Conversaciones de WhatsApp del taller: de aquí el dueño atiende al cliente que pidió una persona. */
export function ConversationsPage() {
  const list = useQuery({
    queryKey: ['conversaciones'],
    queryFn: async () => unwrap(await api.GET('/conversations')).conversaciones,
    refetchInterval: 10_000,
  });
  const convs = [...(list.data ?? [])].sort(orden);
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Conversaciones</h1>
        <p className="text-ink-2">Los mensajes de WhatsApp de tus clientes. Si alguien pidió hablar con una persona, aparece primero.</p>
      </header>
      {list.isError ? <Alert>{list.error.message}</Alert> : null}
      {list.data && convs.length === 0 ? <p className="text-ink-2">Todavía no te ha escrito nadie por WhatsApp.</p> : null}
      <ul className="flex flex-col gap-2">
        {convs.map((c) => (
          <li key={c.id}>
            <Link to={`/panel/conversaciones/${c.id}`} className={`${card} hover:ring-2 hover:ring-primary`}>
              <span className="flex flex-wrap items-center justify-between gap-2">
                <b className="font-mono">{formatoCelular(c.celular)}</b>
                {c.pideHumano ? (
                  <span className="rounded-full bg-err-tint px-3 py-1 text-sm font-bold text-err-ink">Pidió una persona</span>
                ) : c.modo === 'humano' ? (
                  <span className="rounded-full bg-warn-tint px-3 py-1 text-sm font-bold text-warn-ink">La atiendes tú</span>
                ) : null}
              </span>
              {c.ultimoMensaje ? <span className="truncate text-ink-2">{c.ultimoMensaje}</span> : null}
              {c.ultimoEn ? <span className="text-xs text-ink-2">{formatoFechaHora(c.ultimoEn)}</span> : null}
            </Link>
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * Una conversación (HU-24, HU-59): el hilo, la respuesta de una persona del taller y el modo. Mientras la atiende
 * una persona el asistente no propone borradores; "Devolver al asistente" lo vuelve a activar.
 */
export function ConversationPage() {
  const { id = '' } = useParams();
  const qc = useQueryClient();
  const [texto, setTexto] = useState('');
  const key = ['conversacion', id];
  const conv = useQuery({
    queryKey: key,
    queryFn: async () => unwrap(await api.GET('/conversations/{id}', { params: { path: { id } } })),
    refetchInterval: 5_000,
  });
  const refrescar = () => Promise.all([qc.invalidateQueries({ queryKey: key }), qc.invalidateQueries({ queryKey: ['conversaciones'] })]);
  const responder = useMutation({
    mutationFn: async (t: string) => unwrap(await api.POST('/conversations/{id}/messages', { params: { path: { id } }, body: { texto: t } })),
    onSuccess: async () => {
      setTexto('');
      await refrescar();
    },
  });
  const modo = useMutation({
    mutationFn: async (m: 'agente' | 'humano') =>
      unwrap(await api.PATCH('/conversations/{id}', { params: { path: { id } }, body: { modo: m } })),
    onSuccess: refrescar,
  });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (texto.trim()) responder.mutate(texto.trim());
  };

  const c = conv.data?.conversacion;
  // Contra la hora en que llegó la conversación (se refresca cada 5 s), no contra Date.now() en el render.
  const abierta = c?.ventanaAbiertaHasta ? new Date(c.ventanaAbiertaHasta).getTime() > conv.dataUpdatedAt : false;
  return (
    <>
      <header className="flex flex-col gap-1">
        <Link to="/panel/conversaciones" className="self-start font-bold text-primary">
          Conversaciones
        </Link>
        <h1 className="text-[28px] font-extrabold">{c ? formatoCelular(c.celular) : 'Conversación'}</h1>
      </header>
      {conv.isError ? <Alert>{conv.error.message}</Alert> : null}
      {c ? (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,560px)_minmax(0,320px)]">
          <section aria-label="Mensajes" className="flex flex-col gap-3 rounded-card bg-[#ECE5DD] p-4">
            <Thread mensajes={conv.data?.mensajes ?? []} />
            {responder.isError ? <Alert>{responder.error.message}</Alert> : null}
            <form onSubmit={onSubmit} className="flex items-end gap-2">
              <div className="flex-1">
                <Field label="Tu respuesta" value={texto} onChange={(e) => setTexto(e.target.value)} />
              </div>
              <Button
                type="submit"
                busy={responder.isPending}
                disabled={!texto.trim()}
                missing={abierta ? undefined : 'El cliente no ha escrito en 24 horas: WhatsApp no deja enviarle texto libre.'}
              >
                Enviar
              </Button>
            </form>
          </section>
          <section className={card} aria-label="Quién atiende">
            {c.modo === 'humano' ? (
              <>
                <p>
                  <b>{c.pideHumano ? 'El cliente pidió hablar con una persona.' : 'Esta conversación la atiendes tú.'}</b> El asistente no le
                  propone respuestas mientras tanto.
                </p>
                <Button variant="secondary" busy={modo.isPending} onClick={() => modo.mutate('agente')}>
                  Devolver al asistente
                </Button>
              </>
            ) : (
              <>
                <p>El asistente propone respuestas en "Por revisar" y contesta solo el estado del vehículo.</p>
                <Button variant="secondary" busy={modo.isPending} onClick={() => modo.mutate('humano')}>
                  Atender yo
                </Button>
              </>
            )}
            {modo.isError ? <Alert>{modo.error.message}</Alert> : null}
          </section>
        </div>
      ) : null}
    </>
  );
}
