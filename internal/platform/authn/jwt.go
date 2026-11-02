// Package authn emite y verifica los tokens de acceso del realm taller y protege las rutas
// (docs/06 §6.3, ADR-12). El algoritmo es fijo (ES256); nunca se toma del encabezado del token.
package authn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"sweveh/internal/platform/clock"
	"sweveh/internal/shared"
)

// Valores de docs/06 §6.3 (Propuesta).
const (
	AccessTTL    = 10 * time.Minute
	IssuerTaller = "sweveh-taller"
	AudienceAPI  = "sweveh-api"
	RealmTaller  = "taller"
)

// AccessClaims son los claims del token de acceso del realm taller.
type AccessClaims struct {
	TallerID  string   `json:"taller_id"`
	SessionID string   `json:"sid"`
	Realm     string   `json:"realm"`
	Roles     []string `json:"roles"`
	jwt.RegisteredClaims
}

// Signer firma tokens de acceso con una clave ES256 identificada por kid.
type Signer struct {
	key   *ecdsa.PrivateKey
	kid   string
	clock clock.Clock
}

// ParsePrivateKeyPEM lee una clave EC P-256 en PEM (PKCS#8 o SEC 1).
func ParsePrivateKeyPEM(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("authn: PEM inválido")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok || ec.Curve != elliptic.P256() {
			return nil, errors.New("authn: la clave debe ser EC P-256")
		}
		return ec, nil
	}
	ec, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("authn: clave EC inválida: %w", err)
	}
	if ec.Curve != elliptic.P256() {
		return nil, errors.New("authn: la clave debe ser EC P-256")
	}
	return ec, nil
}

// GenerateDevKey crea una clave efímera. Solo para APP_ENV=local o pruebas.
func GenerateDevKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func NewSigner(key *ecdsa.PrivateKey, kid string, c clock.Clock) *Signer {
	return &Signer{key: key, kid: kid, clock: c}
}

// AccessToken es un token firmado con su vencimiento.
type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

// Issue firma un token de acceso de 10 minutos.
func (s *Signer) Issue(userID, tallerID, sessionID shared.ID, roles []string) (AccessToken, error) {
	now := s.clock.Now()
	exp := now.Add(AccessTTL)
	claims := AccessClaims{
		TallerID:  tallerID.String(),
		SessionID: sessionID.String(),
		Realm:     RealmTaller,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    IssuerTaller,
			Audience:  jwt.ClaimStrings{AudienceAPI},
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        shared.NewID().String(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = s.kid
	v, err := tok.SignedString(s.key)
	if err != nil {
		return AccessToken{}, fmt.Errorf("authn: firmar: %w", err)
	}
	return AccessToken{Value: v, ExpiresAt: exp}, nil
}

// Verifier valida tokens. keys guarda la clave actual y la anterior por kid (rotación).
type Verifier struct {
	keys  map[string]*ecdsa.PublicKey
	clock clock.Clock
}

func NewVerifier(keys map[string]*ecdsa.PublicKey, c clock.Clock) *Verifier {
	return &Verifier{keys: keys, clock: c}
}

// ErrInvalidToken es la respuesta única para cualquier token inválido (no da pistas).
var ErrInvalidToken = shared.Unauthorized("token_invalido", "Tu sesión terminó. Vuelve a entrar.")

// Verify valida firma, algoritmo, emisor, audiencia, vencimiento y realm.
func (v *Verifier) Verify(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		k, ok := v.keys[kid]
		if !ok {
			return nil, errors.New("kid desconocido")
		}
		return k, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(IssuerTaller),
		jwt.WithAudience(AudienceAPI),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
		jwt.WithTimeFunc(v.clock.Now),
	)
	if err != nil || claims.Realm != RealmTaller {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
