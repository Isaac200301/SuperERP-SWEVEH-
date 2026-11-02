import { lazy, Suspense, type ComponentType, type LazyExoticComponent } from 'react';
import { createBrowserRouter, Navigate } from 'react-router';

import { RequireSession } from './RequireSession';

// División de código por ruta (skill sweveh-frontend-react §11): cada pantalla se baja al abrirla.
const page = <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K): LazyExoticComponent<ComponentType> =>
  lazy(async () => ({ default: (await load())[name] }));

const RegisterPage = page(() => import('../features/auth/RegisterPage'), 'RegisterPage');
const LoginPage = page(() => import('../features/auth/LoginPage'), 'LoginPage');
const PolicyPage = page(() => import('../features/auth/PolicyPage'), 'PolicyPage');
const OrdersPage = page(() => import('../features/orders/OrdersPage'), 'OrdersPage');
const NewOrderPage = page(() => import('../features/orders/NewOrderPage'), 'NewOrderPage');
const OrderDetailPage = page(() => import('../features/orders/OrderDetailPage'), 'OrderDetailPage');

function Lazy({ Page }: { Page: LazyExoticComponent<ComponentType> }) {
  return (
    <Suspense
      fallback={
        <p role="status" className="p-8 text-ink-2">
          Cargando…
        </p>
      }
    >
      <Page />
    </Suspense>
  );
}

export const router = createBrowserRouter([
  { path: '/', element: <Navigate to="/panel/ordenes" replace /> },
  { path: '/registro', element: <Lazy Page={RegisterPage} /> },
  { path: '/entrar', element: <Lazy Page={LoginPage} /> },
  { path: '/politica-de-datos', element: <Lazy Page={PolicyPage} /> },
  {
    path: '/panel',
    element: <RequireSession />,
    children: [
      { index: true, element: <Navigate to="ordenes" replace /> },
      { path: 'ordenes', element: <Lazy Page={OrdersPage} /> },
      { path: 'ordenes/nueva', element: <Lazy Page={NewOrderPage} /> },
      { path: 'ordenes/:id', element: <Lazy Page={OrderDetailPage} /> },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]);
