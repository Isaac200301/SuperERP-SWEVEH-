import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router';

import { api, ApiError, unwrap } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { card, ProfileForm, type Taller } from './ProfileForm';
import { Schedule } from './WorkshopPage';

const tipos = { ambos: 'Carro y moto', carro: 'Carro', moto: 'Moto' } as const;

function Paso({ n, de, titulo, detalle }: { n: number; de: number; titulo: string; detalle: string }) {
  return (
    <header className="flex flex-col gap-1">
      <span className="font-mono text-[13px] font-bold text-primary">
        PASO {n} DE {de}
      </span>
      <h1 className="text-[28px] font-extrabold tracking-[-.02em]">{titulo}</h1>
      <p className="text-ink-2">{detalle}</p>
    </header>
  );
}

/** HU-04 criterio 4: la precarga se puede renombrar, borrar o ampliar sin salir del registro. */
function PreloadedServices({ t }: { t: Taller }) {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ['servicios'], queryFn: async () => unwrap(await api.GET('/services')).servicios });
  const done = () => qc.invalidateQueries({ queryKey: ['servicios'] });
  const renombrar = useMutation({
    mutationFn: async (v: { id: string; nombre: string; version: number }) =>
      unwrap(await api.PATCH('/services/{id}', { params: { path: { id: v.id } }, body: { nombre: v.nombre, version: v.version } })),
    onSettled: done,
  });
  const borrar = useMutation({
    mutationFn: async (id: string) => {
      const r = await api.DELETE('/services/{id}', { params: { path: { id } } });
      if (!r.response.ok) throw new ApiError(r.response.status, r.error as never);
    },
    onSettled: done,
  });
  const crear = useMutation({
    mutationFn: async (nombre: string) => unwrap(await api.POST('/services', { body: { nombre, tipoVehiculo: t.atiende || 'ambos' } })),
    onSuccess: done,
  });
  const [nuevo, setNuevo] = useState('');
  const agregar = (e: FormEvent) => {
    e.preventDefault();
    crear.mutate(nuevo.trim(), { onSuccess: () => setNuevo('') });
  };
  const error = renombrar.error ?? borrar.error ?? crear.error;

  return (
    <section className={card} aria-labelledby="precarga">
      <h2 id="precarga" className="text-[17px] font-extrabold">
        Tus servicios ({list.data?.length ?? 0})
      </h2>
      <p className="text-sm text-ink-2">
        Van sin precio; lo pones cuando quieras en Servicios y repuestos. También quedaron listas las 7 etapas de la orden y el checklist
        de recepción de {t.atiende === 'moto' ? 'moto' : t.atiende === 'carro' ? 'carro' : 'carro y de moto'}.
      </p>
      {list.isPending ? <p role="status">Cargando…</p> : null}
      <ul className="flex flex-col gap-2">
        {(list.data ?? []).map((s) => (
          <li key={`${s.id}-${s.version}`} className="flex flex-wrap items-end gap-3">
            <div className="min-w-[200px] flex-1">
              <Field
                label={`Nombre de ${s.nombre}`}
                defaultValue={s.nombre}
                onBlur={(e) => {
                  const v = e.target.value.trim();
                  if (v !== '' && v !== s.nombre) renombrar.mutate({ id: s.id, nombre: v, version: s.version });
                }}
              />
            </div>
            <span className="min-h-[52px] content-center text-sm text-ink-2">{tipos[s.tipoVehiculo as keyof typeof tipos] ?? s.tipoVehiculo}</span>
            <button type="button" className="min-h-[52px] px-2 font-bold text-err-ink" onClick={() => borrar.mutate(s.id)}>
              Quitar {s.nombre}
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={agregar} className="flex flex-wrap items-end gap-3">
        <div className="min-w-[200px] flex-1">
          <Field label="Agregar un servicio" value={nuevo} onChange={(e) => setNuevo(e.target.value)} placeholder="Sincronización" />
        </div>
        <Button type="submit" variant="secondary" busy={crear.isPending} missing={nuevo.trim() ? undefined : 'Escribe el nombre del servicio.'}>
          Agregar
        </Button>
      </form>
      {error ? <Alert>{error.message}</Alert> : null}
    </section>
  );
}

/** HU-03 → HU-04 → HU-05: el arranque del taller después de crear la cuenta (pasos 2 y 3 del registro). */
export function OnboardingPage() {
  const navigate = useNavigate();
  const q = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  const [editando, setEditando] = useState(false);
  if (q.isPending) return <p role="status">Cargando…</p>;
  if (q.isError) return <Alert>{q.error.message}</Alert>;
  const t = q.data;

  if (!t.onboardingCompleto || editando) {
    return (
      <>
        <Paso n={2} de={3} titulo="Cuéntanos de tu taller" detalle="Con esto te armamos los servicios, el checklist y las etapas de la orden." />
        <ProfileForm t={t} onSaved={() => setEditando(false)} />
      </>
    );
  }
  return (
    <>
      <Paso n={3} de={3} titulo="Tu taller ya está armado" detalle="Revisa lo que te dejamos listo y abre tu primera orden." />
      <PreloadedServices t={t} />
      <Schedule t={t} />
      <div className="flex flex-wrap gap-3">
        <Button onClick={() => navigate('/panel/ordenes/nueva?primera=1')}>Hacer mi primera orden</Button>
        <Button variant="secondary" onClick={() => setEditando(true)}>
          Cambiar tipo de taller
        </Button>
      </div>
    </>
  );
}
