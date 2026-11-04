// Package demo carga el taller ficticio de la demo, Servitaller El Pistón (HU-97, HU-98, docs/03 "El taller ficticio"),
// en su estado inicial. Todo es inventado: nombres, placas, precios y celulares de prueba.
//
// La carga va por la API real, en el mismo proceso (server.New), con un reloj controlado que pone la historia en el
// pasado: así el taller nace por los mismos caminos y reglas que usa una persona, y nada se salta una validación. Lo
// único que no va por la API es el arranque del taller (crear el taller y la membresía del dueño) y el archivo de la
// demo anterior, que se hacen con el rol dueño de las tablas (sweveh_migrator) y sujeto a RLS (ADR-0031).
//
// Reiniciar no borra: la bitácora es inmutable (audit.evento). La demo anterior se archiva (membresías y canal
// desactivados, nombre con la fecha) y se carga un taller nuevo.
package demo

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sweveh/internal/identity/adapters/out/otp"
	identitypg "sweveh/internal/identity/adapters/out/postgres"
	identitydomain "sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/storage"
	"sweveh/internal/server"
	"sweveh/internal/shared"
	"sweveh/internal/wire"
)

// Name es el nombre del taller ficticio.
const Name = "Servitaller El Pistón"

// Phones son los celulares de la demo (10 dígitos). Los del dueño, el mecánico, la recepción y los celulares B y D
// deben ser los números verificados del equipo; los demás clientes son ficticios y no reciben nada.
type Phones struct {
	Owner, Mechanic, Reception string // Hernán, Jhon, Yesica
	B, D                       string // Andrés (celular B) y Sebastián (celular D)
	Diego, Marta, Carlos       string
}

// DefaultPhones son números de prueba para correr la demo en local.
var DefaultPhones = Phones{Owner: "3105550001", Mechanic: "3105550002", Reception: "3105550003", B: "3005550011", D: "3005550014",
	Diego: "3005550012", Marta: "3005550013", Carlos: "3005550015"}

// Deps son las conexiones: API (rol sweveh_api) para todo lo del taller y Bootstrap (rol sweveh_migrator) para crear
// el taller y archivar la demo anterior.
type Deps struct {
	API       *pgxpool.Pool
	Bootstrap *pgxpool.Pool
	Storage   storage.Store
	Phones    Phones
	Now       time.Time // "hoy" de la demo; vacío = ahora
	Log       *slog.Logger
}

// Result dice qué quedó cargado.
type Result struct {
	TallerID shared.ID
	Archived []shared.ID // talleres de demos anteriores que quedaron archivados
	Orders   map[string]string
}

// ErrExists: ya hay una demo activa y se pidió cargar sin reiniciar.
var ErrExists = errors.New("ya hay un taller de la demo activo para ese dueño: usa reset-demo")

// Seed carga el taller ficticio. Con reset=false falla si el dueño ya tiene un taller activo; con reset=true archiva
// el que tenga y carga uno nuevo.
func Seed(ctx context.Context, d Deps, reset bool) (Result, error) {
	if d.Now.IsZero() {
		d.Now = time.Now()
	}
	if d.Log == nil {
		d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	today := day(d.Now, 0)
	start := today.AddDate(0, 0, -250).Add(9 * time.Hour)
	clk := clock.NewFake(start)
	boot := db.NewTxManager(d.Bootstrap)

	ownerPhone, err := shared.ParseColombianMobile(d.Phones.Owner)
	if err != nil {
		return Result{}, fmt.Errorf("celular del dueño: %w", err)
	}
	res := Result{TallerID: shared.NewID(), Orders: map[string]string{}}
	owner, active, err := ownerState(ctx, boot, ownerPhone)
	if err != nil {
		return res, err
	}
	if len(active) > 0 && !reset {
		return res, ErrExists
	}
	for _, tid := range active {
		if err := archive(ctx, boot, tid, d.Now); err != nil {
			return res, fmt.Errorf("archivar la demo anterior: %w", err)
		}
		res.Archived = append(res.Archived, tid)
	}
	// La carga escribe la historia con un reloj en el pasado: los códigos de entrada que ya usó el dueño en cargas
	// anteriores (vencidos y de un solo uso) quedarían "en el futuro" y frenarían el siguiente pedido de código.
	if err := boot.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		if _, err := db.Exec(ctx, `DELETE FROM identity.otp_desafio WHERE celular = $1`, ownerPhone); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `DELETE FROM identity.otp_bloqueo WHERE celular = $1`, ownerPhone)
		return err
	}); err != nil {
		return res, fmt.Errorf("limpiar los códigos de la carga anterior: %w", err)
	}
	if err := bootstrap(ctx, boot, clk, res.TallerID, owner, ownerPhone); err != nil {
		return res, fmt.Errorf("crear el taller: %w", err)
	}

	l := newLoader(d, clk)
	l.boot, l.tallerID = boot, res.TallerID
	if err := l.login(d.Phones.Owner); err != nil {
		return res, err
	}
	if err := l.load(today, res.Orders); err != nil {
		return res, err
	}
	// Nada de lo cargado debe salir por WhatsApp: los eventos de la historia quedan procesados.
	if err := boot.DoWithSettings(ctx, map[string]string{"app.taller_id": res.TallerID.String()}, func(ctx context.Context) error {
		_, err := db.Exec(ctx, `UPDATE outbox.evento SET procesado_en = now() WHERE taller_id = $1 AND procesado_en IS NULL`, res.TallerID)
		return err
	}); err != nil {
		return res, fmt.Errorf("cerrar los eventos de la carga: %w", err)
	}
	return res, nil
}

