import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

type Taller = Schemas['Taller'];
const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const dias = { lun: 'Lunes', mar: 'Martes', mie: 'Miércoles', jue: 'Jueves', vie: 'Viernes', sab: 'Sábado', dom: 'Domingo' } as const;
const especialidades = {
  mecanica_general: 'Mecánica general',
  frenos_suspension: 'Frenos y suspensión',
  electrico: 'Eléctrico',
  diagnostico_escaner: 'Diagnóstico con escáner',
  llantas_alineacion: 'Llantas y alineación',
  latoneria_pintura: 'Latonería y pintura',
} as const;

/** HU-03 y HU-89: tipo de taller y contrato de transmisión; al terminar se precargan los servicios (HU-04). */
function Profile({ t }: { t: Taller }) {
  const qc = useQueryClient();
  const guardar = useMutation({
    mutationFn: async (b: { nombre: string; atiende: string; especialidades: string[]; ciudad: string; tamano: string; contratoAceptado: boolean }) =>
      unwrap(await api.POST('/workshop/onboarding', { body: b })),
    onSuccess: () => qc.invalidateQueries(),
  });
  const [f, setF] = useState({ nombre: t.nombre, atiende: t.atiende || 'ambos', ciudad: t.ciudad, tamano: t.tamano || 'solo', esp: t.especialidades, contrato: t.contratoAceptadoEn != null });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    guardar.mutate({ nombre: f.nombre, atiende: f.atiende, especialidades: f.esp, ciudad: f.ciudad, tamano: f.tamano, contratoAceptado: f.contrato });
  };
  return (
    <section className={card} aria-labelledby="perfil">
      <h2 id="perfil" className="text-[17px] font-extrabold">Tu taller</h2>
      <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-2">
        <Field label="Nombre del taller" value={f.nombre} onChange={(e) => setF({ ...f, nombre: e.target.value })} />
        <Field label="Ciudad" value={f.ciudad} onChange={(e) => setF({ ...f, ciudad: e.target.value })} />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="atiende" className="text-[15px] font-extrabold">Atiendes</label>
          <select id="atiende" className={select} value={f.atiende} onChange={(e) => setF({ ...f, atiende: e.target.value })}>
            <option value="carro">Carros</option>
            <option value="moto">Motos</option>
            <option value="ambos">Carros y motos</option>
          </select>
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
          <legend className="mb-1 text-[15px] font-extrabold">Especialidades</legend>
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
          <Button type="submit" busy={guardar.isPending} missing={f.contrato ? undefined : 'Para continuar acepta el contrato.'}>
            {t.onboardingCompleto ? 'Guardar cambios' : 'Terminar el arranque'}
          </Button>
        </div>
      </form>
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
    </section>
  );
}

/** HU-08: horario semanal; un día sin horas queda cerrado. */
function Schedule({ t }: { t: Taller }) {
  const qc = useQueryClient();
  const [h, setH] = useState<Record<string, { abre: string; cierra: string } | null>>(() =>
    Object.fromEntries(Object.keys(dias).map((d) => [d, t.horario[d] ?? null])),
  );
  const guardar = useMutation({
    mutationFn: async () => unwrap(await api.PUT('/workshop/schedule', { body: { horario: h } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['taller'] }),
  });
  return (
    <section className={card} aria-labelledby="horario">
      <h2 id="horario" className="text-[17px] font-extrabold">Horario</h2>
      <ul className="flex flex-col gap-2">
        {Object.entries(dias).map(([d, nombre]) => {
          const s = h[d];
          return (
            <li key={d} className="flex flex-wrap items-center gap-3">
              <label className="flex min-h-[44px] w-36 items-center gap-2 font-bold">
                <input type="checkbox" checked={s != null} onChange={(e) => setH({ ...h, [d]: e.target.checked ? { abre: '08:00', cierra: '17:00' } : null })} />
                {nombre}
              </label>
              {s ? (
                <>
                  <input aria-label={`${nombre} abre`} type="time" className={select} value={s.abre} onChange={(e) => setH({ ...h, [d]: { ...s, abre: e.target.value } })} />
                  <input aria-label={`${nombre} cierra`} type="time" className={select} value={s.cierra} onChange={(e) => setH({ ...h, [d]: { ...s, cierra: e.target.value } })} />
                </>
              ) : (
                <span className="text-ink-2">Cerrado</span>
              )}
            </li>
          );
        })}
      </ul>
      <Button variant="secondary" busy={guardar.isPending} onClick={() => guardar.mutate()}>
        Guardar horario
      </Button>
      {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
      {guardar.isSuccess ? <p role="status" className="font-bold text-primary">Horario guardado.</p> : null}
    </section>
  );
}

export function WorkshopPage() {
  const q = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  if (q.isPending) return <p role="status">Cargando…</p>;
  if (q.isError) return <Alert>{q.error.message}</Alert>;
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Mi taller</h1>
      </header>
      <Profile t={q.data} />
      <Schedule t={q.data} />
    </>
  );
}
