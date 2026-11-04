import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Link, useNavigate } from 'react-router';
import { z } from 'zod';

import { celularValido } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { useRequestCode, useVerifyCode } from './api';
import { AuthLayout } from './AuthLayout';
import { CodeForm } from './CodeForm';

const schema = z.object({
  nombre: z.string().trim().min(1, 'Escribe tu nombre.').max(120, 'El nombre puede tener hasta 120 letras.'),
  celular: z.string().refine(celularValido, 'Escribe un celular colombiano de 10 dígitos que empiece por 3.'),
  // HU-02: la casilla aparece sin marcar y sin marcarla no se puede seguir.
  autorizacionDatos: z.boolean().refine((v) => v, 'Para continuar debes aceptar la política de tratamiento de datos.'),
});

type Datos = z.infer<typeof schema>;

const steps = [
  { title: 'Tu cuenta', detail: 'Celular verificado y datos autorizados' },
  { title: 'Tu taller', detail: 'Qué atiendes; te lo dejamos armado' },
  { title: 'Primera orden', detail: 'Con una placa real o de ejemplo' },
];

/** HU-01 + HU-02: crear la cuenta solo con nombre y celular, confirmando con un código. */
export function RegisterPage() {
  const navigate = useNavigate();
  const [datos, setDatos] = useState<Datos | null>(null);
  const requestCode = useRequestCode();
  const verify = useVerifyCode();
  const form = useForm<Datos>({
    resolver: zodResolver(schema),
    defaultValues: { nombre: '', celular: '', autorizacionDatos: false },
  });

  const pedirCodigo = form.handleSubmit(async (d) => {
    await requestCode.mutateAsync(d.celular).then(
      () => setDatos(d),
      () => undefined, // el error se muestra abajo
    );
  });

  return (
    <AuthLayout steps={steps} current={0}>
      <div>
        <span className="font-mono text-[13px] font-bold text-primary">PASO 1 DE 3</span>
        <h1 className="mt-1.5 mb-1.5 text-[32px] font-extrabold tracking-[-.02em]">Crea la cuenta de tu taller</h1>
        <p className="text-base text-ink-2">Solo tu nombre y tu celular. Sin correo y sin contraseña.</p>
      </div>

      {datos === null ? (
        <form onSubmit={pedirCodigo} className="flex max-w-[520px] flex-col gap-5" noValidate>
          <Field label="Tu nombre" autoComplete="name" error={form.formState.errors.nombre?.message} {...form.register('nombre')} />
          <Field
            label="Tu celular"
            inputMode="tel"
            autoComplete="tel-national"
            placeholder="300 123 4567"
            hint="Te mandamos un código para confirmarlo."
            error={form.formState.errors.celular?.message}
            {...form.register('celular')}
          />
          <div className="flex flex-col gap-1.5">
            <label className="flex min-h-[44px] items-start gap-3 text-[15px]">
              <input type="checkbox" className="mt-1 h-5 w-5 accent-primary" {...form.register('autorizacionDatos')} />
              <span>
                Autorizo a SWEVEH a tratar mis datos personales según la{' '}
                <Link to="/politica-de-datos" target="_blank" rel="noopener noreferrer" className="font-bold text-primary underline">
                  política de tratamiento de datos
                </Link>
                .
              </span>
            </label>
            {form.formState.errors.autorizacionDatos ? (
              <span role="alert" className="text-sm font-semibold text-err-ink">
                {form.formState.errors.autorizacionDatos.message}
              </span>
            ) : null}
          </div>
          {requestCode.error ? <Alert>{requestCode.error.message}</Alert> : null}
          <Button type="submit" busy={requestCode.isPending}>
            Recibir código
          </Button>
          <p className="text-sm text-ink-2">
            ¿Ya tienes cuenta?{' '}
            <Link to="/entrar" className="font-bold text-primary">
              Entra con tu celular
            </Link>
          </p>
        </form>
      ) : (
        <div className="max-w-[520px]">
          <CodeForm
            celular={datos.celular}
            busy={verify.isPending}
            error={verify.error}
            onBack={() => {
              verify.reset();
              setDatos(null);
            }}
            onResend={() => {
              verify.reset();
              requestCode.mutate(datos.celular);
            }}
            onSubmit={(codigo) =>
              verify.mutate(
                { celular: datos.celular, codigo, nombre: datos.nombre, autorizacionDatos: datos.autorizacionDatos },
                {
                  // HU-01 criterio 3: si el celular ya tenía cuenta, entra a ella en vez de crear otra.
                  // Cuenta nueva: sigue al paso 2, tipo de taller y precarga (HU-03, HU-04).
                  onSuccess: (s) => navigate(s.cuentaNueva ? '/panel/arranque' : '/panel/ordenes', { replace: true }),
                },
              )
            }
          />
        </div>
      )}
    </AuthLayout>
  );
}
