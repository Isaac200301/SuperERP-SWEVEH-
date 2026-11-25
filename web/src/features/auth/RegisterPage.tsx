import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router';

import { ApiError } from '../../shared/api/client';
import { celularValido } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { useRegisterPIN } from './api';
import { AuthLayout } from './AuthLayout';
import { errorDePin } from './pin';
import { PinInput } from './PinInput';

/** Versión de la política que se muestra y se guarda con la autorización (HU-02, HU-90; borrador hasta AQ-10). */
export const POLITICA_VERSION = 'borrador-sin-validar-2026-11-01';

const steps = [
  { title: 'Tu taller', detail: 'Celular, nombre y PIN' },
  { title: 'Qué atiendes', detail: 'Carros, motos o los dos' },
];

/** Paso 1 del registro (HU-104, V-04; Figma "02 Registro · paso 1"): celular, nombre del taller, PIN y autorización. */
export function RegisterPage() {
  const navigate = useNavigate();
  const registro = useRegisterPIN();
  const [celular, setCelular] = useState('');
  const [taller, setTaller] = useState('');
  const [pin, setPin] = useState('');
  const [pin2, setPin2] = useState('');
  const [acepta, setAcepta] = useState(false);
  const [tocado, setTocado] = useState(false);

  const errores = {
    celular: celularValido(celular) ? undefined : 'Escribe un celular colombiano de 10 dígitos que empiece por 3.',
    taller: taller.trim() ? undefined : 'Escribe el nombre del taller.',
    pin: errorDePin(pin),
    pin2: pin2 === pin ? undefined : 'Los dos PIN no coinciden.',
    // HU-02: la casilla aparece sin marcar y sin marcarla no se puede seguir.
    acepta: acepta ? undefined : 'Para continuar debes aceptar la política de tratamiento de datos.',
  };
  const ver = (e: string | undefined) => (tocado ? e : undefined);
  const yaExiste = registro.error instanceof ApiError && registro.error.code === 'cuenta_existente';

  function crear(e: FormEvent) {
    e.preventDefault();
    setTocado(true);
    if (Object.values(errores).some(Boolean)) return;
    registro.mutate(
      { celular, nombreTaller: taller.trim(), pin, autorizacion: { version: POLITICA_VERSION, aceptada: true } },
      { onSuccess: () => navigate('/panel/arranque', { replace: true }) },
    );
  }

  return (
    <AuthLayout steps={steps} current={0}>
      <div>
        <span className="font-mono text-[13px] font-bold text-primary">PASO 1 DE 2</span>
        <h1 className="mt-1.5 mb-1.5 text-[32px] font-extrabold tracking-[-.02em]">Crea tu taller</h1>
      </div>
      <form onSubmit={crear} className="flex max-w-[520px] flex-col gap-5" noValidate>
        <Field
          label="Tu celular"
          inputMode="tel"
          autoComplete="username"
          placeholder="300 000 0000"
          hint="Con este número entras. No te llega ningún código."
          value={celular}
          onChange={(e) => setCelular(e.target.value)}
          error={ver(errores.celular)}
        />
        <Field label="Nombre del taller" value={taller} maxLength={120} onChange={(e) => setTaller(e.target.value)} error={ver(errores.taller)} />
        <PinInput
          label="Crea tu PIN"
          value={pin}
          onChange={setPin}
          autoComplete="new-password"
          hint="4 números que recuerdes. Puedes cambiarlo después."
          error={ver(errores.pin)}
        />
        <PinInput label="Repite el PIN" value={pin2} onChange={setPin2} autoComplete="new-password" error={ver(errores.pin2)} />
        <div className="flex flex-col gap-1.5">
          <label className="flex min-h-[44px] items-start gap-3 text-[15px]">
            <input type="checkbox" className="mt-1 h-5 w-5 accent-primary" checked={acepta} onChange={(e) => setAcepta(e.target.checked)} />
            <span>
              Acepto la{' '}
              <Link to="/politica-de-datos" target="_blank" rel="noopener noreferrer" className="font-bold text-primary underline">
                política de tratamiento de datos
              </Link>{' '}
              de SWEVEH (versión borrador).
            </span>
          </label>
          {ver(errores.acepta) ? (
            <span role="alert" className="text-sm font-semibold text-err-ink">
              {errores.acepta}
            </span>
          ) : null}
        </div>
        {registro.error ? (
          <Alert>
            {registro.error.message}{' '}
            {yaExiste ? (
              <Link to="/entrar" className="font-bold underline">
                Entrar
              </Link>
            ) : null}
          </Alert>
        ) : null}
        <Button type="submit" busy={registro.isPending}>
          Seguir
        </Button>
        <p className="text-sm text-ink-2">
          ¿Ya tienes cuenta?{' '}
          <Link to="/entrar" className="font-bold text-primary">
            Entra con tu celular y tu PIN
          </Link>
        </p>
      </form>
    </AuthLayout>
  );
}
