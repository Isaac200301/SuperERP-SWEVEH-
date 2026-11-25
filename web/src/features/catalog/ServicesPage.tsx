import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { formatoCOP } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { IconWarn } from '../../shared/ui/icons';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { useFeature } from '../auth/api';
import { categorias } from '../inventory/api';
import { DesktopHeader } from '../panel/DesktopHeader';
import { MiTallerResumen } from '../workshop/MiTallerResumen';
import { PriceListUpload } from './PriceList';

type Categoria = Schemas['Categoria'];
type Servicio = Schemas['Servicio'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const tipos = { ambos: 'Carros y motos', carro: 'Carros', moto: 'Motos' } as const;
const pesos = (s: string) => (s === '' ? undefined : Number(s.replace(/\D/g, '')));

/** El precio de una fila: el valor, o "Sin precio · ponlo" que abre el campo para ponerlo ahí mismo (HU-06, RF-17). */
function Precio({ s, onGuardar, busy }: Readonly<{ s: Servicio; onGuardar: (precio: number) => void; busy: boolean }>) {
  const [editando, setEditando] = useState(false);
  const [valor, setValor] = useState('');
  if (editando) {
    const v = pesos(valor);
    return (
      <form
        className="flex flex-wrap items-center gap-2"
        onKeyDown={(e) => {
          if (e.key === 'Escape') setEditando(false);
        }}
        onSubmit={(e) => {
          e.preventDefault();
          if (v === undefined || v <= 0) return;
          onGuardar(v);
          setEditando(false);
        }}
      >
        <input
          aria-label={`Precio de ${s.nombre}`}
          inputMode="numeric"
          autoFocus
          value={valor}
          onChange={(e) => setValor(e.target.value)}
          className="min-h-[44px] w-28 rounded-btn border-[1.5px] border-line bg-white px-3 font-mono"
        />
        <button type="submit" disabled={busy || v === undefined || v <= 0} className="min-h-[44px] px-2 font-bold text-primary disabled:opacity-50">
          Guardar
        </button>
        <button type="button" className="min-h-[44px] px-2 font-bold text-ink-2" onClick={() => setEditando(false)}>
          Cancelar
        </button>
      </form>
    );
  }
  return (
    <button
      type="button"
      aria-label={s.precio == null ? `Poner el precio de ${s.nombre}` : `Cambiar el precio de ${s.nombre}`}
      className="inline-flex min-h-[44px] items-center self-start"
      onClick={() => {
        setValor(s.precio == null ? '' : String(s.precio));
        setEditando(true);
      }}
    >
      {s.precio == null ? (
        <span className="inline-flex items-center gap-1 rounded-full bg-warn-tint px-2.5 py-1 text-xs font-bold text-warn-ink">
          <IconWarn size={12} strokeWidth={2.4} />
          Sin precio · ponlo
        </span>
      ) : (
        <span className="font-mono font-bold hover:text-primary">{formatoCOP(s.precio)}</span>
      )}
    </button>
  );
}

/** HU-07: servicios con precio, duración, categoría y garantía. Sin precio, la IA dice "el taller te confirma el valor" (RF-17). */
export function ServicesPage() {
  const qc = useQueryClient();
  const foto = useFeature('whatsapp'); // V-01: la lista por foto usa el asistente de WhatsApp (HU-93)
  const mobile = useIsMobile(); // en el celular, Mi taller ya está en Taller (Figma 12)
  const list = useQuery({
    queryKey: ['servicios'],
    queryFn: async () => unwrap(await api.GET('/services', { params: { query: { todos: '1' } } })).servicios,
  });
  const done = () => qc.invalidateQueries({ queryKey: ['servicios'] });
  const crear = useMutation({
    mutationFn: async (b: {
      nombre: string;
      tipoVehiculo: string;
      precio?: number;
      duracionMin?: number;
      categoria?: Categoria;
      garantiaDias?: number;
    }) => unwrap(await api.POST('/services', { body: b })),
    onSuccess: done,
  });
  const activar = useMutation({
    mutationFn: async (v: { id: string; activo: boolean; version: number }) =>
      unwrap(await api.PATCH('/services/{id}', { params: { path: { id: v.id } }, body: { activo: v.activo, version: v.version } })),
    onSettled: done,
  });
  const ponerPrecio = useMutation({
    mutationFn: async (v: { id: string; precio: number; version: number }) =>
      unwrap(await api.PATCH('/services/{id}', { params: { path: { id: v.id } }, body: { precio: v.precio, version: v.version } })),
    onSettled: done,
  });
  const [abierto, setAbierto] = useState(false);
  const [f, setF] = useState({ nombre: '', tipo: 'ambos', precio: '', min: '', categoria: '', garantia: '' });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    const p = pesos(f.precio);
    const d = pesos(f.min);
    const g = pesos(f.garantia);
    crear.mutate(
      {
        nombre: f.nombre,
        tipoVehiculo: f.tipo,
        ...(p !== undefined ? { precio: p } : {}),
        ...(d !== undefined ? { duracionMin: d } : {}),
        ...(f.categoria ? { categoria: f.categoria as Categoria } : {}),
        ...(g !== undefined ? { garantiaDias: g } : {}),
      },
      { onSuccess: () => setF({ ...f, nombre: '', precio: '', min: '', garantia: '' }) },
    );
  };

  return (
    <>
      {mobile ? (
        <header>
          <h1 className="text-[26px] font-extrabold tracking-[-.02em]">Servicios</h1>
          <p className="text-[13px] text-ink-2">Tu lista de precios: los valores los pones tú</p>
        </header>
      ) : (
        <DesktopHeader title="Servicios y Mi taller" subtitle="Tu lista de precios: los valores los pones tú" />
      )}
      {foto ? <PriceListUpload /> : null}
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,656fr)_minmax(0,460fr)]">
        <section className={card} aria-labelledby="servicios">
          <div className="flex items-center justify-between gap-3">
            <h2 id="servicios" className="text-[20px] font-extrabold">
              Servicios
            </h2>
            <button
              type="button"
              aria-expanded={abierto}
              className="inline-flex min-h-[44px] items-center rounded-btn border-[1.5px] border-line bg-white px-[18px] text-[15px] font-extrabold hover:border-primary"
              onClick={() => setAbierto(!abierto)}
            >
              + Agregar servicio
            </button>
          </div>
          {abierto ? (
            <form onSubmit={onSubmit} aria-label="Agregar un servicio" className="grid gap-3 rounded-2xl bg-surface p-4 sm:grid-cols-3">
              <Field
                label="Nombre"
                required
                value={f.nombre}
                onChange={(e) => setF({ ...f, nombre: e.target.value })}
                placeholder="Cambio de aceite"
              />
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
              <Field
                label="Precio (COP)"
                inputMode="numeric"
                value={f.precio}
                onChange={(e) => setF({ ...f, precio: e.target.value })}
                hint="Vacío = se confirma después"
              />
              <Field label="Duración (min)" inputMode="numeric" value={f.min} onChange={(e) => setF({ ...f, min: e.target.value })} />
              <Field
                label="Garantía (días)"
                inputMode="numeric"
                value={f.garantia}
                onChange={(e) => setF({ ...f, garantia: e.target.value })}
                hint="Vacío = sin garantía definida"
              />
              <div className="sm:col-span-3">
                <Button type="submit" busy={crear.isPending} missing={f.nombre.trim() ? undefined : 'Escribe el nombre del servicio.'}>
                  Agregar servicio
                </Button>
              </div>
            </form>
          ) : null}
          {crear.isError ? <Alert>{crear.error.message}</Alert> : null}
          {ponerPrecio.isError ? <Alert>{ponerPrecio.error.message}</Alert> : null}
          <div role="table" aria-label="Servicios" className="flex flex-col">
            <div role="row" className="hidden grid-cols-[minmax(0,1fr)_110px_170px_96px] gap-3 border-b border-line py-2 font-mono text-[11px] text-ink-2 sm:grid">
              <span role="columnheader">SERVICIO</span>
              <span role="columnheader">PARA</span>
              <span role="columnheader">PRECIO</span>
              <span role="columnheader" className="sr-only">
                Acciones
              </span>
            </div>
            {(list.data ?? []).map((s) => (
              <div
                key={s.id}
                role="row"
                className={`grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 border-b border-line py-1 sm:grid-cols-[minmax(0,1fr)_110px_170px_96px] ${s.activo ? '' : 'text-ink-2'}`}
              >
                <span role="cell" className="min-w-0 py-2 text-[15px]">
                  <b className="font-semibold">{s.nombre}</b>
                  {s.categoria ? <span className="text-ink-2"> · {categorias[s.categoria]}</span> : null}
                  {s.garantiaDias != null ? <span className="block text-[13px] text-ink-2">Garantía {s.garantiaDias} días</span> : null}
                  <span className="block text-[13px] text-ink-2 sm:hidden">{tipos[s.tipoVehiculo as keyof typeof tipos] ?? s.tipoVehiculo}</span>
                </span>
                <span role="cell" className="hidden text-[14px] text-ink-2 sm:block">
                  {tipos[s.tipoVehiculo as keyof typeof tipos] ?? s.tipoVehiculo}
                </span>
                <span role="cell" className="justify-self-end sm:justify-self-start">
                  <Precio s={s} busy={ponerPrecio.isPending} onGuardar={(precio) => ponerPrecio.mutate({ id: s.id, precio, version: s.version })} />
                </span>
                <span role="cell" className="col-span-2 justify-self-end sm:col-span-1">
                  <button
                    type="button"
                    className="min-h-[44px] px-2 text-[14px] font-bold text-primary"
                    onClick={() => activar.mutate({ id: s.id, activo: !s.activo, version: s.version })}
                  >
                    {s.activo ? 'Desactivar' : 'Activar'}
                  </button>
                </span>
              </div>
            ))}
          </div>
          <p className="text-[13px] text-ink-2">Los precios los pone el dueño. Un servicio sin precio se pide la primera vez que se cotiza.</p>
        </section>
        {mobile ? null : <MiTallerResumen />}
      </div>
    </>
  );
}
