package server_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"sweveh/internal/platform/config"
	"sweveh/internal/platform/testdb"
	"sweveh/internal/shared"
)

// Pruebas de R2 (HU-104, V-03, V-04, ADR-0037): entrar con celular y PIN. Datos Ficticio.

// pinAuth es la entrada de la entrega del 27-nov: PIN, 5 fallos, 15 minutos, sesión de 30 días.
var pinAuth = config.Auth{Mode: "pin", MaxFailures: 5, Lock: 15 * time.Minute, RefreshTTL: 720 * time.Hour}

func newPINEnv(t *testing.T) *env {
	t.Helper()
	return newEnvRaw(t, config.API{Env: config.EnvLocal, Auth: pinAuth})
}

const ownerPIN = "2580"

// registerPIN es el paso 1 del registro (V-04): celular, nombre del taller, PIN y la autorización.
func (e *env) registerPIN(phone10, taller string) *client {
	e.t.Helper()
	c := e.client()
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/auth/register", map[string]any{
		"celular": phone10, "nombreTaller": taller, "pin": ownerPIN, "autorizacion": map[string]any{"version": "borrador-sin-validar-2026-11-01", "aceptada": true},
	}))
	if r.body["cuentaNueva"] != true {
		e.t.Fatalf("debía ser cuenta nueva: %v", r.body)
	}
	return c
}

// asTaller corre una consulta con el taller fijado (RLS), como lo haría el servidor (testdb.As + queryRow).
func (e *env) asTaller(tallerID shared.ID, sql string, dest ...any) error {
	return e.db.Tx.Do(testdb.As(tallerID), func(ctx context.Context) error { return queryRow(ctx, sql, dest...) })
}

func (e *env) loginPIN(phone10, pin string) (*client, resp) {
	e.t.Helper()
	c := e.client()
	return c, c.do("POST", "/api/v1/auth/login", map[string]any{"celular": phone10, "pin": pin})
}

