package app

// Pruebas del caso de uso con dobles en memoria: cubren lo que la prueba de punta a punta
// no alcanza (fallas de infraestructura, usuarios y membresías inactivos) sin Postgres.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/platform/clock"
	"sweveh/internal/shared"
)

var errBoom = errors.New("falla de infraestructura")

type fakeStore struct {
	failOn     string
	challenges []domain.Challenge
	locks      map[string]domain.Lock
	users      map[string]ports.User
	members    map[shared.ID][]ports.Membership
	refresh    map[string]ports.RefreshRecord
	sessions   map[shared.ID]domain.Session
}

func newFakeStore() *fakeStore {
	return &fakeStore{locks: map[string]domain.Lock{}, users: map[string]ports.User{},
		members: map[shared.ID][]ports.Membership{}, refresh: map[string]ports.RefreshRecord{},
		sessions: map[shared.ID]domain.Session{}}
}

func (f *fakeStore) fail(m string) error {
	if f.failOn == m {
		return errBoom
	}
	return nil
}

func (f *fakeStore) LatestChallengeTime(_ context.Context, phone string) (*time.Time, error) {
	if err := f.fail("LatestChallengeTime"); err != nil {
		return nil, err
	}
	for i := len(f.challenges) - 1; i >= 0; i-- {
		if f.challenges[i].Phone == phone {
			t := f.challenges[i].CreatedAt
			return &t, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) LatestOpenChallenge(_ context.Context, phone string) (domain.Challenge, bool, error) {
	if err := f.fail("LatestOpenChallenge"); err != nil {
		return domain.Challenge{}, false, err
	}
	for i := len(f.challenges) - 1; i >= 0; i-- {
		if c := f.challenges[i]; c.Phone == phone && c.UsedAt == nil {
			return c, true, nil
		}
	}
	return domain.Challenge{}, false, nil
}

func (f *fakeStore) InsertChallenge(_ context.Context, c domain.Challenge) error {
	if err := f.fail("InsertChallenge"); err != nil {
		return err
	}
	f.challenges = append(f.challenges, c)
	return nil
}

func (f *fakeStore) CloseOpenChallenges(_ context.Context, phone string, at time.Time) error {
	if err := f.fail("CloseOpenChallenges"); err != nil {
		return err
	}
	for i := range f.challenges {
		if f.challenges[i].Phone == phone && f.challenges[i].UsedAt == nil {
			f.challenges[i].UsedAt = &at
		}
	}
	return nil
}

func (f *fakeStore) Lock(_ context.Context, phone string) (domain.Lock, error) {
	return f.locks[phone], f.fail("Lock")
}

func (f *fakeStore) LockForUpdate(_ context.Context, phone string) (domain.Lock, error) {
	return f.locks[phone], f.fail("LockForUpdate")
}

func (f *fakeStore) SaveLock(_ context.Context, phone string, l domain.Lock, _ time.Time) error {
	f.locks[phone] = l
	return f.fail("SaveLock")
}

func (f *fakeStore) UserByPhone(_ context.Context, phone string) (ports.User, bool, error) {
	u, ok := f.users[phone]
	return u, ok, f.fail("UserByPhone")
}

func (f *fakeStore) UserByID(_ context.Context, id shared.ID) (ports.User, error) {
	if err := f.fail("UserByID"); err != nil {
		return ports.User{}, err
	}
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return ports.User{}, domain.ErrSessionInvalid
}

func (f *fakeStore) InsertUser(_ context.Context, u ports.User, _ time.Time) error {
	if err := f.fail("InsertUser"); err != nil {
		return err
	}
	f.users[u.Phone] = u
	return nil
}

func (f *fakeStore) InsertMembership(_ context.Context, m ports.Membership, userID shared.ID, _ time.Time) error {
	if err := f.fail("InsertMembership"); err != nil {
		return err
	}
	f.members[userID] = append(f.members[userID], m)
	return nil
}

func (f *fakeStore) ActiveMemberships(_ context.Context, userID shared.ID) ([]ports.Membership, error) {
	return f.members[userID], f.fail("ActiveMemberships")
}

func (f *fakeStore) ActiveRole(_ context.Context, tallerID, userID shared.ID) (string, bool, error) {
	if err := f.fail("ActiveRole"); err != nil {
		return "", false, err
	}
	for _, m := range f.members[userID] {
		if m.TallerID == tallerID {
			return m.Role, true, nil
		}
	}
	return "", false, nil
}

func (f *fakeStore) InsertConsent(context.Context, domain.Consent) error {
	return f.fail("InsertConsent")
}

func (f *fakeStore) InsertSession(_ context.Context, s domain.Session, _ time.Time) error {
	f.sessions[s.ID] = s
	return f.fail("InsertSession")
}

func (f *fakeStore) RevokeSession(_ context.Context, id shared.ID, at time.Time, _ string) error {
	s := f.sessions[id]
	s.RevokedAt = &at
	f.sessions[id] = s
	for k, r := range f.refresh {
		if r.SessionID == id {
			r.RevokedAt = &at
			f.refresh[k] = r
		}
	}
	return f.fail("RevokeSession")
}

func (f *fakeStore) InsertRefresh(_ context.Context, hash []byte, sessionID shared.ID, expires, _ time.Time) error {
	if err := f.fail("InsertRefresh"); err != nil {
		return err
	}
	s := f.sessions[sessionID]
	f.refresh[string(hash)] = ports.RefreshRecord{SessionID: sessionID, UserID: s.UserID, TallerID: s.TallerID,
		ExpiresAt: expires, SessionAbs: s.ExpiresAbs, RevokedAt: s.RevokedAt}
	return nil
}

func (f *fakeStore) RefreshForUpdate(_ context.Context, hash []byte) (ports.RefreshRecord, bool, error) {
	r, ok := f.refresh[string(hash)]
	return r, ok, f.fail("RefreshForUpdate")
}

func (f *fakeStore) MarkRefreshUsed(_ context.Context, hash []byte, at time.Time) error {
	r := f.refresh[string(hash)]
	r.UsedAt = &at
	f.refresh[string(hash)] = r
	return f.fail("MarkRefreshUsed")
}

type fakeTx struct{ failSet bool }

func (fakeTx) DoWithSettings(ctx context.Context, _ map[string]string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (t fakeTx) SetLocal(context.Context, string, string) error {
	if t.failSet {
		return errBoom
	}
	return nil
}

type fakeSender struct {
	last string
	err  error
}

func (s *fakeSender) Send(_ context.Context, _, code string) error { s.last = code; return s.err }

type fakeTokens struct{ err error }

func (t fakeTokens) Issue(shared.ID, shared.ID, shared.ID, []string) (string, time.Time, error) {
	return "token", time.Now().Add(10 * time.Minute), t.err
}

type fakeWorkshops struct{ err error }

func (w fakeWorkshops) Create(_ context.Context, id shared.ID, name string, _ shared.Actor, _ time.Time) (ports.Workshop, error) {
	return ports.Workshop{ID: id, Name: name}, w.err
}

func (w fakeWorkshops) Get(_ context.Context, id shared.ID) (ports.Workshop, error) {
	return ports.Workshop{ID: id, Name: "Taller"}, w.err
}

type fakeAudit struct{ err error }

func (a fakeAudit) Record(context.Context, audit.Entry) error { return a.err }

type fakeEvents struct{ err error }

func (e fakeEvents) Publish(context.Context, shared.ID, shared.Event) error { return e.err }

type fixture struct {
	svc    *Service
	store  *fakeStore
	sender *fakeSender
	clk    *clock.Fake
}

type opts struct {
	tx        fakeTx
	tokens    fakeTokens
	workshops fakeWorkshops
	audit     fakeAudit
	events    fakeEvents
	sendErr   error
}

func newFixture(o opts) fixture {
	st := newFakeStore()
	snd := &fakeSender{err: o.sendErr}
	clk := clock.NewFake(time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC))
	svc := NewService(Deps{Tx: o.tx, Store: st, Sender: snd, Tokens: o.tokens, Workshops: o.workshops,
		Audit: o.audit, Events: o.events, Clock: clk, Pepper: []byte("pepper"),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return fixture{svc: svc, store: st, sender: snd, clk: clk}
}

const phone = "3105550301"

func (f fixture) registerOwner(t *testing.T) Session {
	t.Helper()
	if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
		t.Fatal(err)
	}
	s, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last, Name: "Dueño", ConsentAccepted: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRequestCode_PropagatesFailures(t *testing.T) {
	for _, m := range []string{"Lock", "LatestChallengeTime", "CloseOpenChallenges", "InsertChallenge"} {
		f := newFixture(opts{})
		f.store.failOn = m
		if _, err := f.svc.RequestCode(context.Background(), phone); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", m, err)
		}
	}
	// Si el canal no entrega el código, la persona ve un mensaje claro en vez de un error interno (ADR-0034).
	f := newFixture(opts{sendErr: errBoom})
	if _, err := f.svc.RequestCode(context.Background(), phone); !errors.Is(err, domain.ErrCodeNotSent) {
		t.Errorf("envío fallido: %v", err)
	}
}

func TestVerify_PropagatesFailures(t *testing.T) {
	for _, m := range []string{"LockForUpdate", "LatestOpenChallenge", "UserByPhone", "CloseOpenChallenges", "SaveLock",
		"InsertUser", "InsertMembership", "InsertConsent", "InsertSession", "InsertRefresh"} {
		f := newFixture(opts{})
		if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
			t.Fatal(err)
		}
		f.store.failOn = m
		_, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last, Name: "D", ConsentAccepted: true})
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", m, err)
		}
	}
	for name, o := range map[string]opts{
		"taller":    {workshops: fakeWorkshops{err: errBoom}},
		"auditoría": {audit: fakeAudit{err: errBoom}},
		"evento":    {events: fakeEvents{err: errBoom}},
		"token":     {tokens: fakeTokens{err: errBoom}},
	} {
		f := newFixture(o)
		if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
			t.Fatal(err)
		}
		_, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last, Name: "D", ConsentAccepted: true})
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestVerify_WrongCodeSaveLockFailure(t *testing.T) {
	f := newFixture(opts{})
	if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
		t.Fatal(err)
	}
	f.store.failOn = "SaveLock"
	wrong := "000000"
	if f.sender.last == wrong {
		wrong = "111111"
	}
	if _, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: wrong}); !errors.Is(err, errBoom) {
		t.Fatalf("guardar el fallo: %v", err)
	}
}

