import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router';

import { api, unwrap } from '../../shared/api/client';
import { formatoCOP, resumenHorario } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { usePermission } from '../auth/api';
import { settingsKey, useSettings } from './SettingsForm';

const atiendeTexto: Record<string, string> = { carro: 'carros', moto: 'motos', ambos: 'carros y motos' };

function Fila({ nombre, valor }: Readonly<{ nombre: string; valor: string }>) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-[13px]">
      <dt className="text-ink-2">{nombre}</dt>
      <dd className="font-mono font-bold">{valor}</dd>
    </div>
  );
}

/**
 * "Mi taller" al lado de Servicios (Figma D3, V-06): el interruptor del inventario en otro programa, el horario, el
 * checklist y lo avanzado de un vistazo. Todo se edita en Mi taller; el interruptor se puede mover aquí mismo.
 */
export function MiTallerResumen() {
  const qc = useQueryClient();
  const puede = usePermission('workshop:manage');
  const taller = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  const settings = useSettings();
  const guardar = useMutation({
    mutationFn: async (inventarioExterno: boolean) => {
      if (!settings.data) throw new Error('Todavía no cargan los parámetros.');
      return unwrap(await api.PUT('/workshop/settings', { body: { ...settings.data, inventarioExterno } }));
    },
    onSuccess: (s) => qc.setQueryData(settingsKey, s),
  });
  const p = settings.data;
  const estadosOpcionales = [p?.usarEsperandoRepuesto ? 'Esperando repuesto' : '', p?.usarControlCalidad ? 'Control de calidad' : ''].filter(Boolean).join(' y ');
  return (
    <section aria-labelledby="mi-taller" className="flex flex-col gap-3.5 rounded-card bg-white p-5">
      <div>
        <h2 id="mi-taller" className="text-[20px] font-extrabold">
          Mi taller
        </h2>
        {taller.data ? (
          <p className="text-[13px] text-ink-2">
            {[taller.data.nombre, taller.data.ciudad, atiendeTexto[taller.data.atiende]].filter(Boolean).join(' · ')}
          </p>
        ) : null}
      </div>
      {settings.isError ? <Alert>{settings.error.message}</Alert> : null}
      {p ? (
        <>
          <label className="flex items-start gap-3 rounded-2xl bg-primary-tint p-3.5">
            <span className="flex-1">
              <b className="block text-[15px]">Llevo el inventario en otro programa</b>
              <span className="text-[13px] text-ink-2">No se cuentan existencias. Los repuestos se escriben en la cotización.</span>
            </span>
            <input
              type="checkbox"
              role="switch"
              aria-checked={p.inventarioExterno}
              checked={p.inventarioExterno}
              disabled={!puede || guardar.isPending}
              onChange={(e) => guardar.mutate(e.target.checked)}
              className="peer sr-only"
            />
            <span
              aria-hidden="true"
              className="relative mt-1 h-[26px] w-11 flex-none rounded-full bg-ink-3 transition-colors peer-checked:bg-primary peer-focus-visible:outline-3 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary peer-disabled:opacity-60 after:absolute after:top-[3px] after:left-[3px] after:h-5 after:w-5 after:rounded-full after:bg-white after:transition-transform peer-checked:after:translate-x-[18px]"
            />
          </label>
          {guardar.isError ? <Alert>{guardar.error.message}</Alert> : null}
          <div>
            <h3 className="text-[15px] font-extrabold">Horario</h3>
            <p className="text-[13px] text-ink-2">{taller.data ? resumenHorario(taller.data.horario) : 'Cargando…'}</p>
          </div>
          <div>
            <h3 className="text-[15px] font-extrabold">Checklist de recepción</h3>
            <p className="text-[13px] text-ink-2">
              {p.itemsCarro.length} ítems de carro · {p.itemsMoto.length} de moto
            </p>
          </div>
          <div className="flex flex-col gap-2 rounded-2xl bg-surface p-3.5">
            <h3 className="text-[15px] font-extrabold">Avanzado</h3>
            <dl className="flex flex-col gap-1.5">
              <Fila nombre="IVA" valor={`${p.ivaPct} %`} />
              <Fila nombre="Mis precios ya incluyen IVA" valor={p.preciosConIva ? 'Sí' : 'No'} />
              <Fila nombre="Tarifa por hora de mano de obra" valor={p.tarifaHora == null ? 'Libre' : formatoCOP(p.tarifaHora)} />
              <Fila nombre="Vigencia de la cotización" valor={`${p.vigenciaDias} días`} />
              <Fila nombre="Descuento máximo (todos los roles)" valor={`${p.descuentoMaxPct} %`} />
              <Fila nombre="Fotos mínimas al recibir" valor={String(p.fotosMinimas)} />
              <Fila nombre="Estados opcionales" valor={estadosOpcionales || 'Apagados'} />
            </dl>
          </div>
          {puede ? (
            <Link to="/panel/taller" className="inline-flex min-h-[44px] items-center font-bold text-primary">
              Editar en Mi taller
            </Link>
          ) : null}
        </>
      ) : (
        <p role="status">Cargando…</p>
      )}
    </section>
  );
}