// HU-104 y HU-02: registro → entrar → renovar; la sesión dura unos 30 días; el código ya no existe.
func TestPIN_RegisterLoginRefresh(t *testing.T) {
	e := newPINEnv(t)
	owner := e.registerPIN("3105551001", "Servitaller El Pistón")
	me := owner.must(http.StatusOK, owner.do("GET", "/api/v1/me", nil))
	if nested(me.body, "taller", "nombre") != "Servitaller El Pistón" || nested(me.body, "usuario", "nombre") != "Dueño" || me.body["cambiarPin"] != false {
		t.Fatalf("/me: %v", me.body)
	}
	// HU-02: la prueba de la autorización quedó guardada.
	var n int
	if err := e.db.API.QueryRow(t.Context(), `SELECT count(*) FROM identity.autorizacion_titular a JOIN identity.usuario u ON u.id = a.usuario_id
		WHERE u.celular = '+573105551001' AND a.version_politica = 'borrador-sin-validar-2026-11-01' AND a.canal = 'web'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("autorización: %d %v", n, err)
	}

	c, r := e.loginPIN("3105551001", ownerPIN)
	c.must(http.StatusOK, r)
	var refresh *http.Cookie
	for _, k := range (&http.Response{Header: r.header}).Cookies() {
		if strings.Contains(k.Name, "sw_rt") {
			refresh = k
		}
	}
	if refresh == nil || refresh.Expires.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("el refresh debía durar unos 30 días: %+v", refresh)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/auth/refresh", nil))
	c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))

	// Con PIN, las rutas del código no se montan.
	if r := c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": "3105551001"}); r.status != http.StatusNotFound {
		t.Fatalf("/auth/otp con PIN: %d", r.status)
	}
	// El mismo celular no crea una segunda cuenta.
	r = e.client().do("POST", "/api/v1/auth/register", map[string]any{"celular": "3105551001", "nombreTaller": "Otro", "pin": "9173",
		"autorizacion": map[string]any{"aceptada": true}})
	if r.status != http.StatusConflict || r.str("code") != "cuenta_existente" {
		t.Fatalf("cuenta repetida: %d %v", r.status, r.body)
	}
}

// V-04 y HU-02: validaciones del paso 1.
func TestPIN_RegisterValidation(t *testing.T) {
	e := newPINEnv(t)
	c := e.client()
	base := func(over map[string]any) map[string]any {
		b := map[string]any{"celular": "3105551002", "nombreTaller": "Taller Prueba", "pin": "9173", "autorizacion": map[string]any{"aceptada": true}}
		for k, v := range over {
			b[k] = v
		}
		return b
	}
	for name, tc := range map[string]struct {
		body map[string]any
		code string
	}{
		"PIN fácil":        {base(map[string]any{"pin": "1234"}), "pin_facil"},
		"PIN corto":        {base(map[string]any{"pin": "123"}), "pin_formato"},
		"sin autorización": {base(map[string]any{"autorizacion": map[string]any{"aceptada": false}}), "autorizacion_requerida"},
		"política vieja":   {base(map[string]any{"autorizacion": map[string]any{"version": "v0", "aceptada": true}}), "autorizacion_requerida"},
		"sin taller":       {base(map[string]any{"nombreTaller": "  "}), "nombre_taller"},
		"celular malo":     {base(map[string]any{"celular": "31055"}), "celular_invalido"},
	} {
		time.Sleep(5 * time.Millisecond) // que el límite por IP del registro (ráfaga de 5) no se mezcle con la validación
		r := c.do("POST", "/api/v1/auth/register", tc.body)
		if r.status < 400 || r.str("code") != tc.code {
			t.Fatalf("%s: %d %v", name, r.status, r.body)
		}
	}
	// Nada quedó creado.
	var n int
	if err := e.db.API.QueryRow(t.Context(), `SELECT count(*) FROM identity.usuario WHERE celular = '+573105551002'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("usuarios: %d %v", n, err)
	}
}

// V-03: el error es idéntico exista o no el celular; al 5.º fallo (no al 4.º) se bloquea 15 minutos; luego se libera.
// El PIN nunca aparece en los logs.
func TestPIN_SameErrorAndLock(t *testing.T) {
	e := newPINEnv(t)
	tallerID := tallerOf(t, e.registerPIN("3105551003", "Taller Prueba"))

	_, bad := e.loginPIN("3105551003", "9999")
	_, ghost := e.loginPIN("3105551009", "9999") // nadie tiene este celular
	if bad.status != ghost.status || bad.str("code") != "ingreso_fallido" || ghost.str("code") != "ingreso_fallido" ||
		bad.str("detail") != ghost.str("detail") {
		t.Fatalf("debían ser iguales:\n%d %v\n%d %v", bad.status, bad.body, ghost.status, ghost.body)
	}
	for i := 2; i <= 4; i++ {
		if _, r := e.loginPIN("3105551003", "9999"); r.str("code") != "ingreso_fallido" {
			t.Fatalf("fallo %d: %d %v", i, r.status, r.body)
		}
	}
	_, r := e.loginPIN("3105551003", "9999")
	if r.status != http.StatusTooManyRequests || r.str("code") != "celular_bloqueado" || !strings.Contains(r.str("detail"), "15 minutos") {
		t.Fatalf("5.º fallo: %d %v", r.status, r.body)
	}
	// Bloqueado, ni con el PIN bueno.
	if _, r := e.loginPIN("3105551003", ownerPIN); r.status != http.StatusTooManyRequests {
		t.Fatalf("bloqueado con el PIN bueno: %d", r.status)
	}
	e.clk.Advance(16 * time.Minute)
	c, r := e.loginPIN("3105551003", ownerPIN)
	c.must(http.StatusOK, r)

	// Bitácora del taller: los fallos y el bloqueo, sin el PIN.
	var acts string
	if err := e.asTaller(tallerID, `SELECT coalesce(string_agg(DISTINCT accion, ',' ORDER BY accion), '') FROM audit.evento
		WHERE accion LIKE 'cuenta.%' AND coalesce(despues::text, '') NOT LIKE '%9999%'`, &acts); err != nil {
		t.Fatal(err)
	}
	if acts != "cuenta.bloqueo,cuenta.ingreso,cuenta.ingreso_fallido" {
		t.Fatalf("bitácora: %s", acts)
	}
	logs := e.logs.String()
	for _, secret := range []string{ownerPIN, "9999", "3105551003", "3105551009"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("el log tiene un dato prohibido (%s)", secret)
		}
	}
}

