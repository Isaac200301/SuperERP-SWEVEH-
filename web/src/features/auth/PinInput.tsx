import { useId, useRef, type ClipboardEvent, type KeyboardEvent } from 'react';

/**
 * PIN de 4 casillas numéricas (HU-104, Figma "01 Entrar con PIN"). Cada casilla acepta un dígito y salta a la
 * siguiente; escribir en una casilla llena la reemplaza; borrar vuelve a la anterior; pegar "2580" llena las cuatro.
 * Se muestran puntos, no los números.
 */
export function PinInput({
  label,
  value,
  onChange,
  error,
  hint,
  autoComplete = 'current-password',
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  error?: string | undefined;
  hint?: string | undefined;
  autoComplete?: 'current-password' | 'new-password';
}) {
  const id = useId();
  const refs = useRef<(HTMLInputElement | null)[]>([]);
  const digits = Array.from({ length: 4 }, (_, i) => value[i] ?? '');

  const put = (i: number, d: string) => {
    const next = digits.slice();
    next[i] = d;
    onChange(next.join('').slice(0, 4));
  };
  const fill = (from: number, typed: string) => {
    onChange((value.slice(0, from) + typed).slice(0, 4));
    refs.current[Math.min(from + typed.length, 3)]?.focus();
  };
  const onKey = (i: number, e: KeyboardEvent<HTMLInputElement>) => {
    // Un número del teclado se pone aquí y salta, aunque sea el mismo que ya había (React no avisa ese cambio).
    if (/^[0-9]$/.test(e.key)) {
      e.preventDefault();
      put(i, e.key);
      if (i < 3) refs.current[i + 1]?.focus();
      return;
    }
    if (e.key === 'Backspace' && !digits[i] && i > 0) {
      put(i - 1, '');
      refs.current[i - 1]?.focus();
    }
  };
  const onPaste = (i: number, e: ClipboardEvent<HTMLInputElement>) => {
    const typed = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, 4);
    if (!typed) return;
    e.preventDefault();
    fill(i, typed);
  };
  const described = [hint ? `${id}-ayuda` : '', error ? `${id}-error` : ''].join(' ').trim() || undefined;

  return (
    <fieldset className="flex flex-col gap-1.5" aria-describedby={described}>
      <legend className="mb-1.5 text-[15px] font-extrabold">{label}</legend>
      <div className="flex gap-2.5">
        {digits.map((d, i) => (
          <input
            key={i}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="password"
            inputMode="numeric"
            pattern="[0-9]*"
            autoComplete={i === 0 ? autoComplete : 'off'}
            aria-label={`${label}: número ${i + 1} de 4`}
            aria-invalid={error ? true : undefined}
            value={d}
            onKeyDown={(e) => onKey(i, e)}
            onPaste={(e) => onPaste(i, e)}
            onFocus={(e) => e.target.select()}
            onChange={(e) => {
              const typed = e.target.value.replace(/\D/g, '');
              if (typed.length > 1 && !d) {
                fill(i, typed); // varios números a la vez (autocompletar): se reparten desde esta casilla
                return;
              }
              // En una casilla llena, el número nuevo reemplaza al viejo, quede antes o después del cursor.
              const v = typed.length > 1 && typed.startsWith(d) ? typed.slice(-1) : typed.slice(0, 1);
              put(i, v);
              if (v && i < 3) refs.current[i + 1]?.focus();
            }}
            className={`h-[60px] w-[56px] rounded-btn border-[1.5px] bg-white text-center font-mono text-2xl font-bold outline-none focus:border-primary ${
              error ? 'border-err' : 'border-line'
            }`}
          />
        ))}
      </div>
      {hint ? (
        <span id={`${id}-ayuda`} className="text-sm text-ink-2">
          {hint}
        </span>
      ) : null}
      {error ? (
        <span id={`${id}-error`} role="alert" className="text-sm font-semibold text-err-ink">
          {error}
        </span>
      ) : null}
    </fieldset>
  );
}
