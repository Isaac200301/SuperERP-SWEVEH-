import { useQuery } from '@tanstack/react-query';

import { api, unwrap } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';

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
    </>
  );
}
