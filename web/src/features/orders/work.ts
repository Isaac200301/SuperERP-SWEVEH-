import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap, type EstadoOrden, type Schemas } from '../../shared/api/client';
import { ordersKey } from './api';

export type Checklist = Schemas['Checklist'];
export type Evidencia = Schemas['Evidencia'];
export type Cotizacion = Schemas['Cotizacion'];
export type VistaCliente = Schemas['VistaCliente'];

const keys = {
  checklist: (id: string) => [...ordersKey, id, 'checklist'] as const,
  evidence: (id: string) => [...ordersKey, id, 'evidencias'] as const,
  quote: (id: string) => [...ordersKey, id, 'cotizacion'] as const,
  clientView: (id: string) => [...ordersKey, id, 'vista-cliente'] as const,
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

export type ItemRevisado = Schemas['ItemRevisado'];
export type ChecklistInput = { kilometraje?: number | null; combustible?: string | null; accesorios?: string[]; items?: ItemRevisado[] };

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

// Una nota puede llevar varias fotos (rayones, piezas encontradas); hallazgoId las pega a un hallazgo.
export type EvidenceInput = { lado?: string; nota?: string; mostrarCliente?: boolean; foto?: File; fotos?: File[]; hallazgoId?: string };
export const maxFotosPorNota = 8;

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
            if (v.hallazgoId) fd.append('hallazgoId', v.hallazgoId);
            for (const f of [...(v.foto ? [v.foto] : []), ...(v.fotos ?? [])]) fd.append('foto', f);
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

/** V-10 (ADR-0043): lo que ve el cliente de la cotización vigente, más si ya autorizó sus datos (HU-86). */
export function useClientView(id: string) {
  return useQuery({
    queryKey: keys.clientView(id),
    queryFn: async (): Promise<VistaCliente> => unwrap(await api.GET('/orders/{id}/quote/client-view', { params: { path: { id } } })),
  });
}

/** HU-86: la autorización de datos que el cliente da (o no) en el mostrador, con la prueba que guarda el servidor. */
export function useCounterConsent(orderId: string) {
  const invalidate = useInvalidateOrder(orderId);
  return useMutation({
    mutationFn: async (v: { clienteId: string; acepta: boolean }) =>
      unwrap(await api.POST('/customers/{id}/consent', { params: { path: { id: v.clienteId } }, body: { acepta: v.acepta } })),
    onSettled: invalidate,
  });
}

/** El PDF de la cotización vigente, tal como se le comparte al cliente (V-10). */
export async function fetchQuotePdf(id: string): Promise<{ blob: Blob; nombre: string }> {
  const r = await api.GET('/orders/{id}/quote.pdf', { params: { path: { id } }, parseAs: 'blob' });
  const blob = unwrap(r) as Blob;
  const nombre = /filename="([^"]+)"/.exec(r.response.headers.get('Content-Disposition') ?? '')?.[1] ?? 'cotizacion.pdf';
  return { blob, nombre };
}

export type LineInput = { tipo: 'servicio' | 'repuesto' | 'partida'; refId?: string; descripcion?: string; cantidad: number; precioUnitario?: number; horas?: number };

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
    mutationFn: async (body: { decision: 'aprobar' | 'rechazar' | 'dudas'; canal: 'telefono' | 'mostrador'; motivo?: string }) =>
      unwrap(await api.POST('/orders/{id}/quote/response', { params: { path: { id } }, body })),
    onSettled: invalidate,
  });
  // Cantidad y, solo en precio libre, el precio de un ítem del borrador.
  const update = useMutation({
    mutationFn: async ({ lineId, ...body }: { lineId: string; cantidad?: number; precioUnitario?: number; horas?: number; descuentoPct?: number; descripcion?: string; version?: number }) =>
      unwrap(await api.PATCH('/orders/{id}/quote/lines/{lineId}', { params: { path: { id, lineId } }, body })),
    onSettled: invalidate,
  });
  // HU-38: devolver una unidad de un repuesto ya descontado; responde cuánto queda por devolver.
  const giveBack = useMutation({
    mutationFn: async (v: { partId: string; nombre: string }) => ({
      ...unwrap(await api.POST('/parts/{id}/returns', { params: { path: { id: v.partId } }, body: { cantidad: 1, ordenId: id } })),
      nombre: v.nombre,
    }),
    onSettled: invalidate,
  });
  const renew = useMutation({
    mutationFn: async () => unwrap(await api.POST('/orders/{id}/quote/renew', { params: { path: { id } } })),
    onSettled: invalidate,
  });
  // V-07 (ADR-0040): detalle interno de una partida (no suma al total) y "Mostrar detalle al cliente".
  const detail = useMutation({
    mutationFn: async (v: { lineId: string; version: number; detalle: { descripcion: string; cantidad?: number }[] }) =>
      unwrap(await api.PUT('/orders/{id}/quote/lines/{lineId}/detail', { params: { path: { id, lineId: v.lineId } }, body: { detalle: v.detalle, version: v.version } })),
    onSettled: invalidate,
  });
  const options = useMutation({
    mutationFn: async (v: { mostrarDetalle: boolean; version: number }) => unwrap(await api.PATCH('/orders/{id}/quote', { params: { path: { id } }, body: v })),
    onSettled: invalidate,
  });
  return { add, remove, send, respond, giveBack, update, renew, detail, options };
}

/** Por repuesto, lo que la orden descontó, devolvió y dejó pendiente (HU-33, HU-38). */
export function useOrderParts(id: string, enabled: boolean) {
  return useQuery({
    queryKey: [...ordersKey, id, 'repuestos-movidos'],
    enabled,
    queryFn: async () => unwrap(await api.GET('/orders/{id}/part-movements', { params: { path: { id } } })).repuestos,
  });
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
