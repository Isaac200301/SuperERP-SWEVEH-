import type { ReactNode } from 'react';
import { Navigate } from 'react-router';

import { useFeature, type Funcion } from '../features/auth/api';

/**
 * Una pantalla de una función apagada en el servidor (V-01, V-02) no existe: una URL vieja vuelve al inicio del panel.
 * El servidor tampoco monta sus rutas (404).
 */
export function RequireFeature({ funcion, children }: { funcion: Funcion; children: ReactNode }) {
  return useFeature(funcion) ? children : <Navigate to="/panel" replace />;
}
