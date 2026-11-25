/** Cómo salió el "Compartir PDF": el usuario eligió una app, lo canceló, o se descargó el archivo. */
export type ShareResult = 'compartido' | 'cancelado' | 'descargado';

function download(file: File): void {
  const url = URL.createObjectURL(file);
  const a = document.createElement('a');
  a.href = url;
  a.download = file.name;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

/**
 * V-10 (ADR-0043): abre el menú de compartir del sistema con el PDF cuando el navegador lo permite
 * (`navigator.canShare({ files })`); si no, descarga el archivo. No es una integración con WhatsApp: la persona
 * escoge la app. Cerrar el menú sin elegir no es un error.
 */
export async function sharePdf(blob: Blob, nombre: string, titulo: string): Promise<ShareResult> {
  const file = new File([blob], nombre, { type: 'application/pdf' });
  if (typeof navigator.canShare === 'function' && navigator.canShare({ files: [file] })) {
    try {
      await navigator.share({ files: [file], title: titulo });
      return 'compartido';
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return 'cancelado';
      // Cualquier otro fallo del menú: la persona igual se lleva el archivo.
    }
  }
  download(file);
  return 'descargado';
}
