// Package config lee y valida las variables de entorno al arrancar (12-factor, falla rápido).
// Los secretos solo llegan por el entorno; nunca hay valores reales en el repositorio.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sweveh/internal/shared"
)

type Env string

const (
	EnvLocal      Env = "local"
	EnvStaging    Env = "staging"
	EnvProduction Env = "production"
)

// API es la configuración del proceso api.
type API struct {
	Env              Env
	HTTPAddr         string
	DatabaseURL      string
	DBMaxConns       int32
	DBSimpleProtocol bool
	DBMaxLifetime    time.Duration
	JWTPrivateKeyPEM string // vacío solo en local: se genera una clave efímera
	JWTKeyID         string
	OTPPepper        string
	OTPSender        string // "consola" (solo local) | "memoria" (solo pruebas)
	AllowedOrigins   []string
	TrustProxy       bool
	WebDir           string
	LogLevel         string
	LoginPerMinute   float64
	StorageDir       string
	WhatsApp         WhatsApp
	AI               AI
	Platform         Platform
}

// Platform es lo del administrador del ERP (HU-74, ADR-0030): quiénes son y con qué números se compara el consumo.
// IncludedMessages y COPPerUSD son provisionales hasta decidir el plan (D-04, Q-24).
type Platform struct {
	Admins           []string // celulares en E.164 (+57…) de los administradores del ERP; vacío = nadie
	IncludedMessages int      // mensajes incluidos por taller al mes ("34 de 1.000")
	COPPerUSD        int      // pesos por dólar para mostrar el costo de IA en pesos
}

// WhatsApp configura el canal (HU-101). Provider elige el adaptador: "meta" (Cloud API, oficial) o "local"
// (simulador, no envía nada). Sin WHATSAPP_PROVIDER: "meta" si hay token, si no "local".
type WhatsApp struct {
	Provider      string
	AppSecret     string
	AccessToken   string
	PhoneNumberID string
	VerifyToken   string
	APIBase       string
	WAHA          WAHA
}

// WAHA es el WhatsApp no oficial para la demo (ADR-0027): un número de prueba vinculado con QR. Se activa con
// WAHA_URL y convive con el proveedor del taller (el número "waha:<sesión>" va a WAHA). Prohibido en producción.
type WAHA struct {
	URL           string
	APIKey        string
	WebhookSecret string // la misma clave de WHATSAPP_HOOK_HMAC_KEY en WAHA
	Session       string
}

// Enabled dice si WAHA está configurado.
func (w WAHA) Enabled() bool { return w.URL != "" }

// AI configura el intérprete del agente (D-08 abierta, ADR-0026). "reglas" = local sin costo; "openrouter" =
// modelo de lenguaje por la API compatible con OpenAI, solo fuera de producción hasta cerrar AQ-09.
type AI struct {
	Provider           string
	APIKey             string
	Model              string
	APIBase            string
	MonthlyCapMicroUSD int64
	// VisionModel lee la foto de la lista de precios (HU-93, ADR-0032): uno o varios modelos con imagen, por coma.
	VisionModel string
}

// Worker es la configuración del proceso worker.
type Worker struct {
	Env          Env
	DatabaseURL  string
	PollInterval time.Duration
	BatchSize    int32
	LogLevel     string
	WhatsApp     WhatsApp
	AI           AI
}

