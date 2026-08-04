package privacy

import (
	"strings"
	"testing"
)

// Secretos de prueba. Elegidos para que los detecte internal/redact y para NO caer en su allowlist
// de placeholders (que descarta cualquier cosa con "example", "dummy", "your_", "xxxx", "<...>").
const (
	secAWS    = "AKIA1234567890ABCDEF"
	secGitHub = "ghp_aBcDeFgHiJkLmNoPqRsT1234"
	secAnt    = "sk-ant-api03-QwErTyUiOpAsDfGhJkLzXcVbNm1234567890"
	secJWT    = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVPmB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

// containsAny falla si alguno de los secretos sobrevive en el texto. Es el chequeo que sostiene R0.
func containsAny(t *testing.T, text string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(text, s) {
			t.Fatalf("FUGA: el secreto %q sobrevivió en el texto:\n%s", s, text)
		}
	}
}

// --- R1: reversibilidad exacta -------------------------------------------------------------

func TestR1RoundTripEsExacto(t *testing.T) {
	casos := map[string]string{
		"vacío":              "",
		"sin secretos":       "esto es prosa común y corriente, sin nada que tapar.",
		"un secreto":         "la clave es " + secAWS + " y nada más",
		"al principio":       secAWS + " arranca el texto",
		"al final":           "termina con " + secGitHub,
		"solo el secreto":    secAnt,
		"dos distintos":      "primero " + secAWS + " y después " + secGitHub,
		"dos pegados":        secAWS + " " + secGitHub,
		"mismo repetido":     "usá " + secAWS + " y de nuevo " + secAWS + " otra vez",
		"multilínea":         "línea uno\nclave: " + secAnt + "\nlínea tres\n",
		"jwt":                "Authorization: " + secJWT,
		"env secret":         "API_KEY=s3cr3tovalorquenadiedeberiaver",
		"connstring":         "postgres://usuario:contrasena123@db.interno:5432/base",
		"acentos y emoji":    "configurá la clave " + secGitHub + " en producción 🚀 ¿sí?",
		"tabs y espacios":    "\t  " + secAWS + "  \t\n",
		"tres seguidos":      secAWS + secGitHub + secAnt,
		"secreto entre =":    "x=" + secAWS + "=y",
		"prosa larga + secr": strings.Repeat("palabra ", 200) + secAnt + strings.Repeat(" mas texto", 200),
	}
	for nombre, original := range casos {
		t.Run(nombre, func(t *testing.T) {
			s := NewSession()
			scrubbed := s.Scrub(original)
			vuelto := s.Restore(scrubbed)
			if vuelto != original {
				t.Fatalf("el round-trip perdió información\n original: %q\n scrubbed: %q\n vuelto  : %q",
					original, scrubbed, vuelto)
			}
		})
	}
}

func TestR1RoundTripConVariosTextosEnLaMismaSesion(t *testing.T) {
	s := NewSession()
	sys := "sos un asistente. la clave maestra es " + secAWS
	usr := "revisá " + secGitHub + " y también " + secAWS

	ss, su := s.Scrub(sys), s.Scrub(usr)
	containsAny(t, ss, secAWS, secGitHub)
	containsAny(t, su, secAWS, secGitHub)

	if got := s.Restore(ss); got != sys {
		t.Fatalf("system no volvió igual:\n want %q\n got  %q", sys, got)
	}
	if got := s.Restore(su); got != usr {
		t.Fatalf("user no volvió igual:\n want %q\n got  %q", usr, got)
	}
}

// --- R2: Restore no fabrica ----------------------------------------------------------------

func TestR2MarcadorInventadoPorElModeloSeDejaIntacto(t *testing.T) {
	s := NewSession()
	_ = s.Scrub("la clave es " + secAWS)

	// El modelo devuelve un marcador que ESTA sesión nunca acuñó. No tiene que resolverse a nada.
	respuesta := "mirá también [[MSB:aws-access-key:999]] y [[MSB:inventado:1]] y [[MSB:x]]"
	got := s.Restore(respuesta)
	if got != respuesta {
		t.Fatalf("Restore fabricó una sustitución sobre un marcador que no acuñó:\n want %q\n got  %q",
			respuesta, got)
	}
	containsAny(t, got, secAWS)
}

func TestR2SesionVaciaNoTocaNada(t *testing.T) {
	s := NewSession()
	texto := "respuesta con [[MSB:token:1]] adentro"
	if got := s.Restore(texto); got != texto {
		t.Fatalf("una sesión sin mapeo alteró el texto: %q", got)
	}
}

// --- R3: estabilidad e inyectividad --------------------------------------------------------

