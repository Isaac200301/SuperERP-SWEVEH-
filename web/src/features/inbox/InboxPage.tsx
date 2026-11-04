import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { formatoCelular, formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';

type Borrador = Schemas['Borrador'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const key = ['bandeja'] as const;
const campos: Record<string, string> = { precio: 'Precio', horario: 'Horario' };

/**
 * HU-62, HU-63 y HU-64: la IA propone, el dueño decide. Nada sale al cliente sin pulsar Enviar (RF-18);
 * si edita, sale su texto. "Lo contesto yo" pasa la conversación a una persona (HU-59).
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

export function InboxPage() {
  const q = useQuery({ queryKey: key, queryFn: async () => unwrap(await api.GET('/inbox')).borradores, refetchInterval: 10_000 });
  const stats = useQuery({ queryKey: [...key, 'stats'], queryFn: async () => unwrap(await api.GET('/inbox/stats')) });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Por revisar</h1>
        <p className="text-ink-2">
          El agente prepara la respuesta; tú decides si sale.
          {stats.data && stats.data.enviados > 0 ? ` Editaste ${Math.round(stats.data.porcentajeEditados)} % de los ${stats.data.enviados} enviados.` : ''}
        </p>
      </header>
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
