package auth

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func newAuth(t *testing.T) *Auth {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st)
}

func TestSetupLoginAndChange(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	if ok, _ := a.HasPassword(ctx); ok {
		t.Fatal("banco novo não deveria ter senha")
	}
	if _, err := a.Setup(ctx, "curta"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("senha curta: %v", err)
	}
	session, err := a.Setup(ctx, "senha-boa-123")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.Session(ctx, session); !ok {
		t.Fatal("sessão do setup deveria valer")
	}
	if _, err := a.Setup(ctx, "outra-senha-123"); !errors.Is(err, ErrPasswordSet) {
		t.Fatalf("segundo setup: %v", err)
	}

	ip := netip.MustParseAddr("192.168.1.20")
	if _, err := a.Login(ctx, ip, "errada-123"); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("login errado: %v", err)
	}
	if _, err := a.Login(ctx, ip, "senha-boa-123"); err != nil {
		t.Fatal(err)
	}

	next, err := a.ChangePassword(ctx, "senha-boa-123", "senha-nova-456")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.Session(ctx, session); ok {
		t.Fatal("trocar a senha deveria encerrar as sessões antigas")
	}
	if ok, _ := a.Session(ctx, next); !ok {
		t.Fatal("sessão nova deveria valer")
	}
	if err := a.Logout(ctx, next); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.Session(ctx, next); ok {
		t.Fatal("logout deveria encerrar a sessão")
	}

	if err := ResetPassword(ctx, a.st); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.HasPassword(ctx); ok {
		t.Fatal("reset deveria apagar a senha")
	}
}

func TestLoginLockout(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	now := time.Unix(1_800_000_000, 0)
	a.now = func() time.Time { return now }
	if _, err := a.Setup(ctx, "senha-boa-123"); err != nil {
		t.Fatal(err)
	}
	ip := netip.MustParseAddr("192.168.1.30")
	for range maxFailures {
		a.Login(ctx, ip, "errada-123")
	}
	var locked LockedError
	if _, err := a.Login(ctx, ip, "senha-boa-123"); !errors.As(err, &locked) {
		t.Fatalf("esperava bloqueio, veio %v", err)
	}
	if _, err := a.Login(ctx, netip.MustParseAddr("192.168.1.31"), "senha-boa-123"); err != nil {
		t.Fatalf("outro IP não deveria ser bloqueado: %v", err)
	}
	now = now.Add(lockoutWindow)
	if _, err := a.Login(ctx, ip, "senha-boa-123"); err != nil {
		t.Fatalf("bloqueio deveria expirar: %v", err)
	}
}

func TestAPIToken(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	token, err := a.NewAPIToken(ctx, "Celular")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.APIToken(ctx, token); !ok {
		t.Fatal("token novo deveria valer")
	}
	if ok, _ := a.APIToken(ctx, token+"x"); ok {
		t.Fatal("token alterado não deveria valer")
	}
	tokens, err := a.st.Tokens(ctx)
	if err != nil || len(tokens) != 1 || tokens[0].LastUsed == 0 {
		t.Fatalf("tokens = %+v, %v", tokens, err)
	}
	if err := a.st.DeleteToken(ctx, tokens[0].ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.APIToken(ctx, token); ok {
		t.Fatal("token revogado não deveria valer")
	}
}
