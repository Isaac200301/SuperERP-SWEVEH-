import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { usePermission } from '../auth/api';

type Propuesta = Schemas['PropuestaServicio'];
const key = ['lista-precios'] as const;
const numero = (s: string) => (s.trim() === '' ? null : Number(s.replace(/\D/g, '')));

/** HU-93: sube la foto de la lista de precios; la IA la lee y cada fila queda en Por revisar. */
export function PriceListUpload() {
  const qc = useQueryClient();
  const subir = useMutation({
    mutationFn: async (foto: File) =>
      unwrap(
        await api.POST('/services/photo', {
          body: { foto: '' },
          bodySerializer: () => {
            const fd = new FormData();
            fd.append('foto', foto);
            return fd;
          },
        }),
      ).propuestas,
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });
  return (
    <section className="flex flex-col gap-2 rounded-card bg-white p-5" aria-labelledby="lista-foto">
      <h2 id="lista-foto" className="text-[17px] font-extrabold">
        Pasar mi lista con una foto
      </h2>
      <p className="text-ink-2">
        Tómale una foto a tu lista de precios. Leemos cada servicio y tú lo revisas antes de que quede en el catálogo.
      </p>
      <label className="flex flex-col gap-1 font-bold">
        Foto de la lista (JPG, PNG o WebP)
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          disabled={subir.isPending}
          onChange={e => {
            const f = e.target.files?.[0];
            if (f) subir.mutate(f);
          }}
        />
      </label>
      {subir.isPending ? <p role="status">Leyendo la foto…</p> : null}
      {subir.data ? (
        <p role="status" className="font-bold text-primary">
          Leímos {subir.data.length} servicios.{' '}
          <Link to="/panel/por-revisar" className="underline">
            Revísalos en Por revisar
          </Link>
          .
        </p>
      ) : null}
      {subir.isError ? <Alert>{subir.error.message}</Alert> : null}
    </section>
  );
}

function Fila({ p }: { p: Propuesta }) {
  const qc = useQueryClient();
  const [f, setF] = useState({ nombre: p.nombre, precio: p.precio == null ? '' : String(p.precio), tipo: p.tipoVehiculo });
  const resolver = useMutation({
    mutationFn: async (decision: 'aprobar' | 'descartar') =>
      unwrap(
        await api.POST('/services/proposals/{id}', {
          params: { path: { id: p.id } },
          body: decision === 'aprobar' ? { decision, nombre: f.nombre, precio: numero(f.precio), tipoVehiculo: f.tipo } : { decision },
        }),
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: key });
      void qc.invalidateQueries({ queryKey: ['servicios'] });
    },
  });
  return (
    <li className="flex flex-wrap items-end gap-2 border-t border-line py-2">
      <label className="flex flex-col text-sm">
        Servicio
        <input
          className="min-h-[44px] rounded-btn border-[1.5px] border-line px-3"
          value={f.nombre}
          onChange={e => setF({ ...f, nombre: e.target.value })}
        />
      </label>
      <label className="flex flex-col text-sm">
        Precio (COP)
        <input
          className="min-h-[44px] w-32 rounded-btn border-[1.5px] border-line px-3"
          inputMode="numeric"
          value={f.precio}
          onChange={e => setF({ ...f, precio: e.target.value })}
        />
      </label>
      <label className="flex flex-col text-sm">
        Para
        <select
          className="min-h-[44px] rounded-btn border-[1.5px] border-line bg-white px-3"
          value={f.tipo}
          onChange={e => setF({ ...f, tipo: e.target.value as Propuesta['tipoVehiculo'] })}
        >
          <option value="ambos">Carro y moto</option>
          <option value="carro">Carro</option>
          <option value="moto">Moto</option>
        </select>
      </label>
      <Button
        busy={resolver.isPending}
        missing={f.nombre.trim() ? undefined : 'Escribe el nombre.'}
        onClick={() => resolver.mutate('aprobar')}
      >
        Aprobar
      </Button>
      <Button variant="ghost" disabled={resolver.isPending} onClick={() => resolver.mutate('descartar')}>
        Descartar
      </Button>
      {resolver.isError ? <Alert>{resolver.error.message}</Alert> : null}
    </li>
  );
}

/**
 * HU-93: las filas leídas de la foto, con la foto al lado. Corriges y apruebas cada una; solo al aprobarla queda el
 * servicio en el catálogo.
 */
export function PriceListReview() {
  const puede = usePermission('service:manage');
  const q = useQuery({
    queryKey: key,
    enabled: puede,
    queryFn: async () => unwrap(await api.GET('/services/proposals')).propuestas,
  });
  if (!puede || !q.data || q.data.length === 0) return null;
  const lotes = new Map<string, Propuesta[]>();
  for (const p of q.data) lotes.set(p.loteId, [...(lotes.get(p.loteId) ?? []), p]);
  return (
    <section className="flex flex-col gap-3 rounded-card bg-white p-5" aria-labelledby="lista-revisar">
      <h2 id="lista-revisar" className="text-[17px] font-extrabold">
        Lista de precios por revisar
      </h2>
      {[...lotes.values()].map(filas => (
        <div key={filas[0]?.loteId} className="flex flex-col gap-3 md:flex-row">
          <img
            src={`/api/v1/services/proposals/${filas[0]?.id}/photo`}
            alt="Foto de la lista de precios"
            className="max-h-80 w-full rounded-btn border border-line object-contain md:w-64"
          />
          <ul className="flex-1">
            {filas.map(p => (
              <Fila key={p.id} p={p} />
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}
