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

**Sprint 0 en curso** (noviembre de 2026). Ya funciona en local la primera rebanada: crear la cuenta del taller con el celular y un código, registrar cliente y vehículo (carro o moto, con motor) y abrir una orden con número consecutivo. El detalle por historia está en [`docs/09-estado-de-construccion.md`](docs/09-estado-de-construccion.md).

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