func engine(env Env, errs *[]error) (WhatsApp, AI) {
	wa := WhatsApp{
		AppSecret: os.Getenv("WHATSAPP_APP_SECRET"), AccessToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		PhoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), VerifyToken: os.Getenv("WHATSAPP_VERIFY_TOKEN"),
		APIBase: get("WHATSAPP_API_BASE", "https://graph.facebook.com/v21.0"),
		WAHA: WAHA{URL: os.Getenv("WAHA_URL"), APIKey: os.Getenv("WAHA_API_KEY"), WebhookSecret: os.Getenv("WAHA_WEBHOOK_SECRET"),
			Session: get("WAHA_SESSION", "default")},
	}
	wa.Provider = get("WHATSAPP_PROVIDER", map[bool]string{true: "meta", false: "local"}[wa.AccessToken != ""])
	switch {
	case wa.Provider != "meta" && wa.Provider != "local":
		*errs = append(*errs, fmt.Errorf("WHATSAPP_PROVIDER=%q no es un canal conocido (meta, local)", wa.Provider))
	case wa.Provider == "meta" && (wa.AccessToken == "" || wa.AppSecret == ""):
		*errs = append(*errs, errors.New("WHATSAPP_PROVIDER=meta necesita WHATSAPP_ACCESS_TOKEN y WHATSAPP_APP_SECRET"))
	case wa.Provider == "local" && env == EnvProduction:
		*errs = append(*errs, errors.New("WHATSAPP_PROVIDER=local (simulador) no se permite en producción"))
	}
	switch w := wa.WAHA; {
	case !w.Enabled():
	case env == EnvProduction:
		*errs = append(*errs, errors.New("WAHA_URL es solo para la demo (WhatsApp no oficial): no se permite en producción"))
	case w.APIKey == "" || w.WebhookSecret == "":
		*errs = append(*errs, errors.New("WAHA_URL necesita WAHA_API_KEY y WAHA_WEBHOOK_SECRET (firma de los webhooks)"))
	}
	return wa, aiConfig(env, errs)
}

func aiConfig(env Env, errs *[]error) AI {
	ai := AI{Provider: get("AI_PROVIDER", "reglas"), APIKey: os.Getenv("AI_API_KEY"), Model: os.Getenv("AI_MODEL"),
		APIBase: get("AI_API_BASE", "https://openrouter.ai/api/v1"), MonthlyCapMicroUSD: int64(getInt("AI_MONTHLY_CAP_MICRO_USD", 5_000_000, errs)),
		VisionModel: get("AI_VISION_MODEL", "google/gemma-4-31b-it:free,nvidia/nemotron-3.5-super-vl:free")}
	switch {
	case ai.Provider != "reglas" && ai.Provider != "openrouter":
		*errs = append(*errs, fmt.Errorf("AI_PROVIDER=%q no es un proveedor conocido (reglas, openrouter)", ai.Provider))
	case ai.Provider == "openrouter" && (ai.APIKey == "" || ai.Model == ""):
		*errs = append(*errs, errors.New("AI_PROVIDER=openrouter necesita AI_API_KEY y AI_MODEL"))
	case ai.Provider == "openrouter" && env == EnvProduction:
		*errs = append(*errs, errors.New("AI_PROVIDER=openrouter es solo para la demo: falta cerrar AQ-09 antes de producción"))
	}
	return ai
}

// LoadAI lee solo la configuración de la IA (cmd/agent-eval mide el modelo sin base de datos, HU-102).
func LoadAI() (AI, error) {
	var errs []error
	ai := aiConfig(Env(get("APP_ENV", "local")), &errs)
	return ai, errors.Join(errs...)
}

