// Package redact es una guarda de secretos model-free (sin LLM, sin red, sin dependencias):
// detecta y tapa credenciales en un texto antes de que cruce a la memoria COMPARTIDA del
// cerebro. Es la pieza de seguridad de la captura automática shared-by-default (Fase C2): un
// secreto que el agente capture NO debe terminar en el cerebro que ve todo el equipo. Combina
// reglas por forma (estilo gitleaks, RE2-compatibles) con un catch-all de entropía de Shannon
// para formatos desconocidos. Determinista: mismo texto ⇒ misma salida.
package redact

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Finding es un secreto localizado: su tipo legible y el rango [Start,End) en el texto original.
type Finding struct {
	Type  string
	Start int
	End   int
}

// rule es una regla de detección por forma. Si valueGroup > 0, se redacta SOLO ese grupo de
// captura (p.ej. el valor de KEY=valor, preservando el nombre de la clave); si es 0, el match
// completo.
type rule struct {
	typ        string
	re         *regexp.Regexp
	valueGroup int
}

// entropyThreshold y minTokenLen calibran el catch-all de entropía. Umbral 4.5 bits/char sobre
// tokens base64-ish largos: pega en secretos aleatorios (entropía ~5.5-6) y deja pasar prosa y
// git SHAs de 40 hex (entropía ~3.9, y además el token de entropía NO cubre hex puro).
const (
	entropyThreshold = 4.5
	minTokenLen      = 20
)

var (
	// rules: reglas ancladas por forma. Todas RE2 (sin lookahead).
	rules = []rule{
		{"aws-access-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), 0},
		{"github-token", regexp.MustCompile(`\bgh[opsur]_[0-9A-Za-z]{20,}\b`), 0},
		// GitHub personal access token FINO (github_pat_...): NO lo cubre la regla gh[opsur]_.
		{"github-pat", regexp.MustCompile(`\bgithub_pat_[0-9A-Za-z_]{20,}`), 0},
		{"gitlab-token", regexp.MustCompile(`\bglpat-[0-9A-Za-z_\-]{20,}`), 0},
		{"stripe-key", regexp.MustCompile(`\b(?:sk|rk)_live_[0-9A-Za-z]{16,}\b`), 0},
		// Claves de proveedores de IA (sk-ant- Anthropic, sk-proj-/sk- OpenAI). La usa el propio
		// Musubi: una filtrada en la memoria compartida sería grave. Separador `-` (no `_` de Stripe).
		{"ai-provider-key", regexp.MustCompile(`\bsk-(?:ant-|proj-)?[0-9A-Za-z_\-]{20,}`), 0},
		{"google-api-key", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35,}\b`), 0},
		{"slack-token", regexp.MustCompile(`\bxox[baprse]-[0-9A-Za-z\-]{10,}`), 0},
		{"slack-webhook", regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Za-z0-9/]+`), 0},
		// Token de bot de Telegram (\d{8,10}:base64{35}): lo usa el gateway de chat de Musubi.
		{"telegram-bot-token", regexp.MustCompile(`\b\d{8,10}:[0-9A-Za-z_\-]{35}\b`), 0},
		{"sendgrid-key", regexp.MustCompile(`\bSG\.[0-9A-Za-z_\-]{22}\.[0-9A-Za-z_\-]{43}\b`), 0},
		{"twilio-key", regexp.MustCompile(`\bSK[0-9a-f]{32}\b`), 0},
		{"npm-token", regexp.MustCompile(`\bnpm_[0-9A-Za-z]{36}\b`), 0},
		{"jwt", regexp.MustCompile(`\beyJ[0-9A-Za-z_\-]{6,}\.[0-9A-Za-z_\-]{6,}\.[0-9A-Za-z_\-]{6,}\b`), 0},
		{"private-key", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), 0},
		{"bearer-token", regexp.MustCompile(`(?i)bearer\s+([A-Za-z0-9._\-]{16,})`), 1},
		// KEY=valor / KEY: valor para claves sensibles: redacta SOLO el valor (grupo 2).
		{"env-secret", regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API_?KEY|PRIVATE_?KEY|ACCESS_?KEY|AUTH))\b\s*[:=]\s*["']?([^\s"']{6,})`), 2},
		// Contraseña embebida en un connection string (scheme://user:PASS@host): redacta SOLO la
		// contraseña (grupo 1). Las passwords humanas son de BAJA entropía, así que el catch-all NO
		// las ve — pero un postgres://u:p@host filtrado es una fuga real. Cubre postgres/redis/
		// mongodb/amqp/etc. de una.
		{"connstring-password", regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://[^:@/\s]*:([^@/\s]{3,})@`), 1},
	}
	// entropyToken: candidatos base64-ish para el catch-all (NO hex puro, para no pegar SHAs).
	entropyToken = regexp.MustCompile(`[A-Za-z0-9+/_\-]{` + itoa(minTokenLen) + `,}`)
	// placeholderRe: ejemplos/plantillas que NO son secretos reales.
	placeholderRe = regexp.MustCompile(`(?i)example|xxxx+|your[_-]|placeholder|dummy|<[^>]+>|\.\.\.`)
)

