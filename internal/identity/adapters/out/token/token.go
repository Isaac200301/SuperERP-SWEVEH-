// Package token adapta el firmador de platform/authn al puerto TokenIssuer.
package token

import (
	"time"

	"sweveh/internal/platform/authn"
	"sweveh/internal/shared"
)

type Issuer struct{ signer *authn.Signer }

func NewIssuer(s *authn.Signer) Issuer { return Issuer{signer: s} }

func (i Issuer) Issue(userID, tallerID, sessionID shared.ID, roles []string) (string, time.Time, error) {
	t, err := i.signer.Issue(userID, tallerID, sessionID, roles)
	return t.Value, t.ExpiresAt, err
}
