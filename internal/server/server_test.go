package server_test

// Pruebas de punta a punta por HTTP de la rebanada del Sprint 0:
// HU-01 (cuenta con celular) → HU-09 (cliente) → HU-10/11 (vehículo y motor) → HU-16 (orden),
// más aislamiento entre talleres (HU-79, CA-10), seguridad de sesión y concurrencia (R-07).
// Datos Ficticio: celulares y placas inventados.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/identity/adapters/out/otp"
	"sweveh/internal/outbox"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/storage"
	"sweveh/internal/platform/testdb"
	"sweveh/internal/server"
	"sweveh/internal/shared"
	"sweveh/internal/wire"
)

type env struct {
	t   *testing.T
	srv *httptest.Server
	otp *otp.Memory
	db  *testdb.DB
	log *slog.Logger
	clk *clock.Fake
	mux *chi.Mux
	st  storage.Store
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWith(t, config.API{Env: config.EnvLocal})
}

// newEnvWith es newEnv con la configuración del motor (WhatsApp, IA) y los webhooks públicos montados.
func newEnvWith(t *testing.T, cfg config.API) *env {
	t.Helper()
	d := testdb.New(t)
	key, err := authn.GenerateDevKey()
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(time.Now())
	mem := otp.NewMemory()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// El origen permitido se conoce antes de arrancar: se arma el router y luego se inicia TLS.
	srv := httptest.NewUnstartedServer(nil)
	origin := "https://" + srv.Listener.Addr().String()
	mux := server.New(server.Deps{
		Pool: d.API, Log: log, Clock: clk,
		Signer:    authn.NewSigner(key, "test-1", clk),
		Verifier:  authn.NewVerifier(map[string]*ecdsa.PublicKey{"test-1": &key.PublicKey}, clk),
		OTPSender: mem, OTPPepper: []byte("pepper-de-pruebas-de-32-caracteres!!"),
		AllowedOrigins: []string{origin}, LoginPerMinute: 100000, Storage: st,
		Extra: wire.Extra(cfg), Public: wire.Public(d.API, cfg, log), PlatformAdmins: cfg.Platform.Admins,
	})
	srv.Config.Handler = mux
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv, otp: mem, db: d, log: log, clk: clk, mux: mux, st: st}
}

type client struct {
	e    *env
	http *http.Client
	jar  *cookiejar.Jar
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	c := *e.srv.Client()
	c.Jar = jar
	return &client{e: e, http: &c, jar: jar}
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
}

func (r resp) str(k string) string { s, _ := r.body[k].(string); return s }

func (c *client) do(method, path string, body any, headers ...string) resp {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", c.e.srv.URL)
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (c *client) must(want int, r resp) resp {
	c.e.t.Helper()
	if r.status != want {
		c.e.t.Fatalf("status %d, quería %d: %v", r.status, want, r.body)
	}
	return r
}

// register crea la cuenta de un dueño y deja la sesión en el cliente.
func (e *env) register(phone10, name string) *client {
	e.t.Helper()
	c := e.client()
	c.must(http.StatusAccepted, c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone10}))
	code := e.otp.Last("+57" + phone10)
	r := c.must(http.StatusOK, c.do("POST", "/api/v1/auth/otp/verify", map[string]any{
		"celular": phone10, "codigo": code, "nombre": name, "autorizacionDatos": true,
	}))
	if r.body["cuentaNueva"] != true {
		e.t.Fatalf("debía ser cuenta nueva: %v", r.body)
	}
	return c
}

