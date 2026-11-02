import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';

import { ApiError } from '../../shared/api/client';
import { celularValido } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { useRequestCode, useVerifyCode } from './api';
import { AuthLayout } from './AuthLayout';
import { CodeForm } from './CodeForm';

/** Ingreso con celular + código para quien ya tiene cuenta. */
export function LoginPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [celular, setCelular] = useState('');
  const [enviado, setEnviado] = useState(false);
  const [tocado, setTocado] = useState(false);
  const requestCode = useRequestCode();
  const verify = useVerifyCode();
  const valido = celularValido(celular);
  const volver = params.get('volver');
  const destino = volver?.startsWith('/panel') ? volver : '/panel/ordenes';
  const sinCuenta = verify.error instanceof ApiError && verify.error.code === 'registro_incompleto';

  function pedir(e: FormEvent) {
    e.preventDefault();
    setTocado(true);
    if (valido) requestCode.mutate(celular, { onSuccess: () => setEnviado(true) });
  }

  return (
    <AuthLayout steps={[{ title: 'Entrar', detail: 'Con tu celular y un código' }]} current={0} timer={false}>
      <div>
        <h1 className="mb-1.5 text-[32px] font-extrabold tracking-[-.02em]">Entra a tu taller</h1>
        <p className="text-base text-ink-2">Te mandamos un código a tu celular.</p>
      </div>
      {!enviado ? (
        <form onSubmit={pedir} className="flex max-w-[520px] flex-col gap-5" noValidate>
          <Field
            label="Tu celular"
            inputMode="tel"
            autoComplete="tel-national"
            placeholder="300 123 4567"
            value={celular}
            onChange={(e) => setCelular(e.target.value)}
            error={tocado && !valido ? 'Escribe un celular colombiano de 10 dígitos que empiece por 3.' : undefined}
          />
          {requestCode.error ? <Alert>{requestCode.error.message}</Alert> : null}
          <Button type="submit" busy={requestCode.isPending}>
            Recibir código
          </Button>
          <p className="text-sm text-ink-2">
            ¿Es tu primera vez?{' '}
            <Link to="/registro" className="font-bold text-primary">
              Crea la cuenta de tu taller
            </Link>
          </p>
        </form>
      ) : sinCuenta ? (
        <Alert kind="warn" title="Este celular todavía no tiene cuenta">
          <Link to="/registro" className="font-bold underline">
            Crea la cuenta de tu taller
          </Link>{' '}
          con tu nombre y la autorización de datos.
        </Alert>
      ) : (
        <div className="max-w-[520px]">
          <CodeForm
            celular={celular}
            busy={verify.isPending}
            error={verify.error}
            onBack={() => {
              verify.reset();
              setEnviado(false);
            }}
            onResend={() => {
              verify.reset();
              requestCode.mutate(celular);
            }}
            onSubmit={(codigo) => verify.mutate({ celular, codigo }, { onSuccess: () => navigate(destino, { replace: true }) })}
          />
        </div>
      )}
    </AuthLayout>
  );
}
