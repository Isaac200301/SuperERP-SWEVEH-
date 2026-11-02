import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Button } from './Button';
import { Segmented } from './Segmented';
import { StatePill } from './StatePill';

describe('StatePill (DR-01)', () => {
  it('muestra la palabra del estado, no solo color', () => {
    render(<StatePill estado="esperando_aprobacion" />);
    expect(screen.getByText('Esperando aprobación')).toBeInTheDocument();
  });
});

describe('Button (DR-06)', () => {
  it('deshabilitado explica qué falta', () => {
    render(<Button missing="Falta la placa.">Abrir orden</Button>);
    expect(screen.getByRole('button', { name: 'Abrir orden' })).toBeDisabled();
    expect(screen.getByText('Falta la placa.')).toBeInTheDocument();
  });
});

describe('Segmented (DR-14)', () => {
  it('es un radiogroup y avisa el cambio', async () => {
    const onChange = vi.fn();
    render(
      <Segmented
        label="Turbo"
        value="no"
        onChange={onChange}
        options={[
          { value: 'no', label: 'No' },
          { value: 'si', label: 'Sí' },
        ]}
      />,
    );
    expect(screen.getByRole('radiogroup', { name: 'Turbo' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'No' })).toHaveAttribute('aria-checked', 'true');
    await userEvent.click(screen.getByRole('radio', { name: 'Sí' }));
    expect(onChange).toHaveBeenCalledWith('si');
  });
});
