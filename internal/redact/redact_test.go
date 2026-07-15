package redact

import (
	"strings"
	"testing"
)

func TestRedactKnownSecrets(t *testing.T) {
	// Los secretos de prueba de las reglas nuevas se ARMAN por partes (prefijo + cuerpo) a
	// propósito: así el literal completo NO aparece en el fuente y no dispara el secret-scanning /
	// push protection de GitHub sobre un fixture FALSO. Redact() recibe la cadena ya concatenada,
	// de modo que la prueba ejercita el patrón real igual.
	var (
		ghPat    = "github_pat_" + "11ABCDEFGHIJKLMNOPQRSTUV0123456789"
		gitlab   = "glpat-" + "ABCDEFGHIJ1234567890xyz"
		anthro   = "sk-ant-" + "api03-ABCDEFGHIJKLMNOPQRSTUVWX"
		openai   = "sk-proj-" + "ABCDEFGHIJ0123456789abcdef"
		slackTok = "xoxb-" + "123456789012-ABCDEFGHIJKLM"
		slackHk  = "https://hooks.slack.com/services/" + "T01ABCDEF/B02GHIJKL/aBcDeFgHiJkLmNoPqRsTuV"
		tgram    = "123456789:" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345678"
		sgKey    = "SG." + "ABCDEFGHIJKLMNOPQRSTUV" + "." + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefg"
		twilio   = "SK" + "0123456789abcdef0123456789abcdef"
		npmTok   = "npm_" + "ABCDEFGHIJ0123456789abcdefghij012345"
		connPass = "s3cr3tPass"
	)
	cases := []struct {
		name    string
		input   string
		secret  string // substring que NO debe quedar en la salida
		wantTyp string
	}{
		{"aws", "creds: AKIA1234567890ABCDEF end", "AKIA1234567890ABCDEF", "aws-access-key"},
		{"github", "token ghp_abcdefghij0123456789abcdefghij012345 fin", "ghp_abcdefghij0123456789abcdefghij012345", "github-token"},
		{"stripe", "key sk_live_abcdef0123456789ABCD x", "sk_live_abcdef0123456789ABCD", "stripe-key"},
		{"google", "AIzaSyA1234567890abcdefghijklmnopqrstuvw z", "AIzaSyA1234567890abcdefghijklmnopqrstuvw", "google-api-key"},
		{"jwt", "auth eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0.SflKxwRJSMeKKF2QT4 y", "eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0.SflKxwRJSMeKKF2QT4", "jwt"},
		{"env", "config API_KEY=supersecretvalue123 done", "supersecretvalue123", "env-secret"},
		{"bearer", "Authorization: Bearer abcdefghijklmnop0123 ok", "abcdefghijklmnop0123", "bearer-token"},
		{"github-pat", "pat " + ghPat + " end", ghPat, "github-pat"},
		{"gitlab", "tok " + gitlab + " done", gitlab, "gitlab-token"},
		{"anthropic", "key " + anthro + " end", anthro, "ai-provider-key"},
		{"openai", "key " + openai + " done", openai, "ai-provider-key"},
		{"slack-token", "slack " + slackTok + " end", slackTok, "slack-token"},
		{"slack-webhook", "hook " + slackHk + " done", "T01ABCDEF/B02GHIJKL", "slack-webhook"},
		{"telegram", "bot " + tgram + " ok", tgram, "telegram-bot-token"},
		{"sendgrid", "sg " + sgKey + " end", "SG." + "ABCDEFGHIJKLMNOPQRSTUV", "sendgrid-key"},
		{"twilio", "tw " + twilio + " end", twilio, "twilio-key"},
		{"npm", "npm " + npmTok + " end", npmTok, "npm-token"},
		{"connstring", "db postgres://admin:" + connPass + "@db.internal:5432/app end", connPass, "connstring-password"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clean, finds := Redact(c.input)
			if strings.Contains(clean, c.secret) {
				t.Fatalf("el secreto quedó sin redactar:\n%s", clean)
			}
			if !strings.Contains(clean, "[REDACTED:"+c.wantTyp+"]") {
				t.Fatalf("esperaba [REDACTED:%s], obtuve: %s", c.wantTyp, clean)
			}
			if len(finds) == 0 {
				t.Fatal("esperaba al menos un finding")
			}
		})
	}
}

func TestRedactPEMBlock(t *testing.T) {
	in := "antes\n-----BEGIN RSA PRIVATE KEY-----\nMIIabc123def456ghi789\n-----END RSA PRIVATE KEY-----\ndespués"
	clean, _ := Redact(in)
	if strings.Contains(clean, "MIIabc123def456ghi789") {
		t.Fatalf("la clave privada no se redactó:\n%s", clean)
	}
	if !strings.Contains(clean, "[REDACTED:private-key]") || !strings.Contains(clean, "antes") || !strings.Contains(clean, "después") {
		t.Fatalf("redacción PEM incorrecta:\n%s", clean)
	}
}

func TestRedactHighEntropyCatchAll(t *testing.T) {
	in := "opaque x7Kp2Qm9Zb4Rn8Vc1Ts5Wy0Lj6Hg3Fd7Aa2Be4Cf token"
	clean, _ := Redact(in)
	if strings.Contains(clean, "x7Kp2Qm9Zb4Rn8Vc1Ts5Wy0Lj6Hg3Fd7Aa2Be4Cf") {
		t.Fatalf("token de alta entropía no redactado:\n%s", clean)
	}
	if !strings.Contains(clean, "[REDACTED:high-entropy]") {
		t.Fatalf("esperaba high-entropy, obtuve: %s", clean)
	}
}

func TestRedactDoesNotTouchGitSHAorCleanText(t *testing.T) {
	// git SHA de 40 hex: entropía ~3.9 < 4.5 y el catch-all no cubre hex puro → intacto.
	sha := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	if clean, _ := Redact(sha); clean != sha {
		t.Errorf("un git SHA no debe redactarse; obtuve: %s", clean)
	}
	// Ejemplo canónico de AWS (allowlist): no se redacta.
	ex := "usar AKIAIOSFODNN7EXAMPLE como ejemplo"
	if clean, _ := Redact(ex); clean != ex {
		t.Errorf("el ejemplo AKIA...EXAMPLE no debe redactarse; obtuve: %s", clean)
	}
	// Prosa normal: intacta.
	prose := "Elegimos SQLite en vez de Postgres porque es embebido y model-free."
	if clean, finds := Redact(prose); clean != prose || finds != nil {
		t.Errorf("la prosa no debe tocarse; obtuve: %s (%d finds)", clean, len(finds))
	}
	// Connection string con contraseña PLACEHOLDER: la allowlist (your_) la deja pasar.
	conn := "ejemplo: postgres://user:your_password@host:5432/db"
	if clean, _ := Redact(conn); clean != conn {
		t.Errorf("un connstring con password placeholder no debe redactarse; obtuve: %s", clean)
	}
}

func TestShannonEntropy(t *testing.T) {
	if h := shannonEntropy("aaaaaaaa"); h != 0 {
		t.Errorf("entropía de un char repetido debe ser 0, obtuve %v", h)
	}
	if h := shannonEntropy("x7Kp2Qm9Zb4Rn8Vc1Ts5Wy0Lj6Hg"); h < entropyThreshold {
		t.Errorf("un token aleatorio debería superar el umbral, obtuve %v", h)
	}
}