func nested(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func wrongCode(right string) string {
	if right == "000000" {
		return "111111"
	}
	return "000000"
}

// HU-01 y HU-02.
func TestRegistration_OwnerWithPhoneAndConsent(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	phone := "3105550101"
	// Celular con formato malo.
	r := c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": "31055"})
	if r.status != http.StatusBadRequest || r.str("code") != "celular_invalido" {
		t.Fatalf("celular inválido: %d %v", r.status, r.body)
	}
	c.must(http.StatusAccepted, c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone}))
	code := e.otp.Last("+57" + phone)

	// HU-01 criterio 2: código equivocado dice el problema y cuántos intentos quedan.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": wrongCode(code)})
	if r.status != http.StatusBadRequest || r.str("code") != "codigo_equivocado" || !strings.Contains(r.str("detail"), "4 intentos") {
		t.Fatalf("código equivocado: %d %v", r.status, r.body)
	}
	// Código correcto pero sin nombre: no se gasta el código.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": code})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "registro_incompleto" {
		t.Fatalf("sin nombre: %d %v", r.status, r.body)
	}
	// HU-02 criterio 2: sin la casilla marcada no se puede continuar.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": code, "nombre": "Hernán Ruiz"})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "autorizacion_requerida" {
		t.Fatalf("sin autorización: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/auth/otp/verify", map[string]any{
		"celular": phone, "codigo": code, "nombre": "Hernán Ruiz", "autorizacionDatos": true,
	}))
	if r.body["cuentaNueva"] != true || nested(r.body, "taller", "nombre") != "Taller de Hernán" {
		t.Fatalf("registro: %v", r.body)
	}
	tallerID := nested(r.body, "taller", "id")
	me := c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))
	if nested(me.body, "usuario", "nombre") != "Hernán Ruiz" || nested(me.body, "taller", "id") != tallerID {
		t.Fatalf("me: %v", me.body)
	}
	// El código ya usado no sirve otra vez.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": code})
	if r.status != http.StatusBadRequest || r.str("code") != "codigo_inexistente" {
		t.Fatalf("código reutilizado: %d %v", r.status, r.body)
	}

	// HU-01 criterio 3: un celular con cuenta no crea otra; entra a la misma.
	// Antes, pedir otro código en menos de 60 s se limita.
	if r := c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone}); r.status != http.StatusTooManyRequests || r.str("code") != "reenvio_muy_pronto" {
		t.Fatalf("reenvío inmediato: %d %v", r.status, r.body)
	}
	e.clk.Advance(61 * time.Second)
	other := e.client()
	other.must(http.StatusAccepted, other.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone}))
	r = other.must(http.StatusOK, other.do("POST", "/api/v1/auth/otp/verify", map[string]any{
		"celular": phone, "codigo": e.otp.Last("+57" + phone), "nombre": "Otro nombre", "autorizacionDatos": true,
	}))
	if r.body["cuentaNueva"] != false || nested(r.body, "taller", "id") != tallerID {
		t.Fatalf("segundo registro con el mismo celular: %v", r.body)
	}

	// HU-02 criterio 3: queda la prueba de la autorización (texto, versión, canal, fecha).
	var texto, version, canal string
	err := e.db.API.QueryRow(context.Background(),
		"SELECT texto, version_politica, canal FROM identity.autorizacion_titular a JOIN identity.usuario u ON u.id = a.usuario_id WHERE u.celular = $1",
		"+57"+phone).Scan(&texto, &version, &canal)
	if err != nil || texto == "" || version == "" || canal != "web" {
		t.Fatalf("prueba de autorización: %q %q %q %v", texto, version, canal, err)
	}
}

// HU-01 criterio 2: 5 intentos fallidos bloquean el celular 15 minutos.
func TestOTP_FiveFailuresLockThePhone(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	phone := "3105550102"
	c.must(http.StatusAccepted, c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone}))
	code := e.otp.Last("+57" + phone)
	for i := 1; i <= 4; i++ {
		c.must(http.StatusBadRequest, c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": wrongCode(code)}))
	}
	r := c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": wrongCode(code)})
	if r.status != http.StatusTooManyRequests || r.str("code") != "celular_bloqueado" {
		t.Fatalf("quinto intento: %d %v", r.status, r.body)
	}
	// Ni el código correcto ni uno nuevo sirven mientras dure el bloqueo.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": code, "nombre": "X", "autorizacionDatos": true})
	if r.status != http.StatusTooManyRequests {
		t.Fatalf("código correcto durante bloqueo: %d %v", r.status, r.body)
	}
	r = c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone})
	if r.status != http.StatusTooManyRequests {
		t.Fatalf("pedir código durante bloqueo: %d %v", r.status, r.body)
	}
}

