import { useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router';

import { CsvImport } from '../../shared/import/CsvImport';
import { formatoCOP, formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { usePermission } from '../auth/api';
import { categorias, fotoRepuesto, motivos, plantillaCSV, useMovements, usePartActions, useParts, type Categoria, type Filtros, type Repuesto } from './api';

const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const tipos = { ambos: 'Carro y moto', carro: 'Carro', moto: 'Moto' } as const;
const numero = (s: string) => (s.trim() === '' ? undefined : Number(s.replace(/\D/g, '')));

function Select({ id, label, value, onChange, children }: { id: string; label: string; value: string; onChange: (v: string) => void; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[15px] font-extrabold">
        {label}
      </label>
      <select id={id} className={select} value={value} onChange={(e) => onChange(e.target.value)}>
        {children}
      </select>
    </div>
  );
}

const opcionesCategoria = (
  <>
    <option value="">Sin categoría</option>
    {Object.entries(categorias).map(([v, t]) => (
      <option key={v} value={v}>
        {t}
      </option>
    ))}
  </>
);

const vacio = { referencia: '', nombre: '', tipo: 'ambos', marca: '', categoria: '', ubicacion: '', oem: '', costo: '', precio: '', existencia: '', minimo: '' };

/** HU-32: crear un repuesto con su ficha (marca, categoría, ubicación, referencia original). */
function NewPart() {
  const { create } = usePartActions();
  const [f, setF] = useState(vacio);
  const set = (k: keyof typeof vacio) => (v: string) => setF({ ...f, [k]: v });
  let falta: string | undefined;
  if (!f.referencia.trim() || !f.nombre.trim()) falta = 'Escribe referencia y nombre.';
  else if (f.precio === '') falta = 'Escribe el precio de venta.';
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (falta) return;
    create.mutate(
      {
        referencia: f.referencia, nombre: f.nombre, tipoVehiculo: f.tipo, marca: f.marca, categoria: f.categoria, ubicacion: f.ubicacion,
        referenciaOem: f.oem, costo: numero(f.costo) ?? 0, precioVenta: numero(f.precio) ?? 0, existencia: numero(f.existencia) ?? 0, minimo: numero(f.minimo) ?? 0,
      },
      { onSuccess: () => setF(vacio) },
    );
  };
  return (
    <section className={card} aria-labelledby="nuevo-repuesto">
      <h2 id="nuevo-repuesto" className="text-[17px] font-extrabold">Nuevo repuesto</h2>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-4">
        <Field label="Referencia" value={f.referencia} onChange={(e) => set('referencia')(e.target.value)} mono />
        <Field label="Nombre" value={f.nombre} onChange={(e) => set('nombre')(e.target.value)} placeholder="Pastillas de freno" />
        <Field label="Marca" value={f.marca} onChange={(e) => set('marca')(e.target.value)} placeholder="Bosch" />
        <Select id="nuevo-categoria" label="Categoría" value={f.categoria} onChange={set('categoria')}>
          {opcionesCategoria}
        </Select>
        <Select id="nuevo-tipo" label="Para" value={f.tipo} onChange={set('tipo')}>
          {Object.entries(tipos).map(([v, t]) => (
            <option key={v} value={v}>
              {t}
            </option>
          ))}
        </Select>
        <Field label="Ubicación" value={f.ubicacion} onChange={(e) => set('ubicacion')(e.target.value)} placeholder="Estante A3" />
        <Field label="Referencia original (OEM)" value={f.oem} onChange={(e) => set('oem')(e.target.value)} mono />
        <Field label="Precio de venta (COP)" inputMode="numeric" value={f.precio} onChange={(e) => set('precio')(e.target.value)} />
        <Field label="Costo (COP)" inputMode="numeric" value={f.costo} onChange={(e) => set('costo')(e.target.value)} />
        <Field label="Existencia" inputMode="numeric" value={f.existencia} onChange={(e) => set('existencia')(e.target.value)} />
        <Field label="Mínimo" inputMode="numeric" value={f.minimo} onChange={(e) => set('minimo')(e.target.value)} />
        <div className="self-end">
          <Button type="submit" busy={create.isPending} missing={falta}>
            Agregar repuesto
          </Button>
        </div>
      </form>
      {create.isError ? <Alert>{create.error.message}</Alert> : null}
    </section>
  );
}

/** Detalle: editar la ficha, registrar entrada (HU-35), activar o desactivar (HU-32) y ver movimientos. */
function PartDetail({ p, onClose }: { p: Repuesto; onClose: () => void }) {
  const { update, entry, photo } = usePartActions();
  const [fotoV, setFotoV] = useState(0);
  const verCosto = usePermission('part:cost');
  const movs = useMovements(p.id);
  const [f, setF] = useState({
    nombre: p.nombre, marca: p.marca ?? '', categoria: p.categoria ?? '', ubicacion: p.ubicacion ?? '', oem: p.referenciaOem ?? '',
    precio: String(p.precioVenta), costo: p.costo == null ? '' : String(p.costo), minimo: String(p.minimo),
  });
  const [cantidad, setCantidad] = useState('');
  const set = (k: keyof typeof f) => (v: string) => setF({ ...f, [k]: v });
  const guardar = (e: FormEvent) => {
    e.preventDefault();
    update.mutate({
      id: p.id,
      body: {
        version: p.version, nombre: f.nombre, marca: f.marca, categoria: f.categoria, ubicacion: f.ubicacion, referenciaOem: f.oem,
        precioVenta: numero(f.precio) ?? 0, minimo: numero(f.minimo) ?? 0, ...(verCosto ? { costo: numero(f.costo) ?? 0 } : {}),
      },
    });
  };
  const n = numero(cantidad);
  const error = update.error ?? entry.error ?? photo.error;

  return (
    <section className={card} aria-labelledby="detalle-repuesto">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id="detalle-repuesto" className="flex-1 text-[17px] font-extrabold">
          <span className="font-mono">{p.referencia}</span> · {p.nombre}
        </h2>
        <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={onClose}>
          Cerrar
        </button>
      </div>
      <p className={p.bajoMinimo ? 'font-bold text-warn-ink' : ''}>
        {p.existencia} en bodega · mínimo {p.minimo}
        {p.bajoMinimo ? ' · bajo el mínimo' : ''}
      </p>
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (n && n > 0) entry.mutate({ id: p.id, cantidad: n }, { onSuccess: () => setCantidad('') });
        }}
      >
        <Field label="Cantidad que entró" inputMode="numeric" value={cantidad} onChange={(e) => setCantidad(e.target.value)} className="max-w-[160px]" />
        <Button type="submit" variant="secondary" busy={entry.isPending} missing={n && n > 0 ? undefined : 'Escribe cuántas unidades entraron.'}>
          Registrar entrada
        </Button>
      </form>
      <form onSubmit={guardar} className="grid gap-3 sm:grid-cols-4">
        <Field label="Nombre del repuesto" value={f.nombre} onChange={(e) => set('nombre')(e.target.value)} />
        <Field label="Marca del repuesto" value={f.marca} onChange={(e) => set('marca')(e.target.value)} />
        <Select id="detalle-categoria" label="Categoría del repuesto" value={f.categoria} onChange={set('categoria')}>
          {opcionesCategoria}
        </Select>
        <Field label="Ubicación en bodega" value={f.ubicacion} onChange={(e) => set('ubicacion')(e.target.value)} />
        <Field label="OEM o equivalente" value={f.oem} onChange={(e) => set('oem')(e.target.value)} mono />
        <Field label="Precio (COP)" inputMode="numeric" value={f.precio} onChange={(e) => set('precio')(e.target.value)} />
        {verCosto ? <Field label="Costo del repuesto (COP)" inputMode="numeric" value={f.costo} onChange={(e) => set('costo')(e.target.value)} /> : null}
        <Field label="Mínimo en bodega" inputMode="numeric" value={f.minimo} onChange={(e) => set('minimo')(e.target.value)} />
        <div className="flex flex-wrap gap-3 sm:col-span-4">
          <Button type="submit" busy={update.isPending}>
            Guardar cambios
          </Button>
          <Button variant="secondary" busy={update.isPending} onClick={() => update.mutate({ id: p.id, body: { activo: !p.activo, version: p.version } })}>
            {p.activo ? 'Desactivar' : 'Activar'}
          </Button>
        </div>
      </form>
      <fieldset className="flex flex-col gap-3 rounded-2xl bg-surface p-4">
        <legend className="font-extrabold">Vitrina de WhatsApp</legend>
        <label className="flex min-h-[44px] items-center gap-2 font-bold">
          <input
            type="checkbox"
            role="switch"
            aria-checked={p.ofrecerWhatsapp ?? false}
            checked={p.ofrecerWhatsapp ?? false}
            disabled={update.isPending}
            onChange={(e) => update.mutate({ id: p.id, body: { ofrecerWhatsapp: e.target.checked, version: p.version } })}
          />
          Ofrecer por WhatsApp
        </label>
        <p className="text-sm text-ink-2">El asesor lo ofrece a los clientes con su precio y la foto. Nunca muestra el costo.</p>
        {p.tieneFoto ? <img src={fotoRepuesto(p.id, fotoV)} alt={`Foto de ${p.nombre}`} className="h-32 w-32 rounded-btn border border-line bg-white object-contain" /> : null}
        <label className="flex flex-col gap-1 font-bold">
          {p.tieneFoto ? 'Cambiar la foto' : 'Foto para WhatsApp (JPG, PNG o WebP, hasta 5 MB)'}
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            disabled={photo.isPending}
            onChange={(e) => {
              const foto = e.target.files?.[0];
              if (foto) photo.mutate({ id: p.id, foto }, { onSuccess: () => setFotoV((v) => v + 1) });
            }}
          />
        </label>
      </fieldset>
      {error ? <Alert>{error.message}</Alert> : null}
      <h3 className="font-extrabold">Movimientos</h3>
      {movs.isPending ? <p role="status">Cargando…</p> : null}
      <ul className="flex flex-col gap-1 text-sm">
        {(movs.data ?? []).map((m) => (
          <li key={m.id} className="flex flex-wrap gap-3 border-t border-line py-2">
            <span className="w-36 text-ink-2">{formatoFechaHora(m.creadoEn)}</span>
            <span className="flex-1">{motivos[m.motivo] ?? m.motivo}</span>
            {m.ordenId ? (
              <Link className="font-bold text-primary" to={`/panel/ordenes/${m.ordenId}`}>
                Ver orden
              </Link>
            ) : null}
            <span className={`w-16 text-right font-mono font-bold ${m.cantidad < 0 ? 'text-err-ink' : 'text-ok'}`}>
              {m.cantidad > 0 ? `+${m.cantidad}` : m.cantidad}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Bloque 2: inventario completo (HU-32, HU-34 a HU-37). */
export function PartsPage() {
  const [params] = useSearchParams();
  const [filtros, setFiltros] = useState<Filtros>({ q: '', categoria: '', bajo: params.get('bajo') === '1', inactivos: false });
  const [abierto, setAbierto] = useState<string | null>(null);
  const puedeImportar = usePermission('import:run');
  const qc = useQueryClient();
  const list = useParts(filtros);
  const repuestos = list.data ?? [];
  const sel = repuestos.find((p) => p.id === abierto) ?? null;

  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Repuestos</h1>
        <p className="text-ink-2">Lo que cargues aquí es lo único que se puede cotizar: nunca se inventa un precio.</p>
      </header>
      <section className={card} aria-label="Inventario">
        <div className="grid gap-3 sm:grid-cols-4">
          <div className="sm:col-span-2">
            <Field label="Buscar" value={filtros.q} onChange={(e) => setFiltros({ ...filtros, q: e.target.value })} placeholder="Referencia, nombre, marca u OEM" />
          </div>
          <Select id="filtro-categoria" label="Filtrar categoría" value={filtros.categoria} onChange={(v) => setFiltros({ ...filtros, categoria: v as Categoria | '' })}>
            <option value="">Todas</option>
            {Object.entries(categorias).map(([v, t]) => (
              <option key={v} value={v}>
                {t}
              </option>
            ))}
          </Select>
          <div className="flex flex-col justify-end gap-1">
            <label className="flex min-h-[32px] items-center gap-2">
              <input type="checkbox" checked={filtros.bajo} onChange={(e) => setFiltros({ ...filtros, bajo: e.target.checked })} />
              Solo bajo el mínimo
            </label>
            <label className="flex min-h-[32px] items-center gap-2">
              <input type="checkbox" checked={filtros.inactivos} onChange={(e) => setFiltros({ ...filtros, inactivos: e.target.checked })} />
              Incluir inactivos
            </label>
          </div>
        </div>
        {list.isError ? <Alert>{list.error.message}</Alert> : null}
        {list.isPending ? <p role="status">Cargando…</p> : null}
        {!list.isPending && repuestos.length === 0 ? <p className="text-ink-2">No hay repuestos con esos filtros.</p> : null}
        <ul className="flex flex-col gap-1">
          {repuestos.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => setAbierto(p.id)}
                className={`flex w-full flex-wrap items-center gap-3 rounded-xl px-3 py-2 text-left hover:bg-surface ${p.activo ? '' : 'text-ink-2'}`}
              >
                <span className="w-24 font-mono text-sm">{p.referencia}</span>
                <span className="flex-1 font-bold">
                  {p.nombre}
                  {p.marca ? <span className="font-normal text-ink-2"> · {p.marca}</span> : null}
                  {p.activo ? null : <span className="font-normal"> · inactivo</span>}
                  {p.ofrecerWhatsapp ? <span className="font-normal text-primary"> · en vitrina</span> : null}
                </span>
                {p.ubicacion ? <span className="text-sm text-ink-2">{p.ubicacion}</span> : null}
                <span className={p.bajoMinimo ? 'font-bold text-warn-ink' : ''}>
                  {p.existencia} / mín. {p.minimo}
                </span>
                <span className="w-24 text-right font-mono">{formatoCOP(p.precioVenta)}</span>
              </button>
            </li>
          ))}
        </ul>
      </section>
      {sel ? <PartDetail key={`${sel.id}-${sel.version}`} p={sel} onClose={() => setAbierto(null)} /> : null}
      <NewPart />
      {puedeImportar ? (
        <CsvImport endpoint="/parts/import" title="Importar desde Excel (CSV)" help="Guarda tu Excel como CSV." template={plantillaCSV}
          templateName="plantilla-repuestos.csv" noun="repuestos" onImported={() => void qc.invalidateQueries({ queryKey: ['repuestos'] })} />
      ) : null}
    </>
  );
}
