import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { CsvImport } from '../../shared/import/CsvImport';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
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

/** HU-87: el titular pide una copia de sus datos o que los borren; lo atiende el dueño (data_request:manage). */
function DataRequest({ c }: { c: Cliente }) {
  const qc = useQueryClient();
  const [confirm, setConfirm] = useState(false);
  const exportar = useMutation({
    mutationFn: async () => unwrap(await api.GET('/customers/{id}/export', { params: { path: { id: c.id } } })),
    onSuccess: (data) => {
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = `datos-${c.celular}.json`;
      a.click();
      URL.revokeObjectURL(url);
    },
  });
  const borrar = useMutation({
    mutationFn: async () => unwrap(await api.POST('/customers/{id}/anonymize', { params: { path: { id: c.id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clientes'] }),
  });
  return (
    <div className="flex flex-col gap-2 border-t border-line pt-3">
      <b className="text-sm">Solicitud de datos del cliente</b>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" busy={exportar.isPending} onClick={() => exportar.mutate()}>Descargar sus datos</Button>
        {!confirm ? <Button variant="secondary" onClick={() => setConfirm(true)}>Borrar sus datos</Button> : null}
      </div>
      {confirm ? (
        <div role="alert" className="flex flex-col gap-2 rounded-2xl bg-warn-tint p-3 text-warn-ink">
          <b>Se borran el nombre, el celular, el correo, las placas y las conversaciones de {c.nombre}. Las órdenes y los montos se
            conservan sin identificarlo. No se puede deshacer.</b>
          <div className="flex flex-wrap gap-2">
            <Button busy={borrar.isPending} onClick={() => borrar.mutate()}>Sí, borrar sus datos</Button>
            <Button variant="secondary" onClick={() => setConfirm(false)}>Cancelar</Button>
          </div>
        </div>
      ) : null}
      {exportar.isError ? <Alert>{exportar.error.message}</Alert> : null}
      {borrar.isError ? <Alert>{borrar.error.message}</Alert> : null}
    </div>
  );
}

function Row({ c }: { c: Cliente }) {
  const [open, setOpen] = useState(false);
  const puedeDatos = usePermission('data_request:manage');
  return (
    <li className="rounded-2xl border border-line">
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)}
        className="flex min-h-[52px] w-full flex-wrap items-center justify-between gap-2 px-4 py-2 text-left">
        <span className="font-bold">{c.nombre}</span>
        <span className="font-mono text-ink-2">{c.celular}</span>
        <span className="text-sm text-ink-2">{autorizacion[c.autorizacionEstado] ?? c.autorizacionEstado}</span>
      </button>
      {open ? (
        <div className="flex flex-col gap-3 border-t border-line px-4 py-3">
          <Vehicles id={c.id} />
          {puedeDatos ? <DataRequest c={c} /> : null}
        </div>
      ) : null}
    </li>
  );
}

/**
 * HU-15: clientes que parecen la misma persona (mismo nombre con otro celular), lado a lado con sus vehículos. El dueño
 * elige cuál se conserva; al unir, los vehículos, órdenes y conversaciones del otro pasan a ese.
 */
function Duplicates() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ['clientes', 'repetidos'], queryFn: async () => unwrap(await api.GET('/customers/duplicates')).grupos });
  const [keep, setKeep] = useState<Record<number, string>>({});
  const [confirm, setConfirm] = useState<number | null>(null);
  const unir = useMutation({
    mutationFn: async (v: { conservar: string; unir: string }) => unwrap(await api.POST('/customers/merge', { body: v })),
    onSuccess: async () => {
      setConfirm(null);
      await qc.invalidateQueries({ queryKey: ['clientes'] });
    },
  });
  if (!q.data || q.data.length === 0) return null;
  return (
    <section className="flex flex-col gap-4 rounded-card bg-white p-5" aria-labelledby="repetidos">
      <div>
        <h2 id="repetidos" className="text-[17px] font-extrabold">Posibles repetidos</h2>
        <p className="text-sm text-ink-2">Tienen el mismo nombre con otro celular. Elige cuál queda: el otro le pasa sus vehículos, órdenes y conversaciones.</p>
      </div>
      {q.data.map((g, i) => {
        const [a, b] = g.clientes;
        const conservar = keep[i] ?? a.id;
        const otro = g.clientes.find((c) => c.id !== conservar) ?? b;
        const queda = g.clientes.find((c) => c.id === conservar) ?? a;
        return (
          <div key={g.clientes.map((c) => c.id).join()} className="flex flex-col gap-3 rounded-2xl border border-line p-4">
            <div className="grid gap-3 sm:grid-cols-2">
              {g.clientes.slice(0, 2).map((c) => (
                <label key={c.id} className={`flex flex-col gap-2 rounded-xl p-3 ${conservar === c.id ? 'bg-primary-tint' : 'bg-surface'}`}>
                  <span className="flex items-center gap-2 font-bold">
                    <input type="radio" name={`conservar-${i}`} checked={conservar === c.id} onChange={() => setKeep({ ...keep, [i]: c.id })} />
                    Conservar a {c.nombre} · {c.celular}
                  </span>
                  <span className="text-sm text-ink-2">{autorizacion[c.autorizacionEstado] ?? c.autorizacionEstado}</span>
                  <Vehicles id={c.id} />
                </label>
              ))}
            </div>
            {confirm === i ? (
              <div role="alert" className="flex flex-col gap-2 rounded-2xl bg-warn-tint p-3 text-warn-ink">
                <b>Los vehículos, órdenes y conversaciones de {otro.nombre} ({otro.celular}) pasan a {queda.nombre} ({queda.celular}). No se puede deshacer.</b>
                <div className="flex flex-wrap gap-2">
                  <Button busy={unir.isPending} onClick={() => unir.mutate({ conservar: queda.id, unir: otro.id })}>Sí, unir</Button>
                  <Button variant="secondary" onClick={() => setConfirm(null)}>Cancelar</Button>
                </div>
              </div>
            ) : (
              <div>
                <Button variant="secondary" onClick={() => setConfirm(i)}>Unir en {queda.nombre}</Button>
              </div>
            )}
          </div>
        );
      })}
      {unir.isError ? <Alert>{unir.error.message}</Alert> : null}
    </section>
  );
}

/** HU-82: clientes y vehículos del taller, para el dueño y la recepción (customer:read_all). */
export function CustomersPage() {
  const [q, setQ] = useState('');
  const term = q.trim();
  const qc = useQueryClient();
  const puedeImportar = usePermission('import:run');
  const puedeUnir = usePermission('customer:merge');
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
      {puedeUnir ? <Duplicates /> : null}
      {puedeImportar ? (
        <CsvImport endpoint="/vehicles/import" title="Subir clientes y vehículos (CSV)" noun="vehículos"
          help="Trae tus clientes desde Excel: guárdalo como CSV. Los clientes nuevos quedan con la autorización de datos pendiente."
          template={plantillaClientes} templateName="plantilla-clientes-vehiculos.csv"
          onImported={() => void qc.invalidateQueries({ queryKey: ['clientes'] })} />
      ) : null}
    </>
  );
}
