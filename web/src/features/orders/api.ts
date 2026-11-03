import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, ApiError, unwrap, unwrapWithETag, type Cliente, type Vehiculo } from '../../shared/api/client';
import { normalizarPlaca } from '../../shared/lib/plate';
import type { FichaPayload } from './sheet';

export const ordersKey = ['ordenes'] as const;

export function useRecentOrders() {
  return useQuery({
    queryKey: [...ordersKey, 'recientes'],
    queryFn: async () => unwrap(await api.GET('/orders', { params: { query: { limite: 50 } } })),
    staleTime: 15_000,
  });
}

export function useOrder(id: string) {
  return useQuery({
    queryKey: [...ordersKey, id],
    queryFn: async () => unwrap(await api.GET('/orders/{id}', { params: { path: { id } } })),
  });
}

/** Vehículo por placa; null si no existe en el taller (HU-10: "abre ese vehículo"). */
export function useVehicleByPlate(placa: string, enabled: boolean) {
  const p = normalizarPlaca(placa);
  return useQuery({
    queryKey: ['vehiculos', 'placa', p],
    enabled,
    queryFn: async (): Promise<{ vehiculo: Vehiculo; etag: string } | null> => {
      try {
        const r = unwrapWithETag(await api.GET('/vehicles/by-plate/{placa}', { params: { path: { placa: p } } }));
        return { vehiculo: r.data, etag: r.etag };
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
    staleTime: 10_000,
  });
}

/** Cliente por celular; null si no existe (HU-09: "me muestra ese cliente en vez de crear otro"). */
export function useCustomerByPhone(celular: string, enabled: boolean) {
  return useQuery({
    queryKey: ['clientes', 'celular', celular],
    enabled,
    queryFn: async (): Promise<Cliente | null> => {
      try {
        return unwrap(await api.GET('/customers/by-phone/{celular}', { params: { path: { celular } } }));
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
    staleTime: 10_000,
  });
}

export type NewOrderInput = {
  placa: string;
  vehiculoExistente: Vehiculo | null;
  etag: string;
  cliente: { celular: string; nombre: string; correo?: string };
  vehiculo: {
    tipo?: 'carro' | 'moto';
    confirmarPlacaEspecial: boolean;
    marca?: string;
    linea?: string;
    modeloAnio?: number;
    kilometraje?: number;
  } & FichaPayload;
  motorParaExistente?: FichaPayload;
  sintoma: string;
  estadoInicial: 'por_recibir' | 'recibido';
  confirmarDuplicada: boolean;
  ejemplo?: boolean;
};

/**
 * Abre la orden en el orden que exige el dominio: cliente (HU-09) → vehículo (HU-10/11) → orden (HU-16).
 * Cada paso es idempotente en el servidor (buscar o crear), así que reintentar no duplica nada.
 */
export function useOpenOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: NewOrderInput) => {
      let vehiculo = v.vehiculoExistente;
      if (vehiculo === null) {
        const c = unwrap(await api.POST('/customers', { body: v.cliente }));
        const created = unwrap(
          await api.POST('/vehicles', { body: { placa: v.placa, clienteId: c.cliente.id, ...v.vehiculo } }),
        );
        vehiculo = created.vehiculo;
      } else if (v.motorParaExistente && vehiculo.motorPendiente) {
        vehiculo = unwrap(
          await api.PATCH('/vehicles/{id}/engine', {
            params: { path: { id: vehiculo.id }, header: { 'If-Match': v.etag } },
            body: v.motorParaExistente,
          }),
        );
      }
      return unwrap(
        await api.POST('/orders', {
          body: {
            vehiculoId: vehiculo.id,
            sintoma: v.sintoma,
            estadoInicial: v.estadoInicial,
            confirmarDuplicada: v.confirmarDuplicada,
            ...(v.ejemplo ? { ejemplo: true } : {}),
          },
        }),
      );
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ordersKey });
      void qc.invalidateQueries({ queryKey: ['vehiculos'] });
    },
  });
}