// LoadAPI lee la configuración del api.
func LoadAPI() (API, error) {
	var errs []error
	c := API{
		Env:              Env(get("APP_ENV", "local")),
		HTTPAddr:         get("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTPrivateKeyPEM: os.Getenv("JWT_PRIVATE_KEY_PEM"),
		JWTKeyID:         get("JWT_KID", "local-1"),
		OTPPepper:        os.Getenv("OTP_PEPPER"),
		OTPSender:        get("OTP_SENDER", "consola"),
		AllowedOrigins:   splitList(get("APP_ORIGINS", "http://localhost:5173,http://localhost:8080")),
		TrustProxy:       getBool("TRUST_PROXY", false, &errs),
		WebDir:           os.Getenv("WEB_DIR"),
		LogLevel:         get("LOG_LEVEL", "info"),
		DBMaxConns:       int32(getInt("DB_MAX_CONNS", 10, &errs)),
		DBSimpleProtocol: getBool("DB_SIMPLE_PROTOCOL", false, &errs),
		DBMaxLifetime:    getDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute, &errs),
		LoginPerMinute:   float64(getInt("LOGIN_RATE_PER_MINUTE", 10, &errs)),
		StorageDir:       get("STORAGE_DIR", ".data/fotos"),
	}
	c.WhatsApp, c.AI = engine(c.Env, &errs)
	c.Platform = Platform{IncludedMessages: getInt("PLAN_MENSAJES_INCLUIDOS", 1000, &errs), COPPerUSD: getInt("TRM_COP_POR_USD", 4000, &errs)}
	for _, raw := range splitList(os.Getenv("PLATFORM_ADMINS")) {
		phone, err := shared.ParseColombianMobile(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("PLATFORM_ADMINS: %q no es un celular colombiano", raw))
			continue
		}
		c.Platform.Admins = append(c.Platform.Admins, phone)
	}
	if c.Platform.IncludedMessages <= 0 || c.Platform.COPPerUSD <= 0 {
		errs = append(errs, errors.New("PLAN_MENSAJES_INCLUIDOS y TRM_COP_POR_USD deben ser mayores que cero"))
	}
	if err := c.Env.validate(); err != nil {
		errs = append(errs, err)
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL es obligatoria (rol sweveh_api, sin BYPASSRLS)"))
	}
	if c.Env != EnvLocal {
		if c.JWTPrivateKeyPEM == "" {
			errs = append(errs, errors.New("JWT_PRIVATE_KEY_PEM es obligatoria fuera de local"))
		}
		if len(c.OTPPepper) < 32 {
			errs = append(errs, errors.New("OTP_PEPPER debe tener al menos 32 caracteres fuera de local"))
		}
		if c.OTPSender == "consola" || c.OTPSender == "memoria" {
			errs = append(errs, fmt.Errorf("OTP_SENDER=%s solo se permite en local; falta el proveedor real (Q-13/AQ-03)", c.OTPSender))
		}
	} else if c.OTPPepper == "" {
		c.OTPPepper = "solo-desarrollo-local-no-usar-en-otro-entorno"
	}
	switch c.OTPSender {
	case "consola", "memoria":
	default:
		errs = append(errs, fmt.Errorf("OTP_SENDER=%q no es un canal conocido", c.OTPSender))
	}
	return c, errors.Join(errs...)
}

// LoadWorker lee la configuración del worker.
func LoadWorker() (Worker, error) {
	var errs []error
	c := Worker{
		Env:          Env(get("APP_ENV", "local")),
		DatabaseURL:  os.Getenv("WORKER_DATABASE_URL"),
		PollInterval: getDuration("OUTBOX_POLL_INTERVAL", time.Second, &errs),
		BatchSize:    int32(getInt("OUTBOX_BATCH_SIZE", 50, &errs)),
		LogLevel:     get("LOG_LEVEL", "info"),
	}
	c.WhatsApp, c.AI = engine(c.Env, &errs)
	if err := c.Env.validate(); err != nil {
		errs = append(errs, err)
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("WORKER_DATABASE_URL es obligatoria (rol sweveh_worker)"))
	}
	return c, errors.Join(errs...)
}

func (e Env) validate() error {
	switch e {
	case EnvLocal, EnvStaging, EnvProduction:
		return nil
	}
	return fmt.Errorf("APP_ENV=%q no es válido (local, staging, production)", string(e))
}

func get(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		*errs = append(*errs, fmt.Errorf("%s debe ser un entero positivo", key))
		return def
	}
	return n
}

func getBool(key string, def bool, errs *[]error) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s debe ser true o false", key))
		return def
	}
	return b
}

func getDuration(key string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s debe ser una duración como 30s", key))
		return def
	}
	return d
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
