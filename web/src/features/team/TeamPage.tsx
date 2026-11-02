import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api, unwrap } from '../../shared/api/client';
import { roles } from '../../shared/i18n/es-CO';
import { celularValido, formatoCelular, soloCelular } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

/** HU-80, HU-82 y HU-84: agregar mecánico o recepción por celular, y desactivar a quien se va. */
export function TeamPage() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['usuarios'], queryFn: async () => unwrap(await api.GET('/users')).usuarios });
  const done = () => qc.invalidateQueries({ queryKey: ['usuarios'] });
  const agregar = useMutation({
    mutationFn: async (b: { celular: string; nombre: string; rol: 'mecanico' | 'recepcion' }) => unwrap(await api.POST('/users', { body: b })),
    onSuccess: done,
  });
  const activo = useMutation({
    mutationFn: async (v: { id: string; activo: boolean }) =>
      unwrap(await api.PATCH('/users/{id}', { params: { path: { id: v.id } }, body: { activo: v.activo } })),
    onSettled: done,
  });
  const [nombre, setNombre] = useState('');
  const [celular, setCelular] = useState('');
  const [rol, setRol] = useState<'mecanico' | 'recepcion'>('mecanico');

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    agregar.mutate({ celular: soloCelular(celular), nombre, rol }, { onSuccess: () => { setNombre(''); setCelular(''); } });
  };
  const falta = !nombre.trim() ? 'Escribe el nombre.' : !celularValido(celular) ? 'El celular debe tener 10 dígitos.' : undefined;

  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Equipo</h1>
        <p className="text-ink-2">Entran con su propio celular y un código. El mecánico no ve dinero ni KPIs.</p>
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
            <Button type="submit" busy={agregar.isPending} missing={falta}>
              Agregar al equipo
            </Button>
          </div>
        </form>
        {agregar.isError ? <Alert>{agregar.error.message}</Alert> : null}
        {activo.isError ? <Alert>{activo.error.message}</Alert> : null}
        <ul className="flex flex-col gap-1">
          {(list.data ?? []).map((u) => (
            <li key={u.id} className={`flex flex-wrap items-center gap-3 rounded-xl px-3 py-2 ${u.activo ? '' : 'text-ink-2'}`}>
              <span className="flex-1 font-bold">{u.nombre}</span>
              <span className="font-mono text-sm">{formatoCelular(u.celular)}</span>
              <span className="text-sm">{roles[u.rol] ?? u.rol}</span>
              {u.rol !== 'dueno' ? (
                <button type="button" className="min-h-[44px] px-2 font-bold text-primary" onClick={() => activo.mutate({ id: u.id, activo: !u.activo })}>
                  {u.activo ? 'Desactivar' : 'Activar'}
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
