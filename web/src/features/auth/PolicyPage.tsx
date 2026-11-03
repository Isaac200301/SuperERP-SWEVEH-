import { Brand } from '../../shared/ui/Logo';
import { Alert } from '../../shared/ui/Alert';

/**
 * Política de tratamiento de datos. El texto definitivo se publica con HU-90 y requiere
 * revisión jurídica (AQ-10): aquí no se inventa contenido legal (docs/02 §3.4).
 */
/** Debe coincidir con identity/domain.PolicyVersionDraft: es la versión que guarda cada autorización (HU-02). */
export const policyVersion = 'borrador-sin-validar-2026-11-01';
const policyDate = '1 de noviembre de 2026';

export function PolicyPage() {
  return (
    <div className="mx-auto flex min-h-screen max-w-[760px] flex-col gap-6 p-6">
      <Brand />
      <h1 className="text-[28px] font-extrabold">Política de tratamiento de datos personales</h1>
      <p className="text-sm text-ink-2">
        Versión <span className="font-mono">{policyVersion}</span> · vigente desde el {policyDate}
      </p>
      <Alert kind="warn" title="Borrador sin validar">
        Esta política todavía no está publicada. El texto definitivo lo redacta y revisa un abogado antes de atender talleres
        reales. Mientras tanto, SWEVEH solo se usa con datos de ejemplo.
      </Alert>
      <p className="text-ink-2">
        SWEVEH trata los datos según la Ley 1581 de 2012 y el Decreto 1377 de 2013: con autorización previa, expresa e
        informada, y guardando la prueba de esa autorización.
      </p>
    </div>
  );
}
