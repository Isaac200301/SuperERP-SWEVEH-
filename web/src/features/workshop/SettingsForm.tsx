import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent, type ReactNode } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { card } from './ProfileForm';

type Parametros = Schemas['Parametros'];
type Item = Schemas['ItemRecepcion'];
export const settingsKey = ['parametros'] as const;

/** Los parámetros los leen la recepción y la cotización; sin configurar llegan los valores por defecto. */
export function useSettings() {
  return useQuery({ queryKey: settingsKey, queryFn: async () => unwrap(await api.GET('/workshop/settings')), staleTime: 60_000 });
}

function Check({ label, hint, checked, onChange }: { label: string; hint: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex min-h-[44px] items-start gap-3">
      <input type="checkbox" className="mt-1 h-5 w-5 accent-primary" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>
        <b>{label}</b>
        <span className="block text-sm text-ink-2">{hint}</span>
      </span>
    </label>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-3 border-t border-line pt-3">
      <legend className="pr-2 text-[15px] font-extrabold">{title}</legend>
      {children}
    </fieldset>
  );
}

/** Lista editable de ítems a revisar al recibir (luces, gato, documentos…). */
function Items({ label, items, onChange }: { label: string; items: Item[]; onChange: (v: Item[]) => void }) {
  const [nuevo, setNuevo] = useState('');
  return (
    <div className="flex flex-col gap-2">
      <span className="font-bold">{label}</span>
      <ul className="flex flex-wrap gap-2">
        {items.map((it, i) => (
          <li key={`${it.clave}-${i}`} className="flex items-center gap-1 rounded-full bg-surface py-1 pr-1 pl-3 text-sm">
            {it.nombre}
            <button type="button" aria-label={`Quitar ${it.nombre} de ${label}`} className="h-8 w-8 rounded-full font-bold text-err-ink" onClick={() => onChange(items.filter((_, j) => j !== i))}>
              ×
            </button>
          </li>
        ))}
      </ul>
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-[200px] flex-1">
          <Field label={`Agregar a ${label}`} value={nuevo} onChange={(e) => setNuevo(e.target.value)} placeholder="Extintor" />
        </div>
        <Button
          variant="secondary"
          missing={nuevo.trim() ? undefined : 'Escribe el ítem.'}
          onClick={() => {
            onChange([...items, { clave: '', nombre: nuevo.trim() }]);
            setNuevo('');
          }}
        >
          Agregar
        </Button>
      </div>
    </div>
  );
}

const num = (s: string) => (s.trim() === '' ? null : Number(s.replace(/\D/g, '')));

function Form({ p }: { p: Parametros }) {
  const qc = useQueryClient();
  const [f, setF] = useState(p);
  const [tarifa, setTarifa] = useState(p.tarifaHora == null ? '' : String(p.tarifaHora));
  const guardar = useMutation({
    mutationFn: async (b: Parametros) => unwrap(await api.PUT('/workshop/settings', { body: b })),
    onSuccess: (s) => {
      qc.setQueryData(settingsKey, s);
      setF(s);
    },
  });
  const set = <K extends keyof Parametros>(k: K) => (v: Parametros[K]) => setF({ ...f, [k]: v });
  const n = (k: 'fotosMinimas' | 'ivaPct' | 'descuentoMaxPct' | 'vigenciaDias') => (e: { target: { value: string } }) => setF({ ...f, [k]: num(e.target.value) ?? 0 });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    guardar.mutate({ ...f, tarifaHora: num(tarifa) });
  };
  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4">
      <Group title="Órdenes">
        <Check label="Usar el estado Esperando repuesto" hint="El vehículo queda parado mientras llega una pieza; vuelve a En reparación cuando llega."
          checked={f.usarEsperandoRepuesto} onChange={set('usarEsperandoRepuesto')} />
        <Check label="Usar el estado Control de calidad" hint="Prueba de ruta o revisión final antes de marcar Listo."
          checked={f.usarControlCalidad} onChange={set('usarControlCalidad')} />
      </Group>
      <Group title="Recepción">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Fotos mínimas al recibir" inputMode="numeric" value={String(f.fotosMinimas)} onChange={n('fotosMinimas')} hint="De 0 a 8." />
          <Check label="Kilometraje obligatorio al recibir" hint="Sin kilometraje no pasa a Recibido." checked={f.kmObligatorio} onChange={set('kmObligatorio')} />
        </div>
        <Items label="Ítems de carro" items={f.itemsCarro} onChange={set('itemsCarro')} />
        <Items label="Ítems de moto" items={f.itemsMoto} onChange={set('itemsMoto')} />
      </Group>
      <Group title="Cotización">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="IVA (%)" inputMode="numeric" value={String(f.ivaPct)} onChange={n('ivaPct')} hint="Por defecto 19 %. Confírmalo con tu contador." />
          <Check label="Mis precios ya incluyen IVA" hint="Si no, el IVA se suma al total de la cotización." checked={f.preciosConIva} onChange={set('preciosConIva')} />
          <Field label="Tarifa por hora de mano de obra (COP)" inputMode="numeric" value={tarifa} onChange={(e) => setTarifa(e.target.value)} hint="Vacío = la mano de obra se cotiza con valor libre." />
          <Field label="Vigencia de la cotización (días)" inputMode="numeric" value={String(f.vigenciaDias)} onChange={n('vigenciaDias')} hint="De 1 a 90." />
          <Field label="Descuento máximo (%)" inputMode="numeric" value={String(f.descuentoMaxPct)} onChange={n('descuentoMaxPct')} />
          <Check label="Recepción puede dar descuentos" hint="El dueño siempre puede." checked={f.descuentoRecepcion} onChange={set('descuentoRecepcion')} />
        </div>
      </Group>
      <div>
        <Button type="submit" busy={guardar.isPending}>
          Guardar parámetros
        </Button>
      </div>
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
      {guardar.isSuccess ? <p role="status" className="font-bold text-primary">Parámetros guardados.</p> : null}
    </form>
  );
}

/** "Parametrizar todo" (decisión de Pablo, 3-nov-2026): cada taller ajusta sus reglas sin tocar código. */
export function SettingsForm() {
  const q = useSettings();
  return (
    <section className={card} aria-labelledby="parametros">
      <h2 id="parametros" className="text-[17px] font-extrabold">Parámetros</h2>
      <p className="text-sm text-ink-2">Lo que ya se cotizó o recibió no cambia si cambias un parámetro.</p>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data ? <Form p={q.data} /> : null}
    </section>
  );
}
