# Producto y contexto: qué es SWEVEH y por qué existe

> Lectura obligatoria antes de tocar código. Fuente: memoria y documentos del proyecto hasta el 1-nov-2026. Etiquetas de certeza en `docs/02-reglas-de-decision.md`, sección 2.

## 1. En una frase

**SWEVEH (se pronuncia "Suebe"), *by WEVEH Group*, es la entrada fácil de los talleres de carros y motos a un sistema confiable con automatizaciones reales.** Es una suscripción tipo "Netflix": el taller se registra solo en menos de 5 minutos y recibe todo listo (parámetros, dashboard, gestión básica, automatizaciones que se encienden con un interruptor).

## 2. Personas y equipo

- **Isaac Cano Orozco** y **Pablo Luis Rodríguez Burgos**, estudiantes de Ingeniería de Sistemas, desarrolladores y dueños del proyecto (WEVEH Group). Ambos comparten sesión y son los únicos que aprueban cambios (ver reglas de decisión).
- Rol "Administrador del ERP" dentro del producto = Isaac y Pablo.
- **TecniElectronic** sigue siendo un compromiso ya prometido, aparte de SWEVEH.

## 3. El problema que resuelve (y el que NO)

**Resuelve:** la **barrera de entrada de las pymes —empezando por los talleres— a un sistema y a la IA/automatizaciones**: costo, conocimiento técnico, agencias caras o software a la medida. Muchos talleres pequeños no tienen ningún sistema que respalde lo que le hacen al vehículo.

**Es parte (no el único) del valor:** la **confianza taller–cliente** (fotos, checklist, aprobación por WhatsApp, historial por placa).

**No es:**
- No es un ERP ni compite con los grandes ERP. No se vende como ERP.
- No es "solo IA" ni "el agente de WhatsApp". **El agente de WhatsApp es el demo** de cómo se automatiza un proceso; habrá muchas más automatizaciones.
- No es la app B2C de mantenimiento para el usuario común (esa era WEVEH, se dejó).

**Diferenciador (aclarado por el equipo el 30-oct-2026):** entrada fácil a un sistema confiable con automatizaciones reales de procesos existentes (comerciales, de servicio, etc.). No reducirlo a "IA" ni a "WhatsApp".

## 4. De WEVEH a SWEVEH (historia corta)

WEVEH (app de mantenimiento para el usuario común) se dejó porque el problema era difuso, dependía de que el usuario interactuara y fuera honesto, y dependía por completo de la IA como "asistente mecánico". Con el feedback en campo vieron que sí existe el problema de confianza usuario–mecánico y que muchos talleres pequeños no tienen sistema ni acceso a IA. SWEVEH es el cambio de enfoque: el taller es quien paga la suscripción y tiene un dolor claro.

## 5. Decisiones del equipo (DE-xx): no se tocan sin permiso

