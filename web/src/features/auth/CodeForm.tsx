import { useState, type FormEvent } from 'react';

import { ApiError } from '../../shared/api/client';
import { formatoCelular } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';

/** Paso del código de 6 dígitos (HU-01). Muestra el problema exacto y ofrece pedir otro. */
export function CodeForm({
  celular,
  busy,
  error,
  onSubmit,
  onResend,
  onBack,
}: {
  celular: string;
  busy: boolean;
  error: Error | null;
  onSubmit: (codigo: string) => void;
  onResend: () => void;
  onBack: () => void;
}) {
  const [codigo, setCodigo] = useState('');
  const valido = /^[0-9]{6}$/.test(codigo);
  const apiErr = error instanceof ApiError ? error : null;
  const puedeReenviar = apiErr !== null && ['codigo_vencido', 'codigo_inexistente', 'celular_bloqueado'].includes(apiErr.code);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (valido) onSubmit(codigo);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-5" noValidate>
      <p className="text-ink-2">
        Te enviamos un código de 6 números al <b className="font-mono text-ink">{formatoCelular(celular)}</b>. Vence en 10 minutos.
      </p>
      <Field
        label="Código"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        mono
        autoFocus
        value={codigo}
        onChange={(e) => setCodigo(e.target.value.replace(/\D/g, ''))}
        className="max-w-[220px] text-2xl"
      />
      {error ? (
        <Alert>
          {error.message}
          {puedeReenviar ? (
            <>
              {' '}
              <button type="button" className="font-bold underline" onClick={onResend}>
                Pedir otro código
              </button>
            </>
          ) : null}
        </Alert>
      ) : null}
      <div className="flex flex-wrap items-center gap-3">
        <button type="button" onClick={onBack} className="min-h-[44px] px-3 font-bold text-primary">
          Cambiar celular
        </button>
        <div className="flex-1">
          <Button type="submit" busy={busy} missing={valido ? undefined : 'Escribe los 6 números del código.'} className="w-full">
            Entrar
          </Button>
        </div>
      </div>
    </form>
  );
}
