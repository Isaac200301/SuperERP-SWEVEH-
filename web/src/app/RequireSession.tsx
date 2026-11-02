import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { Navigate, useLocation } from 'react-router';

import { sessionKey, useSession } from '../features/auth/api';
import { PanelLayout } from '../features/panel/PanelLayout';
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
  return <PanelLayout session={q.data} />;
}
