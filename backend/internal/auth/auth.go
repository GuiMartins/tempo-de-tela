// Package auth protege o painel (senha + sessão por cookie) e a API (token de pareamento).
package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

const (
	passwordKey   = "admin_password"
	iterations    = 600_000
	SessionTTL    = 30 * 24 * time.Hour
	maxFailures   = 5
	lockoutWindow = 15 * time.Minute
	TokenPrefix   = "tdt_"
)

var (
	ErrPasswordSet  = errors.New("a senha do painel já foi definida")
	ErrBadPassword  = errors.New("senha incorreta")
	ErrWeakPassword = errors.New("a senha precisa ter pelo menos 8 caracteres")
)

// LockedError informa quanto tempo falta para liberar novas tentativas de login.
type LockedError struct{ Wait time.Duration }

func (e LockedError) Error() string {
	min := int(e.Wait.Round(time.Minute) / time.Minute)
	if min < 1 {
		min = 1
	}
	return fmt.Sprintf("muitas tentativas erradas; tente de novo em %d min", min)
}

type failure struct {
	count int
	first time.Time
}

type Auth struct {
	st  *store.Store
	now func() time.Time

	mu       sync.Mutex
	failures map[netip.Addr]*failure
}

func New(st *store.Store) *Auth {
	return &Auth{st: st, now: time.Now, failures: map[netip.Addr]*failure{}}
}

func (a *Auth) HasPassword(ctx context.Context) (bool, error) {
	_, ok, err := a.st.GetSetting(ctx, passwordKey)
	return ok, err
}

// Setup define a primeira senha e abre uma sessão. Falha se a senha já existir.
func (a *Auth) Setup(ctx context.Context, password string) (string, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	ok, err := a.st.SetSettingIfAbsent(ctx, passwordKey, hash)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrPasswordSet
	}
	return a.newSession(ctx)
}

// ChangePassword troca a senha, encerra todas as sessões e abre uma nova.
func (a *Auth) ChangePassword(ctx context.Context, current, next string) (string, error) {
	if err := a.check(ctx, current); err != nil {
		return "", err
	}
	hash, err := hashPassword(next)
	if err != nil {
		return "", err
	}
	if err := a.st.SetSetting(ctx, passwordKey, hash); err != nil {
		return "", err
	}
	if err := a.st.DeleteSessions(ctx); err != nil {
		return "", err
	}
	return a.newSession(ctx)
}

// Login confere a senha, com bloqueio temporário por IP depois de várias falhas.
func (a *Auth) Login(ctx context.Context, client netip.Addr, password string) (string, error) {
	client = client.Unmap().WithZone("")
	now := a.now()
	a.mu.Lock()
	f := a.failures[client]
	if f != nil && now.Sub(f.first) >= lockoutWindow {
		delete(a.failures, client)
		f = nil
	}
	if f != nil && f.count >= maxFailures {
		wait := lockoutWindow - now.Sub(f.first)
		a.mu.Unlock()
		return "", LockedError{wait}
	}
	a.mu.Unlock()

	if err := a.check(ctx, password); err != nil {
		if errors.Is(err, ErrBadPassword) {
			a.mu.Lock()
			if f := a.failures[client]; f != nil {
				f.count++
			} else {
				a.failures[client] = &failure{count: 1, first: now}
			}
			a.mu.Unlock()
		}
		return "", err
	}
	a.mu.Lock()
	delete(a.failures, client)
	a.mu.Unlock()
	return a.newSession(ctx)
}

func (a *Auth) check(ctx context.Context, password string) error {
	stored, ok, err := a.st.GetSetting(ctx, passwordKey)
	if err != nil {
		return err
	}
	if !ok || !verifyPassword(stored, password) {
		return ErrBadPassword
	}
	return nil
}

func (a *Auth) Session(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return a.st.ValidSession(ctx, digest(token), a.now())
}

func (a *Auth) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return a.st.DeleteSession(ctx, digest(token))
}

func (a *Auth) newSession(ctx context.Context) (string, error) {
	token := rand.Text()
	now := a.now()
	if err := a.st.CreateSession(ctx, digest(token), now, now.Add(SessionTTL)); err != nil {
		return "", err
	}
	return token, nil
}

// NewAPIToken cria um token de pareamento. O texto só existe nesta resposta;
// o banco guarda o hash.
func (a *Auth) NewAPIToken(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "App Android"
	}
	if utf8.RuneCountInString(name) > 64 {
		return "", errors.New("nome do pareamento muito longo (máximo 64 caracteres)")
	}
	token := TokenPrefix + rand.Text() + rand.Text()
	if _, err := a.st.CreateToken(ctx, name, digest(token), a.now()); err != nil {
		return "", err
	}
	return token, nil
}

func (a *Auth) APIToken(ctx context.Context, token string) (bool, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return false, nil
	}
	return a.st.UseToken(ctx, digest(token), a.now())
}

// ResetPassword apaga a senha e as sessões; o próximo acesso ao painel define uma nova.
func ResetPassword(ctx context.Context, st *store.Store) error {
	if err := st.DeleteSetting(ctx, passwordKey); err != nil {
		return err
	}
	return st.DeleteSessions(ctx)
}

func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 8 {
		return "", ErrWeakPassword
	}
	if len(password) > 1024 {
		return "", errors.New("senha longa demais")
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func verifyPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || len(password) > 1024 {
		return false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 1 || n > 10_000_000 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, n, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
