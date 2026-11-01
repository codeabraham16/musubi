package specgraph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errBodyBlocked simula el error del portero de privacidad que veta un cuerpo (R7).
var errBodyBlocked = errors.New("simulación: el portero de privacidad vetó este cuerpo")

// fakeEmbedder es un embebedor DETERMINISTA para los tests. Mapea un texto a un vector de 3
// dimensiones [retry/backoff, auth, ui] contando palabras-concepto (conceptVec): textos del mismo
// concepto quedan colineales (coseno 1), de conceptos distintos ortogonales (coseno 0). failOn
// fuerza un error cuando el texto contiene un substring dado (R5/R7); calls cuenta las llamadas,
// para probar que SIN embebedor no se toca la red (R5).
type fakeEmbedder struct {
	failOn []string
	err    error
	calls  int
}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	f.calls++
	low := strings.ToLower(text)
	for _, sub := range f.failOn {
		if strings.Contains(low, strings.ToLower(sub)) {
			return nil, f.err
		}
	}
	return conceptVec(text), nil
}

// conceptVec cuenta palabras-concepto en 3 dimensiones. No usa "ui" como keyword a propósito: es
// tan corto que pega dentro de "build", "gui", etc. y contaminaría el vector.
func conceptVec(text string) []float32 {
	t := strings.ToLower(text)
	v := make([]float32, 3)
	for _, kw := range []string{"backoff", "exponencial", "exponential", "reintent", "espera", "retry"} {
		v[0] += float32(strings.Count(t, kw))
	}
	for _, kw := range []string{"auth", "token", "login", "sesión", "sesion"} {
		v[1] += float32(strings.Count(t, kw))
	}
	for _, kw := range []string{"color", "banner", "diseñ", "disen"} {
		v[2] += float32(strings.Count(t, kw))
	}
	return v
}

