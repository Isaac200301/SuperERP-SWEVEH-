import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';

import { faltantesFicha, fichaPayload, fichaVacia, type Ficha, type Tipo } from './sheet';
import { VehicleSheet } from './VehicleSheet';

describe('Regla de la ficha (igual a vehicles/domain.Engine.Pending)', () => {
  it('pide combustible y caja; en combustión cilindraje y aspiración; en moto los tiempos', () => {
    expect(faltantesFicha('carro', fichaVacia)).toEqual(['combustible', 'caja']);
    expect(faltantesFicha('carro', { ...fichaVacia, energia: 'diesel', transmision: 'manual' })).toEqual(['cilindraje', 'turbo o atmosférico']);
    expect(faltantesFicha('carro', { ...fichaVacia, energia: 'electrico', transmision: 'automatica' })).toEqual([]);
    // La moto viene atmosférica por defecto: solo faltan los tiempos.
    expect(faltantesFicha('moto', { ...fichaVacia, energia: 'gasolina', transmision: 'manual', cilindraje: '150' })).toEqual(['2 o 4 tiempos']);
  });

  it('el eléctrico no envía datos de combustión', () => {
    expect(fichaPayload({ ...fichaVacia, energia: 'electrico', cilindraje: '1600', aspiracion: 'turbo', color: ' Azul ' })).toEqual({ energia: 'electrico', color: 'Azul' });
  });
});

function Harness({ tipo }: { tipo: Tipo }) {
  const [f, setF] = useState<Ficha>(fichaVacia);
  return <VehicleSheet tipo={tipo} value={f} onChange={setF} />;
}

describe('VehicleSheet', () => {
  it('en moto muestra tiempos y refrigeración, y no tracción ni carrocería', async () => {
    render(<Harness tipo="moto" />);
    expect(screen.getByText('Ficha pendiente')).toBeInTheDocument();
    expect(screen.queryByLabelText('Tracción')).not.toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Combustible'), 'gasolina');
    await userEvent.selectOptions(screen.getByLabelText('Caja'), 'manual');
    await userEvent.type(screen.getByLabelText('Cilindraje'), '150');
    await userEvent.click(screen.getByRole('radio', { name: '4 tiempos' }));
    expect(screen.getByText('Ficha completa')).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Aire' })).toBeInTheDocument();
  });

  it('en carro eléctrico oculta cilindraje y queda completa con combustible y caja', async () => {
    render(<Harness tipo="carro" />);
    expect(screen.getByLabelText('Tracción')).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Combustible'), 'electrico');
    await userEvent.selectOptions(screen.getByLabelText('Caja'), 'automatica');
    expect(screen.queryByLabelText('Cilindraje')).not.toBeInTheDocument();
    expect(screen.getByText('Ficha completa')).toBeInTheDocument();
  });
});
