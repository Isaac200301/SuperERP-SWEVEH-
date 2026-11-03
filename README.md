# SWEVEH

> *by WEVEH Group* 

**SWEVEH es la entrada fácil de los talleres a un sistema confiable con automatizaciones reales.**

## ¿Qué problema resuelve?

Muchos talleres pequeños no tienen un sistema que gestione sus recursos ni respalde lo que le hacen al vehículo. Sienten que la barrera de entrada a esos sistemas es muy alta: costos, conocimiento técnico o la necesidad de un software a la medida. Tampoco acceden a la IA ni a las automatizaciones, porque exigen conocimientos y agencias costosas.

## ¿Qué es?

Una plataforma por suscripción (modelo tipo "Netflix") pensada para talleres. Con un registro de empresa muy sencillo, el taller recibe todo listo:

- **Parámetros preconfigurados** para operar desde el primer día.
- **Dashboard claro** con los datos y KPIs del negocio.
- **Gestión básica del taller:** inventario, órdenes de servicio, contactos y leads.
- **Automatizaciones preestablecidas** de procesos comerciales y de servicio, fáciles de activar y personalizar.
- **Asistente** que guía al dueño a entender su negocio y sus servicios, y que sirve tanto al cliente como al mecánico.
- **Respaldo y confianza** entre taller y cliente sobre lo que se le hace al vehículo.

## ¿Qué lo diferencia?

No es un ERP ni compite con los grandes ERP, y tampoco es "solo IA". Su valor es la **entrada fácil a un sistema confiable con automatizaciones reales** de cualquier proceso existente del negocio. El agente de WhatsApp es el primer caso de demostración de cómo se automatiza un proceso, pero habrá muchas más automatizaciones.

## Estado del proyecto

**MVP en construcción** (noviembre de 2026). Todavía no hay entorno de staging ni datos reales: corre en local con datos ficticios.

Lo que ya se puede hacer desde el panel:

| Área | Qué hace | HU |
|---|---|---|
| Cuenta | Crear el taller con el celular y un código; autorización de datos | HU-01, HU-02 |
| Mi taller | Tipo de taller, especialidades, contrato de transmisión y horario semanal | HU-03, HU-08, HU-89 |
| Equipo | Agregar mecánico o recepción por celular, desactivar a quien se va | HU-80, HU-84 |
| Servicios y repuestos | Precios y duración de servicios; repuestos con existencia, mínimo y entradas | HU-07, HU-32, HU-35 |
| Órdenes | Abrir desde la placa (carro o moto, con motor), mover por los 7 estados, entregar | HU-09…11, HU-16, HU-20, HU-25 |
| Recepción | Checklist con kilometraje, combustible, accesorios y fotos por lado; notas | HU-17, HU-21 |
| Cotización | Armar con servicios y repuestos de la lista del taller, enviar y registrar la respuesta; al aprobar se descuenta el inventario | HU-27, HU-29, HU-33 |
| Piso | Las órdenes del mecánico en el celular; el mecánico no ve dinero ni configuración | HU-23, HU-81 |
| Por revisar | Borradores del agente de WhatsApp: enviar, editar, "Lo contesto yo" o descartar; simulador del número de prueba | HU-62…65 |
| Automatizaciones | Catálogo con interruptor y texto editable de los avisos | HU-40…42, HU-46 |
| Tablero | Los KPIs con "qué significa", órdenes por estado y pendientes de hoy | HU-68…73, HU-76 |

Todavía falta conectar el número de prueba real de Meta (hoy se usa el simulador), la conversación dentro de la orden y la bitácora en pantalla. El estado por historia está en [`docs/09-estado-de-construccion.md`](docs/09-estado-de-construccion.md).

**Calidad:** cada PR pasa por GitHub Actions con pruebas de Go con `-race` contra Postgres real, cobertura mínima del 80 % en dominio y aplicación, `golangci-lint`, Vitest, gitleaks, govulncheck, **Snyk** y **SonarQube Cloud**. Se trabaja en la rama `developer` y se integra a `main` por pull request revisado.

## Cómo correrlo en local

Necesitas Docker, Go 1.27, Node 24 y `make` (en Windows, desde Git Bash). Solo se usan datos ficticios.

```sh
cp .env.example .env      # valores de desarrollo; nunca pongas secretos reales
make up migrate           # Postgres en Docker (puerto 54329) y migraciones
make dev-api              # api en http://localhost:8080; el código de entrada sale en esta terminal
make dev-worker           # (otra terminal) despacha los eventos del outbox
npm --prefix web install  # una sola vez
make dev-web              # (otra terminal) panel en http://localhost:5173
```

Abre `http://localhost:5173/registro`, crea la cuenta y copia el código que imprime la terminal del api (`[SOLO DESARROLLO]`). En staging y producción el api no arranca sin un proveedor real de códigos.

| Comando | Qué hace |
|---|---|
| `make test` | Pruebas de Go con `-race` contra Postgres real (aislamiento entre talleres y concurrencia incluidos) y Vitest |
| `make cover` | Cobertura de Go; falla si dominio o aplicación quedan por debajo del 80 % |
| `make lint` | `golangci-lint` (con `depguard`), ESLint y `tsc` |
| `make gen` | Regenera las consultas `sqlc` y los tipos TypeScript del contrato `api/openapi.yaml` |
| `make ci` | Lo mismo que el pipeline de GitHub Actions |
| `make image` | Imagen de contenedor (api, worker y migrate, sin root) |

La documentación para construir está en [`docs/`](docs/) y las reglas para agentes en [`CLAUDE.md`](CLAUDE.md).

## Origen

SWEVEH nace de WEVEH, una app de mantenimiento para el usuario común. Tras el feedback en campo, el equipo se enfocó en el taller: es quien paga la suscripción y tiene un dolor claro.

---
Desarrollado por **Isaac Cano Orozco** y **Pablo Luis Rodríguez Burgos** · WEVEH Group