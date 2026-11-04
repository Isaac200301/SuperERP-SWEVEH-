import type { Schemas } from '../../shared/api/client';
import { formatoHora } from '../../shared/lib/format';

export type Mensaje = Schemas['Mensaje'];
type Boton = { id: string; titulo: string };

/** Quién envió cada mensaje (HU-24): el cliente, un aviso automático, el asistente o una persona del taller. */
export function quien(m: Mensaje): string {
  if (m.direccion === 'entrante') return 'Cliente';
  switch (m.origen) {
    case 'automatizacion':
      return `Automático${m.automatizacion ? ` · ${m.automatizacion}` : ''}`;
    case 'agente':
      return m.automatizacion === 'consentimiento' ? 'Autorización de datos' : 'Asistente';
    case 'persona':
      return 'Taller';
    default:
      return m.origen;
  }
}

const estadoTexto: Record<string, string> = { pendiente: 'Enviando…', enviado: 'Enviado', entregado: 'Entregado', leido: 'Leído', fallido: 'No se entregó' };

export function botones(m: Mensaje): Boton[] {
  return m.payload?.botones ?? [];
}

/** Foto enviada: "repuesto:<id>" es la foto de la vitrina (ADR-0029); "evidencia:<orden>:<id>", una foto de la orden (HU-56). */
function foto(m: Mensaje): string | null {
  const [tipo, a, b] = (m.payload?.imagen ?? '').split(':');
  if (tipo === 'repuesto' && a) return `/api/v1/parts/${a}/photo`;
  if (tipo === 'evidencia' && a && b) return `/api/v1/orders/${a}/evidence/${b}/file`;
  return null;
}

/**
 * Hilo de WhatsApp como lo ve el cliente (docs/10 §4.5): lo suyo a la izquierda y lo del taller a la derecha, con
 * hora, quién lo envió y si no se entregó. Con onPress los botones se pueden pulsar (simulador).
 */
export function Thread({ mensajes, onPress }: { mensajes: Mensaje[]; onPress?: (b: Boton) => void }) {
  if (mensajes.length === 0) return <p className="text-ink-2">Todavía no hay mensajes.</p>;
  return (
    <ol className="flex flex-col gap-2" aria-label="Mensajes de WhatsApp">
      {mensajes.map((m) => {
        const mio = m.direccion === 'saliente';
        const bs = botones(m);
        return (
          <li key={m.id} className={`flex max-w-[85%] flex-col gap-1 ${mio ? 'self-end items-end' : 'self-start items-start'}`}>
            <div className={`rounded-2xl px-4 py-2 text-[15px] ${mio ? 'bg-[#DCF8C6]' : 'bg-white ring-1 ring-line'} ${m.estado === 'fallido' ? 'ring-2 ring-err' : ''}`}>
              {foto(m) ? <img src={foto(m) ?? ''} alt={m.cuerpo} className="mb-1 max-h-56 rounded-xl bg-white object-contain" /> : null}
              <p className="whitespace-pre-wrap">{m.cuerpo}</p>
              {bs.length > 0 ? (
                <div className="mt-2 flex flex-wrap gap-2">
                  {bs.map((b) =>
                    onPress ? (
                      <button
                        key={b.id}
                        type="button"
                        onClick={() => onPress(b)}
                        className="min-h-[40px] rounded-full bg-white px-4 text-sm font-bold text-primary ring-1 ring-line"
                      >
                        {b.titulo}
                      </button>
                    ) : (
                      <span key={b.id} className="rounded-full bg-white px-3 py-1 text-sm font-bold text-primary ring-1 ring-line">
                        {b.titulo}
                      </span>
                    ),
                  )}
                </div>
              ) : null}
            </div>
            <span className="text-xs text-ink-2">
              {quien(m)} · {formatoHora(m.creadoEn)}
              {mio ? ` · ${estadoTexto[m.estado] ?? m.estado}` : ''}
            </span>
            {m.estado === 'fallido' && m.error ? <span className="text-xs font-bold text-err">{m.error}</span> : null}
          </li>
        );
      })}
    </ol>
  );
}
