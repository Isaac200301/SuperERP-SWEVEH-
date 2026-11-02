import { useId, type InputHTMLAttributes, type ReactNode, type Ref } from 'react';

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: string;
  hint?: ReactNode;
  error?: string | undefined;
  mono?: boolean;
  /** plate: el campo de placa se ve como placa (DR-09). */
  tone?: 'default' | 'plate';
  ref?: Ref<HTMLInputElement>;
};

/** Campo con etiqueta y error asociados (aria-describedby), en español claro junto al campo. */
export function Field({ label, hint, error, mono = false, tone = 'default', className = '', ref, ...rest }: Props) {
  const id = useId();
  const hintId = `${id}-ayuda`;
  const errId = `${id}-error`;
  const describedBy = [hint ? hintId : null, error ? errId : null].filter(Boolean).join(' ') || undefined;
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[15px] font-extrabold">
        {label}
      </label>
      <input
        id={id}
        ref={ref}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={`min-h-[52px] rounded-btn border-[1.5px] px-4 text-base outline-none transition-colors focus:border-primary ${
          tone === 'plate' ? 'border-ink bg-plate font-bold placeholder:text-warn-ink/60' : 'bg-white'
        } ${error ? 'border-err' : tone === 'plate' ? '' : 'border-line'} ${mono ? 'font-mono tracking-wider' : ''} ${className}`}
        {...rest}
      />
      {hint ? (
        <span id={hintId} className="text-sm text-ink-2">
          {hint}
        </span>
      ) : null}
      {error ? (
        <span id={errId} role="alert" className="text-sm font-semibold text-err-ink">
          {error}
        </span>
      ) : null}
    </div>
  );
}
