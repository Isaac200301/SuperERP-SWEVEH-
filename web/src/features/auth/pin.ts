/** PIN fáciles de adivinar: la misma lista que rechaza el servidor (internal/identity/domain/pin.go, HU-104). */
const faciles = new Set([
  '0000', '1111', '2222', '3333', '4444', '5555', '6666', '7777', '8888', '9999',
  '1234', '2345', '3456', '4567', '5678', '6789', '0123', '9876', '8765', '7654', '6543', '5432', '4321', '3210',
]);

export function pinFacil(pin: string): boolean {
  return faciles.has(pin);
}

/** El error de un PIN nuevo, o undefined si sirve. vacio es el mensaje cuando faltan números. */
export function errorDePin(pin: string, vacio = 'Escoge 4 números.'): string | undefined {
  if (pin.length !== 4) return vacio;
  if (pinFacil(pin)) return 'Ese PIN es muy fácil de adivinar. Escoge otros 4 números.';
  return undefined;
}