// Redact devuelve el texto con cada secreto reemplazado por `[REDACTED:<tipo>]`, y la lista de
// hallazgos. Determinista y sin efectos: reglas por forma + catch-all de entropía, con allowlist
// de placeholders. Si no hay hallazgos, devuelve el texto tal cual.
func Redact(text string) (string, []Finding) {
	var finds []Finding

	// 1. Reglas por forma.
	for _, r := range rules {
		for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
			s, e := m[0], m[1]
			if r.valueGroup > 0 && len(m) > 2*r.valueGroup+1 && m[2*r.valueGroup] >= 0 {
				s, e = m[2*r.valueGroup], m[2*r.valueGroup+1]
			}
			if isAllowlisted(text[s:e]) {
				continue
			}
			finds = append(finds, Finding{r.typ, s, e})
		}
	}

	// 2. Catch-all de entropía sobre tokens base64-ish no cubiertos por una regla.
	for _, m := range entropyToken.FindAllStringIndex(text, -1) {
		s, e := m[0], m[1]
		if overlaps(finds, s, e) {
			continue
		}
		tok := text[s:e]
		if isAllowlisted(tok) || shannonEntropy(tok) < entropyThreshold {
			continue
		}
		finds = append(finds, Finding{"high-entropy", s, e})
	}

	if len(finds) == 0 {
		return text, nil
	}

	// 3. Ordenar y descartar solapamientos (se queda el que empieza antes).
	sort.Slice(finds, func(i, j int) bool { return finds[i].Start < finds[j].Start })
	kept := finds[:0:0]
	lastEnd := -1
	for _, f := range finds {
		if f.Start < lastEnd {
			continue
		}
		kept = append(kept, f)
		lastEnd = f.End
	}

	// 4. Reconstruir el texto redactado.
	var b strings.Builder
	prev := 0
	for _, f := range kept {
		b.WriteString(text[prev:f.Start])
		b.WriteString("[REDACTED:" + f.Type + "]")
		prev = f.End
	}
	b.WriteString(text[prev:])
	return b.String(), kept
}

// shannonEntropy calcula la entropía de Shannon en bits por carácter de s.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var freq [256]float64
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	n := float64(len(s))
	var h float64
	for _, c := range freq {
		if c == 0 {
			continue
		}
		p := c / n
		h -= p * math.Log2(p)
	}
	return h
}

// isAllowlisted indica si el fragmento es un placeholder/ejemplo conocido (no un secreto real).
func isAllowlisted(s string) bool {
	return placeholderRe.MatchString(s)
}

// overlaps indica si [s,e) se solapa con algún hallazgo ya registrado.
func overlaps(finds []Finding, s, e int) bool {
	for _, f := range finds {
		if s < f.End && f.Start < e {
			return true
		}
	}
	return false
}

// itoa evita importar strconv solo para armar el patrón de longitud del token de entropía.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