| ID | Decisión | Fecha |
|---|---|---|
| DE-01 | Nombre: **SWEVEH**, "by WEVEH Group" | 29-oct |
| DE-02 | Modelo: **suscripción** con dashboard, KPIs, integraciones y automatizaciones, muy fácil de configurar | 29-oct |
| DE-03 | Nicho: **talleres de carros y de motos** (el vehículo puede ser moto o carro) | 31-oct |
| DE-04 | MVP: mejorar la relación con el cliente + **respaldo de todo lo que se le hace al vehículo**, parametrizado e integrado en una app | 29-oct |
| DE-05 | No se vende como ERP; no competir con los grandes ERP | 29/30-oct |
| DE-06 | El agente de WhatsApp es el **demo** de la automatización, no el producto completo | 30-oct |
| DE-07 | **Solo WhatsApp** en el MVP (sin Instagram/Facebook) | 27-oct |
| DE-08 | **Multi-tenant**; suscripción por plan y usuarios | 27-oct |
| DE-09 | **Cliente multicanal con identidad única** por celular, correo o ID | 27-oct |
| DE-10 | Facturación/contabilidad: **integrar Siigo o Alegra, no construir** | 27-oct |
| DE-11 | **Datos personales con autorización previa** (Ley 1581) | 27-oct |
| DE-12 | IA: **agente autónomo con límites**, borradores, reglas en código, salida a humano; **capa propia multi-proveedor** con costo por conversación | 27-oct |
| DE-13 | Precio: prueba gratis, luego plan fijo con bolsa de conversaciones (hay que traducirla a mensajes: Meta cobra por mensaje) | 27-oct |
| DE-14 | **Implementación acompañada** (migrar desde CSV, fotos de cuadernos) | 27-oct |
| DE-15 | **Asistente** que guía al dueño y sirve a cliente y mecánico (qué hace primero: por decidir, D-06/Q-15) | 29-oct |
| DE-16 | **Sin VIN** al registrar vehículos | 1-nov |
| DE-17 | Todo vehículo tiene **motor con cilindraje y turbo** | 1-nov |
| DE-18 | **HU-74** (medidor de mensajes y costo de IA) es del Administrador del ERP: el costo de IA va dentro de la suscripción mensual | 1-nov |
| DE-19 | **HU-50 cancelada**: el ERP no recuerda técnico-mecánica | 1-nov |
| DE-20 | **Todas las HU son Must**: todo entra en el MVP | 1-nov |
| DE-21 | Stack: **Go**, **Supabase**, **React + TypeScript + Vite + Tailwind CSS**, **JWT + RBAC + cookies seguras**, **GitHub Actions + SonarQube + Snyk** | 1-nov |
| DE-22 | Arquitectura: **monolito modular** con capas internas **hexagonales**, con seguridad, observabilidad, auditoría, resiliencia, control de concurrencia, infraestructura y despliegue | 1-nov |
| DE-23 | Regla de trabajo: **nada de suposiciones sin respaldo**; fuente web (URL) o `Sin dato` | 29-oct |

## 6. Estado del proyecto (1-nov-2026)

- **Fase:** pasó de ideación a **preparación de la construcción**. Aún no existe código de la aplicación. El README del repositorio todavía dice "ideación"; actualizarlo cuando arranque el Sprint 0.
- **Siguiente paso del equipo (README):** "Determinar demo y construir". **No hay fecha de demo ni dedicación semanal** (Q-26/AQ-12, `Sin dato`): el plan se ordena por dependencias, no por calendario.
- **Validación en curso:** encuesta a pymes de talleres (desde 30-oct) y concierge de 10 días hábiles con 3–5 talleres con preventa (≥2 que paguen confirma). Supuestos pendientes: S2 (la barrera es el esfuerzo, no solo el precio), S5 (WhatsApp les quita tiempo), S6 (ya pagan algo mensual).

## 7. La demo (lo que el MVP debe poder mostrar)

Demo de ~15 minutos (hasta 16 con la escena 5-M de moto; límite 17 en CA-12), 10 escenas, con el **taller ficticio Servitaller El Pistón** (Medellín): dueño Hernán, mecánico Jhon, recepción Yesica, clientes Laura (carro) y Sebastián (moto). Se usa el **número de prueba de Meta** (5 destinatarios verificados). Detalle del guion, datos y criterios de aceptación en `docs/03-mvp-requisitos.md` (secciones 7 y 8).

La escena clave que Meta no puede hacer: **responder "¿ya está mi carro?" con el estado real de la orden** (AU-05).

## 8. Competencia (para no reinventar ni confundirse)

| Competidor | Qué es | Dónde gana SWEVEH / qué evitar |
|---|---|---|
| **Kreski** (CO) | ERP horizontal para pymes con vertical de talleres: orden por placa, recepción con fotos y firma, aprobación por ítem por WhatsApp, IA **interna**. $300k–$3M/mes. "Kreski Copilot" es su método de implementación, **no** una IA | Gana en entrada fácil y en IA de cara al cliente final. No competir en amplitud de ERP |
| **Tuulapp** (CO) | Gestión ligera de taller: orden en 8 etapas, fotos, aprobación por WhatsApp/SMS, inventario; sin IA en el taller; precio oculto, trimestre/año; app B2C **Pitzon** | El respaldo (fotos + aprobación) **ya no es diferencial** frente a ellos. Gana: agente que contesta, pago mensual, precio visible, todo incluido |
| **Pitz** (México; CO inicios 2027) | "El Chalán" (IA) en WhatsApp **para el mecánico**; diagnóstico, cotización, marketplace; 7 días gratis; ≈ MXN 999/mes | No competir en diagnóstico ni marketplace. Gana en lado cliente/dueño y automatizaciones que corren solas |
| **Growtek** (Pereira) | CRM + agentes IA de atención y ventas 24/7 (no talleres) | Mitad comercial cercana; sin órdenes, inventario ni placa |
| **Agente de IA de Meta en WhatsApp Business** (jun-2026) | Aprende de chats, web y catálogo del negocio; genérico | "No conoce el taller" **no** se sostiene; lo defendible es IA **conectada a la orden y al historial del vehículo por placa**. La versión app no documenta conexión a sistemas externos |

