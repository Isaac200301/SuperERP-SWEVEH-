import { useQuery } from '@tanstack/react-query';
import { Link, Navigate, useNavigate } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { roles } from '../../shared/i18n/es-CO';
import { IconChevron, IconLogout } from '../../shared/ui/icons';
import { Logo } from '../../shared/ui/Logo';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { can, useLogout, useSession } from '../auth/api';
import { sectionsFor } from './sections';
import { TallerOverview } from './TallerOverview';

/** Lo que ya está en la barra inferior no se repite en la lista. */
const enLaBarra = ['/panel/tablero', '/panel/ordenes', '/panel/por-revisar'];

/**
 * "Taller" / "Más" / "Yo" en el celular (mockup Taller): las demás secciones que el rol puede usar y cerrar sesión.
 * En escritorio no hace falta: el menú lateral ya las muestra.
 */
export function MorePage() {
  const mobile = useIsMobile();
  const session = useSession().data;
  const navigate = useNavigate();
  const logout = useLogout();
  const dueno = can(session, 'workshop:manage');
  const taller = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')), enabled: dueno });
  if (!mobile) return <Navigate to="/panel/ordenes" replace />;
  if (!session) return null;
  const lista = sectionsFor(session, taller.data?.tamano === 'solo').filter((s) => !enLaBarra.includes(s.to));
  return (
    <>
      <header className="-mx-4 -mt-4 flex items-center gap-3 rounded-b-[28px] bg-primary px-[18px] pt-5 pb-5 text-white">
        <Logo size={40} light />
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-[21px] font-extrabold tracking-[-.02em]">{session.taller.nombre}</h1>
          <p className="truncate text-[13px] text-[#CFE9E4]">
            {session.usuario.nombre} · {session.roles.map((r) => roles[r] ?? r).join(', ')}
          </p>
        </div>
      </header>
      {can(session, 'kpi:read') ? <TallerOverview /> : null}
      <nav aria-label="Más secciones" className="flex flex-col gap-2">
        {lista.map((s) => (
          <Link key={s.to} to={s.to} className="flex min-h-[56px] items-center gap-3 rounded-[18px] bg-white px-3.5 text-[15px] font-bold text-ink">
            <span className="grid h-10 w-10 place-items-center rounded-[13px] bg-primary-tint text-primary">
              <s.icon size={20} />
            </span>
            <span className="flex-1">{s.label}</span>
            <IconChevron size={18} className="text-ink-2" />
          </Link>
        ))}
      </nav>
      <button
        type="button"
        className="inline-flex min-h-[52px] items-center justify-center gap-2 rounded-btn bg-white font-extrabold text-primary"
        onClick={() => logout.mutate(undefined, { onSettled: () => navigate('/entrar', { replace: true }) })}
      >
        <IconLogout size={18} />
        Cerrar sesión
      </button>
    </>
  );
}
