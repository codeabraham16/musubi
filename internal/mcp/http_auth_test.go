package mcp

// Tests de autenticación y gating de bind del transporte HTTP (Track 4 / T4.3).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

func TestHTTPBearerAuth(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, token: "s3cr3t", loopbackOnly: true}))
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	do := func(auth string) int {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+mcpHTTPPath, strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := do(""); code != http.StatusUnauthorized {
		t.Errorf("sin Authorization: status = %d, esperaba 401", code)
	}
	if code := do("Bearer wrong"); code != http.StatusUnauthorized {
		t.Errorf("token incorrecto: status = %d, esperaba 401", code)
	}
	if code := do("Bearer s3cr3t"); code != http.StatusOK {
		t.Errorf("token correcto: status = %d, esperaba 200", code)
	}
}

func TestResolveServiceAuth(t *testing.T) {
	const envName = "MUSUBI_TEST_TOKEN_T43"

	t.Run("loopback sin token: OK", func(t *testing.T) {
		tok, lb, err := resolveServiceAuth(config.ServiceConfig{Addr: "127.0.0.1:7717"})
		if err != nil || tok != "" || !lb {
			t.Fatalf("got tok=%q lb=%v err=%v", tok, lb, err)
		}
	})

	t.Run("no-loopback sin auth_token_env: ERROR", func(t *testing.T) {
		_, _, err := resolveServiceAuth(config.ServiceConfig{Addr: "0.0.0.0:7717"})
		if err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("esperaba error de gating mencionando loopback, got %v", err)
		}
	})

	t.Run("no-loopback con env vacía: ERROR", func(t *testing.T) {
		t.Setenv(envName, "")
		_, _, err := resolveServiceAuth(config.ServiceConfig{Addr: "0.0.0.0:7717", AuthTokenEnv: envName})
		if err == nil {
			t.Fatal("esperaba error: token env vacío en bind no-loopback")
		}
	})

	t.Run("no-loopback con token: OK", func(t *testing.T) {
		t.Setenv(envName, "  abc123  ") // se espera trim
		tok, lb, err := resolveServiceAuth(config.ServiceConfig{Addr: "0.0.0.0:7717", AuthTokenEnv: envName})
		if err != nil || tok != "abc123" || lb {
			t.Fatalf("got tok=%q lb=%v err=%v", tok, lb, err)
		}
	})

	t.Run("loopback con token: OK (token igual aplica)", func(t *testing.T) {
		t.Setenv(envName, "tok")
		tok, lb, err := resolveServiceAuth(config.ServiceConfig{Addr: "127.0.0.1:7717", AuthTokenEnv: envName})
		if err != nil || tok != "tok" || !lb {
			t.Fatalf("got tok=%q lb=%v err=%v", tok, lb, err)
		}
	})

	t.Run("loopback con auth_token_env pero env vacia: ERROR (fail-closed)", func(t *testing.T) {
		t.Setenv(envName, "") // var nombrada pero vacía => auth NO debe quedar silenciosamente off
		_, _, err := resolveServiceAuth(config.ServiceConfig{Addr: "127.0.0.1:7717", AuthTokenEnv: envName})
		if err == nil {
			t.Fatal("esperaba error: auth_token_env nombrada pero vacía no debe deshabilitar auth en silencio")
		}
	})
}

func TestListenAndServeHTTPTLSGating(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("TLS medio-seteado (solo cert): ERROR", func(t *testing.T) {
		err := s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: "127.0.0.1:0", TLSCertFile: "cert.pem"})
		if err == nil || !strings.Contains(err.Error(), "TLS incompleta") {
			t.Fatalf("esperaba error de TLS incompleta, got %v", err)
		}
	})

	t.Run("remoto + token + sin TLS + sin opt-in: ERROR (fail-closed)", func(t *testing.T) {
		t.Setenv("MUSUBI_TLSGATE_TOK", "secret")
		err := s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: "0.0.0.0:0", AuthTokenEnv: "MUSUBI_TLSGATE_TOK"})
		if err == nil || !strings.Contains(err.Error(), "texto plano") {
			t.Fatalf("esperaba error de token en texto plano, got %v", err)
		}
	})
}

func TestValidBearer(t *testing.T) {
	cases := []struct {
		header, want string
		ok           bool
	}{
		{"Bearer abc", "abc", true},
		{"Bearer  abc ", "abc", true}, // trim del valor
		{"abc", "abc", false},         // sin prefijo
		{"Bearer wrong", "abc", false},
		{"", "abc", false},
	}
	for _, c := range cases {
		if got := validBearer(c.header, c.want); got != c.ok {
			t.Errorf("validBearer(%q, %q) = %v, esperaba %v", c.header, c.want, got, c.ok)
		}
	}
}
