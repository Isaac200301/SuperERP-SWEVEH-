import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { Segmented } from '../../shared/ui/Segmented';

export type Taller = Schemas['Taller'];
export const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const especialidades = {
  mecanica_general: 'Mecánica general',
  frenos_suspension: 'Frenos y suspensión',
  electrico: 'Eléctrico',
  diagnostico_escaner: 'Diagnóstico con escáner',
  llantas_alineacion: 'Llantas y alineación',
  latoneria_pintura: 'Latonería y pintura',
} as const;

type Atiende = 'carro' | 'moto' | 'ambos' | '';

/** HU-03 y HU-89: tipo de taller y contrato de transmisión; al guardar se precargan los servicios (HU-04). */
export function ProfileForm({ t, onSaved }: { t: Taller; onSaved?: () => void }) {
  const qc = useQueryClient();
  const guardar = useMutation({
    mutationFn: async (b: { nombre: string; atiende: string; especialidades: string[]; ciudad: string; tamano: string; contratoAceptado: boolean }) =>
      unwrap(await api.POST('/workshop/onboarding', { body: b })),
    onSuccess: async () => {
      await qc.invalidateQueries();
      onSaved?.();
    },
  });
  const [f, setF] = useState({
    nombre: t.nombre, atiende: (t.atiende || '') as Atiende, ciudad: t.ciudad, tamano: t.tamano || 'solo', esp: t.especialidades,
    contrato: t.contratoAceptadoEn != null,
  });
  // HU-03 criterio 2: el tipo de vehículo es obligatorio antes de seguir.
  const missing = f.atiende === '' ? 'Escoge qué vehículos atiendes.' : !f.contrato ? 'Para continuar acepta el contrato.' : undefined;
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (missing) return;
    guardar.mutate({ nombre: f.nombre, atiende: f.atiende, especialidades: f.esp, ciudad: f.ciudad, tamano: f.tamano, contratoAceptado: f.contrato });
  };
  return (
    <section className={card} aria-labelledby="perfil">
      <h2 id="perfil" className="text-[17px] font-extrabold">Tu taller</h2>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-2">
        <Field label="Nombre del taller" value={f.nombre} onChange={(e) => setF({ ...f, nombre: e.target.value })} />
        <Field label="Ciudad" value={f.ciudad} onChange={(e) => setF({ ...f, ciudad: e.target.value })} />
        <div className="sm:col-span-2">
          <Segmented
            label="¿Qué vehículos atiendes?"
            value={f.atiende}
            onChange={(v) => setF({ ...f, atiende: v })}
            options={[
              { value: 'carro', label: 'Carros' },
              { value: 'moto', label: 'Motos' },
              { value: 'ambos', label: 'Ambos' },
            ]}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="tamano" className="text-[15px] font-extrabold">¿Cuántos trabajan?</label>
          <select id="tamano" className={select} value={f.tamano} onChange={(e) => setF({ ...f, tamano: e.target.value })}>
            <option value="solo">Solo yo</option>
            <option value="2a5">2 a 5</option>
            <option value="6mas">6 o más</option>
          </select>
        </div>
        <fieldset className="flex flex-wrap gap-3 sm:col-span-2">
          <legend className="mb-1 text-[15px] font-extrabold">¿Qué servicios haces?</legend>
          {Object.entries(especialidades).map(([v, txt]) => (
            <label key={v} className="flex min-h-[44px] items-center gap-2">
              <input
                type="checkbox"
                checked={f.esp.includes(v)}
                onChange={(e) => setF({ ...f, esp: e.target.checked ? [...f.esp, v] : f.esp.filter((x) => x !== v) })}
              />
              {txt}
            </label>
          ))}
        </fieldset>
        <label className="flex min-h-[44px] items-center gap-2 sm:col-span-2">
          <input type="checkbox" checked={f.contrato} onChange={(e) => setF({ ...f, contrato: e.target.checked })} />
          Acepto el contrato de transmisión de datos con SWEVEH (borrador, pendiente de revisión jurídica)
        </label>
        <div className="sm:col-span-2">
          <Button type="submit" busy={guardar.isPending} missing={missing}>
            {t.onboardingCompleto ? 'Guardar cambios' : 'Armar mi taller'}
          </Button>
        </div>
      </form>
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
    </section>
  );
}
