---
name: sweveh-seguridad-auth
description: Autenticación y seguridad de SWEVEH: JWT ES256 de 10 min con refresh opaco rotativo, cookies __Host- HttpOnly SameSite=Strict, CSRF, RBAC recurso:accion con dos realms (taller y Administrador del ERP), anti-IDOR, validación de entrada, secretos, límites de tasa, cabeceras y CSP. Úsala al tocar login, sesiones, permisos, endpoints o cabeceras.
---

# Seguridad y autenticación (R-05)

Fuente: `docs/02` R-05 y `docs/06` §6–7. Cumple OWASP ASVS y las guías de OWASP para sesiones, CSRF y JWT. Referencias: https://owasp.org/www-project-application-security-verification-standard/ · https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html · https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html · https://datatracker.ietf.org/doc/html/rfc8725 (mejores prácticas JWT).

**Estado de decisiones:** JWT, RBAC y cookies seguras = decisión del equipo (ADR-05 a 07). Detalles (ES256, 10 min, refresh rotativo, auth propia celular+OTP) = `Propuesta`. **AQ-04 (auth propia vs Supabase Auth) es ⛔ pendiente**: mantén la identidad detrás de un puerto (`TokenIssuer`, `Authenticator`) y no cierres la decisión sin el equipo.

## 1. Tokens

| Pieza | Valor propuesto | Notas |
|---|---|---|
| Access token | JWT **ES256**, 10 min | claims: `sub`, `taller_id`, `realm`, `roles`, `jti`, `iat`, `exp`, `iss`, `aud` |
| Refresh token | **opaco** (aleatorio 256 bits), rotativo | se guarda **hash** en BD; un uso; detección de reutilización |
| Cookies | `__Host-sweveh_at`, `__Host-sweveh_rt` | `HttpOnly; Secure; SameSite=Strict; Path=/` sin `Domain` |

Validación del JWT en el servidor: algoritmo **fijado** (rechaza `none` y cambios de algoritmo), `iss`, `aud`, `exp`, `nbf`; clave pública por `kid`; reloj con tolerancia pequeña.

```go
tok, err := jwt.ParseWithClaims(raw, &Claims{}, keyFunc,
	jwt.WithValidMethods([]string{"ES256"}),
	jwt.WithIssuer(cfg.Issuer), jwt.WithAudience(cfg.Audience),
	jwt.WithExpirationRequired(), jwt.WithLeeway(5*time.Second))
```

**Rotación del refresh con detección de reutilización:** cada refresh pertenece a una *familia*. Si llega un refresh ya usado, se **revoca toda la familia** y se exige login (posible robo). Registrar el evento en auditoría.

**Logout:** borra cookies y revoca la familia. **Cambio de rol/permisos:** el token vive 10 min; para efecto inmediato, versión de permisos (`perm_ver`) en el claim comparada contra BD en operaciones sensibles.

## 2. Cookies y CSRF

- Mismo origen para front y API (ADR de "mismo origen"): sin CORS abierto. Si algún día hay otro origen, lista blanca explícita, nunca `*` con credenciales.
- `SameSite=Strict` ayuda, pero **no sustituye** CSRF: exige encabezado personalizado (`X-Requested-With: sweveh` o token) **y** valida `Origin`/`Sec-Fetch-Site` en métodos no seguros (POST/PUT/PATCH/DELETE).
- Nunca guardes tokens en `localStorage`/`sessionStorage` ni en el JS.
- Webhook de WhatsApp: no usa cookies ni CSRF; usa firma HMAC (ver `sweveh-whatsapp-ia`).

## 3. RBAC

- Permisos con formato `recurso:accion` (`orders:read`, `orders:transition`, `quotes:approve`, `inventory:adjust`, `kpi:read`…). Los roles son agrupaciones de permisos (6 roles, ver `docs/04`).
- **Dos realms separados:** `taller` y `admin_erp` (HU-74): emisores, rutas y claims distintos; un token de taller jamás sirve en rutas admin.
- Verificación en el **servidor**, por endpoint, con middleware declarativo; el front solo oculta botones.
- Restricciones de negocio reflejadas en permisos: el mecánico **no ve costos ni KPIs de dinero** (RF-21); el agente de IA es un actor con permisos mínimos (puede crear borradores, no aprobar ni ajustar inventario ni cambiar precios).