func TestR3MismoSecretoMismoMarcador(t *testing.T) {
	s := NewSession()
	out := s.Scrub("A " + secAWS + " B " + secAWS + " C")

	if n := s.Count(); n != 1 {
		t.Fatalf("el mismo secreto repetido tenía que contar 1, contó %d", n)
	}
	// El marcador tiene que aparecer exactamente dos veces (una por ocurrencia).
	var tok string
	for k := range s.byToken {
		tok = k
	}
	if c := strings.Count(out, tok); c != 2 {
		t.Fatalf("el marcador %q tenía que aparecer 2 veces, apareció %d en %q", tok, c, out)
	}
}

func TestR3SecretosDistintosMarcadoresDistintos(t *testing.T) {
	s := NewSession()
	_ = s.Scrub(secAWS + " " + secGitHub + " " + secAnt)

	if n := s.Count(); n != 3 {
		t.Fatalf("esperaba 3 secretos distintos, hubo %d", n)
	}
	vistos := make(map[string]bool)
	for tok, val := range s.byToken {
		if vistos[tok] {
			t.Fatalf("marcador repetido para valores distintos: %q", tok)
		}
		vistos[tok] = true
		if val == "" {
			t.Fatalf("marcador %q mapeado a valor vacío", tok)
		}
	}
}

// --- R5: el marcador no colisiona ----------------------------------------------------------

func TestR5EntradaQueYaTieneFormaDeMarcador(t *testing.T) {
	// El usuario escribió algo que se parece a nuestro marcador. No lo tenemos que pisar, y el
	// round-trip tiene que seguir siendo exacto.
	original := "el marcador [[MSB:aws-access-key:1]] es literal, y la clave real es " + secAWS
	s := NewSession()
	scrubbed := s.Scrub(original)

	containsAny(t, scrubbed, secAWS)
	// El literal del usuario tiene que seguir ahí, intacto.
	if !strings.Contains(scrubbed, "[[MSB:aws-access-key:1]]") {
		t.Fatalf("se pisó el marcador literal del usuario: %q", scrubbed)
	}
	if got := s.Restore(scrubbed); got != original {
		t.Fatalf("colisión de marcador rompió el round-trip:\n want %q\n got  %q", original, got)
	}
}

func TestR5VariasColisionesSeguidas(t *testing.T) {
	// Bloqueamos varios índices a la vez: el acuñador tiene que saltarlos todos.
	original := "[[MSB:aws-access-key:1]] [[MSB:aws-access-key:2]] [[MSB:aws-access-key:3]] real=" + secAWS
	s := NewSession()
	scrubbed := s.Scrub(original)

	containsAny(t, scrubbed, secAWS)
	if got := s.Restore(scrubbed); got != original {
		t.Fatalf("round-trip roto con colisiones múltiples:\n want %q\n got  %q", original, got)
	}
	for i := 1; i <= 3; i++ {
		lit := "[[MSB:aws-access-key:" + string(rune('0'+i)) + "]]"
		if !strings.Contains(scrubbed, lit) {
			t.Fatalf("se pisó el literal %q: %q", lit, scrubbed)
		}
	}
}

// --- Superficie de auditoría ---------------------------------------------------------------

func TestFindingsYTypesNoFiltranValores(t *testing.T) {
	s := NewSession()
	_ = s.Scrub("clave " + secAWS + " y token " + secGitHub)

	if len(s.Findings()) == 0 {
		t.Fatal("Findings() quedó vacío con secretos presentes")
	}
	tipos := s.Types()
	if len(tipos) == 0 {
		t.Fatal("Types() quedó vacío con secretos presentes")
	}
	// Los tipos son metadatos; jamás pueden contener el valor.
	for _, ty := range tipos {
		containsAny(t, ty, secAWS, secGitHub)
	}
}

func TestFindingsDevuelveCopia(t *testing.T) {
	s := NewSession()
	_ = s.Scrub("clave " + secAWS)
	f := s.Findings()
	if len(f) == 0 {
		t.Fatal("sin hallazgos")
	}
	f[0].Type = "MUTADO"
	if s.Findings()[0].Type == "MUTADO" {
		t.Fatal("Findings() expuso el estado interno: mutarlo de afuera cambió la sesión")
	}
}

func TestTextoLimpioNoAcunaNada(t *testing.T) {
	s := NewSession()
	texto := "un texto totalmente inocente sobre el clima de hoy"
	if got := s.Scrub(texto); got != texto {
		t.Fatalf("texto sin secretos fue alterado: %q", got)
	}
	if s.Count() != 0 {
		t.Fatalf("se acuñaron %d marcadores sobre texto limpio", s.Count())
	}
}
