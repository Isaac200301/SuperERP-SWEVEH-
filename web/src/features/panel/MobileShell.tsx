import { useEffect, useState } from 'react';
import { Outlet, useLocation } from 'react-router';

import type { Sesion } from '../../shared/api/client';
import { can } from '../auth/api';
import { usePendingCards } from '../inbox/usePendingCards';
import { WhatsappDown } from '../messaging/WhatsappDown';
import { NewOrderSheet } from '../orders/NewOrderSheet';
import { nuevaOrdenEvento, TabBar } from './TabBar';

/** Pantallas de un solo trabajo, sin barra inferior: la orden abierta y la recepción (mockups Orden y PisoChecklist). */
function fullScreen(path: string): boolean {
  return /^\/panel\/ordenes\/(?!nueva$)[^/]+$/.test(path) || path.startsWith('/panel/piso/recibir');
}

/** El panel en el celular (mockups móviles v1): contenido arriba y barra inferior con las pestañas del rol. */
export function MobileShell({ session }: { session: Sesion }) {
  const loc = useLocation();
  // HU-66: el contador de Por revisar también va en la barra del celular.
  const pending = usePendingCards(can(session, 'inbox:manage'));
  const [nueva, setNueva] = useState(false);
  useEffect(() => {
    const open = () => setNueva(true);
    window.addEventListener(nuevaOrdenEvento, open);
    return () => window.removeEventListener(nuevaOrdenEvento, open);
  }, []);
  const bar = !fullScreen(loc.pathname);
  return (
    <div className={`min-h-dvh bg-surface-2 ${bar ? 'pb-[calc(84px+env(safe-area-inset-bottom))]' : ''}`}>
      <main className="flex min-w-0 flex-col gap-4 px-4 pt-4 pb-4">
        <WhatsappDown enabled={can(session, 'workshop:manage')} />
        <Outlet />
      </main>
      {bar ? <TabBar session={session} pending={pending} /> : null}
      {can(session, 'order:create') ? <NewOrderSheet open={nueva} onClose={() => setNueva(false)} /> : null}
    </div>
  );
}
