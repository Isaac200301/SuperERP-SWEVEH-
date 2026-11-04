import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { card } from '../workshop/ProfileForm';

const estados: Record<string, string> = {
  NOT_FOUND: 'Sin conectar',
  STOPPED: 'Apagado',
  STARTING: 'Arrancando…',
  SCAN_QR_CODE: 'Esperando que escanees el código',
  WORKING: 'Conectado',
  FAILED: 'Se desconectó',
  SIN_CONEXION: 'WAHA no responde',
};

/** "573009999999@c.us" → "+57 300 999 9999". */
function numero(chat?: string) {
  const d = (chat ?? '').split('@')[0] ?? '';
  return d.length === 12 && d.startsWith('57') ? `+57 ${d.slice(2, 5)} ${d.slice(5, 8)} ${d.slice(8)}` : d;
}

/**
 * WhatsApp de prueba para la demo (WAHA, no oficial, ADR-0027): se vincula un número de prueba escaneando el QR y
 * el taller pasa a responder por ahí. Solo aparece si el servidor tiene WAHA configurado y nunca en producción.
 */
export function WahaCard() {
  const qc = useQueryClient();
  const s = useQuery({
    queryKey: ['waha'],
    retry: false,
    queryFn: async () => unwrap(await api.GET('/whatsapp/waha')),
    // El QR cambia cada 20 a 60 segundos y el estado pasa solo a Conectado al escanear.
    refetchInterval: (q) => (['SCAN_QR_CODE', 'STARTING'].includes(q.state.data?.estado ?? '') ? 3000 : false),
  });
  const canal = useQuery({ queryKey: ['canal'], retry: false, queryFn: async () => unwrap(await api.GET('/whatsapp/channel')) });
  const arrancar = useMutation({
    mutationFn: async () => unwrap(await api.POST('/whatsapp/waha/start')),
    onSuccess: (d) => qc.setQueryData(['waha'], d),
  });
  const usar = useMutation({
    mutationFn: async (proveedor: 'waha' | 'prueba_local') => unwrap(await api.PUT('/whatsapp/channel', { body: { proveedor } })),
    onSuccess: (d) => qc.setQueryData(['canal'], d),
  });

  if (!s.data?.configurado) return null;
  const estado = s.data.estado ?? 'NOT_FOUND';
  const enUso = canal.data?.canal?.proveedor === 'waha';
  const error = arrancar.error ?? usar.error;

  return (
    <section className={card} aria-labelledby="waha">
      <h2 id="waha" className="text-[17px] font-extrabold">WhatsApp de prueba</h2>
      <p className="text-ink-2">
        Solo para la demo: es un WhatsApp no oficial. Usa un número de prueba, nunca el del taller, y no escribas datos de clientes reales.
      </p>
      <p>
        Estado: <strong>{estados[estado] ?? estado}</strong>
        {estado === 'WORKING' && s.data.numero ? ` · ${numero(s.data.numero)}` : null}
      </p>

      {['NOT_FOUND', 'STOPPED', 'FAILED'].includes(estado) ? (
        <Button variant="secondary" busy={arrancar.isPending} onClick={() => arrancar.mutate()}>
          Conectar número de prueba
        </Button>
      ) : null}
      {s.data.qr ? (
        <figure className="flex flex-col items-start gap-2">
          <img src={s.data.qr} alt="Código QR para vincular el WhatsApp de prueba" className="h-64 w-64 rounded-btn border border-line bg-white p-2" />
          <figcaption className="text-ink-2">En el celular de prueba: WhatsApp → Dispositivos vinculados → Vincular un dispositivo.</figcaption>
        </figure>
      ) : null}
      {estado === 'WORKING' && !enUso ? (
        <Button busy={usar.isPending} onClick={() => usar.mutate('waha')}>
          Responder por este número
        </Button>
      ) : null}
      {enUso ? (
        <>
          <p role="status" className="font-bold text-primary">El taller está respondiendo por el WhatsApp de prueba.</p>
          <Button variant="secondary" busy={usar.isPending} onClick={() => usar.mutate('prueba_local')}>
            Volver al simulador
          </Button>
        </>
      ) : null}
      {error ? <Alert>{error.message}</Alert> : null}
    </section>
  );
}
