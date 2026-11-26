import { useState } from 'react';

import { Alert } from '../../shared/ui/Alert';
import { useOrderFloorChat, type ChatPiso } from './api';
import { FloorChat } from './FloorChat';
import { VideoReception } from './VideoReception';

/**
 * "Asistente de esta orden" (V-11, HU-107; Figma 07 y D2): la conversación de la orden con la ficha en vivo. En
 * escritorio va fija a la derecha; en el celular es una pestaña de la orden. Si la orden está por recibir, arriba va
 * "Recibir con un video" (R9, HU-108; Figma 08).
 */
export function OrderAssistantCard({ ordenId, estado, titulo = true }: Readonly<{ ordenId: string; estado?: string; titulo?: boolean }>) {
  const q = useOrderFloorChat(ordenId);
  const [actual, setActual] = useState<ChatPiso | null>(null);
  const chat = actual ?? q.data?.chat ?? null;
  return (
    <section aria-labelledby={titulo ? 'asistente-orden' : undefined} aria-label={titulo ? undefined : 'Asistente de esta orden'} className="flex flex-col gap-3 rounded-card bg-surface-2/40 p-1">
      {titulo ? (
        <h2 id="asistente-orden" className="px-1 text-[17px] font-extrabold">
          Asistente de esta orden
        </h2>
      ) : null}
      {q.isPending ? <p role="status">Cargando el asistente…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.isSuccess && estado === 'por_recibir' ? <VideoReception ordenId={ordenId} chat={chat} onChat={setActual} /> : null}
      {q.isSuccess ? <FloorChat chat={chat} ordenId={ordenId} disponible={chat ? chat.disponible : q.data.disponible} voz={q.data.voz} onChat={setActual} /> : null}
    </section>
  );
}
