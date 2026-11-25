import { describe, expect, it } from 'vitest';

import { celularValido, formatoCelular, formatoCOP, resumenHorario, soloCelular } from './format';
import { normalizarPlaca, reconocerPlaca, tipoPorPlaca } from './plate';

describe('placa (HU-10, igual que el dominio en Go)', () => {
  it.each([
    ['KLM482', 'carro'],
    ['XVN43F', 'moto'],
    ['XVN43', 'moto_antigua'],
    ['klm-482', 'carro'],
    ['R12345', 'especial'],
    ['', 'vacia'],
    ['##', 'invalida'],
  ] as const)('%s → %s', (raw, formato) => {
    expect(reconocerPlaca(raw)).toBe(formato);
  });

  it('normaliza minúsculas, espacios y guiones', () => {
    expect(normalizarPlaca(' klm-48.2 ')).toBe('KLM482');
  });

  it('deduce el tipo solo si el formato lo dice', () => {
    expect(tipoPorPlaca('carro')).toBe('carro');
    expect(tipoPorPlaca('moto_antigua')).toBe('moto');
    expect(tipoPorPlaca('especial')).toBeNull();
  });
});

describe('formatos de docs/08', () => {
  it('COP sin decimales con punto de miles', () => {
    expect(formatoCOP(128000).replace(/\s/g, ' ')).toMatch(/^\$ ?128\.000$/);
    expect(formatoCOP(1250000.6).replace(/\s/g, ' ')).toMatch(/^\$ ?1\.250\.001$/);
  });

  it('celular colombiano de 10 dígitos que empieza por 3', () => {
    expect(celularValido('300 123 4567')).toBe(true);
    expect(celularValido('+57 300 123 4567')).toBe(true);
    expect(celularValido('6012345678')).toBe(false);
    expect(celularValido('30012345')).toBe(false);
    expect(soloCelular('+57 (300) 123-4567')).toBe('3001234567');
    expect(formatoCelular('3001234567')).toBe('300 123 4567');
  });
});

describe('resumenHorario (Figma D3: Mi taller)', () => {
  const dia = (abre: string, cierra: string) => ({ abre, cierra });
  it('junta los días seguidos con las mismas horas', () => {
    const h = { lun: dia('07:30', '18:00'), mar: dia('07:30', '18:00'), mie: dia('07:30', '18:00'), jue: dia('07:30', '18:00'), vie: dia('07:30', '18:00'), sab: dia('08:00', '13:00'), dom: null };
    expect(resumenHorario(h)).toBe('Lun a vie 7:30 a. m. – 6:00 p. m. · sáb 8:00 a. m. – 1:00 p. m.');
  });
  it('un día cerrado en medio parte el grupo y dos días seguidos se dicen con "y"', () => {
    const h = { lun: dia('08:00', '17:00'), mar: dia('08:00', '17:00'), mie: null, jue: dia('08:00', '17:00') };
    expect(resumenHorario(h)).toBe('Lun y mar 8:00 a. m. – 5:00 p. m. · jue 8:00 a. m. – 5:00 p. m.');
  });
  it('sin horas dice que no hay horario y el mediodía y la medianoche usan 12', () => {
    expect(resumenHorario({})).toBe('Sin horario cargado');
    expect(resumenHorario({ lun: dia('00:00', '12:15') })).toBe('Lun 12:00 a. m. – 12:15 p. m.');
  });
});