```go
r.With(authz.Require("orders:transition")).Post("/orders/{id}/transition", h.Transition)
```

- **Anti-IDOR:** además del permiso, comprobar propiedad del objeto (`taller_id`, y asignación si aplica). Recurso de otro taller → **404**.
- Deny by default: ruta sin permiso declarado = test de arquitectura que falla.

## 4. Entrada, salida y datos

- Valida contra OpenAPI en el borde (tipo, longitud, formato) y otra vez en el dominio (invariantes). Tamaño máximo del cuerpo (`http.MaxBytesReader`).
- SQL siempre parametrizado (sqlc); nunca concatenar. `LIKE` con escapes.
- Salidas: no devolver campos internos; `Content-Type` correcto; `application/problem+json` sin trazas.
- Registros (logs) sin PII ni secretos: enmascara celular (`+57300***1234`), nunca token, OTP ni cuerpo de mensajes completo.
- Subidas (fotos del vehículo): valida tipo por contenido, tamaño, renombra, almacena en Supabase Storage con política por `taller_id`, URLs firmadas de corta vida.
- Contraseñas: no aplica en el MVP si el login es celular+OTP (`Propuesta`); si se añaden, argon2id.
- OTP: 6 dígitos, vida corta, intentos limitados, límite por número y por IP; **costo y proveedor del SMS/WhatsApp OTP = `Sin dato`** (no inventar precios).

## 5. Límites de tasa y abuso

Por IP, por usuario y por taller; más estrictos en login/OTP y webhooks; respuesta 429 con `Retry-After`. Cuotas de IA por taller/mes (R-08). Los umbrales son `Propuesta`; hazlos configurables.

## 6. Cabeceras y navegador

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https://*.supabase.co; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'
Strict-Transport-Security: max-age=63072000; includeSubDomains
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(self), geolocation=(), microphone=()
Cross-Origin-Opener-Policy: same-origin
```
Ajusta `img-src`/`connect-src` según el dominio real de Supabase Storage; Tailwind con Vite genera CSS en archivo (sin `unsafe-inline`). Si la PWA necesita cámara para fotos, `camera=(self)`.

## 7. Secretos y configuración

- Solo en el gestor de la plataforma (Fly secrets/GitHub Environments/Supabase); **nunca** en el repo, `.env` versionado, imagen Docker ni logs. Plantilla `.env.example` con valores vacíos.
- Rotación documentada; claves JWT con `kid` para rotar sin caída.
- `gitleaks` en pre-commit/CI; si se filtra un secreto: rotar primero, limpiar después.
- Imagen de contenedor sin root, base mínima (distroless), sistema de archivos de solo lectura cuando sea posible.

## 8. Cadena de suministro

Versiones fijadas (`go.sum`, `package-lock.json`), acciones de GitHub fijadas por **SHA**, `govulncheck`, Snyk (SCA/Container/Code) y SonarQube Cloud en el pipeline (ver `sweveh-ci-cd-calidad`). Dependencias nuevas con runtime se justifican en el PR (docs/02 §9).

## 9. Pruebas de seguridad mínimas por endpoint

- Sin token → 401; token de otro realm → 403/401; sin permiso → 403.
- Recurso de otro taller → 404.
- CSRF: POST sin encabezado o con `Origin` ajeno → rechazado.
- Refresh reutilizado → familia revocada.
- JWT con `alg: none`, firma alterada, `exp` vencido, `aud` incorrecto → rechazado.
- Entradas hostiles (SQLi, XSS, tamaños gigantes) → 400 sin eco del payload.

## 10. Qué NO hacer (nunca sin excepción, `docs/02` §3.4)

Desactivar RLS/CSRF/verificación de firma "para probar"; guardar tokens en JS; loguear secretos; usar `service_role` en la API; comparar firmas con `==`; confiar en `taller_id` del cliente; añadir endpoints sin permiso declarado; pegar secretos reales en el chat, issues o PR.

## 11. Checklist

- [ ] Permiso `recurso:accion` declarado y probado (403 sin él).
- [ ] Propiedad del objeto verificada (404 cruzado).
- [ ] Entrada validada por OpenAPI + dominio.
- [ ] Sin PII/secretos en logs.
- [ ] Cabeceras y cookies conforme a §1–§2.
- [ ] Ningún cambio en AQ-04 sin decisión del equipo.
