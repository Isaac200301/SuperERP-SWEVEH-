import { useMutation, useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Link } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { celularValido, soloCelular } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { Thread } from './Thread';

/** Busca la conversación del celular y trae sus mensajes; se refresca seguido porque el worker responde aparte. */
function useConversation(celular: string) {
  return useQuery({
    queryKey: ['simulador', celular],
    enabled: celularValido(celular),
    refetchInterval: 1500,
    queryFn: async () => {
      const list = unwrap(await api.GET('/conversations')).conversaciones;
      const c = list.find((x) => x.celular === celular);
      if (!c) return [];
      return unwrap(await api.GET('/conversations/{id}', { params: { path: { id: c.id } } })).mensajes;
    },
  });
}

/**
 * Probar WhatsApp sin cuenta de Meta (canal local, ADR-0025): escribo como si fuera el cliente y veo lo que le
 * respondería el taller, con sus botones. Los mensajes recorren el mismo camino que uno real (webhook → worker).
 * El servidor no expone el simulador en producción.
 */
export function SimulatorPage() {
  const [celular, setCelular] = useState('3001234567');
  const [nombre, setNombre] = useState('Cliente de prueba');
  const [texto, setTexto] = useState('');
  const conv = useConversation(celular);
  const enviar = useMutation({
    mutationFn: async (b: { texto: string; boton?: string }) =>
      unwrap(await api.POST('/simulator/inbound', { body: { celular, nombre, texto: b.texto, ...(b.boton ? { boton: b.boton } : {}) } })),
    onSuccess: () => setTexto(''),
  });
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (texto.trim()) enviar.mutate({ texto: texto.trim() });
  };
  const valido = celularValido(celular);
  let chat = null;
  if (valido && conv.isError) chat = <Alert>{conv.error.message}</Alert>;
  else if (valido) chat = <Thread mensajes={conv.data ?? []} onPress={(b) => enviar.mutate({ texto: b.titulo, boton: b.id })} />;

  return (
    <>
      <header className="flex flex-col gap-1">
        <Link to="/panel/taller" className="self-start font-bold text-primary">
          Mi taller
        </Link>
        <h1 className="text-[28px] font-extrabold">Probar WhatsApp</h1>
        <p className="text-ink-2">
          Escribe como si fueras el cliente. Las respuestas, avisos y botones son los que recibiría por WhatsApp. No se envía nada a nadie.
        </p>
      </header>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,320px)_minmax(0,420px)]">
        <section className="flex flex-col gap-3 rounded-card bg-white p-5">
          <Field
            label="Celular del cliente"
            inputMode="numeric"
            value={celular}
            onChange={(e) => setCelular(soloCelular(e.target.value))}
            error={celular && !valido ? 'Escribe un celular de 10 dígitos que empiece por 3.' : undefined}
          />
          <Field label="Nombre en WhatsApp" value={nombre} onChange={(e) => setNombre(e.target.value)} />
          <p className="text-sm text-ink-2">Usa el celular de un cliente para ver sus avisos, o uno nuevo para probar la autorización.</p>
        </section>
        <section aria-label="Teléfono del cliente" className="flex min-h-[520px] flex-col rounded-[32px] bg-[#ECE5DD] p-4 ring-8 ring-ink">
          <div className="flex-1 overflow-y-auto pb-3">
            {chat}
          </div>
          {enviar.isError ? <Alert>{enviar.error.message}</Alert> : null}
          <form onSubmit={onSubmit} className="flex items-end gap-2">
            <div className="flex-1">
              <Field label="Mensaje" value={texto} onChange={(e) => setTexto(e.target.value)} placeholder="¿Ya está mi carro?" />
            </div>
            <Button type="submit" busy={enviar.isPending} disabled={!valido || !texto.trim()}>
              Enviar
            </Button>
          </form>
        </section>
      </div>
    </>
  );
}
