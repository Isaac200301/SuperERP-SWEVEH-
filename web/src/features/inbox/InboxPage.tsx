import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { celularValido, formatoCelular, formatoFechaHora, soloCelular } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

type Borrador = Schemas['Borrador'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const key = ['bandeja'] as const;

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
      {d.fuentes && d.fuentes.length > 0 ? <p className="text-sm text-ink-2">Datos tomados de: {d.fuentes.join(', ')}</p> : null}
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

/** Simulador del número de prueba (D-01): escribe como si fueras el cliente, sin cuenta de Meta. */
function Simulator() {
  const qc = useQueryClient();
  const [celular, setCelular] = useState('');
  const [texto, setTexto] = useState('');
  const enviar = useMutation({
    mutationFn: async () => unwrap(await api.POST('/simulator/inbound', { body: { celular: soloCelular(celular), nombre: 'Cliente de prueba', texto } })),
    onSuccess: () => {
      setTexto('');
      setTimeout(() => void qc.invalidateQueries({ queryKey: key }), 1500);
    },
  });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    enviar.mutate();
  };
  return (
    <section className={card} aria-labelledby="simulador">
      <h2 id="simulador" className="text-[17px] font-extrabold">
        Probar como cliente (número de prueba)
      </h2>
      <p className="text-sm text-ink-2">Solo existe fuera de producción. Usa celulares ficticios.</p>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-[200px_1fr_auto] sm:items-end">
        <Field label="Celular del cliente" inputMode="tel" value={celular} onChange={(e) => setCelular(e.target.value)} />
        <Field label="Mensaje" value={texto} onChange={(e) => setTexto(e.target.value)} placeholder="¿Ya está mi carro?" />
        <Button
          type="submit"
          variant="secondary"
          busy={enviar.isPending}
          missing={!celularValido(celular) ? 'Celular de 10 dígitos.' : texto.trim() ? undefined : 'Escribe el mensaje.'}
        >
          Enviar por WhatsApp
        </Button>
      </form>
      {enviar.isError ? <Alert>{enviar.error.message}</Alert> : null}
      {enviar.isSuccess ? <p role="status">Mensaje recibido. El borrador aparece en unos segundos.</p> : null}
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
      <Simulator />
    </>
  );
}
