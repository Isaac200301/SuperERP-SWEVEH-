import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap, type EstadoOrden, type Schemas } from '../../shared/api/client';
import { ordersKey } from './api';

export type Checklist = Schemas['Checklist'];
export type Evidencia = Schemas['Evidencia'];
export type Cotizacion = Schemas['Cotizacion'];

const keys = {
  checklist: (id: string) => [...ordersKey, id, 'checklist'] as const,
  evidence: (id: string) => [...ordersKey, id, 'evidencias'] as const,
  quote: (id: string) => [...ordersKey, id, 'cotizacion'] as const,
};

/** Tras cualquier cambio la orden (estado, versión, historial) se vuelve a leer. */
function useInvalidateOrder(id: string) {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: [...ordersKey, id] });
}

export type TransitionInput = {
  estado: EstadoOrden;
  version: number;
  motivo?: string;
  recogidoPor?: string;
  accesoriosConfirmados?: boolean;
};

/** HU-20: un paso adelante o atrás; el servidor valida las puertas y la versión (409 si alguien se adelantó). */
export function useTransition(id: string) {
  const invalidate = useInvalidateOrder(id);
  return useMutation({
    mutationFn: async (body: TransitionInput) =>
      unwrap(await api.POST('/orders/{id}/transition', { params: { path: { id } }, body })),
    onSettled: invalidate,
  });
}

/** HU-23: primero las asignadas a mí por fecha prometida; si el taller no asigna, todas las abiertas (Q-14). */
export function useMyOrders() {
  return useQuery({
    queryKey: [...ordersKey, 'mias'],
    queryFn: async () => unwrap(await api.GET('/orders/mine')).ordenes,
    refetchInterval: 30_000,
  });
}

export function useChecklist(id: string) {
  return useQuery({
    queryKey: keys.checklist(id),
    queryFn: async () => unwrap(await api.GET('/orders/{id}/checklist', { params: { path: { id } } })),
  });
}

export type ChecklistInput = { kilometraje?: number | null; combustible?: string | null; accesorios?: string[] };

export function useSaveChecklist(id: string) {
  const invalidate = useInvalidateOrder(id);
  return useMutation({
    mutationFn: async (body: ChecklistInput) =>
      unwrap(await api.PUT('/orders/{id}/checklist', { params: { path: { id } }, body })),
    onSettled: invalidate,
  });
}

export function useEvidence(id: string) {
  return useQuery({
    queryKey: keys.evidence(id),
    queryFn: async () => unwrap(await api.GET('/orders/{id}/evidence', { params: { path: { id } } })).evidencias,
  });
}

export type EvidenceInput = { lado?: string; nota?: string; mostrarCliente?: boolean; foto?: File };

/** HU-21: la foto va al final del formulario para que el servidor la procese en streaming. */
export function useAddEvidence(id: string) {
  const invalidate = useInvalidateOrder(id);
  return useMutation({
    mutationFn: async (v: EvidenceInput) =>
      unwrap(
        await api.POST('/orders/{id}/evidence', {
          params: { path: { id } },
          body: {},
          bodySerializer: () => {
            const fd = new FormData();
            if (v.lado) fd.append('lado', v.lado);
            if (v.nota) fd.append('nota', v.nota);
            fd.append('mostrarCliente', String(v.mostrarCliente ?? false));
            if (v.foto) fd.append('foto', v.foto);
            return fd;
          },
        }),
      ),
    onSettled: invalidate,
  });
}

export function photoUrl(orderId: string, evidenceId: string): string {
  return `/api/v1/orders/${orderId}/evidence/${evidenceId}/file`;
}

export function useQuote(id: string, enabled = true) {
  return useQuery({
    queryKey: keys.quote(id),
    enabled,
    queryFn: async (): Promise<Cotizacion | null> =>
      unwrap(await api.GET('/orders/{id}/quote', { params: { path: { id } } })).cotizacion,
  });
}

export type LineInput = { tipo: 'servicio' | 'repuesto'; refId?: string; descripcion?: string; cantidad: number; precioUnitario?: number };

export function useQuoteActions(id: string) {
  const invalidate = useInvalidateOrder(id);
  const add = useMutation({
    mutationFn: async (body: LineInput) =>
      unwrap(await api.POST('/orders/{id}/quote/lines', { params: { path: { id } }, body })),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: async (lineId: string) =>
      unwrap(await api.DELETE('/orders/{id}/quote/lines/{lineId}', { params: { path: { id, lineId } } })),
    onSettled: invalidate,
  });
  const send = useMutation({
    mutationFn: async () => unwrap(await api.POST('/orders/{id}/quote/send', { params: { path: { id } } })),
    onSettled: invalidate,
  });
  const respond = useMutation({
    mutationFn: async (body: { decision: 'aprobar' | 'rechazar' | 'dudas'; canal: 'telefono' | 'mostrador' }) =>
      unwrap(await api.POST('/orders/{id}/quote/response', { params: { path: { id } }, body })),
    onSettled: invalidate,
  });
  return { add, remove, send, respond };
}

/** Servicios y repuestos del taller para cotizar (HU-07, HU-34). */
export function useCatalog(q: string) {
  return useQuery({
    queryKey: ['catalogo', q],
    queryFn: async () => {
      const [s, p] = await Promise.all([
        api.GET('/services'),
        api.GET('/parts', { params: { query: { q } } }),
      ]);
      const term = q.trim().toLowerCase();
      return {
        servicios: unwrap(s).servicios.filter((x) => x.activo && x.nombre.toLowerCase().includes(term)),
        repuestos: unwrap(p).repuestos.filter((x) => x.activo),
      };
    },
    staleTime: 30_000,
  });
}
