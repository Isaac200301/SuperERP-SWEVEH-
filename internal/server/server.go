// Package server arma el proceso api: instancia módulos, adaptadores y rutas.
// Es el composition root compartido por cmd/api y las pruebas HTTP de punta a punta;
// no contiene reglas de negocio.
package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	cataloghttp "sweveh/internal/catalog/adapters/in/http"
	customershttp "sweveh/internal/customers/adapters/in/http"
	"sweveh/internal/identity"
	identityhttp "sweveh/internal/identity/adapters/in/http"
	identitypg "sweveh/internal/identity/adapters/out/postgres"
	"sweveh/internal/identity/adapters/out/token"
	identityws "sweveh/internal/identity/adapters/out/workshops"
	identityapp "sweveh/internal/identity/app"
	identityports "sweveh/internal/identity/ports"
	inventoryhttp "sweveh/internal/inventory/adapters/in/http"
	ordershttp "sweveh/internal/orders/adapters/in/http"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/storage"
	quoteshttp "sweveh/internal/quotes/adapters/in/http"
	"sweveh/internal/search"
	"sweveh/internal/shared"
	tenancyhttp "sweveh/internal/tenancy/adapters/in/http"
	vehicleshttp "sweveh/internal/vehicles/adapters/in/http"
)

// Deps son las piezas de infraestructura ya construidas.
type Deps struct {
	Pool           *pgxpool.Pool
	Log            *slog.Logger
	Clock          clock.Clock
	Signer         *authn.Signer
	Verifier       *authn.Verifier
	OTPSender      identityports.OTPSender
	OTPPepper      []byte
	AllowedOrigins []string
	TrustProxy     bool
	HSTS           bool
	WebDir         string
	LoginPerMinute float64
	Storage        storage.Store
	// Extra monta rutas de módulos que viven fuera de este paquete (mensajería, agente, KPIs). Puede ser nil.
	Extra func(m *Modules, g *authn.Guard) []func(chi.Router)
	// Public monta rutas sin sesión de usuario (webhook de WhatsApp). Puede ser nil.
	Public func(r chi.Router)
}

// New devuelve el router HTTP completo.
func New(d Deps) *chi.Mux {
	tx := db.NewTxManager(d.Pool)
	m := Build(tx, d.Storage, d.Clock, d.Log)
	guard := authn.NewGuard(d.Verifier, identity.HasPermission, d.Log)

	identitySvc := identityapp.NewService(identityapp.Deps{
		Tx: tx, Store: identitypg.NewStore(), Sender: d.OTPSender, Tokens: token.NewIssuer(d.Signer),
		Workshops: identityws.New(m.Workshops), Audit: m.Audit, Events: m.Events, Clock: d.Clock,
		Pepper: d.OTPPepper, Log: d.Log,
	})
	identitySvc.SetMembers(identitypg.Members{})

	identityH := identityhttp.NewHandler(identitySvc, d.Log, httpx.NewRateLimiter(d.LoginPerMinute, 5), d.TrustProxy)
	customersH := customershttp.NewHandler(m.Customers, guard, d.Log)
	vehiclesH := vehicleshttp.NewHandler(m.Vehicles, guard, d.Log)
	ordersH := ordershttp.NewHandler(m.Orders, m.Flow, guard, d.Log)
	routes := []func(chi.Router){
		identityH.PrivateRoutes, customersH.Routes, customersH.ExtraRoutes, vehiclesH.Routes, vehiclesH.ExtraRoutes, ordersH.Routes,
		func(r chi.Router) { identityH.UserRoutes(r, guard) },
		tenancyhttp.NewHandler(m.Admin, guard, d.Log).Routes,
		cataloghttp.NewHandler(m.Catalog, guard, d.Log).Routes,
		inventoryhttp.NewHandler(m.Inventory, guard, d.Log).Routes,
		quoteshttp.NewHandler(m.Quotes, guard, d.Log).Routes,
		search.NewHandler(tx, m.VehDir, m.CustDir, m.OrdersF, guard, d.Log).Routes,
	}
	if d.Extra != nil {
		routes = append(routes, d.Extra(m, guard)...)
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.Recover(d.Log), httpx.SecurityHeaders(d.HSTS), httpx.AccessLog(d.Log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	if d.Public != nil {
		d.Public(r) // /webhooks/*: sin cookies ni CSRF, con firma propia
	}
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(httpx.CSRF(d.AllowedOrigins, d.Log))
		identityH.PublicRoutes(api)
		api.Group(func(priv chi.Router) {
			priv.Use(guard.Authenticate)
			for _, mount := range routes {
				mount(priv)
			}
		})
		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, d.Log, errRouteNotFound)
		})
	})

	if d.WebDir != "" {
		r.NotFound(spa(d.WebDir))
	}
	return r
}

var errRouteNotFound = shared.NotFound("ruta_no_encontrada", "Esa ruta no existe.")

// spa sirve el frontend compilado y devuelve index.html para las rutas del cliente (mismo origen, ADR-18).
// os.DirFS tiene raíz en dir y rechaza nombres con "..": no se puede salir de la carpeta.
func spa(dir string) http.HandlerFunc {
	root := os.DirFS(dir)
	files := http.FileServerFS(root)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if st, err := fs.Stat(root, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, root, "index.html")
	}
}
