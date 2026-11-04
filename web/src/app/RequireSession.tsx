import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { Navigate, useLocation } from 'react-router';

import { can, sessionKey, useSession } from '../features/auth/api';
import { PanelLayout } from '../features/panel/PanelLayout';
import { api, unwrap, type Sesion } from '../shared/api/client';
import { Alert } from '../shared/ui/Alert';

/** Protege el panel: sin sesión manda a /entrar y conserva la ruta para volver. */
export function RequireSession() {
  const q = useSession();
  const qc = useQueryClient();
  const loc = useLocation();

  useEffect(() => {
    const onEnd = () => qc.setQueryData(sessionKey, null);
    window.addEventListener('sweveh:sesion-terminada', onEnd);
    return () => window.removeEventListener('sweveh:sesion-terminada', onEnd);
  }, [qc]);

  if (q.isPending) {
    return (
      <p role="status" className="p-8 text-ink-2">
        Abriendo tu taller…
      </p>
    );
  }
  if (q.isError) {
    return (
      <div className="p-8">
        <Alert title="No pudimos abrir tu sesión">{q.error.message}</Alert>
      </div>
    );
  }
  if (q.data === null) {
    return <Navigate to={`/entrar?volver=${encodeURIComponent(loc.pathname + loc.search)}`} replace />;
  }
  return <Panel session={q.data} />;
}

/** Un taller sin arranque va primero a /panel/arranque (pasos 2 y 3 del registro, HU-03 a HU-05). */
function Panel({ session }: { session: Sesion }) {
  const loc = useLocation();
  const dueno = can(session, 'workshop:manage');
  const t = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')), enabled: dueno });
  if (dueno && t.data && !t.data.onboardingCompleto && loc.pathname !== '/panel/arranque') {
    return <Navigate to="/panel/arranque" replace />;
  }
  return <PanelLayout session={session} soloYo={t.data?.tamano === 'solo'} />;
}
