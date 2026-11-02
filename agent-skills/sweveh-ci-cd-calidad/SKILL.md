---
name: sweveh-ci-cd-calidad
description: Pipeline de SWEVEH con GitHub Actions, SonarQube Cloud, Snyk, govulncheck y gitleaks, más despliegue (Fly.io o Cloud Run, pendiente AQ-01), entornos, ramas protegidas y qué hacer cuando el pipeline falla. Úsala al tocar workflows, calidad de código, dependencias, Docker o despliegues.
---

# CI/CD y calidad

Fuente: `docs/06` §12–13 y `docs/02` R-11, R-14, §7, §9. El uso de **GitHub Actions + SonarQube + Snyk** es decisión del equipo; el resto (gitleaks, govulncheck, Fly.io/Cloud Run) es `Propuesta`. **AQ-01 (Fly.io vs Cloud Run) es ⛔ pendiente**: diseña el despliegue como contenedor portable y no cierres la plataforma sin el equipo. Referencias: https://docs.github.com/actions · https://docs.github.com/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions · https://docs.sonarsource.com/sonarqube-cloud/ · https://docs.snyk.io/ · https://go.dev/doc/security/vuln/

**Límites y precios de planes gratuitos (Snyk, SonarQube Cloud, Fly.io, Supabase):** `Sin dato` hasta verificarlos en la página oficial vigente el día que se vayan a usar; si un límite bloquea el pipeline, avisa (docs/02 §3.3 punto 3), no lo ocultes.

## 1. Un solo comando local = el pipeline

`make ci` ejecuta lo mismo que el pipeline: generación sin diferencias, lint (golangci-lint con `depguard`, ESLint), tipos (`tsc --noEmit`), pruebas (`go test -race -cover`, Vitest), govulncheck y gitleaks. Si pasa local, pasa en CI; si no, es un bug de paridad.

## 2. Workflow de PR (esqueleto)

```yaml
# .github/workflows/ci.yml
name: ci
on:
  pull_request:
  push: { branches: [main] }
permissions: { contents: read }          # mínimo; sube por job solo si hace falta
concurrency: { group: ci-${{ github.ref }}, cancel-in-progress: true }

jobs:
  backend:
    runs-on: ubuntu-24.04
    services:
      postgres:
        image: postgres:16            # versión alineada con Supabase (verificar)
        env: { POSTGRES_PASSWORD: ci }
        options: >-
          --health-cmd "pg_isready" --health-interval 5s --health-retries 10
        ports: ["5432:5432"]
    steps:
      - uses: actions/checkout@<SHA-completo>        # fijar por SHA (comentario con la versión)
      - uses: actions/setup-go@<SHA-completo>
        with: { go-version-file: go.mod, cache: true }
      - run: make gen && git diff --exit-code         # sin diferencias de código generado
      - run: make lint
      - run: go test -race -coverprofile=cover.out ./...
      - run: go run golang.org/x/vuln/cmd/govulncheck@<versión-fijada> ./...
      - uses: actions/upload-artifact@<SHA-completo>
        with: { name: coverage, path: cover.out }

  frontend:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<SHA-completo>
      - uses: actions/setup-node@<SHA-completo>
        with: { node-version-file: web/.nvmrc, cache: npm, cache-dependency-path: web/package-lock.json }
      - run: npm ci --prefix web
      - run: npm run lint --prefix web && npm run typecheck --prefix web && npm test --prefix web -- --run
      - run: npm run build --prefix web

  security:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<SHA-completo>
        with: { fetch-depth: 0 }
      - uses: gitleaks/gitleaks-action@<SHA-completo>   # verificar licencia/uso en repos de organización
      # Snyk SCA (dependencias) en PR; Container/Code según límites del plan:
      # - uses: snyk/actions/golang@<SHA-completo>  con SNYK_TOKEN (secreto del repo)
      # SonarQube Cloud: análisis con cobertura (SONAR_TOKEN)
```
- **Todas las acciones fijadas por SHA** completo (con comentario de la versión), nunca por etiqueta móvil; Dependabot/Renovate las actualiza vía PR.
- `permissions` mínimos por job; `GITHUB_TOKEN` de solo lectura por defecto.
- PR desde forks: no exponer secretos; los jobs con secretos corren en `push`/`workflow_run` controlado.
- Secretos en GitHub (Environments con revisores para producción); **nunca** en logs (`::add-mask::`) ni en el repo.
- Cache de módulos de Go/npm para velocidad, sin cachear secretos.

