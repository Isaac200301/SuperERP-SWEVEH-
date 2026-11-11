import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { api, ApiError, unwrap } from '../../shared/api/client';
import { estados } from '../../shared/i18n/es-CO';
import { normalizarPlaca, reconocerPlaca } from '../../shared/lib/plate';
import { Alert } from '../../shared/ui/Alert';
import { BottomSheet } from '../../shared/ui/BottomSheet';
import { Button } from '../../shared/ui/Button';
import { IconCheck } from '../../shared/ui/icons';
import { useOpenOrder, useVehicleByPlate } from './api';

/**
 * HU-16 desde el celular: se escribe la placa y, si el vehículo ya vino, se abre la orden ahí mismo.
 * Si es nuevo, la pantalla completa pide cliente y ficha (HU-09, HU-10, HU-11).
 */
export function NewOrderSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  const [placa, setPlaca] = useState('');
  const [sintoma, setSintoma] = useState('');
  const p = normalizarPlaca(placa);
  const formato = reconocerPlaca(placa);
  const lista = formato !== 'vacia' && formato !== 'invalida';
  const vq = useVehicleByPlate(placa, open && lista);
  const vehiculo = vq.data?.vehiculo ?? null;
  // Quién es el dueño y si ya tiene una orden abierta, con la búsqueda de HU-14.
  const found = useQuery({
    queryKey: ['buscar', p],
    enabled: open && vehiculo !== null,
    queryFn: async () => unwrap(await api.GET('/search', { params: { query: { q: p } } })).vehiculos.find((v) => v.placa === p) ?? null,
  });
  const visitas = useQuery({
    queryKey: ['vehiculos', vehiculo?.id, 'historial'],
    enabled: open && vehiculo !== null,
    queryFn: async () => unwrap(await api.GET('/vehicles/{id}/history', { params: { path: { id: vehiculo?.id ?? '' } } })).ordenes,
  });
  const openOrder = useOpenOrder();

  function close() {
    setPlaca('');
    setSintoma('');
    openOrder.reset();
    onClose();
  }

  function crear(recibir: boolean) {
    if (!vehiculo) return;
    openOrder.mutate(
      {
        placa: p,
        vehiculoExistente: vehiculo,
        etag: vq.data?.etag ?? '',
        cliente: { celular: '', nombre: '' },
        vehiculo: { confirmarPlacaEspecial: false },
        sintoma: sintoma.trim(),
        estadoInicial: 'por_recibir',
        confirmarDuplicada: false,
      },
      {
        onSuccess: (o) => {
          close();
          navigate(`/panel/ordenes/${o.id}${recibir ? '?recibir=1' : ''}`);
        },
      },
    );
  }

  const ultima = visitas.data?.[0]?.orden;
  const abierta = found.data?.ordenAbiertaId ? found.data : null;
  const err = openOrder.error instanceof ApiError ? openOrder.error.message : openOrder.error?.message;
  return (
    <BottomSheet open={open} onClose={close} title="Nueva orden">
      <p className="-mt-3 text-sm text-ink-2">Escribe la placa. Si ya vino antes, traemos su historial.</p>
      <label className="flex flex-col gap-1.5 text-[13px] font-bold text-ink-2">
        Placa
        <input
          value={placa}
          onChange={(e) => setPlaca(e.target.value)}
          autoCapitalize="characters"
          autoComplete="off"
          maxLength={8}
          className="min-h-[60px] rounded-xl border-[2.5px] border-ink bg-plate text-center font-mono text-[28px] font-bold tracking-[.12em] text-ink uppercase outline-none"
        />
      </label>
      {lista && vq.isPending ? <p role="status" className="text-sm text-ink-2">Buscando la placa en tu taller…</p> : null}
      {vehiculo ? (
        <>
          <div className="flex items-center gap-3 rounded-[18px] bg-primary-tint px-3.5 py-3">
            <IconCheck size={22} className="flex-none text-primary" />
            <span className="text-sm">
              <b>
                {[found.data?.descripcion || [vehiculo.marca, vehiculo.linea, vehiculo.modeloAnio].filter(Boolean).join(' ') || p, found.data?.cliente]
                  .filter(Boolean)
                  .join(' · ')}
              </b>
              <br />
              <span className="text-ink-2">
                {visitas.data && visitas.data.length > 0
                  ? `Ya vino ${visitas.data.length} ${visitas.data.length === 1 ? 'vez' : 'veces'}${ultima ? ` · última: ${ultima.codigo}` : ''}`
                  : 'Primera visita'}
              </span>
            </span>
          </div>
          {abierta ? (
            <Alert title={`Ya tiene abierta la ${abierta.ordenAbierta ?? 'orden'}`}>
              Está en {abierta.estadoOrden ? estados[abierta.estadoOrden] : 'curso'}.{' '}
              <Link to={`/panel/ordenes/${abierta.ordenAbiertaId}`} onClick={close} className="font-bold underline">
                Ver la orden
              </Link>
            </Alert>
          ) : (
            <>
              <label className="flex flex-col gap-1.5 text-[15px] font-extrabold">
                ¿Qué dice el cliente?
                <textarea
                  rows={2}
                  value={sintoma}
                  onChange={(e) => setSintoma(e.target.value)}
                  placeholder="Ej.: chilla al frenar"
                  className="rounded-btn border-[1.5px] border-line px-4 py-3 text-base font-normal"
                />
              </label>
              {err ? <Alert>{err}</Alert> : null}
              <Button
                busy={openOrder.isPending}
                missing={sintoma.trim() ? undefined : 'Falta lo que dice el cliente.'}
                onClick={() => crear(true)}
              >
                Crear y recibir ahora
              </Button>
              <button
                type="button"
                disabled={!sintoma.trim() || openOrder.isPending}
                onClick={() => crear(false)}
                className="min-h-[44px] font-bold text-primary disabled:opacity-50"
              >
                Dejarla Por recibir
              </button>
            </>
          )}
        </>
      ) : null}
      {lista && vq.isSuccess && vehiculo === null ? (
        <>
          <p className="text-sm text-ink-2">
            <b className="text-ink">{p}</b> no ha venido a tu taller. Registra al cliente y la ficha del vehículo.
          </p>
          <Button
            onClick={() => {
              close();
              navigate(`/panel/ordenes/nueva?placa=${encodeURIComponent(p)}`);
            }}
          >
            Registrar vehículo nuevo
          </Button>
        </>
      ) : null}
      {formato === 'invalida' && p.length >= 5 ? <p className="text-sm text-err-ink">Usa solo letras y números, por ejemplo ABC123.</p> : null}
    </BottomSheet>
  );
}