// R-07: 10 intentos fallidos a la vez se cuentan todos (la fila del celular se bloquea con FOR UPDATE): 4 fallan
// normal y 6 ya encuentran el celular bloqueado; ninguno se pierde. Van en dos tandas de 5 simultáneos porque el
// límite por IP del login (ráfaga de 5) frena la sexta solicitud a la vez, y eso también está bien.
func TestPIN_ConcurrentFailuresAreAllCounted(t *testing.T) {
	e := newPINEnv(t)
	e.registerPIN("3105551004", "Taller Prueba")
	codes := make(chan string, 10)
	for range 2 {
		var wg sync.WaitGroup
		for range 5 {
			wg.Go(func() {
				_, r := e.loginPIN("3105551004", "9999")
				codes <- r.str("code")
			})
		}
		wg.Wait()
		time.Sleep(50 * time.Millisecond) // que se recargue la ráfaga del límite por IP
	}
	close(codes)
	count := map[string]int{}
	for c := range codes {
		count[c]++
	}
	if count["ingreso_fallido"] != 4 || count["celular_bloqueado"] != 6 {
		t.Fatalf("intentos: %v", count)
	}
}

// HU-80 y HU-104: el dueño agrega a alguien con un PIN temporal y se lo restablece; solo quien tiene user:manage.
// Con el temporal, la persona entra con cambiarPin y escoge uno propio.
func TestPIN_TeamTemporaryPINAndReset(t *testing.T) {
	e := newPINEnv(t)
	owner := e.registerPIN("3105551005", "Taller Prueba")
	tallerID := tallerOf(t, owner)
	// Sin PIN temporal no se puede agregar a alguien nuevo.
	r := owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005551006", "nombre": "Jhon Cardona", "rol": "mecanico"})
	if r.status < 400 || r.str("code") != "pin_formato" {
		t.Fatalf("sin PIN: %d %v", r.status, r.body)
	}
	r = owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005551006", "nombre": "Jhon Cardona", "rol": "mecanico", "pin": "4826"}))
	mecID := r.str("id")

	mec, lr := e.loginPIN("3005551006", "4826")
	lr = mec.must(http.StatusOK, lr)
	if lr.body["cambiarPin"] != true {
		t.Fatalf("debía pedir cambiar el PIN: %v", lr.body)
	}
	// Sin pedir el actual (temporal) escoge el suyo; ya no se le pide cambiarlo.
	mec.must(http.StatusNoContent, mec.do("POST", "/api/v1/auth/pin", map[string]any{"pin": "7391"}))
	if me := mec.must(http.StatusOK, mec.do("GET", "/api/v1/me", nil)); me.body["cambiarPin"] != false {
		t.Fatalf("/me: %v", me.body)
	}
	// Con su PIN propio, cambiarlo pide el actual.
	if r := mec.do("POST", "/api/v1/auth/pin", map[string]any{"pinActual": "0000", "pin": "5173"}); r.str("code") != "pin_actual_equivocado" {
		t.Fatalf("PIN actual malo: %d %v", r.status, r.body)
	}
	// El mecánico no restablece PINes (403 en el servidor).
	if r := mec.do("POST", "/api/v1/users/"+mecID+"/pin-reset", map[string]any{"pin": "6284"}); r.status != http.StatusForbidden {
		t.Fatalf("mecánico restableciendo: %d", r.status)
	}
	// El dueño se lo restablece: el temporal sirve, cierra la sesión que tenía y vuelve a pedir cambiarlo.
	owner.must(http.StatusNoContent, owner.do("POST", "/api/v1/users/"+mecID+"/pin-reset", map[string]any{"pin": "6284"}))
	if r := mec.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("la sesión vieja debía cerrarse: %d", r.status)
	}
	if _, r := e.loginPIN("3005551006", "7391"); r.str("code") != "ingreso_fallido" {
		t.Fatalf("el PIN viejo ya no sirve: %v", r.body)
	}
	_, r = e.loginPIN("3005551006", "6284")
	if r.status != http.StatusOK || r.body["cambiarPin"] != true {
		t.Fatalf("temporal nuevo: %d %v", r.status, r.body)
	}
	// Nadie se restablece a sí mismo por aquí, ni a alguien de otro taller.
	me := owner.must(http.StatusOK, owner.do("GET", "/api/v1/me", nil))
	if r := owner.do("POST", "/api/v1/users/"+nested(me.body, "usuario", "id").(string)+"/pin-reset", map[string]any{"pin": "6284"}); r.status < 400 {
		t.Fatalf("a sí mismo: %d", r.status)
	}
	other := e.registerPIN("3105551007", "Otro Taller")
	if r := other.do("POST", "/api/v1/users/"+mecID+"/pin-reset", map[string]any{"pin": "6284"}); r.status != http.StatusNotFound {
		t.Fatalf("de otro taller: %d %v", r.status, r.body)
	}
	var acts int
	if err := e.asTaller(tallerID, `SELECT count(*) FROM audit.evento WHERE accion = 'cuenta.pin_reset'
		AND coalesce(despues::text, '') NOT LIKE '%6284%' AND coalesce(antes::text, '') NOT LIKE '%6284%'`, &acts); err != nil || acts != 1 {
		t.Fatalf("bitácora del reset (sin el PIN): %d %v", acts, err)
	}
}

