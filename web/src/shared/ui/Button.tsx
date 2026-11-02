import type { ButtonHTMLAttributes, ReactNode } from 'react';

type Variant = 'primary' | 'secondary' | 'ghost';

const styles: Record<Variant, string> = {
  // DR-05: un solo botón primary relleno por pantalla.
  primary: 'bg-primary text-white hover:bg-primary-pressed disabled:bg-line disabled:text-ink-2',
  secondary: 'bg-white text-primary border-[1.5px] border-primary hover:bg-primary-tint disabled:opacity-60',
  ghost: 'bg-transparent text-primary hover:bg-primary-tint disabled:opacity-60',
};

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  icon?: ReactNode;
  busy?: boolean;
  /** DR-06: si está deshabilitado, decir qué falta en vez de un botón gris mudo. */
  missing?: string | undefined;
};

export function Button({ variant = 'primary', icon, busy = false, missing, disabled, children, className = '', ...rest }: Props) {
  const isDisabled = disabled === true || busy || (missing !== undefined && missing !== '');
  return (
    <span className="flex flex-col gap-1">
      <button
        type="button"
        {...rest}
        disabled={isDisabled}
        aria-busy={busy || undefined}
        className={`inline-flex min-h-[52px] items-center justify-center gap-2 rounded-btn px-5 text-base font-extrabold transition-colors disabled:cursor-not-allowed ${styles[variant]} ${className}`}
      >
        {icon}
        {busy ? 'Un momento…' : children}
      </button>
      {missing ? <span className="text-sm text-ink-2">{missing}</span> : null}
    </span>
  );
}
