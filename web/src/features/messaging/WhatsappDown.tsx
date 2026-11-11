import { useQuery } from '@tanstack/react-query';
import { Link, useLocation } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';

const motivo: Record<string, string> = {
  FAILED: 'WhatsApp cerró la sesión del número de prueba.',
  STOPPED: 'El número de prueba está apagado.',
  NOT_FOUND: 'El número de prueba ya no está vinculado.',
  SCAN_QR_CODE: 'El número de prueba espera que escanees el código QR.',
  SIN_CONEXION: 'El servidor de WhatsApp de prueba (WAHA) no responde.',
};

/**
 * Aviso en todo el panel cuando el taller responde por el WhatsApp de prueba (WAHA, ADR-0027) y la sesión se cayó:
 * sin él, el asistente "no responde" y nadie se entera (los envíos se quedan reintentando). Solo para quien puede
 * reconectarlo (workshop:manage); revisa cada minuto.
 */
export function WhatsappDown({ enabled }: { enabled: boolean }) {
  const { pathname } = useLocation();
  const canal = useQuery({
    queryKey: ['canal'],
    enabled,
    retry: false,
    queryFn: async () => unwrap(await api.GET('/whatsapp/channel')),
  });
  const usaWaha = canal.data?.canal?.proveedor === 'waha';
  const waha = useQuery({
    queryKey: ['waha'],
    enabled: enabled && usaWaha,
    retry: false,
    refetchInterval: 60_000,
    queryFn: async () => unwrap(await api.GET('/whatsapp/waha')),
  });
  const estado = waha.data?.estado;
  // En Mi taller ya está la tarjeta con el estado y el QR.
  if (!usaWaha || !waha.data?.configurado || !estado || estado === 'WORKING' || estado === 'STARTING' || pathname === '/panel/taller') {
    return null;
  }
  return (
    <Alert title="WhatsApp desconectado: los mensajes no están saliendo">
      {motivo[estado] ?? 'El número de prueba no está conectado.'} El asistente no puede responder hasta que lo vuelvas a
      vincular.{' '}
      <Link to="/panel/taller" className="font-bold underline">
        Reconectar en Mi taller
      </Link>
    </Alert>
  );
}