// La rebanada vertical completa: HU-09 → HU-10/11 → HU-16.
func TestSlice_CustomerVehicleOrder(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550103", "Hernán Ruiz")

	// HU-09: crear y, con el mismo celular, recibir el existente.
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "300 555 0001", "nombre": "Laura Gómez"}))
	clienteID := nested(r.body, "cliente", "id").(string)
	if nested(r.body, "cliente", "autorizacionEstado") != "pendiente" {
		t.Fatalf("autorización inicial: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550001", "nombre": "Otra"}))
	if nested(r.body, "cliente", "id") != clienteID || r.body["existente"] != true {
		t.Fatalf("mismo celular debía devolver el existente: %v", r.body)
	}
	c.must(http.StatusOK, c.do("GET", "/api/v1/customers/by-phone/3005550001", nil))

	// HU-10/11: carro sin motor → motor pendiente; misma placa escrita distinto → el mismo.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": "KLM482", "clienteId": clienteID, "marca": "Chevrolet", "linea": "Spark GT", "modeloAnio": 2015,
	}))
	vehiculoID := nested(r.body, "vehiculo", "id").(string)
	if nested(r.body, "vehiculo", "tipo") != "carro" || nested(r.body, "vehiculo", "motorPendiente") != true {
		t.Fatalf("vehículo: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "klm-482", "clienteId": clienteID}))
	if nested(r.body, "vehiculo", "id") != vehiculoID {
		t.Fatalf("placa normalizada debía abrir el mismo vehículo: %v", r.body)
	}
	// Moto: tipo por la placa y turbo en No.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "XVN43F", "clienteId": clienteID, "cilindraje": "150"}))
	if nested(r.body, "vehiculo", "tipo") != "moto" || nested(r.body, "vehiculo", "turbo") != false || nested(r.body, "vehiculo", "cilindrajeCc") != float64(150) {
		t.Fatalf("moto: %v", r.body)
	}
	// Placa especial sin confirmar → 422.
	r = c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "R12345", "clienteId": clienteID})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "placa_especial_sin_confirmar" {
		t.Fatalf("placa especial: %d %v", r.status, r.body)
	}

	// HU-16 + HU-11 criterio 4: Recibido con motor pendiente se rechaza; Por recibir se abre.
	r = c.do("POST", "/api/v1/orders", map[string]any{"placa": "KLM482", "sintoma": "Suena al frenar", "estadoInicial": "recibido"})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "motor_pendiente" {
		t.Fatalf("recibido con motor pendiente: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "KLM482", "sintoma": "Suena al frenar"}))
	if r.str("codigo") != "OS-0001" || r.str("estado") != "por_recibir" {
		t.Fatalf("primera orden: %v", r.body)
	}
	// HU-16 criterio 2: con una orden abierta avisa; con confirmación la abre.
	r = c.do("POST", "/api/v1/orders", map[string]any{"placa": "KLM482", "sintoma": "Otra cosa"})
	if r.status != http.StatusConflict || r.str("code") != "orden_abierta_existente" {
		t.Fatalf("segunda orden sin confirmar: %d %v", r.status, r.body)
	}

	// HU-11 criterio 2 y 5: cilindraje en litros se guarda en cc; cambio con If-Match y bitácora.
	v := c.must(http.StatusOK, c.do("GET", "/api/v1/vehicles/"+vehiculoID, nil))
	etag := v.header.Get("ETag")
	r = c.do("PATCH", "/api/v1/vehicles/"+vehiculoID+"/engine", map[string]any{"cilindraje": "1.2 L", "turbo": false})
	if r.status != http.StatusBadRequest || r.str("code") != "if_match_requerido" {
		t.Fatalf("sin If-Match: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/vehicles/"+vehiculoID+"/engine", map[string]any{"cilindraje": "1.2 L", "turbo": false, "energia": "gasolina", "transmision": "manual"}, "If-Match", etag))
	if r.body["cilindrajeCc"] != float64(1200) || r.body["motorPendiente"] != false {
		t.Fatalf("motor actualizado: %v", r.body)
	}
	r = c.do("PATCH", "/api/v1/vehicles/"+vehiculoID+"/engine", map[string]any{"turbo": true}, "If-Match", etag)
	if r.status != http.StatusConflict || r.str("code") != "version_conflicto" {
		t.Fatalf("If-Match viejo: %d %v", r.status, r.body)
	}
	var antes, despues string
	err := e.db.Tx.Do(testdb.As(tallerOf(t, c)), func(ctx context.Context) error {
		return queryRow(ctx, `SELECT antes::text, despues::text FROM audit.evento WHERE accion = 'vehiculo.motor.cambiar'`, &antes, &despues)
	})
	if err != nil || !strings.Contains(antes, `"cilindraje_cc": null`) || !strings.Contains(despues, `"cilindraje_cc": 1200`) {
		t.Fatalf("bitácora del motor: antes=%s después=%s err=%v", antes, despues, err)
	}

	// Con el motor completo ya se puede abrir en Recibido (confirmando la duplicada).
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{
		"vehiculoId": vehiculoID, "sintoma": "Cambio de aceite", "estadoInicial": "recibido", "confirmarDuplicada": true,
	}))
	ordenID := r.str("id")
	if r.str("codigo") != "OS-0002" || r.str("estado") != "recibido" {
		t.Fatalf("segunda orden: %v", r.body)
	}

	list := c.must(http.StatusOK, c.do("GET", "/api/v1/orders", nil))
	ordenes := list.body["ordenes"].([]any)
	if len(ordenes) != 2 || nested(ordenes[0].(map[string]any), "cliente", "nombre") != "Laura Gómez" {
		t.Fatalf("lista de órdenes: %v", list.body)
	}
	det := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+ordenID, nil))
	if hist := det.body["historial"].([]any); len(hist) != 1 || hist[0].(map[string]any)["hacia"] != "recibido" {
		t.Fatalf("historial: %v", det.body)
	}

	// Auditoría y outbox en la misma transacción: hay eventos y el worker los despacha.
	dsp := outbox.NewDispatcher(e.db.Worker, e.clk, e.log, 100)
	n, err := dsp.RunOnce(context.Background())
	if err != nil || n < 6 {
		t.Fatalf("outbox despachó %d eventos (%v); esperaba al menos 6", n, err)
	}
	if n, _ := dsp.RunOnce(context.Background()); n != 0 {
		t.Fatalf("un evento se despachó dos veces (%d)", n)
	}
}

