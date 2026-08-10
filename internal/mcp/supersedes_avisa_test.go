package mcp

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

// supersedes_avisa_test.go — «ocultar una nota no puede ser silencioso».
//
// EL CASO REAL QUE LO ORIGINA (2026-08-10): una terminal consolidó dos notas en una versión nueva y
// les aplicó `supersedes` minutos antes de que otra terminal leyera las respuestas construidas
// encima. Las citas quedaron apuntando a observaciones ocultas del recall, y NADA avisó. Lo levantó
// quien lo provocó, no el sistema.

// guardarObsEnServer deja una observación y devuelve su id.
func guardarObsEnServer(t *testing.T, s *McpServer, id, topic, contenido string) string {
	t.Helper()
	if _, rerr := call(t, s, "musubi_save_observation", map[string]interface{}{
		"id": id, "topic_key": topic, "content": contenido,
	}); rerr != nil {
		t.Fatalf("save_observation %s: %+v", id, rerr)
	}
	return id
}

// relacionar crea una relación entre dos observaciones y devuelve su id canónico.
func relacionar(t *testing.T, e *memory.DbEngine, source, target, rel string) string {
	t.Helper()
	id, err := e.UpsertObsRelation(memory.ObsRelation{
		SourceID: source, TargetID: target, Relation: rel,
		Confidence: 0.9, Status: memory.RelStatusPending,
	})
	if err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}
	return id
}

func juzgar(t *testing.T, s *McpServer, relID, veredicto string) string {
	t.Helper()
	out, rerr := call(t, s, "musubi_judge", map[string]interface{}{
		"relation_id": relID, "relation": veredicto,
	})
	if rerr != nil {
		t.Fatalf("musubi_judge: %+v", rerr)
	}
	return out.(CallToolResponse).Content[0].Text
}

// ★ A1 — SUPERSEDES AVISA CUÁNTAS OTRAS RELACIONES TOCAN AL TARGET.
func TestSupersedesAvisaDeLasHuerfanas(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Skip("el server de prueba no usa DbEngine")
	}

	vieja := guardarObsEnServer(t, s, "obs-vieja", "tema/x", "la versión vieja, con respuestas encima")
	nueva := guardarObsEnServer(t, s, "obs-nueva", "tema/x", "la consolidación que la reemplaza")
	otra := guardarObsEnServer(t, s, "obs-respuesta", "tema/x", "una respuesta construida sobre la vieja")

	// La respuesta CITA a la vieja: es la relación que quedaría huérfana.
	relacionar(t, eng, otra, vieja, memory.RelRelated)
	aOcultar := relacionar(t, eng, nueva, vieja, memory.RelPending)

	txt := juzgar(t, s, aOcultar, memory.RelSupersedes)

	if !strings.Contains(txt, "oculta del recall") {
		t.Errorf("tiene que seguir diciendo que la ocultó; dijo: %q", txt)
	}
	if !strings.Contains(txt, "otras 1 relaciones") {
		t.Errorf("tiene que avisar que hay otra relación tocando al target; dijo: %q", txt)
	}
	if !strings.Contains(txt, memory.RelRelated) {
		t.Errorf("el aviso tiene que decir CON QUÉ veredicto, no sólo cuántas; dijo: %q", txt)
	}
}

// A2 — SIN NADIE APUNTANDO, NO SE INVENTA UN AVISO.
//
// Un aviso que aparece siempre es una valla que marca de más, y ésas se apagan solas porque nadie
// las mira.
func TestSinReferenciasNoHayAviso(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Skip("el server de prueba no usa DbEngine")
	}

	vieja := guardarObsEnServer(t, s, "sola-vieja", "tema/y", "nadie construyó nada encima")
	nueva := guardarObsEnServer(t, s, "sola-nueva", "tema/y", "la reemplaza")
	rel := relacionar(t, eng, nueva, vieja, memory.RelPending)

	txt := juzgar(t, s, rel, memory.RelSupersedes)
	if strings.Contains(txt, "OJO") {
		t.Errorf("sin otras relaciones no corresponde avisar; dijo: %q", txt)
	}
}

// A3 — LOS OTROS VEREDICTOS NO AVISAN NADA.
//
// Sólo `supersedes` oculta. Avisar en `compatible` sería ruido en el veredicto más común.
func TestSoloSupersedesAvisa(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Skip("el server de prueba no usa DbEngine")
	}

	a := guardarObsEnServer(t, s, "c-a", "tema/z", "una")
	b := guardarObsEnServer(t, s, "c-b", "tema/z", "otra")
	c := guardarObsEnServer(t, s, "c-c", "tema/z", "una tercera que cita a la primera")
	relacionar(t, eng, c, a, memory.RelRelated)
	rel := relacionar(t, eng, b, a, memory.RelPending)

	txt := juzgar(t, s, rel, memory.RelCompatible)
	if strings.Contains(txt, "OJO") || strings.Contains(txt, "oculta del recall") {
		t.Errorf("un veredicto que no oculta nada no puede avisar de huérfanas; dijo: %q", txt)
	}
}

// A4 — EL AVISO NUNCA PUEDE COSTAR UN VEREDICTO.
//
// Misma garantía que el ledger de uso: la telemetría y los avisos son extras. Un relation_id que ni
// existe hace fallar el veredicto por su propia validación, no por el aviso — y avisoDeHuerfanas
// devuelve "" en vez de romper.
func TestElAvisoNoRompeNada(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	if got := s.avisoDeHuerfanas("no-existe-esta-relacion"); got != "" {
		t.Errorf("con una relación inexistente el aviso tiene que ser vacío, dijo: %q", got)
	}
}
