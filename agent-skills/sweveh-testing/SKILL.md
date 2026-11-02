---
name: sweveh-testing
description: Estrategia y recetas de pruebas de SWEVEH: pirámide, pruebas de dominio, aplicación con fakes, integración con Postgres real, aislamiento multi-tenant, concurrencia con -race, contrato OpenAPI, adversariales de IA, Vitest/Testing Library y Playwright para la demo. Úsala al escribir o revisar pruebas.
---

# Pruebas en SWEVEH (R-11)

Umbral: cobertura del código **nuevo** ≥ 80 % en dominio y aplicación; `go test -race ./...`; nunca se desactiva una prueba ni se baja un umbral para poner el pipeline en verde (`docs/02` §7). Código generado (`sqlc`, `oapi-codegen`) y `cmd/` se excluyen de la cobertura con justificación.

## 1. Pirámide propuesta

| Nivel | Qué prueba | Herramienta | Velocidad |
|---|---|---|---|
| Unitarias de dominio | invariantes, máquina de estados, value objects | `testing`, tablas | ms |
| Aplicación | casos de uso con fakes de puertos | `testing` + fakes manuales | ms |
| Integración | repositorios sqlc, RLS, outbox, migraciones | Postgres real (contenedor o instancia local) | s |
| Contrato/HTTP | handlers contra OpenAPI, authz, problem+json | `httptest` + validador OpenAPI | s |
| Aislamiento y concurrencia | dos talleres, N goroutines | integración + `-race` | s |
| IA/WhatsApp | adversariales, firma, idempotencia | fakes + `httptest` | s |
| Front unitarias | lógica, hooks, componentes | Vitest + Testing Library | ms |
| E2E | recorrido de la demo (10 escenas) | Playwright | min |

Proporción: muchas pruebas pequeñas rápidas, pocas E2E. Lo que se puede probar en dominio no se prueba por HTTP.

## 2. Dominio: tablas con nombres legibles

```go
func TestOrder_Transition(t *testing.T) {
	tests := []struct {
		name    string
		from, to OrderState
		engine  EngineStatus
		wantErr error
	}{
		{"recibir con motor completo", ToReceive, Received, EngineComplete, nil},
		{"recibir con motor pendiente se bloquea", ToReceive, Received, EnginePending, ErrEngineRequired},
		{"saltar de Recibido a Listo es inválido", Received, Ready, EngineComplete, ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := anOrder(withState(tt.from), withEngine(tt.engine))
			err := o.Transition(tt.to, anActor())
			if !errors.Is(err, tt.wantErr) { t.Fatalf("got %v, want %v", err, tt.wantErr) }
		})
	}
}
```
- Nombres describen **comportamiento**, no implementación.
- Constructores de prueba (`anOrder(opts...)`) en lugar de literales gigantes (Builder, ver `patrones-gof`).
- Reloj, IDs y aleatoriedad inyectados: pruebas deterministas.
- Propiedad: considera `testing/quick` o `rapid` para value objects (placa, dinero) si aporta.

## 3. Aplicación con fakes (no mocks mágicos)

Escribe fakes pequeños que implementan el puerto (`FakeQuotes`, `FakeClock`, `FakeCanal`). Verifica **resultado y efectos observables** (estado guardado, eventos del outbox, auditoría), no el orden interno de llamadas. Usa un mock generado solo cuando el fake sería más complejo que el mock.

## 4. Integración con Postgres real

- Las migraciones se aplican de verdad en cada ejecución (misma ruta que producción).
- Una base/esquema por prueba o transacción con rollback; paralelizable.
- Se prueba con el rol `sweveh_api` (sin `BYPASSRLS`), no con el superusuario: si no, RLS no se está probando.
- Datos **Ficticio** siempre (R-10).

```go
db := testdb.New(t)            // crea esquema, aplica migraciones, rol sweveh_api
tenantA, tenantB := db.NewTenant(t), db.NewTenant(t)
```

## 5. Aislamiento (por cada tabla nueva) y concurrencia

Ver recetas completas en `sweveh-multitenant-rls` §4 y `sweveh-concurrencia-idempotencia` §8. Reglas:
- Aislamiento: BD **y** HTTP, con dos talleres; recurso ajeno → 404.
- Concurrencia: N ≥ 20–50 goroutines, resultado exacto verificable (no "no explota"), con `-race`.
- Evita `time.Sleep` para sincronizar: usa canales, `sync.WaitGroup`, `errgroup` o `eventually` con timeout.

## 6. Contrato y seguridad HTTP

- Validar respuestas reales contra `api/openapi.yaml` (middleware/validador en pruebas).
- Tabla de autorización por endpoint: sin token, token de otro realm, sin permiso, con permiso, recurso de otro taller (ver `sweveh-seguridad-auth` §9).
- Errores siempre `application/problem+json` y sin detalles internos.

## 7. Frontend

- **Vitest + Testing Library:** consulta por rol/etiqueta (como el usuario), no por clases CSS ni estructura. Pruebas de formularios (RHF + Zod), estados de carga/error/vacío, 409 de concurrencia.
- **MSW** (Mock Service Worker) para simular la API en pruebas unitarias; los tipos salen del cliente generado desde OpenAPI para evitar divergencias.
- **Accesibilidad:** `jest-axe`/axe en componentes críticos; pantallas de piso (PWA móvil) con botones grandes y contraste.
- **Playwright:** el recorrido de la demo en navegador móvil y de escritorio; selectores por rol/texto; sin esperas fijas; trazas y capturas en fallo.

## 8. Datos de prueba

- `make seed-demo` crea el taller ficticio **Servitaller El Pistón** (datos **Ficticio**); `make reset-demo` permite repetir la demo (HU-99).
- Placas, celulares y nombres sintéticos (celulares de prueba con prefijo claramente falso; nunca números reales).
- Fábricas con valores válidos por defecto y sobrescritura puntual.

## 9. Pruebas flaky: política

Una prueba inestable es un bug. Reproduce con `-count=100 -race`, encuentra la causa (reloj, orden, red, datos compartidos) y arréglala. Poner `t.Skip` o reintentos automáticos para ocultarla está prohibido sin avisar (R-15).

## 10. Comandos

```
go test -race -cover ./...                 # todo el backend
go test -race -run TestConsumeStock ./internal/inventory/...
go test -race -count=50 ./internal/inventory/...   # caza de flakiness
npm test --prefix web                       # Vitest
make e2e                                    # Playwright
make ci                                     # igual que el pipeline
```

## 11. Checklist

- [ ] Cada criterio de aceptación de la HU tiene prueba nombrada por comportamiento.
- [ ] Dominio probado sin I/O; fakes para puertos.
- [ ] Integración con rol sin `BYPASSRLS`.
- [ ] Aislamiento (2 talleres) y concurrencia si corresponde, con `-race`.
- [ ] Errores y casos límite (vacíos, longitudes, Unicode, zona horaria America/Bogota).
- [ ] Sin dependencias de red externa ni de datos reales.
- [ ] Front: estados de carga/error/vacío y accesibilidad básica.
