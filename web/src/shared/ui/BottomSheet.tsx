import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

import { IconX } from './icons';

/**
 * Hoja inferior del celular (mockups móviles): diálogo modal que sube desde abajo.
 * Se cierra con el fondo, con Escape o con la X, y devuelve el foco a quien la abrió.
 */
export function BottomSheet({
  open,
  title,
  onClose,
  children,
  eyebrow,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** Texto pequeño sobre el título (p. ej. "¿QUÉ SIGNIFICA?"). */
  eyebrow?: string;
}) {
  const id = useId();
  const panel = useRef<HTMLDivElement>(null);
  const opener = useRef<Element | null>(null);
  // onClose cambia en cada render de quien la usa: con un ref el efecto no se repite y no roba el foco al escribir.
  const closeRef = useRef(onClose);
  useEffect(() => {
    closeRef.current = onClose;
  });

  useEffect(() => {
    if (!open) return;
    opener.current = document.activeElement;
    panel.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeRef.current();
    };
    document.addEventListener('keydown', onKey);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = overflow;
      if (opener.current instanceof HTMLElement) opener.current.focus();
    };
  }, [open]);

  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex flex-col justify-end bg-[rgba(6,18,17,.5)]">
      <button type="button" aria-label="Cerrar" tabIndex={-1} className="flex-1 cursor-default" onClick={onClose} />
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={id}
        tabIndex={-1}
        className="flex max-h-[90dvh] flex-col gap-4 overflow-y-auto rounded-t-[28px] bg-white px-5 pt-2.5 pb-[calc(24px+env(safe-area-inset-bottom))] outline-none"
      >
        <span aria-hidden="true" className="h-[5px] w-11 self-center rounded-full bg-line" />
        <div className="flex items-start gap-3">
          <div className="flex min-w-0 flex-1 flex-col">
            {eyebrow ? <span className="text-[13px] font-extrabold text-primary">{eyebrow}</span> : null}
            <h2 id={id} className="text-[22px] font-extrabold tracking-[-.02em]">
              {title}
            </h2>
          </div>
          <button type="button" aria-label="Cerrar" onClick={onClose} className="grid h-11 w-11 flex-none place-items-center rounded-full bg-surface">
            <IconX size={18} />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  );
}
