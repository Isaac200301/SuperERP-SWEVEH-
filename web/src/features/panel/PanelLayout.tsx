import { NavLink, Outlet, useLocation, useNavigate } from 'react-router';

import type { Sesion } from '../../shared/api/client';
import { roles } from '../../shared/i18n/es-CO';
import { Brand } from '../../shared/ui/Logo';
import { IconLogout } from '../../shared/ui/icons';
import { can, feature, useLogout } from '../auth/api';
import { usePendingCards } from '../inbox/usePendingCards';
import { WhatsappDown } from '../messaging/WhatsappDown';
import { useInventarioExterno } from '../workshop/SettingsForm';
import { sectionsFor } from './sections';

/**
 * Pantallas con diseño propio de escritorio (Figma D1, D2 y D3, V-18): usan todo el ancho. Las demás van como la columna
 * del celular, centrada y con el menú lateral. Provisional: la columna lleva los componentes de escritorio de siempre, no
 * los del celular, porque los del celular dependen de la barra inferior y de la hoja inferior (docs/10 §4.8).
 */
export function anchoCompleto(path: string): boolean {
  return (
    path === '/panel/tablero' ||
    path === '/panel/servicios' ||
    /^\/panel\/ordenes\/(?!nueva$)[^/]+$/.test(path)
  );
}

/** soloYo: el taller dijo "solo yo" en el registro y no ve la gestión de usuarios (HU-83). */
export function PanelLayout({ session, soloYo = false }: { session: Sesion; soloYo?: boolean }) {
  const navigate = useNavigate();
  const logout = useLogout();
  const { pathname } = useLocation();
  // HU-66: contador y sonido cuando llega una tarjeta a Por revisar (dueño y recepción).
  const sinInventario = useInventarioExterno();
  const pending = usePendingCards(feature(session, 'whatsapp') && can(session, 'inbox:manage'));
  const ancho = anchoCompleto(pathname);
  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-[240px] flex-none flex-col gap-5 overflow-y-auto rounded-br-[28px] bg-white p-4">
        <div className="px-0 pt-2">
          <Brand subtitle={session.taller.nombre} />
        </div>
        <nav aria-label="Secciones del panel" className="flex flex-col gap-1.5">
          {/* Figma D1: quien ve Hoy no ve Piso en el menú; Piso es la pantalla del mecánico (en el celular sigue en Taller). */}
          {sectionsFor(session, soloYo, sinInventario)
            .filter((n) => !(n.to === '/panel/piso' && can(session, 'kpi:read')))
            .map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              className={({ isActive }) =>
                `flex min-h-[44px] items-center gap-2 rounded-full px-3.5 text-[15px] ${
                  isActive ? 'bg-nav font-extrabold text-ink' : 'font-semibold text-ink-2 hover:bg-surface'
                }`
              }
            >
              {n.label}
              {n.to === '/panel/por-revisar' && pending > 0 ? (
                <span className="ml-auto rounded-full bg-primary px-2 text-xs font-extrabold text-white">
                  {pending}
                  <span className="sr-only"> por revisar</span>
                </span>
              ) : null}
            </NavLink>
          ))}
        </nav>
        <div className="mt-auto flex flex-col gap-1 rounded-[22px] bg-surface p-4 text-sm">
          <b>{session.usuario.nombre}</b>
          <span className="text-ink-2">{session.roles.map((r) => roles[r] ?? r).join(', ')}</span>
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
      <main className="min-w-0 flex-1 px-8 py-8">
        <div className={`flex flex-col gap-5 ${ancho ? 'max-w-[1136px]' : 'mx-auto max-w-[820px]'}`}>
          <WhatsappDown enabled={feature(session, 'whatsapp') && can(session, 'workshop:manage')} />
          <Outlet />
        </div>
      </main>
    </div>
  );
}
