import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Orden } from '../../shared/api/client';
import { formatoCOP, pesos } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { usePermission } from '../auth/api';
import { ordersKey } from './api';
import { useAddEvidence } from './work';

const key = (id: string) => [...ordersKey, id, 'gastos'] as const;

function useExpenses(id: string, enabled: boolean) {
  return useQuery({
    queryKey: key(id),
    enabled,
    queryFn: async () => unwrap(await api.GET('/orders/{id}/expenses', { params: { path: { id } } })),
  });
}

/**
 * "Lo gastado en la obra" (V-08, HU-106, ADR-0040; Figma "09"): cotizado · gastado · quedan, con la barra de avance y
 * "+ Anotar un gasto" (qué fue, el valor y, si quiere, la foto de la factura). Interno: nunca lo ve el cliente.
 * La frase y la foto leídas por el asistente llegan en R8.
 */
export function SpendPanel({ orden }: { orden: Orden }) {
  const puede = usePermission('expense:manage');
  const qc = useQueryClient();
  const q = useExpenses(orden.id, puede);
  const foto = useAddEvidence(orden.id);
  const done = () => qc.invalidateQueries({ queryKey: key(orden.id) });
  const anotar = useMutation({
    mutationFn: async (b: { id: string; descripcion: string; valor: number; origen: 'manual' | 'factura'; fotoId?: string }) =>
      unwrap(await api.POST('/orders/{id}/expenses', { params: { path: { id: orden.id } }, body: b })),
    onSettled: done,
  });
  const anular = useMutation({
    mutationFn: async (gid: string) => unwrap(await api.POST('/orders/{id}/expenses/{gid}/void', { params: { path: { id: orden.id, gid } } })),
    onSettled: done,
  });
  const [abierto, setAbierto] = useState(false);
  const [desc, setDesc] = useState('');
  const [valor, setValor] = useState('');
  const [archivo, setArchivo] = useState<File | null>(null);
  // El id lo genera la pantalla: si el envío se repite (red lenta, doble toque), el gasto se anota una sola vez.
  const [envio, setEnvio] = useState(() => crypto.randomUUID());
  if (!puede) return null;
  if (q.isPending) return <p role="status">Cargando lo gastado…</p>;
  if (q.isError) return <Alert>{q.error.message}</Alert>;
  const { resumen, gastos } = q.data;
  const pct = resumen.cotizado > 0 ? Math.min(100, Math.round((resumen.gastado * 100) / resumen.cotizado)) : 0;
  const pasado = resumen.quedan < 0;
  const v = pesos(valor);
  let falta: string | undefined;
  if (!desc.trim()) falta = 'Escribe qué fue el gasto.';
  else if (v === undefined || v <= 0) falta = 'Escribe el valor.';

  async function guardar(e: FormEvent) {
    e.preventDefault();
    if (falta || v === undefined) return;
    let fotoId: string | undefined;
    if (archivo) fotoId = (await foto.mutateAsync({ nota: `Factura: ${desc.trim()}`, foto: archivo, mostrarCliente: false })).id;
    await anotar.mutateAsync({ id: envio, descripcion: desc.trim(), valor: v, origen: fotoId ? 'factura' : 'manual', ...(fotoId ? { fotoId } : {}) });
    setDesc('');
    setValor('');
    setArchivo(null);
    setEnvio(crypto.randomUUID());
    setAbierto(false);
  }

  return (
    <section aria-labelledby="gastado" className="flex flex-col gap-3 rounded-card bg-white p-5">
      <h2 id="gastado" className="text-[17px] font-extrabold">
        Lo gastado en la obra
      </h2>
      <p className="text-xs font-bold text-ink-2">Solo el taller: el cliente no lo ve.</p>
      <div
        role="meter"
        aria-label="Gastado de lo cotizado"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        className="h-3 overflow-hidden rounded-full bg-surface"
      >
        <div className={`h-full ${pasado ? 'bg-err' : 'bg-primary'}`} style={{ width: `${pasado ? 100 : pct}%` }} />
      </div>
      <dl className="grid grid-cols-3 gap-2 text-sm">
        <div>
          <dt className="text-ink-2">Cotizado</dt>
          <dd className="font-mono font-bold">{formatoCOP(resumen.cotizado)}</dd>
        </div>
        <div>
          <dt className="text-ink-2">Gastado</dt>
          <dd className="font-mono font-bold">{formatoCOP(resumen.gastado)}</dd>
        </div>
        <div>
          <dt className="text-ink-2">{pasado ? 'Se pasó' : 'Quedan'}</dt>
          <dd className={`font-mono font-bold ${pasado ? 'text-err-ink' : ''}`}>{formatoCOP(Math.abs(resumen.quedan))}</dd>
        </div>
      </dl>
      {gastos.length > 0 ? (
        <ul className="flex flex-col gap-1 text-sm">
          {gastos.map((g) => (
            <li key={g.id} className={`flex flex-wrap items-center gap-2 ${g.anulado ? 'text-ink-2 line-through' : ''}`}>
              <span className="flex-1">
                {g.descripcion}
                {g.origen === 'factura' ? <span className="ml-1 text-xs no-underline">· con factura</span> : null}
              </span>
              <span className="font-mono">{formatoCOP(g.valor)}</span>
              {g.anulado ? (
                <span className="text-xs font-bold no-underline">anulado</span>
              ) : (
                <button type="button" className="min-h-[44px] px-2 font-bold text-err-ink" onClick={() => anular.mutate(g.id)}>
                  Anular {g.descripcion}
                </button>
              )}
            </li>
          ))}
        </ul>
      ) : null}
      {abierto ? (
        <form aria-label="Anotar un gasto" onSubmit={guardar} className="grid gap-3 rounded-2xl bg-surface p-3 sm:grid-cols-3">
          <div className="sm:col-span-2">
            <Field label="¿Qué fue?" value={desc} onChange={(e) => setDesc(e.target.value)} />
          </div>
          <Field label="Valor del gasto (COP)" inputMode="numeric" value={valor} onChange={(e) => setValor(e.target.value)} />
          <label className="flex flex-col gap-1 text-sm font-bold sm:col-span-3">
            Foto de la factura (opcional)
            <input type="file" accept="image/*" capture="environment" onChange={(e) => setArchivo(e.target.files?.[0] ?? null)} />
          </label>
          <div className="flex flex-wrap gap-3 sm:col-span-3">
            <Button type="submit" busy={anotar.isPending || foto.isPending} missing={falta}>
              Guardar el gasto
            </Button>
            <Button variant="secondary" onClick={() => setAbierto(false)}>
              Cancelar
            </Button>
          </div>
        </form>
      ) : (
        <Button variant="secondary" onClick={() => setAbierto(true)}>
          + Anotar un gasto (frase o foto de la factura)
        </Button>
      )}
      {anotar.error ?? anular.error ?? foto.error ? <Alert>{(anotar.error ?? anular.error ?? foto.error)?.message}</Alert> : null}
    </section>
  );
}
