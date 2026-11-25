/**
 * "Asistente de esta orden" (Figma D2, V-11): el chat fijo a la derecha de la orden. Hasta R8 (HU-107) es un aviso;
 * cuando llegue el asistente, aquí va la conversación de la orden.
 */
export function OrderAssistantCard() {
  return (
    <section aria-labelledby="asistente-orden" className="flex flex-col gap-2 rounded-card bg-white p-5">
      <h2 id="asistente-orden" className="text-[17px] font-extrabold">
        Asistente de esta orden
      </h2>
      <p className="rounded-2xl bg-surface p-3.5 text-[14px] text-ink-2">
        <b className="text-ink">Pronto.</b> Aquí le vas a hablar al asistente de esta orden: dictar la recepción, armar la cotización y anotar gastos.
      </p>
    </section>
  );
}
