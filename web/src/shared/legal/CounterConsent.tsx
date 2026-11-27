import { Segmented } from '../ui/Segmented';
import { textoAutorizacionCliente } from './consent';

/** La respuesta del cliente: undefined mientras no responde (el silencio no es autorización, R-10). */
export type Respuesta = boolean | undefined;

/**
 * HU-86 (R11): antes de guardar a un cliente nuevo se le lee o se le muestra la autorización de datos y se marca lo que
 * respondió. "No autoriza" también es respuesta: el cliente y la orden se crean igual, sin avisos.
 */
export function CounterConsent({ value, onChange, error }: { value: Respuesta; onChange: (v: boolean) => void; error?: string | undefined }) {
  const actual = value === undefined ? '' : value ? 'si' : 'no';
  return (
    <div className="flex flex-col gap-2 rounded-2xl bg-surface p-4">
      <b className="text-[15px]">Autorización de datos del cliente</b>
      <p className="text-sm text-ink-2">Léele o muéstrale este texto y marca lo que responda:</p>
      <blockquote className="border-l-4 border-primary pl-3 text-[15px]">{textoAutorizacionCliente}</blockquote>
      <Segmented<'si' | 'no' | ''>
        label="¿El cliente autoriza?"
        value={actual}
        options={[
          { value: 'si', label: 'Autoriza' },
          { value: 'no', label: 'No autoriza' },
        ]}
        onChange={(v) => onChange(v === 'si')}
      />
      {value === false ? <p className="text-sm text-ink-2">Se guarda igual, marcado: no le llegará ningún aviso.</p> : null}
      {error ? <p role="alert" className="text-sm font-bold text-err-ink">{error}</p> : null}
    </div>
  );
}
