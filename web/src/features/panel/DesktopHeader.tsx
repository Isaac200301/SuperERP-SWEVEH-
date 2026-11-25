import { Link } from 'react-router';

import { SearchBox } from './SearchBox';

/**
 * Encabezado de las pantallas con diseño de escritorio (Figma D1 y D3): título y subtítulo a la izquierda; a la derecha,
 * el buscador de placa, cliente o celular (HU-14) y "Abrir asistente" (V-11), que lleva al chat.
 */
export function DesktopHeader({ title, subtitle }: Readonly<{ title: string; subtitle: string }>) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-[32px] leading-tight font-extrabold tracking-[-.02em]">{title}</h1>
        <p className="text-[15px] text-ink-2">{subtitle}</p>
      </div>
      <div className="flex items-center gap-4">
        <div className="w-[320px]">
          <SearchBox placeholder="Buscar placa, cliente o celular" />
        </div>
        <Link
          to="/panel/asistente"
          className="inline-flex min-h-[44px] items-center rounded-btn bg-primary px-[18px] text-[15px] font-extrabold text-white hover:bg-primary-pressed"
        >
          Abrir asistente
        </Link>
      </div>
    </header>
  );
}