// HU-79 / CA-10: un taller no ve ni toca los datos de otro, y recibe 404 (no 403).
func TestIsolation_TwoWorkshops(t *testing.T) {
	e := newEnv(t)
	a := e.register("3105550104", "Dueño A")
	b := e.register("3105550105", "Dueño B")

	r := a.must(http.StatusCreated, a.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550002", "nombre": "Cliente de A"}))
	clienteA := nested(r.body, "cliente", "id").(string)
	r = a.must(http.StatusCreated, a.do("POST", "/api/v1/vehicles", map[string]any{"placa": "GBW118", "clienteId": clienteA}))
	vehiculoA := nested(r.body, "vehiculo", "id").(string)
	r = a.must(http.StatusCreated, a.do("POST", "/api/v1/orders", map[string]any{"placa": "GBW118", "sintoma": "No arranca"}))
	ordenA := r.str("id")

	for _, path := range []string{
		"/api/v1/orders/" + ordenA,
		"/api/v1/customers/" + clienteA,
		"/api/v1/customers/by-phone/3005550002",
		"/api/v1/vehicles/" + vehiculoA,
		"/api/v1/vehicles/by-plate/GBW118",
	} {
		if r := b.do("GET", path, nil); r.status != http.StatusNotFound {
			t.Errorf("B en %s: %d %v (debía ser 404)", path, r.status, r.body)
		}
	}
	if l := b.must(http.StatusOK, b.do("GET", "/api/v1/orders", nil)); len(l.body["ordenes"].([]any)) != 0 {
		t.Fatalf("B ve órdenes de A: %v", l.body)
	}
	// B no puede colgar un vehículo de un cliente de A ni abrir orden con el vehículo de A.
	if r := b.do("POST", "/api/v1/vehicles", map[string]any{"placa": "HTR315", "clienteId": clienteA}); r.status != http.StatusBadRequest {
		t.Fatalf("B usó el cliente de A: %d %v", r.status, r.body)
	}
	if r := b.do("POST", "/api/v1/orders", map[string]any{"vehiculoId": vehiculoA, "sintoma": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("B abrió orden con el vehículo de A: %d %v", r.status, r.body)
	}
	if r := b.do("PATCH", "/api/v1/vehicles/"+vehiculoA+"/engine", map[string]any{"turbo": true}, "If-Match", `"v1"`); r.status != http.StatusNotFound {
		t.Fatalf("B cambió el motor del vehículo de A: %d %v", r.status, r.body)
	}
	// La misma placa y el mismo celular son independientes en cada taller; la numeración también.
	r = b.must(http.StatusCreated, b.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550002", "nombre": "Cliente de B"}))
	clienteB := nested(r.body, "cliente", "id").(string)
	b.must(http.StatusCreated, b.do("POST", "/api/v1/vehicles", map[string]any{"placa": "GBW118", "clienteId": clienteB}))
	r = b.must(http.StatusCreated, b.do("POST", "/api/v1/orders", map[string]any{"placa": "GBW118", "sintoma": "Revisión"}))
	if r.str("codigo") != "OS-0001" {
		t.Fatalf("la numeración de B debía empezar en OS-0001: %v", r.body)
	}
}

func TestSecurity_SessionAndCSRF(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	if r := anon.do("GET", "/api/v1/orders", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", r.status)
	}
	c := e.register("3105550106", "Dueña C")
	// CSRF: sin encabezado personalizado o con Origin ajeno se rechaza.
	if r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550003", "nombre": "X"}, "X-Requested-With", ""); r.status != http.StatusForbidden || r.str("code") != "csrf" {
		t.Fatalf("sin X-Requested-With: %d %v", r.status, r.body)
	}
	if r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550003", "nombre": "X"}, "Origin", "https://evil.example"); r.status != http.StatusForbidden {
		t.Fatalf("Origin ajeno: %d %v", r.status, r.body)
	}
	// Token alterado → 401.
	u, _ := url.Parse(e.srv.URL)
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == authn.AccessCookie {
			bad := e.client()
			bad.jar.SetCookies(u, []*http.Cookie{{Name: authn.AccessCookie, Value: ck.Value[:len(ck.Value)-4] + "AAAA", Path: "/"}})
			if r := bad.do("GET", "/api/v1/me", nil); r.status != http.StatusUnauthorized {
				t.Fatalf("token alterado: %d", r.status)
			}
		}
	}
	// Cabeceras de seguridad presentes.
	r := c.do("GET", "/api/v1/me", nil)
	if !strings.Contains(r.header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || r.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("cabeceras: %v", r.header)
	}
	// Cuerpo con campos desconocidos → 400 (validación estricta en el borde).
	if r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550003", "nombre": "X", "taller_id": "otro"}); r.status != http.StatusBadRequest {
		t.Fatalf("campo desconocido (taller_id del cliente): %d %v", r.status, r.body)
	}
}

