import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { Alert } from '../../shared/ui/Alert';
import { useFloorChat, useLatestFloorChat, type ChatPiso } from './api';
import { FloorChat } from './FloorChat';

/**
 * Pestaña "Asistente" (V-11, HU-107; Figma 06): el chat general del piso. Retoma la última conversación de la persona
 * (12 h); cuando hay placa, la conversación sigue dentro de su orden y aquí queda el enlace. Con ?chat= abre esa
 * conversación: es la que se creó con "Abrir orden nueva con estos datos" desde el chat de otra orden.
 */
export function AssistantPage() {
  const [params] = useSearchParams();
  const pedida = params.get('chat') ?? undefined;
  const q = useLatestFloorChat();
  const qc = useFloorChat(pedida);
  // undefined = la que diga el servidor; null = una conversación nueva.
  const [actual, setActual] = useState<ChatPiso | null | undefined>(undefined);
  const delServidor = pedida ? (qc.data ?? null) : (q.data?.chat ?? null);
  const chat = actual === undefined ? delServidor : actual;
  const cargando = q.isPending || (!!pedida && qc.isPending);
  return (
    <>
      <header className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-[26px] font-extrabold tracking-[-.02em]">Asistente</h1>
          <p className="text-[13px] text-ink-2">Recibe, diagnostica y cotiza hablándole. Nada se guarda hasta que confirmas.</p>
        </div>
        {chat ? (
          <button type="button" onClick={() => setActual(null)} className="min-h-[44px] shrink-0 rounded-btn bg-white px-3 text-sm font-bold text-primary">
            Nueva conversación
          </button>
        ) : null}
      </header>
      {cargando ? <p role="status">Cargando la conversación…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {qc.isError ? <Alert>{qc.error.message}</Alert> : null}
      {q.isSuccess && !cargando ? (
        <FloorChat key={chat?.id ?? 'nueva'} chat={chat} disponible={chat ? chat.disponible : q.data.disponible} voz={q.data.voz} onChat={setActual} conEnlace />
      ) : null}
    </>
  );
}
