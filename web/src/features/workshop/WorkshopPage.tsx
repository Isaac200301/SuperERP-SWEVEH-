import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { WahaCard } from '../messaging/WahaCard';
import { card, ProfileForm, type Taller } from './ProfileForm';
import { SettingsForm } from './SettingsForm';

const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';
const dias = { lun: 'Lunes', mar: 'Martes', mie: 'Miércoles', jue: 'Jueves', vie: 'Viernes', sab: 'Sábado', dom: 'Domingo' } as const;

/** HU-08: horario semanal; un día sin horas queda cerrado. onSaved sirve para seguir con lo que pidió el horario (HU-06). */
export function Schedule({ t, onSaved }: { t: Taller; onSaved?: () => void }) {
  const qc = useQueryClient();
  const [h, setH] = useState<Record<string, { abre: string; cierra: string } | null>>(() =>
    Object.fromEntries(Object.keys(dias).map((d) => [d, t.horario[d] ?? null])),
  );
  const guardar = useMutation({
    mutationFn: async () => unwrap(await api.PUT('/workshop/schedule', { body: { horario: h } })),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ['taller'] });
      onSaved?.();
    },
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
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-[28px] font-extrabold">Mi taller</h1>
        <Link to="/panel/whatsapp-prueba" className="inline-flex min-h-[44px] items-center font-bold text-primary">
          Probar WhatsApp
        </Link>
      </header>
      <ProfileForm t={q.data} />
      <Schedule t={q.data} />
      <SettingsForm />
      <WahaCard />
    </>
  );
}
