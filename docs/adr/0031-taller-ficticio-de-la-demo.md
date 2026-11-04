# ADR-0031 · Cómo se carga y se reinicia el taller ficticio de la demo

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Equipo; revisión de Isaac y Pablo en el PR
- **Relacionado con:** HU-97, HU-98, docs/03 "El taller ficticio de la demo" y "La demo, escena por escena", CA-12, HU-05, HU-71

## Contexto
La demo corre 10 escenas sobre **Servitaller El Pistón**, que ya debe tener sus datos cargados. HU-97 pide un script que lo deje en el estado inicial para correr la demo dos veces seguidas; HU-98 agrega la moto de Sebastián. Hay dos restricciones del sistema:
- **La bitácora es inmutable**: el trigger `evento_inmutable` impide borrar `audit.evento`. Un taller con historia no se puede borrar.
- **Cada taller está aislado con RLS**, y el api no puede ni listar talleres.

## Decisión
- **La carga va por la API real, en el mismo proceso** (`internal/demo`, `cmd/demo`):
  - usa `server.New` con el rol `sweveh_api`;
  - el reloj está controlado y pone la historia en el pasado (órdenes de hace 240 días, entregas de los últimos 30 días, cotización enviada hace 26 h);
  - el taller nace por los mismos caminos y validaciones que usa una persona. Por ejemplo, el sistema no deja recibir un vehículo con el motor pendiente.
- **Solo dos cosas van directo a la base**, con el rol dueño de las tablas (`sweveh_migrator`) y siempre sujetas a RLS:
  - **crear el taller y la membresía del dueño**, como hace el registro, reutilizando `tenancy.Workshops.Create` y el store de identity;
  - **fijar el contador de órdenes**, para que los números sean los del guion (OS-0031, OS-0038, OS-0040 a OS-0044; Laura recibe la OS-0045 en vivo).
- **Reiniciar archiva y no borra:**
  - las membresías del taller anterior se desactivan, así que nadie entra;
  - su canal de WhatsApp de prueba también, y el número queda libre;
  - su nombre queda "… (demo archivada AAAA-MM-DD HH:MM)";
  - se carga un taller nuevo, y el dueño, al entrar, cae en él porque es su única membresía activa;
  - los códigos de entrada ya usados del dueño de la demo se limpian, porque el reloj de la carga empieza en el pasado.
- **Nada de lo cargado sale por WhatsApp:** los eventos de la historia quedan procesados en el outbox. La recepción vuelve a pedir fotos y kilometraje al terminar, para que la escena 5 se vea completa.
- **Nunca en producción:** `cmd/demo` se niega con `APP_ENV=production`.
- **`make seed-demo`** carga el taller y no hace nada si ya hay uno activo. **`make reset-demo`** archiva el anterior y carga uno nuevo, en unos 3,5 s.

### Lo que el guion no fijaba (decidido aquí)

| Punto | Decisión | Por qué |
|---|---|---|
| Servicios y motos | El taller atiende carros y motos (HU-98). Los servicios del guion quedan para ambos, salvo alineación y amortiguadores (solo carro) | HU-98 |
| OS-0041 | Moto de Sebastián (XVN43F) "Por recibir" con cita hoy a las 10:00 y Jhon asignado | Escena 5-M y AU-06 |
| OS-0042 | Kia Picanto de Carlos "En diagnóstico" (tercera visita) | El guion no dice cuál es |
| Las 20 órdenes de ejemplo | Sobre los vehículos de Andrés, Diego, Marta y Sebastián, no de Carlos | Así Carlos conserva sus 2 visitas entregadas y quedan los 5 clientes del guion |
| Vehículo con "motor pendiente" | Un segundo vehículo de Marta, Chevrolet Aveo 2012 (WMR552), sin órdenes | El sistema no deja recibir un vehículo con el motor pendiente, y todos los del guion tienen órdenes recibidas |
| OS-0043 | Lleva una foto de prueba marcada para el cliente | Para que AU-05 la mande en la escena 7. Es un recuadro gris generado: la demo no usa fotos de vehículos reales |
| Celulares | Por variables de entorno (`DEMO_CELULAR_*`) | Los del equipo no van en el repositorio. Sin ellas se usan números de prueba que no reciben nada |

## Consecuencias
- La demo se ensaya y se repite sin tocar código ni la base a mano (HU-97, CA-12).
- La base acumula un taller archivado por cada reinicio. Son datos ficticios y pocos; si estorban en staging, se borra la base de staging entera.
- `internal/demo` está exento de la regla "entre módulos, solo la fachada" del `arch_test`, igual que `server` y `wire`, porque arma el sistema completo.
- **Falta lo manual:** correr las 10 escenas dos veces y medir que tarden menos de 17 minutos (HU-97, criterio 3). La carga ya no es el cuello de botella.
