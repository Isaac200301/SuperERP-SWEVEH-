import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, ApiError, unwrap, type Orden, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { Segmented } from '../../shared/ui/Segmented';
import { formatoCOP } from '../../shared/lib/format';
import { usePermission } from '../auth/api';
import { cotizarEvento } from './OrderWork';
import { maxFotosPorNota, photoUrl, useAddEvidence, useEvidence, useQuoteActions } from './work';

type Sistema = Schemas['SistemaVehiculo'];
type Gravedad = Schemas['Gravedad'];

const sistemas: Record<Sistema, string> = {
  motor: 'Motor', frenos: 'Frenos', suspension: 'Suspensión', direccion: 'Dirección', electrico: 'Eléctrico', transmision: 'Caja y transmisión',
  refrigeracion: 'Refrigeración', llantas: 'Llantas', carroceria: 'Carrocería', otro: 'Otro',
};
const gravedades: Record<Gravedad, { label: string; tone: string }> = {
  urgente: { label: 'Urgente', tone: 'bg-err-tint text-err-ink' },
  recomendado: { label: 'Recomendado', tone: 'bg-warn-tint text-warn-ink' },
  puede_esperar: { label: 'Puede esperar', tone: 'bg-surface text-ink-2' },
};
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
// Igual que orders/domain.Order.CanAddFindings y CanRemoveFindings.
const agregaEn = ['recibido', 'en_diagnostico', 'en_reparacion'];
const borraEn = ['recibido', 'en_diagnostico'];

/**
 * Diagnóstico en la orden (HU-103 propuesta): hallazgos por sistema con gravedad y códigos del escáner.
 * Lo registra una persona del taller; la IA no diagnostica. No muestra precios: desde cada hallazgo se cotiza.
 */
