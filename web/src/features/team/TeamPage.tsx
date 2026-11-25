import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, ApiError, unwrap } from '../../shared/api/client';
import { roles } from '../../shared/i18n/es-CO';
import { celularValido, formatoCelular, soloCelular } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { errorDePin } from '../auth/pin';
import { PinInput } from '../auth/PinInput';

/**
 * HU-80, HU-82 y HU-84: agregar mecánico o recepción por celular, y desactivar a quien se va. HU-104: la persona
 * nueva entra con un PIN temporal que el dueño le dice de palabra, y el dueño se lo restablece si lo olvida.
 */
export function TeamPage() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['usuarios'], queryFn: async () => unwrap(await api.GET('/users')).usuarios });
  const done = () => qc.invalidateQueries({ queryKey: ['usuarios'] });
  const agregar = useMutation({
    mutationFn: async (b: { celular: string; nombre: string; rol: 'mecanico' | 'recepcion'; pin: string }) => unwrap(await api.POST('/users', { body: b })),
    onSuccess: done,
  });
  const activo = useMutation({
    mutationFn: async (v: { id: string; activo: boolean }) =>
      unwrap(await api.PATCH('/users/{id}', { params: { path: { id: v.id } }, body: { activo: v.activo } })),
    onSettled: done,
  });
  const restablecer = useMutation({
    mutationFn: async (v: { id: string; pin: string }) => {
      const r = await api.POST('/users/{id}/pin-reset', { params: { path: { id: v.id } }, body: { pin: v.pin } });
      if (!r.response.ok) throw new ApiError(r.response.status, r.error as never);
    },
  });
  const [pinDe, setPinDe] = useState<{ id: string; nombre: string } | null>(null);
  const [pinNuevo, setPinNuevo] = useState('');
  const [listo, setListo] = useState<string | null>(null);
  const [pin, setPin] = useState('');
  const [nombre, setNombre] = useState('');
  const [celular, setCelular] = useState('');
  const [rol, setRol] = useState<'mecanico' | 'recepcion'>('mecanico');

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    agregar.mutate({ celular: soloCelular(celular), nombre, rol, pin }, { onSuccess: () => { setNombre(''); setCelular(''); setPin(''); } });
  };
  const pinMal = (p: string) => errorDePin(p, 'Escoge un PIN temporal de 4 números.');
  let falta: string | undefined = pinMal(pin);
  if (!celularValido(celular)) falta = 'El celular debe tener 10 dígitos.';
  if (!nombre.trim()) falta = 'Escribe el nombre.';

  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Equipo</h1>
        <p className="text-ink-2">Entran con su celular y un PIN. Ponle un PIN temporal y díselo de palabra: al entrar escoge el suyo.</p>
      </header>
      <section className="flex flex-col gap-3 rounded-card bg-white p-5">
        <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-3">
          <Field label="Nombre" value={nombre} onChange={(e) => setNombre(e.target.value)} />
          <Field label="Celular" inputMode="tel" value={celular} onChange={(e) => setCelular(e.target.value)} placeholder="300 123 4567" />
          <div className="flex flex-col gap-1.5">
            <label htmlFor="rol" className="text-[15px] font-extrabold">
              Rol
            </label>
            <select
              id="rol"
              className="min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base"
              value={rol}
              onChange={(e) => setRol(e.target.value as 'mecanico' | 'recepcion')}
            >
              <option value="mecanico">Mecánico</option>
              <option value="recepcion">Recepción</option>
            </select>
          </div>
          <div className="sm:col-span-3">
            <PinInput label="PIN temporal" value={pin} onChange={setPin} autoComplete="new-password" hint="Se lo dices de palabra; no se muestra en ninguna parte." />
          </div>
          <div className="sm:col-span-3">
            <Button type="submit" busy={agregar.isPending} missing={falta}>
              Agregar al equipo
            </Button>
          </div>
        </form>
        {agregar.isError ? <Alert>{agregar.error.message}</Alert> : null}
        {activo.isError ? <Alert>{activo.error.message}</Alert> : null}
        {listo ? <p role="status" className="rounded-xl bg-primary-tint p-3 text-sm font-bold text-primary">{listo}</p> : null}
        <ul className="flex flex-col gap-1">
          {(list.data ?? []).map((u) => (
            <li key={u.id} className={`flex flex-wrap items-center gap-3 rounded-xl px-3 py-2 ${u.activo ? '' : 'text-ink-2'}`}>
              <span className="flex-1 font-bold">{u.nombre}</span>
              <span className="font-mono text-sm">{formatoCelular(u.celular)}</span>
              <span className="text-sm">{roles[u.rol] ?? u.rol}</span>
              {u.rol !== 'dueno' ? (
                <>
                  <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => { setPinDe({ id: u.id, nombre: u.nombre }); setPinNuevo(''); setListo(null); restablecer.reset(); }}>
                    Restablecer PIN
                  </button>
                  <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => activo.mutate({ id: u.id, activo: !u.activo })}>
                    {u.activo ? 'Desactivar' : 'Activar'}
                  </button>
                </>
              ) : null}
            </li>
          ))}
        </ul>
        {pinDe ? (
          <form
            className="flex flex-col gap-3 rounded-2xl bg-surface p-4"
            aria-label={`Restablecer el PIN de ${pinDe.nombre}`}
            onSubmit={(e) => {
              e.preventDefault();
              if (pinMal(pinNuevo)) return;
              restablecer.mutate(
                { id: pinDe.id, pin: pinNuevo },
                { onSuccess: () => { setListo(`Listo: dile a ${pinDe.nombre} su PIN temporal. Al entrar escoge el suyo.`); setPinDe(null); } },
              );
            }}
          >
            <PinInput label={`PIN temporal para ${pinDe.nombre}`} value={pinNuevo} onChange={setPinNuevo} autoComplete="new-password" />
            {restablecer.isError ? <Alert>{restablecer.error.message}</Alert> : null}
            <div className="flex flex-wrap gap-3">
              <Button type="submit" busy={restablecer.isPending} missing={pinMal(pinNuevo)}>
                Poner PIN temporal
              </Button>
              <Button variant="secondary" onClick={() => setPinDe(null)}>
                Cancelar
              </Button>
            </div>
          </form>
        ) : null}
      </section>
    </>
  );
}