// day es la medianoche (hora de Colombia) de hoy más n días.
func day(now time.Time, n int) time.Time {
	loc := bogota()
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day()+n, 0, 0, 0, 0, loc)
}

func bogota() *time.Location {
	if l, err := time.LoadLocation("America/Bogota"); err == nil {
		return l
	}
	return time.FixedZone("COT", -5*3600)
}

// ownerState busca al dueño y sus talleres activos.
func ownerState(ctx context.Context, boot *db.TxManager, phone string) (*ports.User, []shared.ID, error) {
	var (
		owner  *ports.User
		active []shared.ID
	)
	store := identitypg.NewStore()
	err := boot.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		u, found, err := store.UserByPhone(ctx, phone)
		if err != nil || !found {
			return err
		}
		owner = &u
		return nil
	})
	if err != nil || owner == nil {
		return owner, nil, err
	}
	err = boot.DoWithSettings(ctx, map[string]string{"app.usuario_id": owner.ID.String()}, func(ctx context.Context) error {
		ms, err := store.ActiveMemberships(ctx, owner.ID)
		for _, m := range ms {
			active = append(active, m.TallerID)
		}
		return err
	})
	return owner, active, err
}

// archive deja la demo anterior fuera de uso sin borrar nada: nadie entra (membresías inactivas), su número de
// WhatsApp de prueba queda libre (canal inactivo) y el nombre dice la fecha.
func archive(ctx context.Context, boot *db.TxManager, tallerID shared.ID, now time.Time) error {
	return boot.DoWithSettings(ctx, map[string]string{"app.taller_id": tallerID.String()}, func(ctx context.Context) error {
		if _, err := db.Exec(ctx, `UPDATE identity.membresia SET activo = false WHERE taller_id = $1`, tallerID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE messaging.canal SET activo = false WHERE taller_id = $1`, tallerID); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `UPDATE tenancy.taller SET nombre = left(nombre, 80) || ' (demo archivada ' || $2 || ')', updated_at = now()
			WHERE id = $1 AND nombre NOT LIKE '%(demo archivada%'`, tallerID, now.In(bogota()).Format("2006-01-02 15:04"))
		return err
	})
}

// bootstrap crea el taller y la membresía del dueño (y el dueño, si no existe), como lo hace el registro (HU-01).
func bootstrap(ctx context.Context, boot *db.TxManager, clk *clock.Fake, tallerID shared.ID, owner *ports.User, phone string) error {
	now := clk.Now()
	m := server.Build(boot, nil, clk, slog.New(slog.NewTextHandler(io.Discard, nil)))
	store := identitypg.NewStore()
	userID := shared.NewID()
	if owner != nil {
		userID = owner.ID
	}
	actor := shared.Actor{Type: shared.ActorUser, ID: userID}
	return boot.DoWithSettings(ctx, map[string]string{"app.taller_id": tallerID.String(), "app.usuario_id": userID.String()},
		func(ctx context.Context) error {
			if _, err := m.Workshops.Create(ctx, tallerID, Name, actor, now); err != nil {
				return err
			}
			if owner == nil {
				if err := store.InsertUser(ctx, ports.User{ID: userID, Phone: phone, Name: "Hernán Ríos", Active: true}, now); err != nil {
					return err
				}
				if err := store.InsertConsent(ctx, identitydomain.Consent{ID: shared.NewID(), UserID: userID, Text: identitydomain.ConsentText,
					PolicyVersion: identitydomain.PolicyVersionDraft, Channel: identitydomain.ConsentChannelWeb, AcceptedAt: now}); err != nil {
					return err
				}
			}
			return store.InsertMembership(ctx, ports.Membership{TallerID: tallerID, Role: string(identitydomain.RoleOwner)}, userID, now)
		})
}

// loader maneja la API del taller en el mismo proceso, como lo haría el dueño desde el navegador.
type loader struct {
	h        http.Handler
	clk      *clock.Fake
	otp      *otp.Memory
	cookies  map[string]*http.Cookie
	phones   Phones
	ids      map[string]string // placa → vehículo; nombre → cliente; "jhon" → usuario
	photo    []byte
	now      time.Time // "ahora" de la demo: la carga termina aquí
	boot     *db.TxManager
	tallerID shared.ID
}

const origin = "https://demo.sweveh.local"

func newLoader(d Deps, clk *clock.Fake) *loader {
	key, _ := authn.GenerateDevKey()
	mem := otp.NewMemory()
	cfg := config.API{Env: config.EnvLocal, WhatsApp: config.WhatsApp{Provider: "local"}, AI: config.AI{Provider: "reglas"}}
	h := server.New(server.Deps{
		Pool: d.API, Log: d.Log, Clock: clk,
		Signer:    authn.NewSigner(key, "demo-1", clk),
		Verifier:  authn.NewVerifier(map[string]*ecdsa.PublicKey{"demo-1": &key.PublicKey}, clk),
		OTPSender: mem, OTPPepper: []byte("carga-de-la-demo-solo-en-este-proceso"),
		AllowedOrigins: []string{origin}, LoginPerMinute: 1e6, Storage: d.Storage, Extra: wire.Extra(cfg),
	})
	return &loader{h: h, clk: clk, otp: mem, cookies: map[string]*http.Cookie{}, phones: d.Phones, ids: map[string]string{}, photo: samplePhoto(),
		now: d.Now}
}

// at mueve el reloj de la carga a t (nunca hacia atrás) y vuelve a entrar si pasó más de lo que dura la sesión.
func (l *loader) at(t time.Time) error {
	if d := t.Sub(l.clk.Now()); d > 0 {
		l.clk.Advance(d)
		if d > 5*time.Minute {
			return l.login(l.phones.Owner)
		}
	}
	return nil
}

func (l *loader) login(phone10 string) error {
	l.cookies = map[string]*http.Cookie{}
	if _, err := l.call(http.StatusAccepted, "POST", "/api/v1/auth/otp", map[string]any{"celular": phone10}); err != nil {
		return fmt.Errorf("pedir código: %w", err)
	}
	code := l.otp.Last("+57" + phone10)
	if _, err := l.call(http.StatusOK, "POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone10, "codigo": code}); err != nil {
		return fmt.Errorf("entrar como dueño: %w", err)
	}
	return nil
}

// call hace una petición y exige el estado esperado; devuelve el cuerpo.
func (l *loader) call(want int, method, path string, body any) (map[string]any, error) {
	var rd io.Reader
	ctype := ""
	switch b := body.(type) {
	case nil:
	case *multipart.Writer:
		return nil, errors.New("usa upload")
	default:
		raw, _ := json.Marshal(b)
		rd, ctype = bytes.NewReader(raw), "application/json"
	}
	req := httptest.NewRequestWithContext(context.Background(), method, origin+path, rd)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	return l.send(want, req)
}

func (l *loader) send(want int, req *http.Request) (map[string]any, error) {
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", origin)
	for _, c := range l.cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	l.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		l.cookies[c.Name] = c
	}
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != want {
		return out, fmt.Errorf("%s %s: %d (esperaba %d): %s", req.Method, req.URL.Path, rec.Code, want, strings.TrimSpace(rec.Body.String()))
	}
	return out, nil
}

// upload sube una foto de evidencia marcada para el cliente (la que acompaña el estado, HU-56).
func (l *loader) upload(orderID, side string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("lado", side)
	_ = mw.WriteField("mostrarCliente", "true")
	fw, _ := mw.CreateFormFile("foto", "trabajo.png")
	_, _ = fw.Write(l.photo)
	_ = mw.Close()
	req := httptest.NewRequestWithContext(context.Background(), "POST", origin+"/api/v1/orders/"+orderID+"/evidence", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	_, err := l.send(http.StatusCreated, req)
	return err
}

// samplePhoto es una imagen de prueba (un recuadro gris): la demo no usa fotos de vehículos reales.
func samplePhoto() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for x := 0; x < 64; x++ {
		for y := 0; y < 48; y++ {
			img.Set(x, y, color.RGBA{R: 120, G: 130, B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
