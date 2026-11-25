/**
 * Pestaña "Asistente" del piso (V-11, HU-107). Por ahora es el chat vacío; el asistente que recibe, diagnostica y
 * cotiza llega en la rebanada R8 de docs/plan-entrega-viernes.md.
 */
export function AssistantPage() {
  return (
    <>
      <header className="flex flex-col gap-1">
        <h1 className="text-[26px] font-extrabold tracking-[-.02em]">Asistente</h1>
        <p className="text-[13px] text-ink-2">Recibe, diagnostica y cotiza hablándole al asistente.</p>
      </header>
      <section aria-label="Conversación con el asistente" className="flex min-h-[240px] flex-col items-center justify-center gap-2 rounded-[22px] bg-white p-6 text-center">
        <b className="text-[17px]">Pronto</b>
        <p className="text-sm text-ink-2">Mientras tanto, abre la orden y sigue con el formulario de siempre.</p>
      </section>
    </>
  );
}
