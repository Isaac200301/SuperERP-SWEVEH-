import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { formatoCOP } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { categorias } from '../inventory/api';

type Categoria = Schemas['Categoria'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const tipos = { ambos: 'Carro y moto', carro: 'Carro', moto: 'Moto' } as const;
const pesos = (s: string) => (s === '' ? undefined : Number(s.replace(/\D/g, '')));

/** HU-07: servicios con precio, duración, categoría y garantía. Sin precio, la IA dice "el taller te confirma el valor" (RF-17). */
export function ServicesPage() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['servicios'], queryFn: async () => unwrap(await api.GET('/services', { params: { query: { todos: '1' } } })).servicios });
  const done = () => qc.invalidateQueries({ queryKey: ['servicios'] });
  const crear = useMutation({
    mutationFn: async (b: { nombre: string; tipoVehiculo: string; precio?: number; duracionMin?: number; categoria?: Categoria; garantiaDias?: number }) =>
      unwrap(await api.POST('/services', { body: b })),
    onSuccess: done,
  });
  const activar = useMutation({
    mutationFn: async (v: { id: string; activo: boolean; version: number }) =>
      unwrap(await api.PATCH('/services/{id}', { params: { path: { id: v.id } }, body: { activo: v.activo, version: v.version } })),
    onSettled: done,
  });
  const [f, setF] = useState({ nombre: '', tipo: 'ambos', precio: '', min: '', categoria: '', garantia: '' });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    const p = pesos(f.precio);
    const d = pesos(f.min);
    const g = pesos(f.garantia);
    crear.mutate(
      {
        nombre: f.nombre, tipoVehiculo: f.tipo, ...(p !== undefined ? { precio: p } : {}), ...(d !== undefined ? { duracionMin: d } : {}),
        ...(f.categoria ? { categoria: f.categoria as Categoria } : {}), ...(g !== undefined ? { garantiaDias: g } : {}),
      },
      { onSuccess: () => setF({ ...f, nombre: '', precio: '', min: '', garantia: '' }) },
    );
  };

  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Servicios</h1>
        <p className="text-ink-2">Lo que cargues aquí es lo único que se puede cotizar: nunca se inventa un precio.</p>
      </header>
      <section className={card} aria-labelledby="servicios">
        <h2 id="servicios" className="text-[17px] font-extrabold">Servicios</h2>
        <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-3">
          <Field label="Nombre" required value={f.nombre} onChange={(e) => setF({ ...f, nombre: e.target.value })} placeholder="Cambio de aceite" />
          <div className="flex flex-col gap-1.5">
            <label htmlFor="tipo-veh" className="text-[15px] font-extrabold">
              Para
            </label>
            <select id="tipo-veh" className={select} value={f.tipo} onChange={(e) => setF({ ...f, tipo: e.target.value })}>
              {Object.entries(tipos).map(([v, t]) => (
                <option key={v} value={v}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="cat-serv" className="text-[15px] font-extrabold">
              Categoría
            </label>
            <select id="cat-serv" className={select} value={f.categoria} onChange={(e) => setF({ ...f, categoria: e.target.value })}>
              <option value="">Sin categoría</option>
              {Object.entries(categorias).map(([v, t]) => (
                <option key={v} value={v}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <Field label="Precio (COP)" inputMode="numeric" value={f.precio} onChange={(e) => setF({ ...f, precio: e.target.value })} hint="Vacío = se confirma después" />
          <Field label="Duración (min)" inputMode="numeric" value={f.min} onChange={(e) => setF({ ...f, min: e.target.value })} />
          <Field label="Garantía (días)" inputMode="numeric" value={f.garantia} onChange={(e) => setF({ ...f, garantia: e.target.value })} hint="Vacío = sin garantía definida" />
          <div className="sm:col-span-3">
            <Button type="submit" busy={crear.isPending} missing={f.nombre.trim() ? undefined : 'Escribe el nombre del servicio.'}>
              Agregar servicio
            </Button>
          </div>
        </form>
        {crear.isError ? <Alert>{crear.error.message}</Alert> : null}
        <ul className="flex flex-col gap-1">
          {(list.data ?? []).map((s) => (
            <li key={s.id} className={`flex flex-wrap items-center gap-3 rounded-xl px-3 py-2 ${s.activo ? '' : 'text-ink-2'}`}>
              <span className="flex-1 font-bold">
                {s.nombre}
                {s.categoria ? <span className="font-normal text-ink-2"> · {categorias[s.categoria]}</span> : null}
              </span>
              <span className="text-sm">{tipos[s.tipoVehiculo as keyof typeof tipos] ?? s.tipoVehiculo}</span>
              {s.garantiaDias != null ? <span className="text-sm">Garantía {s.garantiaDias} días</span> : null}
              <span className="font-mono">{s.precio == null ? 'por confirmar' : formatoCOP(s.precio)}</span>
              <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => activar.mutate({ id: s.id, activo: !s.activo, version: s.version })}>
                {s.activo ? 'Desactivar' : 'Activar'}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
