import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router';

import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { useChangePIN, useSession } from './api';
import { errorDePin } from './pin';
import { PinInput } from './PinInput';

/**
 * Escoger un PIN propio (HU-104). Si se entró con el PIN temporal que puso el dueño, no se pide el actual y no se
 * puede usar el panel sin cambiarlo; si no, pide el actual.
 */
export function ChangePinPage() {
  const navigate = useNavigate();
  const temporal = useSession().data?.cambiarPin === true;
  const cambiar = useChangePIN();
  const [actual, setActual] = useState('');
  const [pin, setPin] = useState('');
  const [pin2, setPin2] = useState('');
  const [tocado, setTocado] = useState(false);
  const errores = {
    actual: temporal || actual.length === 4 ? undefined : 'Escribe tu PIN actual.',
    pin: errorDePin(pin),
    pin2: pin2 === pin ? undefined : 'Los dos PIN no coinciden.',
  };
  const ver = (e: string | undefined) => (tocado ? e : undefined);

  function guardar(e: FormEvent) {
    e.preventDefault();
    setTocado(true);
    if (Object.values(errores).some(Boolean)) return;
    cambiar.mutate(temporal ? { pin } : { pinActual: actual, pin }, { onSuccess: () => navigate('/panel', { replace: true }) });
  }

  return (
    <section className="flex max-w-[520px] flex-col gap-5 rounded-card bg-white p-5">
      <header>
        <h1 className="text-[26px] font-extrabold tracking-[-.02em]">{temporal ? 'Escoge tu PIN' : 'Cambiar mi PIN'}</h1>
        <p className="text-ink-2">
          {temporal ? 'Entraste con el PIN que te dio el dueño. Escoge uno tuyo para seguir.' : '4 números que recuerdes. Nadie más debe saberlo.'}
        </p>
      </header>
      <form onSubmit={guardar} className="flex flex-col gap-5" noValidate>
        {temporal ? null : <PinInput label="PIN actual" value={actual} onChange={setActual} error={ver(errores.actual)} />}
        <PinInput label="PIN nuevo" value={pin} onChange={setPin} autoComplete="new-password" error={ver(errores.pin)} />
        <PinInput label="Repite el PIN nuevo" value={pin2} onChange={setPin2} autoComplete="new-password" error={ver(errores.pin2)} />
        {cambiar.error ? <Alert>{cambiar.error.message}</Alert> : null}
        <Button type="submit" busy={cambiar.isPending}>
          Guardar mi PIN
        </Button>
      </form>
    </section>
  );
}
