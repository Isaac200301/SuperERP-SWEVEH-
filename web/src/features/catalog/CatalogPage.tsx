import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap } from '../../shared/api/client';
import { formatoCOP } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const tipos = { ambos: 'Carro y moto', carro: 'Carro', moto: 'Moto' } as const;
const pesos = (s: string) => (s === '' ? undefined : Number(s.replace(/\D/g, '')));

function TipoSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor="tipo-veh" className="text-[15px] font-extrabold">
        Para
      </label>
      <select id="tipo-veh" className={select} value={value} onChange={(e) => onChange(e.target.value)}>
        {Object.entries(tipos).map(([v, t]) => (
          <option key={v} value={v}>
            {t}
          </option>
        ))}
      </select>
    </div>
  );
}

/** HU-07: servicios con precio y duración. Sin precio, la IA dice "el taller te confirma el valor" (RF-17). */
function Services() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['servicios'], queryFn: async () => unwrap(await api.GET('/services', { params: { query: { todos: '1' } } })).servicios });
  const done = () => qc.invalidateQueries({ queryKey: ['servicios'] });
  const crear = useMutation({
    mutationFn: async (b: { nombre: string; tipoVehiculo: string; precio?: number; duracionMin?: number }) => unwrap(await api.POST('/services', { body: b })),
    onSuccess: done,
  });
  const activar = useMutation({
    mutationFn: async (v: { id: string; activo: boolean; version: number }) =>
      unwrap(await api.PATCH('/services/{id}', { params: { path: { id: v.id } }, body: { activo: v.activo, version: v.version } })),
    onSettled: done,
  });
  const [nombre, setNombre] = useState('');
  const [tipo, setTipo] = useState('ambos');
  const [precio, setPrecio] = useState('');
  const [min, setMin] = useState('');

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    const p = pesos(precio);
    const d = pesos(min);
    crear.mutate(
      { nombre, tipoVehiculo: tipo, ...(p !== undefined ? { precio: p } : {}), ...(d !== undefined ? { duracionMin: d } : {}) },
      { onSuccess: () => { setNombre(''); setPrecio(''); setMin(''); } },
    );
  };

  return (
    <section className={card} aria-labelledby="servicios">
      <h2 id="servicios" className="text-[17px] font-extrabold">Servicios</h2>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-4">
        <Field label="Nombre" required value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Cambio de aceite" />
        <TipoSelect value={tipo} onChange={setTipo} />
        <Field label="Precio (COP)" inputMode="numeric" value={precio} onChange={(e) => setPrecio(e.target.value)} hint="Vacío = se confirma después" />
        <Field label="Duración (min)" inputMode="numeric" value={min} onChange={(e) => setMin(e.target.value)} />
        <div className="sm:col-span-4">
          <Button type="submit" busy={crear.isPending} missing={nombre.trim() ? undefined : 'Escribe el nombre del servicio.'}>
            Agregar servicio
          </Button>
        </div>
      </form>
      {crear.isError ? <Alert>{crear.error.message}</Alert> : null}
      <ul className="flex flex-col gap-1">
        {(list.data ?? []).map((s) => (
          <li key={s.id} className={`flex flex-wrap items-center gap-3 rounded-xl px-3 py-2 ${s.activo ? '' : 'text-ink-2'}`}>
            <span className="flex-1 font-bold">{s.nombre}</span>
            <span className="text-sm">{tipos[s.tipoVehiculo as keyof typeof tipos] ?? s.tipoVehiculo}</span>
            <span className="font-mono">{s.precio == null ? 'por confirmar' : formatoCOP(s.precio)}</span>
            <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => activar.mutate({ id: s.id, activo: !s.activo, version: s.version })}>
              {s.activo ? 'Desactivar' : 'Activar'}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** HU-32, HU-35 y HU-36: repuestos con existencia, mínimo y entradas de mercancía. */
function Parts() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['repuestos'], queryFn: async () => unwrap(await api.GET('/parts', { params: { query: { todos: '1' } } })).repuestos });
  const done = () => qc.invalidateQueries({ queryKey: ['repuestos'] });
  const crear = useMutation({
    mutationFn: async (b: { referencia: string; nombre: string; tipoVehiculo: string; costo: number; precioVenta: number; existencia: number; minimo: number }) =>
      unwrap(await api.POST('/parts', { body: b })),
    onSuccess: done,
  });
  const entrada = useMutation({
    mutationFn: async (v: { id: string; cantidad: number }) =>
      unwrap(await api.POST('/parts/{id}/entries', { params: { path: { id: v.id } }, body: { cantidad: v.cantidad } })),
    onSettled: done,
  });
  const [f, setF] = useState({ referencia: '', nombre: '', tipo: 'ambos', costo: '', precio: '', existencia: '', minimo: '' });
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    crear.mutate(
      {
        referencia: f.referencia,
        nombre: f.nombre,
        tipoVehiculo: f.tipo,
        costo: pesos(f.costo) ?? 0,
        precioVenta: pesos(f.precio) ?? 0,
        existencia: pesos(f.existencia) ?? 0,
        minimo: pesos(f.minimo) ?? 0,
      },
      { onSuccess: () => setF({ ...f, referencia: '', nombre: '', costo: '', precio: '', existencia: '', minimo: '' }) },
    );
  };
  const falta = !f.referencia.trim() || !f.nombre.trim() ? 'Escribe referencia y nombre.' : f.precio === '' ? 'Escribe el precio de venta.' : undefined;

  return (
    <section className={card} aria-labelledby="repuestos">
      <h2 id="repuestos" className="text-[17px] font-extrabold">Repuestos</h2>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-4">
        <Field label="Referencia" value={f.referencia} onChange={set('referencia')} />
        <Field label="Nombre" value={f.nombre} onChange={set('nombre')} placeholder="Pastillas de freno" />
        <TipoSelect value={f.tipo} onChange={(v) => setF({ ...f, tipo: v })} />
        <Field label="Precio de venta (COP)" inputMode="numeric" value={f.precio} onChange={set('precio')} />
        <Field label="Costo (COP)" inputMode="numeric" value={f.costo} onChange={set('costo')} />
        <Field label="Existencia" inputMode="numeric" value={f.existencia} onChange={set('existencia')} />
        <Field label="Mínimo" inputMode="numeric" value={f.minimo} onChange={set('minimo')} />
        <div className="self-end">
          <Button type="submit" busy={crear.isPending} missing={falta}>
            Agregar repuesto
          </Button>
        </div>
      </form>
      {crear.isError ? <Alert>{crear.error.message}</Alert> : null}
      {entrada.isError ? <Alert>{entrada.error.message}</Alert> : null}
      <ul className="flex flex-col gap-1">
        {(list.data ?? []).map((p) => (
          <li key={p.id} className="flex flex-wrap items-center gap-3 rounded-xl px-3 py-2">
            <span className="font-mono text-sm text-ink-2">{p.referencia}</span>
            <span className="flex-1 font-bold">{p.nombre}</span>
            <span className={p.bajoMinimo ? 'font-bold text-warn-ink' : ''}>
              {p.existencia} en bodega{p.bajoMinimo ? ' · bajo el mínimo' : ''}
            </span>
            <span className="font-mono">{formatoCOP(p.precioVenta)}</span>
            <button
              type="button"
              className="min-h-[44px] px-2 font-bold text-primary"
              onClick={() => {
                const n = Number(window.prompt(`¿Cuántas unidades de "${p.nombre}" entraron?`) ?? '');
                if (Number.isInteger(n) && n > 0) entrada.mutate({ id: p.id, cantidad: n });
              }}
            >
              Registrar entrada
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

export function CatalogPage() {
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Servicios y repuestos</h1>
        <p className="text-ink-2">Lo que cargues aquí es lo único que se puede cotizar: nunca se inventa un precio.</p>
      </header>
      <Services />
      <Parts />
    </>
  );
}
