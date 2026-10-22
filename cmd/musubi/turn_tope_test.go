package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"musubi/internal/config"
)

// TestBuildTurnRecallAcotaLaConsulta: un prompt más largo que turnRecallMaxRunes llega al corrector y
// al recall YA acotado a ese tope. Es la guarda del follow-up #1 de la review de #751: sin ella, el
// FTS de Recall sobre el prompt entero se cuelga (medido: 32 KB de prosa real → 12,6 s, 64 KB → se
// cuelga minutos) y el hook muere por el "timeout":10 de agente_plugin.go, dejando el turno SIN
// memoria. Acota la CONSULTA con que se recupera, no el prompt que ve el modelo, así que sólo se
// verifica qué recibieron el corrector y Recall.
func TestBuildTurnRecallAcotaLaConsulta(t *testing.T) {
	largo := strings.Repeat("memoria relevante del hook con recall hibrido y corrector de tipeo ", 100)
	if utf8.RuneCountInString(largo) <= turnRecallMaxRunes {
		t.Fatalf("precondición: el prompt de prueba (%d runas) tiene que superar el tope (%d)",
			utf8.RuneCountInString(largo), turnRecallMaxRunes)
	}
	store := &fakeTurnStore{recall: itemDelPi()}
	buildTurnRecall(store, parametrosDelTurno{
		sesion: "s-tope", prompt: largo, presupuesto: 250, memCfg: config.Default().Memory, propio: "musubi",
	})
	if !store.llamoAlCorrector {
		t.Fatal("precondición: el corrector tenía que correr (RecallTypoCorrection viene on por defecto)")
	}
	if n := utf8.RuneCountInString(store.corrigioConsulta); n != turnRecallMaxRunes {
		t.Errorf("el corrector recibió %d runas; tenía que recibir el tope %d", n, turnRecallMaxRunes)
	}
	if n := utf8.RuneCountInString(store.lastQuery); n != turnRecallMaxRunes {
		t.Errorf("Recall recibió %d runas; tenía que recibir el tope %d", n, turnRecallMaxRunes)
	}
}

// TestBuildTurnRecallNoAcotaLoQueEntra: un prompt que YA entra en el tope llega intacto. La guarda no
// puede cambiar el recall de un turno normal (que es lo que mide el banco de calidad).
func TestBuildTurnRecallNoAcotaLoQueEntra(t *testing.T) {
	corto := "cómo se cierra la planilla del turno noche"
	store := &fakeTurnStore{recall: itemDelPi()}
	buildTurnRecall(store, parametrosDelTurno{
		sesion: "s-corto", prompt: corto, presupuesto: 250, memCfg: config.Default().Memory, propio: "musubi",
	})
	if store.lastQuery != corto {
		t.Errorf("Recall recibió %q; un prompt bajo el tope tiene que llegar intacto (%q)", store.lastQuery, corto)
	}
}

// TestAcotarConsultaDelTurno cubre la función de corte por sí sola: frontera de rune, el borde exacto
// y el caso multibyte (no partir un carácter).
func TestAcotarConsultaDelTurno(t *testing.T) {
	for _, c := range []struct {
		nombre string
		in     string
		max    int
		quiere string
	}{
		{"entra justo", "abcde", 5, "abcde"},
		{"entra de sobra", "abc", 10, "abc"},
		{"recorta ascii", "abcdef", 3, "abc"},
		{"max 0 no toca", "abcdef", 0, "abcdef"},
		{"max negativo no toca", "abcdef", -1, "abcdef"},
		{"multibyte sin partir", "áéíóú", 3, "áéí"},
		{"multibyte en el borde", "áéíóú", 5, "áéíóú"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			got := acotarConsultaDelTurno(c.in, c.max)
			if got != c.quiere {
				t.Errorf("acotarConsultaDelTurno(%q, %d) = %q; quiero %q", c.in, c.max, got, c.quiere)
			}
			if !utf8.ValidString(got) {
				t.Errorf("acotarConsultaDelTurno(%q, %d) = %q: no es UTF-8 válido (partió un rune)", c.in, c.max, got)
			}
		})
	}
}
