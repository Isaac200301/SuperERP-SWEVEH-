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
}

// WhatsApp configura el canal (HU-101). Sin token se usa el canal de prueba local (no envía nada a Meta).
type WhatsApp struct {
	AppSecret     string
	AccessToken   string
	PhoneNumberID string
	VerifyToken   string
	APIBase       string
}

// AI configura el proveedor del agente (D-08 abierta). "reglas" = clasificador local sin costo.
type AI struct {
	Provider           string
	APIKey             string
	Model              string
	MonthlyCapMicroUSD int64
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

func engine(errs *[]error) (WhatsApp, AI) {
	return WhatsApp{
		AppSecret: os.Getenv("WHATSAPP_APP_SECRET"), AccessToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		PhoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), VerifyToken: os.Getenv("WHATSAPP_VERIFY_TOKEN"),
		APIBase: get("WHATSAPP_API_BASE", "https://graph.facebook.com/v21.0"),
	}, AI{Provider: get("AI_PROVIDER", "reglas"), APIKey: os.Getenv("AI_API_KEY"), Model: os.Getenv("AI_MODEL"),
		MonthlyCapMicroUSD: int64(getInt("AI_MONTHLY_CAP_MICRO_USD", 5_000_000, errs))}
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
	c.WhatsApp, c.AI = engine(&errs)
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
	c.WhatsApp, c.AI = engine(&errs)
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
