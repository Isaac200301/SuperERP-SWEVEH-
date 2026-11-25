# ADR-0037 · Entrada con celular y PIN

- **Estado:** Aceptado para la entrega del 27-nov (decisión de Pablo, V-03, 24-nov-2026); **Isaac debe revisarlo en el PR**
- **Fecha:** 2026-11-25
- **Quién decide / aprueba:** Pablo (rango 1 de `docs/02`); revisión de Isaac
- **Cambia:** "identidad por celular + OTP" (decisión `Equipo` de `docs/01` y HU-01) y ADR-0034 (código por WhatsApp)
- **Relacionado con:** HU-104 (nueva), HU-01, HU-02, HU-80, docs/06 §6, Q-40, Q-43

## Contexto
El código de entrada salía por el WhatsApp de prueba (WAHA, ADR-0034). WAHA se cayó varias veces (la sesión se cierra y el QR no vuelve, bitácora del 11-nov) y el número vinculado era personal. Para la entrega del 27-nov WhatsApp queda apagado (V-01), así que no hay por dónde mandar un código. Pablo pidió entrar con el celular y un PIN de 4 números elegido por la persona, sin verificar el número.

## Decisión
- **`AUTH_MODE=pin`** (por defecto). Con `otp` sigue vivo el flujo de antes (HU-01): no se borra nada. Las rutas se montan según el modo, no las dos: `/auth/register`, `/auth/login`, `/auth/pin` y `/users/{id}/pin-reset` con PIN; `/auth/otp*` con código. En el contrato van marcadas con `x-feature: auth-pin | auth-otp`.
- **PIN:** 4 dígitos; se rechazan los fáciles (repetidos y escaleras, lista corta en `identity/domain/pin.go`, igual en la web).
- **Hash:** argon2id con sal por usuario y un pepper del servidor (`PIN_PEPPER`; si falta, `OTP_PEPPER`; ≥ 32 caracteres fuera de local), mezclado con HMAC-SHA256 antes del argon2. Parámetros de OWASP para argon2id: m = 19 MiB, t = 2, p = 1 ([OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)), configurables. Formato PHC en `identity.usuario.pin_hash` (migración 00032). Comparación en tiempo constante. Dependencia nueva directa: `golang.org/x/crypto` v0.59.0 (BSD-3, del equipo de Go; ya estaba como indirecta).
- **Intentos:** 5 fallos (`PIN_MAX_INTENTOS`) bloquean el celular 15 minutos (`PIN_BLOQUEO`). El conteo es **por celular, exista o no la cuenta**, en la misma tabla y regla de HU-01 (`identity.otp_bloqueo`, fila bloqueada con `FOR UPDATE`, R-07). Encima va el límite por IP que ya existía en el login.
- **Sin esperar al argon2 con la base tomada:** el ingreso reserva el intento en una transacción corta (fila del celular con `FOR UPDATE`, el intento cuenta como fallo), compara el hash **fuera** de la transacción y, si acierta, pone el contador en cero en otra transacción corta. Nunca hay más de 5 intentos en vuelo y ninguna conexión del pool espera al hash. Un semáforo (máximo `max(2, CPU)` argon2 a la vez) evita que muchos ingresos simultáneos llenen la memoria (hallazgo del revisor).
- **El PIN es de la persona y sirve en todos sus talleres:** el dueño de un taller solo pone PIN temporal a alguien nuevo, y no restablece el de quien también trabaja en otro taller (`pin_de_otro_taller`); eso lo hace la persona o un administrador con `make pin-reset`. Sin esto, el dueño de A podía agregar al dueño de B como mecánico, restablecerle el PIN y entrar como dueño de B (hallazgo del revisor, `TestPIN_CannotTakeOverAnotherWorkshopsAccount`).
- **Mismo error** para un celular sin cuenta y un PIN equivocado ("Celular o PIN incorrectos. Te quedan N intentos…"). Sin cuenta se hace el mismo trabajo de argon2 contra un hash de relleno, para que tarde lo mismo.
- **Sesión:** se mantienen el JWT ES256 de 10 minutos y el refresh rotativo con detección de reutilización (R-05). Solo cambia el TTL del refresh: `SESSION_REFRESH_TTL` = 720 h (unos 30 días, el máximo absoluto de la sesión ya era 30 días).
- **Equipo:** el dueño agrega a alguien con un **PIN temporal** que le dice de palabra (`pin_cambiar = true`); al entrar, la pantalla obliga a escoger uno propio. "Restablecer PIN" (solo `user:manage`) pone otro temporal, desbloquea el celular y cierra sus sesiones. Si el dueño olvida el suyo: `make pin-reset CELULAR=… PIN_NUEVO=…` con el rol de migraciones (Q-40).
- **Registro (V-04):** celular, nombre del taller, PIN (dos veces) y la autorización de datos (HU-02, misma prueba de siempre). El nombre de la persona no se pide: queda "Dueño" y se cambia en Equipo (Q-43).
- **Bitácora** sin el PIN: `usuario.registrar` (con `conPin`), `cuenta.ingreso`, `cuenta.ingreso_fallido`, `cuenta.bloqueo`, `cuenta.pin_cambio`, `cuenta.pin_reset`. Para un celular sin cuenta no hay taller donde anotar el fallo: solo cuenta el intento.

## Alternativas descartadas
- **OTP por SMS:** cuesta por mensaje y no hay proveedor contratado (Q-13, AQ-03; gastar requiere al equipo).
- **Correo con enlace mágico:** los dueños de taller no usan correo a diario (`docs/01`, "Sin correo y sin contraseña").
- **Passkeys (WebAuthn):** lo más seguro, pero no todos los celulares de los talleres lo soportan bien y no cabe en la entrega.
- **Contraseña libre:** más difícil de recordar en el piso que 4 números.

## Consecuencias
- **El celular no se verifica:** cualquiera puede crear una cuenta con un número ajeno. Riesgo aceptado para el piloto; antes de producción hay que verificarlo (Q-40).
- 10.000 combinaciones: lo que protege es el bloqueo y el hash lento. Con 5 intentos cada 15 minutos por celular, probarlos todos toma unos 21 días sin parar (10.000 / 5 × 15 min) y en promedio la mitad; el límite por IP lo frena más.
- Las cuentas creadas con código (antes del 25-nov) no tienen PIN: el dueño les pone uno temporal desde Equipo o con `make pin-reset`.

## Evidencia
- Dominio: `TestParsePIN`, `TestLockPolicy` (bloqueo en el 5.º fallo y no en el 4.º).
- Adaptador: `TestHashAndVerify` (formato PHC, sal distinta, el pepper importa, hash vacío o dañado da false).
- HTTP (`internal/server/pin_test.go`): registro → ingreso → renovación con refresh de 30 días; validaciones del registro; error idéntico con y sin cuenta; bloqueo y desbloqueo con el reloj controlado; el PIN y el celular no salen en los logs; 10 fallos a la vez se cuentan todos; PIN temporal, cambio, restablecer solo con `user:manage` (403 al mecánico) y no a otro taller.
- `TestRouterHidesFeaturesThatAreOff`: con `AUTH_MODE=pin` no existe `/auth/otp`.
