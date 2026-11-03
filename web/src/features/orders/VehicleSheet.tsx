import { useId } from 'react';

import { Field } from '../../shared/ui/Field';
import { IconCheck, IconWarn } from '../../shared/ui/icons';
import { Segmented } from '../../shared/ui/Segmented';
import { conDefectos, esElectrico, faltantesFicha, opciones, type Ficha, type Tipo } from './sheet';

const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';

function Select({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string }[]; onChange: (v: string) => void }) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[15px] font-extrabold">
        {label}
      </label>
      <select id={id} className={select} value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">Sin dato</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}

/**
 * Ficha del vehículo: lo que el taller necesita para trabajar, según sea carro o moto. Combustible, caja y,
 * en combustión, cilindraje y aspiración (o tiempos en moto) se piden antes de pasar a Recibido (HU-11).
 */
export function VehicleSheet({ tipo, value, onChange, error }: { tipo: Tipo | null; value: Ficha; onChange: (f: Ficha) => void; error?: (campo: string) => string | undefined }) {
  const t: Tipo = tipo ?? 'carro';
  const op = opciones[t];
  const f = conDefectos(tipo, value);
  const set = (k: keyof Ficha) => (v: string) => onChange({ ...value, [k]: v });
  const falta = faltantesFicha(tipo, value);
  const combustion = !esElectrico(f);

  return (
    <section aria-label="Ficha del vehículo" className={`flex flex-col gap-4 rounded-card p-5 ${falta.length ? 'bg-warn-tint' : 'bg-white'}`}>
      <h2 className="flex items-center gap-2 text-[17px] font-extrabold">
        {falta.length ? <IconWarn size={18} className="text-warn-ink" /> : <IconCheck size={18} className="text-ok" />}
        {falta.length ? 'Ficha pendiente' : 'Ficha completa'}
      </h2>
      <p className="text-sm text-ink-2">
        {falta.length ? `Para Recibido falta: ${falta.join(', ')}. Puedes dejarlo para después.` : 'Con esto el taller sabe qué aceite, repuestos y procedimiento lleva.'}
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        <Select label="Combustible" value={f.energia} options={op.energia} onChange={set('energia')} />
        <Select label="Caja" value={f.transmision} options={op.transmision} onChange={set('transmision')} />
        {combustion ? (
          <>
            <Field
              label="Cilindraje"
              value={f.cilindraje}
              onChange={(e) => set('cilindraje')(e.target.value)}
              placeholder={t === 'moto' ? '150' : '1600 o 1.6 L'}
              hint="Se guarda siempre en cc."
              error={error?.('cilindrajeCc')}
            />
            <Segmented label="Motor" value={f.aspiracion} onChange={set('aspiracion')} options={op.aspiracion} />
          </>
        ) : null}
        {t === 'moto' && combustion ? <Segmented label="Tiempos" value={f.ciclo} onChange={set('ciclo')} options={op.ciclo} /> : null}
        {t === 'moto' ? <Segmented label="Refrigeración" value={f.refrigeracion} onChange={set('refrigeracion')} options={op.refrigeracion} /> : null}
        {t === 'carro' ? <Select label="Tracción" value={f.traccion} options={op.traccion} onChange={set('traccion')} /> : null}
        {t === 'carro' ? <Select label="Carrocería" value={f.carroceria} options={op.carroceria} onChange={set('carroceria')} /> : null}
        <Field label="Color" value={f.color} onChange={(e) => set('color')(e.target.value)} placeholder="Gris" error={error?.('color')} />
        <Field
          label="Medida de llanta"
          value={f.llanta}
          onChange={(e) => set('llanta')(e.target.value)}
          placeholder={t === 'moto' ? '90/90-17' : '205/55 R16'}
          error={error?.('llanta')}
        />
      </div>
    </section>
  );
}
