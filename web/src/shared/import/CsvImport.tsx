import { useMutation } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { api, ApiError } from '../api/client';
import { Alert } from '../ui/Alert';
import { Button } from '../ui/Button';

/** Respuesta de las importaciones por CSV (HU-37 repuestos, HU-92 clientes y vehículos). */
export type ImportResult = {
  filas: { fila: number; referencia: string; nombre: string; estado: 'ok' | 'error'; errores: string[] }[];
  conError: number;
  importadas: number;
  clientesNuevos?: number;
  avisos: string[];
};

type Endpoint = '/parts/import' | '/vehicles/import';

/**
 * Sube el CSV. Sin confirmar es la vista previa. Con errores el servidor responde 422 con las mismas filas, así que
 * se devuelve el cuerpo en vez de lanzar el error.
 */
function useCsvImport(endpoint: Endpoint, onImported?: () => void) {
  return useMutation({
    mutationFn: async (v: { archivo: File; confirmar: boolean }): Promise<ImportResult> => {
      const r = await api.POST(endpoint as '/parts/import', {
        params: { query: { confirmar: v.confirmar ? '1' : '0' } },
        body: { archivo: '' },
        bodySerializer: () => {
          const fd = new FormData();
          fd.append('archivo', v.archivo);
          return fd;
        },
      });
      if (r.data) return r.data as ImportResult;
      if (r.response.status === 422 && r.error && 'filas' in (r.error as object)) return r.error as unknown as ImportResult;
      throw new ApiError(r.response.status, r.error as never);
    },
    onSuccess: (res) => {
      if (res.importadas > 0) onImported?.();
    },
  });
}

function resumen(v: ImportResult, noun: string): string {
  if (v.importadas > 0) {
    const nuevos = v.clientesNuevos ? ` (${v.clientesNuevos} ${v.clientesNuevos === 1 ? 'cliente nuevo' : 'clientes nuevos'})` : '';
    return `Listo: se importaron ${v.importadas} ${noun}${nuevos}.`;
  }
  if (v.conError) return `${v.conError} de ${v.filas.length} filas tienen errores. Corrígelas en el archivo y vuelve a subirlo.`;
  return `Las ${v.filas.length} filas están bien.`;
}

/**
 * Subir un CSV, revisar fila por fila y confirmar (HU-37, HU-92). Con una fila en error no se importa nada.
 * La plantilla se descarga desde el mismo bloque.
 */
export function CsvImport({ endpoint, title, help, template, templateName, noun, onImported }: {
  endpoint: Endpoint; title: string; help: ReactNode; template: string; templateName: string; noun: string; onImported?: () => void;
}) {
  const imp = useCsvImport(endpoint, onImported);
  const [archivo, setArchivo] = useState<File | null>(null);
  const [vista, setVista] = useState<ImportResult | null>(null);
  const plantilla = `data:text/csv;charset=utf-8,${encodeURIComponent(template)}`;
  const revisar = (f: File) => {
    setArchivo(f);
    imp.mutate({ archivo: f, confirmar: false }, { onSuccess: setVista });
  };
  return (
    <section className="flex flex-col gap-3 rounded-card bg-white p-5" aria-labelledby={`importar-${endpoint}`}>
      <h2 id={`importar-${endpoint}`} className="text-[17px] font-extrabold">{title}</h2>
      <p className="text-sm text-ink-2">
        {help} Usa las columnas de la{' '}
        <a className="font-bold text-primary underline" href={plantilla} download={templateName}>
          plantilla
        </a>
        . Primero ves cada fila revisada; nada se importa hasta que confirmes y todas estén bien.
      </p>
      <label className="flex flex-col gap-1.5 text-[15px] font-extrabold">
        Archivo CSV
        <input type="file" accept=".csv,text/csv" onChange={(e) => e.target.files?.[0] && revisar(e.target.files[0])} />
      </label>
      {imp.isError ? <Alert>{imp.error.message}</Alert> : null}
      {vista ? (
        <>
          <p role="status" className={vista.conError ? 'font-bold text-warn-ink' : 'font-bold text-ok'}>
            {resumen(vista, noun)}
          </p>
          {(vista.avisos ?? []).map((a) => (
            <p key={a} className="text-sm font-semibold text-warn-ink">
              {a}
            </p>
          ))}
          <ul className="flex max-h-72 flex-col gap-1 overflow-auto text-sm">
            {vista.filas.map((r) => (
              <li key={r.fila} className={`flex flex-wrap gap-3 rounded-xl px-3 py-2 ${r.estado === 'error' ? 'bg-err-tint' : ''}`}>
                <span className="w-14 text-ink-2">Fila {r.fila}</span>
                <span className="font-mono">{r.referencia}</span>
                <span className="flex-1">{r.nombre}</span>
                {r.errores.length ? <span className="font-bold text-err-ink">{r.errores.join(' · ')}</span> : <span className="text-ok">ok</span>}
              </li>
            ))}
          </ul>
          {archivo && vista.importadas === 0 ? (
            <Button
              busy={imp.isPending}
              missing={vista.conError ? 'Corrige las filas con error antes de importar.' : undefined}
              onClick={() => imp.mutate({ archivo, confirmar: true }, { onSuccess: setVista })}
            >
              Importar {vista.filas.length} {noun}
            </Button>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