func TestLogin_InactiveUserAndNoMembership(t *testing.T) {
	f := newFixture(opts{})
	s := f.registerOwner(t)
	f.clk.Advance(time.Minute)

	// Sin membresía activa.
	delete(f.store.members, s.User.ID)
	if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last}); !errors.Is(err, domain.ErrNoMembership) {
		t.Fatalf("sin membresía: %v", err)
	}
	// Usuario desactivado.
	f.clk.Advance(time.Minute)
	u := f.store.users["+57"+phone]
	u.Active = false
	f.store.users["+57"+phone] = u
	if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last}); !errors.Is(err, domain.ErrNoMembership) {
		t.Fatalf("usuario inactivo: %v", err)
	}
}

func TestLogin_PropagatesFailures(t *testing.T) {
	cases := map[string]func(*fixture){
		"membresías": func(f *fixture) { f.store.failOn = "ActiveMemberships" },
	}
	for name, mut := range cases {
		f := newFixture(opts{})
		f.registerOwner(t)
		f.clk.Advance(time.Minute)
		if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
			t.Fatal(err)
		}
		mut(&f)
		if _, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last}); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// SetLocal y taller fallan al entrar.
	for name, o := range map[string]opts{"set_local": {tx: fakeTx{failSet: true}}} {
		f := newFixture(o)
		f.registerOwner(t)
		f.clk.Advance(time.Minute)
		if _, err := f.svc.RequestCode(context.Background(), phone); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Verify(context.Background(), VerifyInput{Phone: phone, Code: f.sender.last}); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRefresh_Paths(t *testing.T) {
	f := newFixture(opts{})
	s := f.registerOwner(t)
	if _, err := f.svc.Refresh(context.Background(), ""); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("vacío: %v", err)
	}
	if _, err := f.svc.Refresh(context.Background(), "no-existe"); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("desconocido: %v", err)
	}
	// Membresía inactiva: se revoca la sesión.
	saved := f.store.members[s.User.ID]
	delete(f.store.members, s.User.ID)
	if _, err := f.svc.Refresh(context.Background(), s.RefreshToken); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("membresía inactiva: %v", err)
	}
	if f.store.sessions[f.store.refresh[string(hashRefresh(s.RefreshToken))].SessionID].RevokedAt == nil {
		t.Fatal("la sesión debía quedar revocada")
	}
	f.store.members[s.User.ID] = saved

	for _, m := range []string{"RefreshForUpdate", "ActiveRole", "MarkRefreshUsed", "InsertRefresh", "UserByID"} {
		g := newFixture(opts{})
		gs := g.registerOwner(t)
		g.store.failOn = m
		if _, err := g.svc.Refresh(context.Background(), gs.RefreshToken); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", m, err)
		}
	}
	for name, o := range map[string]opts{"set_local": {tx: fakeTx{failSet: true}}, "token": {tokens: fakeTokens{}}} {
		g := newFixture(o)
		gs := g.registerOwner(t)
		if name == "token" {
			g.svc.tokens = fakeTokens{err: errBoom}
		}
		if _, err := g.svc.Refresh(context.Background(), gs.RefreshToken); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Reutilización con falla al revocar.
	g := newFixture(opts{})
	gs := g.registerOwner(t)
	if _, err := g.svc.Refresh(context.Background(), gs.RefreshToken); err != nil {
		t.Fatal(err)
	}
	g.store.failOn = "RevokeSession"
	if _, err := g.svc.Refresh(context.Background(), gs.RefreshToken); !errors.Is(err, errBoom) {
		t.Fatalf("revocar por reuso: %v", err)
	}
}

func TestLogoutAndMe(t *testing.T) {
	f := newFixture(opts{})
	s := f.registerOwner(t)
	if err := f.svc.Logout(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	f.store.failOn = "RefreshForUpdate"
	if err := f.svc.Logout(context.Background(), s.RefreshToken); !errors.Is(err, errBoom) {
		t.Fatalf("logout: %v", err)
	}
	f.store.failOn = "UserByID"
	if _, _, err := f.svc.Me(context.Background(), s.User.ID, s.Workshop.ID); !errors.Is(err, errBoom) {
		t.Fatalf("me: %v", err)
	}
	f.store.failOn = ""
	u, ws, err := f.svc.Me(context.Background(), s.User.ID, s.Workshop.ID)
	if err != nil || u.ID != s.User.ID || ws.ID != s.Workshop.ID {
		t.Fatalf("me: %+v %+v %v", u, ws, err)
	}
}
