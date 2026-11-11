import { useQuery } from '@tanstack/react-query';
import { useEffect, useId, useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { IconSearch } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';

/** Espera a que la persona deje de escribir antes de buscar. */
function useDebounced(value: string, ms = 300): string {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

/**
 * HU-14 en el celular: buscar por placa, nombre o celular y saltar a la orden abierta del vehículo.
 * tone surface: sobre un encabezado blanco el campo va en gris claro para que se vea.
 */
export function SearchBox({ placeholder = 'Placa, cliente o celular', tone = 'white' }: { placeholder?: string; tone?: 'white' | 'surface' }) {
  const id = useId();
  const [q, setQ] = useState('');
  const term = useDebounced(q.trim());
  const r = useQuery({
    queryKey: ['buscar', term],
    enabled: term.length >= 2,
    queryFn: async () => unwrap(await api.GET('/search', { params: { query: { q: term } } })),
    staleTime: 10_000,
  });
  const vehiculos = [...(r.data?.vehiculos ?? []), ...(r.data?.clientes ?? []).flatMap((c) => c.vehiculos)].filter(
    (v, i, all) => all.findIndex((x) => x.id === v.id) === i,
  );
  const show = term.length >= 2 && q.trim().length >= 2;
  return (
    <div className="relative">
      <label className={`flex min-h-[50px] items-center gap-2.5 rounded-2xl px-3.5 text-ink-2 ${tone === 'white' ? 'bg-white' : 'bg-surface'}`}>
        <IconSearch size={20} />
        <input
          type="search"
          aria-label="Buscar por placa, cliente o celular"
          aria-controls={show ? id : undefined}
          placeholder={placeholder}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="min-w-0 flex-1 bg-transparent text-[15px] text-ink outline-none"
        />
      </label>
      {show ? (
        <div id={id} className="absolute inset-x-0 top-full z-30 mt-2 flex max-h-[60dvh] flex-col gap-1 overflow-y-auto rounded-2xl bg-white p-2 text-ink shadow-[0_10px_30px_rgba(15,31,29,.18)]">
          {r.isPending ? <p role="status" className="p-3 text-sm text-ink-2">Buscando…</p> : null}
          {r.isError ? <p className="p-3 text-sm text-err-ink">{r.error.message}</p> : null}
          {r.isSuccess && vehiculos.length === 0 ? <p className="p-3 text-sm text-ink-2">No encontramos nada con “{term}”.</p> : null}
          {vehiculos.map((v) => {
            const body = (
              <>
                <span className="flex items-center gap-2">
                  <Plate value={v.placa} />
                  <b className="truncate text-sm">{v.descripcion}</b>
                </span>
                <span className="flex flex-wrap items-center gap-2 text-xs text-ink-2">
                  {v.cliente}
                  {v.estadoOrden ? <StatePill estado={v.estadoOrden} /> : 'Sin orden abierta'}
                </span>
              </>
            );
            return v.ordenAbiertaId ? (
              <Link key={v.id} to={`/panel/ordenes/${v.ordenAbiertaId}`} className="flex min-h-[44px] flex-col gap-1 rounded-xl p-2.5 hover:bg-surface">
                {body}
              </Link>
            ) : (
              <div key={v.id} className="flex min-h-[44px] flex-col gap-1 rounded-xl p-2.5">
                {body}
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
