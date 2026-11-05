import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { CsvImport } from '../../shared/import/CsvImport';
import { Alert } from '../../shared/ui/Alert';
import { Field } from '../../shared/ui/Field';
import { Plate } from '../../shared/ui/Plate';
import { usePermission } from '../auth/api';

type Cliente = Schemas['Cliente'];

// HU-92: encabezados como los entiende el servidor (vehicles/domain.importAliases). Obligatorias: nombre, celular, placa
// y tipo. Sin cilindraje, turbo, energía o caja el vehículo queda con motor pendiente.
export const plantillaClientes =
  'Nombre;Celular;Placa;Tipo de vehículo;Marca;Línea;Modelo;Kilometraje;Cilindraje;Turbo;Energía;Caja\n' +
  'Carlos Ruiz;3001234567;ABC123;Carro;Mazda;3;2015;85000;2000;No;Gasolina;Manual\n' +
  'Carlos Ruiz;3001234567;GNO66F;Moto;Yamaha;NMAX;2022;;;;;\n';

const autorizacion: Record<string, string> = {
  autorizada: 'Autorizó sus datos',
  pendiente: 'Autorización pendiente',
  rechazada: 'No autorizó',
  revocada: 'Retiró la autorización',
};

/** Vehículos de un cliente (HU-12): se cargan al abrir el cliente. */
function Vehicles({ id }: { id: string }) {
  const q = useQuery({
    queryKey: ['clientes', id, 'vehiculos'],
    queryFn: async () => unwrap(await api.GET('/customers/{id}/vehicles', { params: { path: { id } } })).vehiculos,
  });
  if (q.isPending) return <p role="status" className="text-ink-2">Cargando vehículos…</p>;
  if (q.isError) return <Alert>{q.error.message}</Alert>;
  if (q.data.length === 0) return <p className="text-ink-2">Sin vehículos registrados.</p>;
  return (
    <ul className="flex flex-col gap-2">
      {q.data.map((v) => (
        <li key={v.id} className="flex flex-wrap items-center gap-3">
          <Plate value={v.placa} />
          <span>{[v.marca, v.linea, v.modeloAnio].filter(Boolean).join(' ') || (v.tipo === 'moto' ? 'Moto' : 'Carro')}</span>
          {v.motorPendiente ? <span className="rounded-full bg-warn-tint px-2 py-0.5 text-xs font-bold text-warn-ink">Motor pendiente</span> : null}
        </li>
      ))}
    </ul>
  );
}

function Row({ c }: { c: Cliente }) {
  const [open, setOpen] = useState(false);
  return (
    <li className="rounded-2xl border border-line">
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)}
        className="flex min-h-[52px] w-full flex-wrap items-center justify-between gap-2 px-4 py-2 text-left">
        <span className="font-bold">{c.nombre}</span>
        <span className="font-mono text-ink-2">{c.celular}</span>
        <span className="text-sm text-ink-2">{autorizacion[c.autorizacionEstado] ?? c.autorizacionEstado}</span>
      </button>
      {open ? (
        <div className="border-t border-line px-4 py-3">
          <Vehicles id={c.id} />
        </div>
      ) : null}
    </li>
  );
}

/** HU-82: clientes y vehículos del taller, para el dueño y la recepción (customer:read_all). */
export function CustomersPage() {
  const [q, setQ] = useState('');
  const term = q.trim();
  const qc = useQueryClient();
  const puedeImportar = usePermission('import:run');
  const list = useQuery({
    queryKey: ['clientes', term],
    queryFn: async () => unwrap(await api.GET('/customers', { params: { query: term ? { q: term } : {} } })).clientes,
  });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold tracking-[-.02em]">Clientes</h1>
        <p className="text-ink-2">Abre un cliente para ver sus vehículos.</p>
      </header>
      <section className="flex flex-col gap-4 rounded-card bg-white p-5" aria-label="Clientes del taller">
        <Field label="Buscar por nombre o celular" value={q} onChange={(e) => setQ(e.target.value)} />
        {list.isPending ? <p role="status" className="text-ink-2">Cargando clientes…</p> : null}
        {list.isError ? <Alert>{list.error.message}</Alert> : null}
        {list.data && list.data.length === 0 ? (
          <p className="text-ink-2">{term ? 'Ningún cliente coincide con la búsqueda.' : 'Todavía no hay clientes. Se crean al abrir una orden.'}</p>
        ) : null}
        <ul className="flex flex-col gap-2">
          {(list.data ?? []).map((c) => (
            <Row key={c.id} c={c} />
          ))}
        </ul>
      </section>
      {puedeImportar ? (
        <CsvImport endpoint="/vehicles/import" title="Subir clientes y vehículos (CSV)" noun="vehículos"
          help="Trae tus clientes desde Excel: guárdalo como CSV. Los clientes nuevos quedan con la autorización de datos pendiente."
          template={plantillaClientes} templateName="plantilla-clientes-vehiculos.csv"
          onImported={() => void qc.invalidateQueries({ queryKey: ['clientes'] })} />
      ) : null}
    </>
  );
}
