// Package pinhash guarda el PIN de entrada con argon2id (HU-104, ADR-0037).
//
// Con 4 dígitos hay 10.000 combinaciones: un hash rápido se rompe en segundos si alguien se lleva la base. Por eso
// argon2id con sal por usuario y un pepper del servidor que no está en la base (se mezcla con HMAC-SHA256 antes del
// argon2). Parámetros por defecto de OWASP para argon2id: m = 19 MiB, t = 2, p = 1
// (https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html). Formato PHC:
// $argon2id$v=19$m=19456,t=2,p=1$<sal>$<hash> (base64 sin relleno).
package pinhash

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params son los parámetros de argon2id (PIN_ARGON_MEMORY_KIB, PIN_ARGON_TIME, PIN_ARGON_THREADS).
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

// OWASP es la configuración mínima recomendada (ver el comentario del paquete).
var OWASP = Params{MemoryKiB: 19456, Time: 2, Threads: 1}

const (
	saltLen = 16
	keyLen  = 32
)

// Hasher implementa ports.PINHasher.
type Hasher struct {
	pepper []byte
	p      Params
	dummy  string // hash de relleno para los celulares sin cuenta (mismo costo)
	// sem limita cuántos argon2 corren a la vez (cada uno usa MemoryKiB): muchos ingresos simultáneos esperan su turno
	// en vez de llenar la memoria del servidor.
	sem chan struct{}
}

// New arma el hasher. El pepper es secreto del servidor (PIN_PEPPER).
func New(pepper []byte, p Params) (*Hasher, error) {
	if len(pepper) == 0 {
		return nil, errors.New("pinhash: falta el pepper")
	}
	h := &Hasher{pepper: pepper, p: p, sem: make(chan struct{}, max(2, runtime.NumCPU()))}
	d, err := h.Hash("relleno-que-no-es-un-pin")
	if err != nil {
		return nil, err
	}
	h.dummy = d
	return h, nil
}

// key deriva la clave con argon2id esperando turno en el semáforo.
func (h *Hasher) key(pin string, salt []byte, p Params, n uint32) []byte {
	h.sem <- struct{}{}
	defer func() { <-h.sem }()
	return argon2.IDKey(h.peppered(pin), salt, p.Time, p.MemoryKiB, p.Threads, n)
}

func (h *Hasher) peppered(pin string) []byte {
	m := hmac.New(sha256.New, h.pepper)
	m.Write([]byte(pin))
	return m.Sum(nil)
}

// Hash devuelve el PIN en formato PHC con una sal nueva.
func (h *Hasher) Hash(pin string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := h.key(pin, salt, h.p, keyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.p.MemoryKiB, h.p.Time, h.p.Threads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify compara en tiempo constante con los parámetros guardados en el hash (sirven aunque cambie la configuración).
// Con encoded vacío o dañado compara contra el hash de relleno y devuelve false.
func (h *Hasher) Verify(pin, encoded string) bool {
	p, salt, want, err := decode(encoded)
	if err != nil {
		p, salt, want, _ = decode(h.dummy)
		_ = h.key(pin, salt, p, uint32(len(want)))
		return false
	}
	got := h.key(pin, salt, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("pinhash: formato inválido")
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return Params{}, nil, nil, errors.New("pinhash: versión inválida")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, errors.New("pinhash: parámetros inválidos")
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, err
	}
	key, err := enc.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, errors.New("pinhash: hash inválido")
	}
	return p, salt, key, nil
}