// R-05 y R-06 (hallazgo del revisor en R2): el PIN es de la persona, no del taller. El dueño de un taller no puede
// tomar la cuenta de alguien que ya tiene cuenta en otro: agregarlo no le toca el PIN y restablecerlo no se permite.
func TestPIN_CannotTakeOverAnotherWorkshopsAccount(t *testing.T) {
	e := newPINEnv(t)
	a := e.registerPIN("3105551011", "Taller A")
	e.registerPIN("3105551012", "Taller B")

	// A agrega como mecánico el celular del dueño de B, con un PIN "temporal".
	r := a.must(http.StatusCreated, a.do("POST", "/api/v1/users", map[string]any{"celular": "3105551012", "nombre": "Dueño B", "rol": "mecanico", "pin": "4826"}))
	bID := r.str("id")
	if _, r := e.loginPIN("3105551012", "4826"); r.status == http.StatusOK {
		t.Fatal("agregar a alguien que ya tiene cuenta no debe cambiarle el PIN")
	}
	// Y restablecerle el PIN tampoco se permite: lo hace la persona o un administrador (make pin-reset).
	if r := a.do("POST", "/api/v1/users/"+bID+"/pin-reset", map[string]any{"pin": "4826"}); r.status < 400 || r.str("code") != "pin_de_otro_taller" {
		t.Fatalf("restablecer el PIN de alguien de otro taller: %d %v", r.status, r.body)
	}
	if _, r := e.loginPIN("3105551012", "4826"); r.status == http.StatusOK {
		t.Fatal("el dueño de A entró como el dueño de B")
	}
	// B sigue entrando con su PIN.
	c, r := e.loginPIN("3105551012", ownerPIN)
	c.must(http.StatusOK, r)
}