export function DiagnosisPanel({ orden }: { orden: Orden }) {
  const qc = useQueryClient();
  const key = ['hallazgos', orden.id];
  const puede = usePermission('order:diagnose');
  const cotiza = usePermission('quote:edit');
  const verDinero = usePermission('money:read') || cotiza;
  const quote = useQuoteActions(orden.id);
  const ev = useEvidence(orden.id);
  const subir = useAddEvidence(orden.id);
  const list = useQuery({
    queryKey: key,
    queryFn: async () => unwrap(await api.GET('/orders/{id}/findings', { params: { path: { id: orden.id } } })).hallazgos,
  });
  const done = () => qc.invalidateQueries({ queryKey: key });
  const add = useMutation({
    mutationFn: async (b: { sistema: Sistema; descripcion: string; gravedad: Gravedad; codigosFalla: string[]; manoObra?: number }) =>
      unwrap(await api.POST('/orders/{id}/findings', { params: { path: { id: orden.id } }, body: b })),
    onSuccess: done,
  });
  const remove = useMutation({
    mutationFn: async (fid: string) => {
      const r = await api.DELETE('/orders/{id}/findings/{fid}', { params: { path: { id: orden.id, fid } } });
      if (!r.response.ok) throw new ApiError(r.response.status, r.error as never);
    },
    onSettled: done,
  });
  const [sistema, setSistema] = useState<Sistema | ''>('');
  const [descripcion, setDescripcion] = useState('');
  const [gravedad, setGravedad] = useState<Gravedad | ''>('');
  const [codigos, setCodigos] = useState('');
  const [manoObra, setManoObra] = useState('');

  let missing: string | undefined;
  if (sistema === '') missing = 'Escoge el sistema.';
  else if (descripcion.trim() === '') missing = 'Describe lo que encontraste.';
  else if (gravedad === '') missing = 'Escoge la gravedad.';

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (missing || sistema === '' || gravedad === '') return;
    const codigosFalla = codigos.split(/[\s,;]+/).filter(Boolean);
    const mo = manoObra.trim() === '' ? undefined : Number(manoObra.replace(/\D/g, ''));
    add.mutate(
      { sistema, descripcion: descripcion.trim(), gravedad, codigosFalla, ...(mo !== undefined ? { manoObra: mo } : {}) },
      { onSuccess: () => { setDescripcion(''); setCodigos(''); setGravedad(''); setManoObra(''); } },
    );
  };
  const error = add.error ?? remove.error ?? quote.add.error ?? subir.error;
  // "Cotizar esto": agrega la mano de obra estimada como ítem de precio libre y busca los repuestos de ese sistema.
  const cotizar = (h: (typeof hallazgos)[number]) => {
    if (cotiza && h.manoObra != null) {
      quote.add.mutate({ tipo: 'servicio', descripcion: `Mano de obra · ${sistemas[h.sistema]}: ${h.descripcion}`.slice(0, 200), cantidad: 1, precioUnitario: h.manoObra });
    }
    window.dispatchEvent(new CustomEvent(cotizarEvento, { detail: sistemas[h.sistema] }));
  };
  const hallazgos = list.data ?? [];

  return (
    <section className="flex flex-col gap-3 rounded-card bg-white p-5" aria-labelledby="diagnostico">
      <h2 id="diagnostico" className="text-[17px] font-extrabold">
        Diagnóstico ({hallazgos.length})
      </h2>
      {list.isError ? <Alert>{list.error.message}</Alert> : null}
      {hallazgos.length === 0 && !list.isPending ? <p className="text-ink-2">Aún no hay hallazgos.</p> : null}
      <ul className="flex flex-col gap-2">
        {hallazgos.map((h) => (
          <li key={h.id} className="flex flex-wrap items-center gap-3 rounded-2xl border border-line p-3">
            <span className={`rounded-full px-3 py-1 text-sm font-bold ${gravedades[h.gravedad].tone}`}>{gravedades[h.gravedad].label}</span>
            <span className="font-bold">{sistemas[h.sistema]}</span>
            <span className="min-w-[200px] flex-1">{h.descripcion}</span>
            {h.codigosFalla.length ? <span className="font-mono text-sm">{h.codigosFalla.join(' · ')}</span> : null}
            {verDinero && h.manoObra != null ? <span className="font-mono text-sm">Mano de obra {formatoCOP(h.manoObra)}</span> : null}
            {(ev.data ?? []).filter((x) => x.hallazgoId === h.id && x.tieneFoto).map((x) => (
              <a key={x.id} href={photoUrl(orden.id, x.id)} target="_blank" rel="noopener noreferrer">
                <img src={photoUrl(orden.id, x.id)} alt={`Foto de ${h.descripcion}`} className="h-12 w-12 rounded-lg object-cover" loading="lazy" />
              </a>
            ))}
            {puede && agregaEn.includes(orden.estado) ? (
              <label className="min-h-[44px] cursor-pointer content-center px-2 font-bold text-primary">
                Agregar fotos
                <input
                  type="file"
                  accept="image/*"
                  capture="environment"
                  multiple
                  aria-label={`Fotos de ${h.descripcion}`}
                  className="sr-only"
                  onChange={(e) => {
                    const fotos = [...(e.target.files ?? [])].slice(0, maxFotosPorNota);
                    if (fotos.length) subir.mutate({ fotos, hallazgoId: h.id });
                    e.target.value = '';
                  }}
                />
              </label>
            ) : null}
            <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => cotizar(h)}>
              Cotizar esto
            </button>
            {puede && borraEn.includes(orden.estado) ? (
              <button type="button" className="min-h-[44px] px-2 font-bold text-err-ink" onClick={() => remove.mutate(h.id)}>
                Quitar
              </button>
            ) : null}
          </li>
        ))}
      </ul>
      {puede && agregaEn.includes(orden.estado) ? (
        <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="sistema" className="text-[15px] font-extrabold">
              Sistema
            </label>
            <select id="sistema" className={select} value={sistema} onChange={(e) => setSistema(e.target.value as Sistema)}>
              <option value="">Escoge…</option>
              {Object.entries(sistemas).map(([v, t]) => (
                <option key={v} value={v}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <Field label="Códigos de falla (opcional)" value={codigos} onChange={(e) => setCodigos(e.target.value)} placeholder="P0301 P0420" mono />
          {verDinero ? (
            <Field label="Mano de obra estimada (COP)" inputMode="numeric" value={manoObra} onChange={(e) => setManoObra(e.target.value)} hint="Opcional; con Cotizar esto pasa a la cotización." />
          ) : null}
          <div className="sm:col-span-2">
            <Field label="Qué encontraste" value={descripcion} onChange={(e) => setDescripcion(e.target.value)} placeholder="Pastillas delanteras al 10 %" />
          </div>
          <div className="sm:col-span-2">
            <Segmented
              label="Gravedad"
              value={gravedad}
              onChange={setGravedad}
              options={Object.entries(gravedades).map(([v, g]) => ({ value: v as Gravedad, label: g.label }))}
            />
          </div>
          <div className="sm:col-span-2">
            <Button type="submit" busy={add.isPending} missing={missing}>
              Agregar hallazgo
            </Button>
          </div>
        </form>
      ) : null}
      {error ? <Alert>{error.message}</Alert> : null}
    </section>
  );
}