Nadie junta **entrada fácil + automatización comercial + automatización de servicio** (huecos H-01 a H-08 del análisis). Dato de contexto: la cifra "9 % de talleres con software" viene de Innocar, casa matriz de Tuulapp (parte interesada).

## 9. Lo visto en campo (28–29 oct, relato del equipo)

- **Metro Frenos:** sistema "Buse" (no identificado), fotos al cliente, cliente espera en sala, aviso por WhatsApp **manual**.
- **Energiteca:** sistema propio de Cali; registra vehículo, piezas y garantía; el cliente no lo ve.
- **Taller del amigo:** sin nada (perfil objetivo).
- Ninguna visita fue a un taller de motos (Q-02).

## 10. Lo que se dejó fuera a propósito (no construir)

Factura electrónica DIAN y contabilidad (Siigo/Alegra después), Instagram/Facebook, diagnóstico con IA, marketplace de repuestos, voz del mecánico, constructor visual de automatizaciones, campañas de marketing, pagos en línea, nómina, varias sedes, app nativa en tiendas, agenda con calendario completo, portal web del cliente con su historial y la conexión del número real del taller (llega con el primer cliente real).

## 11. Datos legales y de plataforma que condicionan el diseño

- **Ley 1581 de 2012 / Decreto 1377 de 2013** (hoy compilado en el Decreto 1074 de 2015): autorización previa, expresa, informada y con prueba; el silencio no es autorización. Taller = responsable, SWEVEH = encargado (concepto SIC 23-394496, `Tercero`).
- **Ley 1480 de 2011, art. 18:** el recibo del bien debe llevar fecha, datos de quien entrega, identificación del bien, servicio, abonos y garantía (Q-06).
- **WhatsApp (Meta):** ventana de 24 h; plantillas aprobadas fuera de ella; desde el 1-oct-2026 los mensajes de servicio se cobran tras 1.000/mes por número; prohíbe IA de propósito general en la API desde 15-ene-2026; botones máx. 3 y 20 caracteres. Plantillas propias y duración del token en el número de prueba: `Sin dato`.
- **Placa:** carro 3 letras + 3 números (Res. 3800 de 1995); moto ABC12D y antiguo ABC12 solo por Wikipedia (Q-01). **RTM:** motos primera revisión a los 2 años, carros a los 5. Las motos son el 63 % del parque (RUNT 2025).

## 12. Documentos fuente originales (artefactos del proyecto)

| Documento | URL |
|---|---|
| MVP SWEVEH (alcance, módulos, RF/RNF, guion, D-01..D-13) | https://claude.ai/artifact/9Cbp2G4mjX6rkVbJyUjCcq |
| Historias de usuario (v3) | https://claude.ai/artifact/XENupGcm9WgWp9aKvb4rYy |
| Arquitectura SWEVEH (Claude Doc) | https://claude.ai/artifact/LUQHmSChKP37B22XQAAgXU |
| SWEVEH by WEVEH Group (qué es, cómo sobresale) | https://claude.ai/artifact/8CN29idhd89BHc5jvja1ur |
| Arranque SWEVEH (registro, migración, WhatsApp) | https://claude.ai/artifact/Hg5y3kJVHUstHMuTrz5kdv |
| Huecos de la competencia | https://claude.ai/artifact/VeGd6DRH1ykSTWeRhJZcAz |
| Ejercicios MakersXver+ | https://claude.ai/artifact/6MieucEHoZRqSMmkkSj3ZX |
| Entrada fácil SWEVEH | https://claude.ai/artifact/MsDkj1f7dLpQESFksi1cAe |
| Huecos en talleres | https://claude.ai/artifact/NStvE8AbG8yYRztkP5E7PN |

Las copias de trabajo de estos documentos están en `docs/` y son las que el agente debe usar; los enlaces son el original por si hay que contrastar.
