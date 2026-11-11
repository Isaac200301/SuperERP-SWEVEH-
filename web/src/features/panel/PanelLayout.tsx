import { NavLink, Outlet, useNavigate } from 'react-router';

import type { Sesion } from '../../shared/api/client';
import { roles } from '../../shared/i18n/es-CO';
import { Brand } from '../../shared/ui/Logo';
import { IconLogout } from '../../shared/ui/icons';
import { can, useLogout } from '../auth/api';
import { usePendingCards } from '../inbox/usePendingCards';
import { sectionsFor } from './sections';

/** soloYo: el taller dijo "solo yo" en el registro y no ve la gestión de usuarios (HU-83). */
export function PanelLayout({ session, soloYo = false }: { session: Sesion; soloYo?: boolean }) {
  const navigate = useNavigate();
  const logout = useLogout();
  // HU-66: contador y sonido cuando llega una tarjeta a Por revisar (dueño y recepción).
  const pending = usePendingCards(can(session, 'inbox:manage'));
  return (
    <div className="mx-auto flex min-h-screen max-w-[1600px] flex-col gap-5 p-4 md:flex-row md:p-5">
      <aside className="flex flex-col gap-5 md:w-[240px] md:flex-none">
        <div className="px-1.5 pt-1.5">
          <Brand subtitle={session.taller.nombre} />
        </div>
        <nav aria-label="Secciones del panel" className="flex flex-row gap-2 md:flex-col">
          {sectionsFor(session, soloYo).map(n => (
              <NavLink
                key={n.to}
                to={n.to}
                className={({ isActive }) =>
                  `flex min-h-[44px] items-center gap-2.5 rounded-full px-4 text-[15px] ${
                    isActive ? 'bg-nav font-extrabold text-ink' : 'bg-surface font-semibold text-[#33443F] hover:bg-white'
                  }`
                }
              >
                <n.icon />
                {n.label}
                {n.to === '/panel/por-revisar' && pending > 0 ? (
                  <span className="ml-auto rounded-full bg-primary px-2 text-xs font-extrabold text-white">
                    {pending}
                    <span className="sr-only"> por revisar</span>
                  </span>
                ) : null}
              </NavLink>
            ))}
          <button
            type="button"
            className="ml-auto inline-flex min-h-[44px] items-center gap-2 rounded-full px-4 font-bold text-primary md:hidden"
            onClick={() => logout.mutate(undefined, { onSettled: () => navigate('/entrar', { replace: true }) })}
          >
            <IconLogout size={18} />
            Salir
          </button>
        </nav>
        <div className="mt-auto hidden flex-col gap-1 rounded-[22px] bg-white p-4 text-sm md:flex">
          <b>{session.usuario.nombre}</b>
          <span className="text-ink-2">{session.roles.map(r => roles[r] ?? r).join(', ')}</span>
          <button
            type="button"
            className="mt-2 inline-flex min-h-[44px] items-center gap-2 font-bold text-primary"
            onClick={() => logout.mutate(undefined, { onSettled: () => navigate('/entrar', { replace: true }) })}
          >
            <IconLogout size={18} />
            Cerrar sesión
          </button>
        </div>
      </aside>
      <main className="flex min-w-0 flex-1 flex-col gap-5">
        <Outlet />
      </main>
    </div>
  );
}