## 3. Calidad con SonarQube Cloud

- Cobertura importada desde `cover.out` (convertida al formato soportado) y LCOV de Vitest.
- **Quality Gate** sobre *código nuevo* (new code): cobertura ≥ 80 %, 0 *bugs*/vulnerabilidades nuevos, duplicación baja, *security hotspots* revisados. Los umbrales exactos del gate son `Propuesta`; se registran en el repo (`sonar-project.properties`).
- Exclusiones justificadas: código generado (`sqlcgen`, cliente OpenAPI), `cmd/`, migraciones.
- Un hallazgo se corrige o se marca falso positivo con justificación y fecha de revisión en el mismo PR (docs/02 §7).

## 4. Seguridad con Snyk (y complementos)

Prioridad si hay límites de plan (docs/02 §7): **SCA en PR → Container en `main` → Code (SAST) en `main`**.
- Umbral de fallo: severidad **alta/crítica** alcanzable. Excepciones con `.snyk` (política) con motivo y vencimiento.
- `govulncheck` complementa a Snyk para Go (análisis de alcance real de la vulnerabilidad).
- `gitleaks` en CI y como *pre-commit* opcional. Si se detecta un secreto: **rotar primero**, limpiar historial después.
- Imagen de contenedor: base mínima (distroless), usuario sin root, sin herramientas de depuración, escaneada antes de publicar.
- SBOM y firma de imágenes: deseables (`Propuesta`), no requeridos para el MVP.

## 5. Despliegue

- Imagen única multi-etapa (build → runtime mínimo); `cmd/api` y `cmd/worker` como procesos separados de la misma imagen o dos imágenes (decisión de bajo impacto).
- Entornos: `local` → `staging` (datos **Ficticio**) → `producción`. **El agente no despliega a producción** (docs/02 §3.3 punto 7); abre el PR y el equipo decide.
- Despliegue con *rollout* gradual y *health checks* (`/healthz` liveness, `/readyz` readiness incluyendo BD); *rollback* documentado y probado.
- Migraciones antes del despliegue (rol `sweveh_migrator`), compatibles hacia atrás (`sweveh-db-migraciones-sqlc`).
- Autenticación de CI en la nube con **OIDC** (sin claves de larga vida), cuando la plataforma lo permita.
- Configuración por variables de entorno (12-factor), validadas al arrancar; `.env.example` sin valores reales.
- Dominio y TLS gestionados por la plataforma; HSTS en la API.
- Front: servido por la API (mismo origen, ADR) o CDN con la misma política de cabeceras; decisión según AQ-01.

## 6. Ramas y revisión (R-14)

`main` protegida: PR obligatorio, revisión de **la otra persona** (Isaac ↔ Pablo), verificaciones obligatorias (backend, frontend, security, Sonar quality gate), historial lineal, sin *force-push*. Commits en español, imperativo, con id de HU. Dependabot/Renovate para dependencias con agrupación semanal.

## 7. Qué hacer si el pipeline falla (docs/02 §7)

1. Reproducir en local con el mismo comando.
2. Corregir la causa; **jamás** desactivar la prueba, bajar un umbral o añadir `|| true`.
3. Fallo de herramienta externa (límite de plan, caída): avisar con evidencia; no ocultar.
4. Falso positivo: ignorar con justificación y vencimiento en el mismo PR.
5. Pipeline lento: paralelizar y cachear antes de quitar comprobaciones.

## 8. Checklist para cambiar el pipeline

- [ ] Acciones por SHA, permisos mínimos, secretos fuera del código.
- [ ] Paridad con `make ci`.
- [ ] Tiempo total razonable (objetivo `Propuesta`: < 10 min en PR).
- [ ] Sin dependencia de un plan de pago no aprobado.
- [ ] Cambio documentado en `docs/05` (comandos) o ADR si es estructural.
