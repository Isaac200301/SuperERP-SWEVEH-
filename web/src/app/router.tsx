import { lazy, Suspense, type ComponentType, type LazyExoticComponent } from 'react';
import { createBrowserRouter, Navigate } from 'react-router';

import { RequireFeature } from './RequireFeature';
import { RequireSession } from './RequireSession';

// División de código por ruta (skill sweveh-frontend-react §11): cada pantalla se baja al abrirla.
const page = <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K): LazyExoticComponent<ComponentType> =>
  lazy(async () => ({ default: (await load())[name] }));

const RegisterPage = page(() => import('../features/auth/RegisterPage'), 'RegisterPage');
const LoginPage = page(() => import('../features/auth/LoginPage'), 'LoginPage');
const PolicyPage = page(() => import('../features/auth/PolicyPage'), 'PolicyPage');
const OrdersPage = page(() => import('../features/orders/OrdersPage'), 'OrdersPage');
const CustomersPage = page(() => import('../features/customers/CustomersPage'), 'CustomersPage');
const NewOrderPage = page(() => import('../features/orders/NewOrderPage'), 'NewOrderPage');
const OrderDetailPage = page(() => import('../features/orders/OrderDetailPage'), 'OrderDetailPage');
const FloorPage = page(() => import('../features/orders/FloorPage'), 'FloorPage');
const InboxPage = page(() => import('../features/inbox/InboxPage'), 'InboxPage');
const ConversationsPage = page(() => import('../features/messaging/ConversationsPage'), 'ConversationsPage');
const ConversationPage = page(() => import('../features/messaging/ConversationsPage'), 'ConversationPage');
const SimulatorPage = page(() => import('../features/messaging/SimulatorPage'), 'SimulatorPage');
const PlatformPage = page(() => import('../features/platform/PlatformPage'), 'PlatformPage');
const AutomationsPage = page(() => import('../features/automations/AutomationsPage'), 'AutomationsPage');
const DashboardPage = page(() => import('../features/dashboard/DashboardPage'), 'DashboardPage');
const ServicesPage = page(() => import('../features/catalog/ServicesPage'), 'ServicesPage');
const PartsPage = page(() => import('../features/inventory/PartsPage'), 'PartsPage');
const TeamPage = page(() => import('../features/team/TeamPage'), 'TeamPage');
const WorkshopPage = page(() => import('../features/workshop/WorkshopPage'), 'WorkshopPage');
const OnboardingPage = page(() => import('../features/workshop/OnboardingPage'), 'OnboardingPage');
const MorePage = page(() => import('../features/panel/MorePage'), 'MorePage');
const ChangePinPage = page(() => import('../features/auth/ChangePinPage'), 'ChangePinPage');
const AssistantPage = page(() => import('../features/assistant/AssistantPage'), 'AssistantPage');
const ReceivePage = page(() => import('../features/orders/FloorMobile'), 'ReceivePage');

/** Pantalla de una función que se puede apagar (V-01, V-02). */
function Gated({ funcion, Page }: { funcion: 'whatsapp' | 'plataforma'; Page: LazyExoticComponent<ComponentType> }) {
  return (
    <RequireFeature funcion={funcion}>
      <Lazy Page={Page} />
    </RequireFeature>
  );
}

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
      { path: 'piso', element: <Lazy Page={FloorPage} /> },
      // Recibir desde el piso (mockup PisoChecklist): la próxima por recibir o una orden.
      { path: 'piso/recibir', element: <Lazy Page={ReceivePage} /> },
      { path: 'piso/recibir/:id', element: <Lazy Page={ReceivePage} /> },
      { path: 'clientes', element: <Lazy Page={CustomersPage} /> },
      { path: 'por-revisar', element: <Gated funcion="whatsapp" Page={InboxPage} /> },
      { path: 'conversaciones', element: <Gated funcion="whatsapp" Page={ConversationsPage} /> },
      { path: 'conversaciones/:id', element: <Gated funcion="whatsapp" Page={ConversationPage} /> },
      { path: 'automatizaciones', element: <Gated funcion="whatsapp" Page={AutomationsPage} /> },
      { path: 'tablero', element: <Lazy Page={DashboardPage} /> },
      { path: 'servicios', element: <Lazy Page={ServicesPage} /> },
      { path: 'repuestos', element: <Lazy Page={PartsPage} /> },
      { path: 'catalogo', element: <Navigate to="/panel/repuestos" replace /> },
      { path: 'equipo', element: <Lazy Page={TeamPage} /> },
      { path: 'taller', element: <Lazy Page={WorkshopPage} /> },
      { path: 'whatsapp-prueba', element: <Gated funcion="whatsapp" Page={SimulatorPage} /> },
      // HU-104: escoger el PIN propio (obligatorio si se entró con un PIN temporal).
      { path: 'cambiar-pin', element: <Lazy Page={ChangePinPage} /> },
      // V-11: el asistente del piso (R8); por ahora el chat vacío.
      { path: 'asistente', element: <Lazy Page={AssistantPage} /> },
      { path: 'arranque', element: <Lazy Page={OnboardingPage} /> },
      { path: 'plataforma', element: <Gated funcion="plataforma" Page={PlatformPage} /> },
      // Celular: "Taller" / "Más" / "Yo" de la barra inferior (mockups móviles v1).
      { path: 'mas', element: <Lazy Page={MorePage} /> },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]);