// Refresh rotativo: el viejo deja de servir y, si se reutiliza, se revoca toda la sesión.
func TestRefresh_RotationAndReuseDetection(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550107", "Dueño D")
	refreshURL, _ := url.Parse(e.srv.URL + "/api/v1/auth/refresh")
	old := cookie(c.jar, refreshURL, authn.RefreshCookie)
	if old == "" {
		t.Fatal("no hay cookie de refresh")
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/auth/refresh", nil))
	rotated := cookie(c.jar, refreshURL, authn.RefreshCookie)
	if rotated == "" || rotated == old {
		t.Fatal("el refresh no rotó")
	}
	// Alguien reutiliza el refresh viejo → 401 y la sesión queda revocada.
	thief := e.client()
	thief.jar.SetCookies(refreshURL, []*http.Cookie{{Name: authn.RefreshCookie, Value: old, Path: authn.RefreshPath}})
	if r := thief.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("refresh reutilizado: %d", r.status)
	}
	if r := c.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("tras la reutilización, el refresh legítimo también debía caer: %d", r.status)
	}
	// Cierre de sesión idempotente.
	c.must(http.StatusNoContent, c.do("POST", "/api/v1/auth/logout", nil))
	c.must(http.StatusNoContent, c.do("POST", "/api/v1/auth/logout", nil))
}

// R-07: el contador por taller no repite números aunque lleguen muchas órdenes a la vez.
func TestConcurrency_OrderNumbersAreUniqueAndConsecutive(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550108", "Dueño E")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550004", "nombre": "Cliente"}))
	clienteID := nested(r.body, "cliente", "id").(string)
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "FQD264", "clienteId": clienteID}))

	const n = 25
	var wg sync.WaitGroup
	numbers := make(chan float64, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := c.do("POST", "/api/v1/orders", map[string]any{"placa": "FQD264", "sintoma": "Concurrencia", "confirmarDuplicada": true})
			if r.status == http.StatusCreated {
				numbers <- r.body["numero"].(float64)
			} else {
				t.Errorf("orden concurrente: %d %v", r.status, r.body)
			}
		}()
	}
	wg.Wait()
	close(numbers)
	seen := map[float64]bool{}
	for num := range numbers {
		if seen[num] {
			t.Fatalf("número repetido: %v", num)
		}
		seen[num] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[float64(i)] {
			t.Fatalf("falta el número %d (hay %d)", i, len(seen))
		}
	}
}

// RF-04: dos personas registrando el mismo celular a la vez → un solo cliente.
func TestConcurrency_SamePhoneSameCustomer(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550109", "Dueño F")
	const n = 10
	var wg sync.WaitGroup
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550005", "nombre": "Marta"})
			ids <- nested(r.body, "cliente", "id").(string)
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("dos clientes con el mismo celular: %s y %s", first, id)
		}
	}
}

func tallerOf(t *testing.T, c *client) shared.ID {
	t.Helper()
	me := c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))
	id, err := shared.ParseID(nested(me.body, "taller", "id").(string))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func queryRow(ctx context.Context, sql string, dest ...any) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	return conn.QueryRow(ctx, sql).Scan(dest...)
}

func cookie(jar *cookiejar.Jar, u *url.URL, name string) string {
	for _, ck := range jar.Cookies(u) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}
