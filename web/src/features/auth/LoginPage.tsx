import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';

import { celularValido } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { useLoginPIN } from './api';
import { AuthLayout } from './AuthLayout';
import { PinInput } from './PinInput';

/**
 * Entrar con celular y PIN (HU-104, V-03; Figma "01 Entrar con PIN"). El error es el mismo exista o no el celular;
 * al 5.º fallo el celular queda bloqueado 15 minutos. El código por WhatsApp (CodeForm) queda para AUTH_MODE=otp.
 */
export function LoginPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [celular, setCelular] = useState('');
  const [pin, setPin] = useState('');
  const [tocado, setTocado] = useState(false);
  const login = useLoginPIN();
  const volver = params.get('volver');
  const destino = volver?.startsWith('/panel') ? volver : '/panel';
  const celularOk = celularValido(celular);

  function entrar(e: FormEvent) {
    e.preventDefault();
    setTocado(true);
    if (!celularOk || pin.length !== 4) return;
    login.mutate(
      { celular, pin },
      {
        onSuccess: (s) => navigate(s.cambiarPin ? '/panel/cambiar-pin' : destino, { replace: true }),
        onError: () => setPin(''),
      },
    );
  }

  return (
    <AuthLayout steps={[{ title: 'Entrar', detail: 'Con tu celular y tu PIN' }]} current={0} timer={false}>
      <div>
        <h1 className="mb-1.5 text-[32px] font-extrabold tracking-[-.02em]">Entra a tu taller</h1>
        <p className="text-base text-ink-2">Con tu celular y tu PIN de 4 números. Sin códigos ni correos.</p>
      </div>
      <form onSubmit={entrar} className="flex max-w-[520px] flex-col gap-5" noValidate>
        <Field
          label="Tu celular"
          inputMode="tel"
          autoComplete="username"
          placeholder="300 000 0000"
          value={celular}
          onChange={(e) => setCelular(e.target.value)}
          error={tocado && !celularOk ? 'Escribe un celular colombiano de 10 dígitos que empiece por 3.' : undefined}
        />
        <PinInput label="Tu PIN" value={pin} onChange={setPin} error={tocado && pin.length !== 4 && !login.error ? 'Escribe los 4 números de tu PIN.' : undefined} />
        {login.error ? <Alert>{login.error.message}</Alert> : null}
        <Button type="submit" busy={login.isPending}>
          Entrar
        </Button>
        <p className="text-sm text-ink-2">¿Olvidaste tu PIN? Pídele al dueño del taller que te lo restablezca desde Equipo.</p>
        <p className="text-sm text-ink-2">
          ¿Tu taller es nuevo?{' '}
          <Link to="/registro" className="font-bold text-primary">
            Créalo en 1 minuto
          </Link>
        </p>
      </form>
    </AuthLayout>
  );
}