// mkEntry arma un SpecContentEntry con frontmatter id+type+title (lo que Recall lee vía Scalar) y un
// Content crudo coherente (lo que se embebe y se usa para keyword/snippet).
func mkEntry(path, id, typ, title, body string) SpecContentEntry {
	fm := Frontmatter{FieldID: id, FieldType: typ, FieldTitle: title}
	content := fmt.Sprintf("---\nid: %s\ntype: %s\ntitle: %s\n---\n\n%s", id, typ, title, body)
	return SpecContentEntry{Path: path, Content: content, Frontmatter: fm}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// R4: el significado gana al texto literal. La spec A habla de "espera exponencial entre
// reintentos" y NO contiene "backoff"; el coseno debe ponerla arriba igual. SABOTAJE: si cosine
// devolviera 0 (o se cayera a keyword), A puntuaría 0 —no tiene el literal— y la aserción
// score(A) > score(B) se pondría roja.
func TestRecall_SemanticBeatsLiteral(t *testing.T) {
	a := mkEntry("specs/a/SPEC.md", "spec-a", "module-design", "Reintentos",
		"El cliente hace espera exponencial entre reintentos sucesivos.")
	b := mkEntry("specs/b/SPEC.md", "spec-b", "module-design", "Auth",
		"El login valida el token de sesión del usuario.")
	if strings.Contains(strings.ToLower(a.Content), "backoff") {
		t.Fatal("el fixture perdió sentido: A no debe contener 'backoff' literal")
	}

	emb := &fakeEmbedder{}
	r := NewRecaller("fake")
	res, err := r.Recall(context.Background(), []SpecContentEntry{b, a},
		RecallOptions{Query: "política de backoff"}, emb.Embed)
	if err != nil {
		t.Fatalf("Recall devolvió error: %v", err)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("esperaba 2 hits, hubo %d", len(res.Hits))
	}
	if res.Hits[0].ID != "spec-a" {
		t.Errorf("esperaba spec-a arriba, fue %q", res.Hits[0].ID)
	}
	if !(res.Hits[0].Score > res.Hits[1].Score) {
		t.Errorf("el score de A (%v) debía superar al de B (%v) por SEMÁNTICA", res.Hits[0].Score, res.Hits[1].Score)
	}
}

// R5 tiene DOS caras, las dos acá: (a) sin embebedor (el handler pasa nil cuando embedding.Enabled
// es false) y (b) con embebedor pero el QUERY falla. Las dos degradan a keyword SIN error. SABOTAJE
// (a): si Recall llamara al embebedor ignorando el nil, el `embed(ctx, query)` con embed==nil entra
// en pánico ⇒ rojo; si devolviera error ante nil, err!=nil ⇒ rojo. SABOTAJE (b): si al fallar el
// query Recall NO cortara el modo semántico (dejara semantic=true), seguiría embebiendo spec por
// spec y calls pasaría de 1 ⇒ la aserción calls==1 se pone roja.
func TestRecall_NoEmbedderFallsBackToKeyword(t *testing.T) {
	a := mkEntry("specs/a/SPEC.md", "spec-a", "module-design", "Reintentos", "Reintentos con espera creciente.")
	b := mkEntry("specs/b/SPEC.md", "spec-b", "module-design", "Auth", "Login y token de sesión.")

	// (a) embed == nil: ni un intento de red, keyword puro, sin error.
	r := NewRecaller("none")
	res, err := r.Recall(context.Background(), []SpecContentEntry{a, b},
		RecallOptions{Query: "reintentos"}, nil)
	if err != nil {
		t.Fatalf("sin embebedor NO debe haber error (invariante model-free R5): %v", err)
	}
	if len(res.Hits) == 0 {
		t.Fatal("esperaba resultados por keyword, hubo 0")
	}
	if res.Hits[0].ID != "spec-a" {
		t.Errorf("esperaba spec-a arriba por keyword ('reintentos' pega en A), fue %q", res.Hits[0].ID)
	}

	// (b) embebedor presente pero el QUERY falla ⇒ TODA la llamada cae a keyword y NO se embebe spec
	// por spec: calls queda en 1 (el único intento, el del query). Es la otra cara de R5.
	emb := &fakeEmbedder{failOn: []string{"reintentos"}, err: errors.New("el query no embebe")}
	res2, err := r.Recall(context.Background(), []SpecContentEntry{a, b},
		RecallOptions{Query: "reintentos"}, emb.Embed)
	if err != nil {
		t.Fatalf("un query que falla NO debe fallar la llamada (R5): %v", err)
	}
	if emb.calls != 1 {
		t.Errorf("al fallar el query, NO se debe embeber spec por spec: hubo %d llamadas (esperaba 1)", emb.calls)
	}
	if len(res2.Hits) == 0 {
		t.Fatal("esperaba resultados por keyword tras el fallo del query, hubo 0")
	}
}

// R6: orden determinista. Dos specs con el MISMO body empatan en score; el desempate es por id asc
// sin importar el orden de entrada. SABOTAJE: si se quita el desempate por id (ordenar sólo por
// score), el sort —que NO es estable— deja el orden de entrada [spec-zzz, spec-aaa] y la aserción
// se pone roja.
func TestRecall_DeterministicTieBreak(t *testing.T) {
	body := "Contenido idéntico para forzar empate de score."
	hi := mkEntry("specs/z/SPEC.md", "spec-aaa", "module-design", "T", body)
	lo := mkEntry("specs/a/SPEC.md", "spec-zzz", "module-design", "T", body)

	r := NewRecaller("none")
	res, err := r.Recall(context.Background(), []SpecContentEntry{lo, hi},
		RecallOptions{Query: "idéntico"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("esperaba 2 hits, hubo %d", len(res.Hits))
	}
	if res.Hits[0].ID != "spec-aaa" || res.Hits[1].ID != "spec-zzz" {
		t.Errorf("empate mal roto (debía ser id asc): %q, %q", res.Hits[0].ID, res.Hits[1].ID)
	}
}

// R7: un cuerpo vetado por el portero cae a keyword, NO desaparece y NO falla la llamada. SABOTAJE:
// si el error del embed por-spec se propagara (en vez de caer a keyword), err!=nil se pondría rojo;
// si el spec vetado se excluyera, foundA sería false.
func TestRecall_BlockedSpecFallsToKeyword(t *testing.T) {
	// A: su CUERPO dispara el veto del portero (marcador "zzqqx"), pero el QUERY embebe bien (no lo
	// contiene) ⇒ semantic=true y se ejercita el camino R7 (un spec vetado EN modo semántico), no el
	// R5 (query que falla ⇒ toda la llamada a keyword). Es la distinción que el sabotaje destapó.
	a := mkEntry("specs/a/SPEC.md", "spec-a", "module-design", "Reintentos",
		"Política de reintentos con espera. zzqqx")
	b := mkEntry("specs/b/SPEC.md", "spec-b", "module-design", "Auth", "Login y token de sesión.")

	emb := &fakeEmbedder{failOn: []string{"zzqqx"}, err: errBodyBlocked}
	r := NewRecaller("fake")
	res, err := r.Recall(context.Background(), []SpecContentEntry{a, b},
		RecallOptions{Query: "política de reintentos"}, emb.Embed)
	if err != nil {
		t.Fatalf("un cuerpo vetado NO debe fallar la llamada (R7): %v", err)
	}
	var foundA bool
	for _, h := range res.Hits {
		if h.ID == "spec-a" {
			foundA = true
		}
	}
	if !foundA {
		t.Error("spec-a desapareció: un cuerpo vetado debe rankearse por keyword, no excluirse")
	}
	if res.Hits[0].ID != "spec-a" {
		t.Errorf("esperaba spec-a arriba por keyword ('política de reintentos' pega literal en A), fue %q", res.Hits[0].ID)
	}
}

// R8: el diario SDD no es un spec. Un proposal.md (frontmatter artifact+schema_version, sin
// id+type) se camina pero lo descarta ContentEntries vía IsSpec. SABOTAJE: si scan() dejara de
// filtrar IsSpec, el proposal.md aparecería en los hits y la aserción se pondría roja. Es un test
// de integración Index→Recall: ata la frescura FS-nativa de F1 al recall de F3.
func TestRecall_SDDJournalExcluded(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "mod", "SPEC.md"),
		"---\nid: real-spec\ntype: module-design\ntitle: Real\n---\n\nCuerpo con reintentos y espera exponencial.")
	writeTestFile(t, filepath.Join(dir, "specs", "cambio", "proposal.md"),
		"---\nartifact: proposal\nschema_version: \"1.0\"\nchange: cambio\n---\n\nCuerpo con reintentos y espera también.")

	idx := NewIndex(dir)
	entries := idx.ContentEntries()
	r := NewRecaller("none")
	res, err := r.Recall(context.Background(), entries, RecallOptions{Query: "reintentos"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range res.Hits {
		if strings.Contains(filepath.ToSlash(h.Path), "proposal.md") {
			t.Errorf("el diario SDD proposal.md apareció en los resultados: %q", h.Path)
		}
	}
	var foundReal bool
	for _, h := range res.Hits {
		if h.ID == "real-spec" {
			foundReal = true
		}
	}
	if !foundReal {
		t.Error("la spec real no apareció (excluir el diario no debe excluir la spec real)")
	}
}

// R9: frescura por hash del Content. El MISMO spec (misma Path) cambia su cuerpo entre dos llamadas
// al MISMO Recaller; el cache keyed por hash debe invalidar el vector viejo y reembeber. SABOTAJE:
// si el cache se keyeara sólo por path (ignorando el hash), la 2ª llamada serviría el vector viejo
// y score2 no superaría a score1.
func TestRecall_ContentChangeReembeds(t *testing.T) {
	r := NewRecaller("fake")
	emb := &fakeEmbedder{}
	query := RecallOptions{Query: "espera exponencial backoff"}

	// 1ª vuelta: el cuerpo de A habla de AUTH ⇒ coseno bajo contra un query de backoff.
	a1 := mkEntry("specs/a/SPEC.md", "spec-a", "module-design", "A", "Login y token de sesión.")
	res1, err := r.Recall(context.Background(), []SpecContentEntry{a1}, query, emb.Embed)
	if err != nil {
		t.Fatal(err)
	}
	score1 := res1.Hits[0].Score

	// 2ª vuelta: el cuerpo de A ahora habla de backoff ⇒ coseno alto, si el cache se invalidó.
	a2 := mkEntry("specs/a/SPEC.md", "spec-a", "module-design", "A", "Espera exponencial entre reintentos: backoff.")
	res2, err := r.Recall(context.Background(), []SpecContentEntry{a2}, query, emb.Embed)
	if err != nil {
		t.Fatal(err)
	}
	score2 := res2.Hits[0].Score

	if !(score2 > score1) {
		t.Errorf("tras cambiar el cuerpo el ranking no reflejó el contenido nuevo: score1=%v score2=%v (¿cache no invalidado por hash?)", score1, score2)
	}
}

// R10a: el filtro type devuelve SÓLO specs de ese tipo. SABOTAJE: si el filtro se ignorara, el
// task-spec pasaría y la aserción del type (y del conteo) se pondría roja.
func TestRecall_TypeFilter(t *testing.T) {
	md := mkEntry("specs/a/SPEC.md", "a", "module-design", "A", "Reintentos.")
	ts := mkEntry("specs/b/SPEC.md", "b", "task-spec", "B", "Reintentos también.")

	r := NewRecaller("none")
	res, err := r.Recall(context.Background(), []SpecContentEntry{md, ts},
		RecallOptions{Query: "reintentos", Type: "module-design"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("esperaba 1 hit (sólo module-design), hubo %d", len(res.Hits))
	}
	for _, h := range res.Hits {
		if h.Type != "module-design" {
			t.Errorf("el filtro type dejó pasar un %q", h.Type)
		}
	}
}

// R10b: paginado estable espejando GrepResult (Truncated + NextOffset), y offset negativo = error.
// SABOTAJE: si Truncated/NextOffset se calcularan mal, o el offset negativo no diera error, las
// aserciones correspondientes se pondrían rojas.
// arnes: archivo="internal/specgraph/recall.go"
// arnes: de="if opts.Offset < 0 {"
// arnes: a="if opts.Offset < 0 && false {"
func TestRecall_Pagination(t *testing.T) {
	entries := make([]SpecContentEntry, 0, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("spec-%d", i)
		entries = append(entries, mkEntry("specs/"+id+"/SPEC.md", id, "module-design", "T", "Reintentos comunes."))
	}
	r := NewRecaller("none")

	// Todas empatan en score (mismo body) ⇒ orden por id: spec-0..spec-4.
	res, err := r.Recall(context.Background(), entries, RecallOptions{Query: "reintentos", Limit: 2, Offset: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("esperaba 2 hits, hubo %d", len(res.Hits))
	}
	if !res.Truncated {
		t.Error("esperaba Truncated=true (quedan más)")
	}
	if res.NextOffset != 2 {
		t.Errorf("esperaba NextOffset=2, fue %d", res.NextOffset)
	}
	if res.Hits[0].ID != "spec-0" || res.Hits[1].ID != "spec-1" {
		t.Errorf("página 1 inesperada: %q, %q", res.Hits[0].ID, res.Hits[1].ID)
	}

	res3, err := r.Recall(context.Background(), entries, RecallOptions{Query: "reintentos", Limit: 2, Offset: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.Hits) != 1 {
		t.Fatalf("esperaba 1 hit en la última página, hubo %d", len(res3.Hits))
	}
	if res3.Truncated {
		t.Error("la última página no debe estar truncada")
	}

	if _, err := r.Recall(context.Background(), entries, RecallOptions{Query: "x", Offset: -1}, nil); err == nil {
		t.Error("un offset negativo debe dar error (espeja GrepSpecs)")
	}
}
