/**
 * Ficha del vehículo (decisión del 3-nov-2026): opciones por tipo y la misma regla de "pendiente" que
 * vehicles/domain.Engine.Pending. La UI solo la usa para avisar; el servidor decide.
 */
import type { Schemas } from '../../shared/api/client';

export type Tipo = 'carro' | 'moto';

/** Lo que se envía al servidor: cilindraje en texto ("1.6 L") y el resto con los enums del contrato. */
export type FichaPayload = { cilindraje?: string } & Partial<
  Pick<Schemas['Vehiculo'], 'energia' | 'aspiracion' | 'transmision' | 'traccion' | 'ciclo' | 'refrigeracion' | 'color' | 'carroceria' | 'llanta'>
>;

export type Ficha = {
  energia: string;
  aspiracion: string;
  cilindraje: string;
  transmision: string;
  traccion: string;
  ciclo: string;
  refrigeracion: string;
  color: string;
  carroceria: string;
  llanta: string;
};

export const fichaVacia: Ficha = {
  energia: '', aspiracion: '', cilindraje: '', transmision: '', traccion: '', ciclo: '', refrigeracion: '', color: '', carroceria: '', llanta: '',
};

type Opcion = { value: string; label: string };
const o = (value: string, label: string): Opcion => ({ value, label });

export const opciones: Record<Tipo, Record<'energia' | 'aspiracion' | 'transmision' | 'traccion' | 'ciclo' | 'refrigeracion' | 'carroceria', Opcion[]>> = {
  carro: {
    energia: [o('gasolina', 'Gasolina'), o('diesel', 'Diésel'), o('gas', 'Gas (GNV/GLP)'), o('hibrido', 'Híbrido'), o('hibrido_enchufable', 'Híbrido enchufable'), o('electrico', 'Eléctrico')],
    aspiracion: [o('atmosferico', 'Atmosférico'), o('turbo', 'Turbo'), o('supercargado', 'Supercargado')],
    transmision: [o('manual', 'Manual'), o('automatica', 'Automática'), o('cvt', 'CVT'), o('doble_embrague', 'Doble embrague'), o('amt', 'Robotizada (AMT)')],
    traccion: [o('delantera', '4x2 delantera'), o('trasera', '4x2 trasera'), o('awd', 'AWD'), o('4x4', '4x4')],
    ciclo: [],
    refrigeracion: [],
    carroceria: [o('sedan', 'Sedán'), o('hatchback', 'Hatchback'), o('suv', 'SUV'), o('pickup', 'Pick-up'), o('van', 'Van'), o('otra', 'Otra')],
  },
  moto: {
    energia: [o('gasolina', 'Gasolina'), o('electrico', 'Eléctrica')],
    aspiracion: [o('atmosferico', 'Atmosférico'), o('turbo', 'Turbo')],
    transmision: [o('manual', 'De cambios'), o('automatica', 'Automática (scooter)'), o('semiautomatica', 'Semiautomática')],
    traccion: [],
    ciclo: [o('4t', '4 tiempos'), o('2t', '2 tiempos')],
    refrigeracion: [o('aire', 'Aire'), o('liquido', 'Líquido')],
    carroceria: [],
  },
};

export const esElectrico = (f: Ficha) => f.energia === 'electrico';

/** Con los valores por defecto del servidor: la moto viene atmosférica (HU-11 criterio 3). */
export function conDefectos(tipo: Tipo | null, f: Ficha): Ficha {
  return tipo === 'moto' && f.aspiracion === '' ? { ...f, aspiracion: 'atmosferico' } : f;
}

/** Qué falta para poder recibir el vehículo; vacío = la ficha está completa. */
export function faltantesFicha(tipo: Tipo | null, ficha: Ficha): string[] {
  const f = conDefectos(tipo, ficha);
  const falta: string[] = [];
  if (f.energia === '') falta.push('combustible');
  if (f.transmision === '') falta.push('caja');
  if (!esElectrico(f) && f.energia !== '') {
    if (f.cilindraje.trim() === '') falta.push('cilindraje');
    if (f.aspiracion === '') falta.push('turbo o atmosférico');
    if (tipo === 'moto' && f.ciclo === '') falta.push('2 o 4 tiempos');
  }
  return falta;
}

/** Solo los campos con valor, con los nombres del contrato (POST /vehicles y PATCH /vehicles/{id}/engine). */
export function fichaPayload(f: Ficha): FichaPayload {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(f)) {
    if (v.trim() !== '') out[k] = v.trim();
  }
  if (esElectrico(f)) {
    delete out.cilindraje;
    delete out.aspiracion;
    delete out.ciclo;
  }
  return out as FichaPayload;
}
