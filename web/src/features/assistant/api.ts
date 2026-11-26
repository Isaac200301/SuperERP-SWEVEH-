import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { ordersKey } from '../orders/api';

export type ChatPiso = Schemas['ChatPiso'];
export type PropuestaPiso = Schemas['PropuestaPiso'];
export type MensajePiso = ChatPiso['mensajes'][number];

export const floorKey = ['asistente-piso'] as const;

/** La última conversación de la persona (12 h): la pestaña Asistente la retoma. */
export function useLatestFloorChat(enabled = true) {
  return useQuery({
    queryKey: [...floorKey, 'ultima'],
    enabled,
    queryFn: async () => unwrap(await api.GET('/assistant/floor/latest')),
  });
}

/** La conversación del asistente de esta orden (null si todavía no hay). */
export function useOrderFloorChat(orderId: string) {
  return useQuery({
    queryKey: [...floorKey, 'orden', orderId],
    queryFn: async () => unwrap(await api.GET('/orders/{id}/assistant', { params: { path: { id: orderId } } })),
  });
}

export type FloorMessage = { texto: string; chatId?: string | undefined; ordenId?: string | undefined; audio?: Blob | undefined; fotos?: File[] };

const LADO_MAX = 1600;

/**
 * Reduce la foto a 1600 px por el lado más largo, en JPEG, antes de subirla: la del celular pesa varios MB y el envío
 * completo tiene un tope (y el modelo de visión no necesita más). Si el navegador no puede, va la original.
 */
export async function reducir(f: File): Promise<File> {
  if (typeof createImageBitmap !== 'function' || typeof OffscreenCanvas === 'undefined') return f;
  try {
    const img = await createImageBitmap(f);
    const escala = Math.min(1, LADO_MAX / Math.max(img.width, img.height));
    if (escala === 1 && f.size < 1_500_000) return f;
    const canvas = new OffscreenCanvas(Math.round(img.width * escala), Math.round(img.height * escala));
    canvas.getContext('2d')?.drawImage(img, 0, 0, canvas.width, canvas.height);
    const blob = await canvas.convertToBlob({ type: 'image/jpeg', quality: 0.85 });
    return new File([blob], f.name.replace(/\.\w+$/, '') + '.jpg', { type: 'image/jpeg' });
  } catch {
    return f;
  }
}

/** Manda un mensaje (multipart: texto, nota de voz y fotos ya reducidas). */
export function useSendFloor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (m: FloorMessage) => {
      const fd = new FormData();
      if (m.chatId) fd.append('chatId', m.chatId);
      else if (m.ordenId) fd.append('ordenId', m.ordenId);
      if (m.texto.trim()) fd.append('texto', m.texto.trim());
      if (m.audio) fd.append('audio', m.audio, m.audio.type.includes('mp4') ? 'nota.mp4' : 'nota.webm');
      for (const f of await Promise.all((m.fotos ?? []).map(reducir))) fd.append('fotos', f, f.name);
      const r = await api.POST('/assistant/floor/messages', { body: {}, bodySerializer: () => fd });
      return unwrap(r).chat;
    },
    onSuccess: (chat) => {
      qc.setQueryData([...floorKey, 'chat', chat.id], chat);
      void qc.invalidateQueries({ queryKey: floorKey });
    },
  });
}

/** Guarda lo que la persona cambió o quitó en la ficha en vivo (versión optimista: 409 si alguien más la cambió). */
export function useEditFloor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ chat, propuesta }: { chat: ChatPiso; propuesta: PropuestaPiso }) =>
      unwrap(await api.PATCH('/assistant/floor/chats/{id}', { params: { path: { id: chat.id } }, body: { version: chat.version, propuesta } })).chat,
    onSettled: () => qc.invalidateQueries({ queryKey: floorKey }),
  });
}

/** Confirmar: aplica la propuesta. La clave la genera la pantalla: un doble toque no aplica dos veces. */
export function useConfirmFloor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ chat, key }: { chat: ChatPiso; key: string }) =>
      unwrap(
        await api.POST('/assistant/floor/chats/{id}/confirm', {
          params: { path: { id: chat.id }, header: { 'Idempotency-Key': key } },
          body: { version: chat.version },
        }),
      ),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: floorKey });
      void qc.invalidateQueries({ queryKey: ordersKey });
    },
  });
}
