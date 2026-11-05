import { useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap } from '../../shared/api/client';
import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

const estados: Record<string, string> = {
  recibido: 'Recibido', pendiente: 'Pendiente de envío', enviado: 'Enviado', entregado: 'Entregado', leido: 'Leído', fallido: 'Falló',
};

/**
 * HU-100: registro técnico de mensajes. Por número, qué entró y salió en cada taller con su estado, error y hora, y si
 * el webhook llegó y se procesó. No muestra el texto de los mensajes.
 */
function MessageLog() {
  const [celular, setCelular] = useState('');
  const [buscar, setBuscar] = useState('');
  const q = useQuery({
    queryKey: ['plataforma', 'mensajes', buscar],
    enabled: buscar !== '',
    retry: false,
    queryFn: async () => unwrap(await api.GET('/platform/messages', { params: { query: { celular: buscar } } })),
  });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    setBuscar(celular.trim());
  };
  return (
    <section className="flex flex-col gap-3 rounded-card bg-white p-5" aria-labelledby="registro">
      <h2 id="registro" className="text-[17px] font-extrabold">Registro técnico de mensajes</h2>
      <form onSubmit={onSubmit} className="flex flex-wrap items-end gap-2">
        <Field label="Celular del cliente" inputMode="tel" value={celular} onChange={(e) => setCelular(e.target.value)} />
        <Button type="submit" variant="secondary" missing={celular.trim() === '' ? 'Escribe el celular.' : undefined}>Buscar</Button>
      </form>
      {q.isFetching ? <p role="status">Buscando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data ? (
        <>
          {q.data.mensajes.length === 0 ? <p className="text-ink-2">Ningún taller tiene mensajes con {q.data.celular}.</p> : (
            <table className="w-full text-left text-sm">
              <thead className="text-ink-2">
                <tr><th className="py-2">Hora</th><th className="py-2">Taller</th><th className="py-2">Sentido</th><th className="py-2">Estado</th><th className="py-2">Error</th></tr>
              </thead>
              <tbody>
                {q.data.mensajes.map((m, i) => (
                  <tr key={`${m.fecha}-${i}`} className="border-t border-line">
                    <td className="py-2 font-mono">{formatoFechaHora(m.fecha)}</td>
                    <td className="py-2">{m.taller}</td>
                    <td className="py-2">{m.direccion === 'entrante' ? 'Entró' : 'Salió'} · {m.tipo}</td>
                    <td className={`py-2 font-bold ${m.estado === 'fallido' ? 'text-err-ink' : ''}`}>{estados[m.estado] ?? m.estado}</td>
                    <td className="py-2 text-err-ink">{m.error ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <h3 className="font-extrabold">Webhooks de los últimos {q.data.diasWebhooks} días</h3>
          {q.data.webhooks.length === 0 ? <p className="text-ink-2">Ningún webhook menciona ese número.</p> : (
            <ul className="flex flex-col gap-1 text-sm">
              {q.data.webhooks.map((w) => (
                <li key={w.id} className="flex flex-wrap gap-3">
                  <span className="font-mono">{formatoFechaHora(w.recibido)}</span>
                  <span>{w.procesado ? `Procesado ${formatoFechaHora(w.procesado)}` : 'Sin procesar'}</span>
                  {w.error ? <span className="font-bold text-err-ink">{w.error}</span> : null}
                </li>
              ))}
            </ul>
          )}
        </>
      ) : null}
    </section>
  );
}

/**
 * HU-74 (ADR-0030): el medidor del administrador del ERP. Por taller, los mensajes enviados del mes contra los incluidos
 * y el costo de IA en pesos, el que más cuesta primero. El servidor solo responde a administradores del ERP; un
 * usuario de taller recibe "sin permiso".
 */
export function PlatformPage() {
  const q = useQuery({ queryKey: ['plataforma'], retry: false, queryFn: async () => unwrap(await api.GET('/platform/usage')) });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Consumo de los talleres</h1>
        <p className="text-ink-2">
          Mensajes enviados en el mes y costo de la IA.{' '}
          {q.data ? `Mes ${q.data.mes}; ${q.data.incluidos} mensajes incluidos por taller.` : ''}
        </p>
      </header>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data ? (
        <section className="rounded-card bg-white p-5" aria-label="Talleres">
          <table className="w-full text-left">
            <thead className="text-sm text-ink-2">
              <tr>
                <th className="py-2">Taller</th>
                <th className="py-2">Mensajes del mes</th>
                <th className="py-2">Costo de IA</th>
              </tr>
            </thead>
            <tbody>
              {q.data.talleres.map(t => (
                <tr key={t.id} className="border-t border-line">
                  <td className="py-2 font-bold">{t.nombre}</td>
                  <td className={`py-2 ${t.sePasa ? 'font-bold text-err-ink' : ''}`}>
                    {t.mensajesTexto}
                    {t.sePasa ? ' · se pasó' : ''}
                  </td>
                  <td className="py-2">{t.iaTexto}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-3 text-sm text-ink-2">
            Los mensajes incluidos y los pesos por dólar ({q.data.pesosPorDolar}) son provisionales hasta definir el plan (D-04, Q-24).
          </p>
        </section>
      ) : null}
      {q.data ? <MessageLog /> : null}
    </>
  );
}
