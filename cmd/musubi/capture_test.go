package main

import (
	"errors"
	"strings"
	"testing"

	"musubi/internal/memory"
)

func TestClassifyCommit(t *testing.T) {
	cases := []struct {
		subject string
		typ     string
		imp     float64
		skip    bool
	}{
		{"fix: null pointer in parser", "episodic", 0.7, false},
		{"revert broken migration change", "episodic", 0.7, false},
		{"feat: add provision command", "episodic", 0.5, false},
		{"refactor the redaction guard", "episodic", 0.5, false},
		{"improve the parser robustness", "episodic", 0.4, false}, // desconocido, no trivial
		{"chore: bump dependencies", "", 0, true},
		{"docs: update the readme file", "", 0, true},
		{"wip", "", 0, true},
		{"typo", "", 0, true}, // < 10 chars
		{"Merge pull request #12 from x", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.subject, func(t *testing.T) {
			typ, imp, skip := classifyCommit(c.subject)
			if typ != c.typ || imp != c.imp || skip != c.skip {
				t.Fatalf("classify(%q) = (%q,%v,%v); quería (%q,%v,%v)", c.subject, typ, imp, skip, c.typ, c.imp, c.skip)
			}
		})
	}
}

// recordingStore implementa captureStore y registra el embedding del último guardado, para
// verificar que la captura pasa (o no) el vector.
type recordingStore struct {
	meta      map[string]string
	lastEmbed []float32
	saved     int
	// dedupeAll fuerza que TODO id se considere ya existente (simula el UPSERT de un gemelo).
	dedupeAll bool
	// byID simula la tabla observations: id → contenido.
	byID map[string]string
	// scopeByID: con qué scope se guardó cada commit (C5.2: 'shared' en team mode).
	scopeByID map[string]string
}

func (r *recordingStore) GetMeta(k string) (string, bool, error) { v, ok := r.meta[k]; return v, ok, nil }
func (r *recordingStore) SetMeta(k, v string) error {
	if r.meta == nil {
		r.meta = map[string]string{}
	}
	r.meta[k] = v
	return nil
}
// byID simula la tabla: id → contenido. Es lo que permite testear el UPSERT del gemelo del squash.
func (r *recordingStore) SaveObservationTyped(id, _, content string, _ float64, _, scope string, emb []float32) error {
	if r.byID == nil {
		r.byID = map[string]string{}
		r.scopeByID = map[string]string{}
	}
	r.byID[id] = content
	r.scopeByID[id] = scope
	r.lastEmbed = emb
	r.saved++
	return nil
}

func (r *recordingStore) ObservationExists(id string) (bool, error) {
	if r.dedupeAll {
		return true, nil
	}
	_, ok := r.byID[id]
	return ok, nil
}

// C5.2 — en TEAM MODE la captura es CENTRAL por naturaleza: los commits se guardan 'shared' y viajan
// al cerebro. Sin esto, lo ÚNICO que Musubi captura SOLO era justo lo único que NUNCA cruzaba de
// máquina: medido en la memoria real, la PC tenía 481 observaciones locales y la laptop 70 — ~400
// commits capturados de un lado eran invisibles del otro.
func TestCaptureCommitsSharedEnTeamMode(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	if n, err := captureCommits(store, g, nil, nil, memory.ScopeShared); err != nil || n != 1 {
		t.Fatalf("captureCommits = (%d, %v)", n, err)
	}
	for id, sc := range store.scopeByID {
		if sc != memory.ScopeShared {
			t.Errorf("commit %s guardado con scope %q, esperaba 'shared' (team mode)", id, sc)
		}
	}
}

// …y en un proyecto personal (team mode apagado) sigue siendo LOCAL: la memoria de un repo privado
// no se va al cerebro sólo por existir. Es el comportamiento histórico y no puede romperse.
func TestCaptureCommitsLocalSinTeamMode(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	if n, err := captureCommits(store, g, nil, nil, memory.ScopeLocal); err != nil || n != 1 {
		t.Fatalf("captureCommits = (%d, %v)", n, err)
	}
	for id, sc := range store.scopeByID {
		if sc != memory.ScopeLocal {
			t.Errorf("commit %s guardado con scope %q, esperaba 'local' (proyecto personal)", id, sc)
		}
	}
}

// A2 (captura origin-side multi-repo): con una CLAVE DE CURSOR POR REPO, varios repos se capturan en
// la MISMA memoria (el cerebro central) sin pisarse el HEAD entre sí. Es lo que habilita el timer que
// corre `capture --repo <mirror>` por cada repo de Forgejo.
func TestCaptureCommitsKeyedCursorPorRepo(t *testing.T) {
	store := &recordingStore{}
	repoA := &fakeGit{head: "a1", commits: []commit{{SHA: "a1", Subject: "feat: algo grande del repo A"}}}
	repoB := &fakeGit{head: "b1", commits: []commit{{SHA: "b1", Subject: "fix: algo grande del repo B"}}}

	keyA := repoCursorKey("/srv/repos/a.git")
	keyB := repoCursorKey("/srv/repos/b.git")
	if keyA == keyB {
		t.Fatal("dos repos distintos deben tener claves de cursor distintas")
	}

	if n, err := captureCommitsKeyed(store, repoA, nil, nil, memory.ScopeShared, keyA); err != nil || n != 1 {
		t.Fatalf("repo A: (%d, %v), esperaba 1", n, err)
	}
	// Repo B con SU propio cursor captura, sin que el HEAD de A lo bloquee.
	if n, err := captureCommitsKeyed(store, repoB, nil, nil, memory.ScopeShared, keyB); err != nil || n != 1 {
		t.Fatalf("repo B: (%d, %v), esperaba 1 (el cursor de A no debe interferir)", n, err)
	}
	// Cada cursor apunta al HEAD de SU repo.
	if store.meta[keyA] != "a1" || store.meta[keyB] != "b1" {
		t.Errorf("cursores cruzados: keyA=%q (quería a1), keyB=%q (quería b1)", store.meta[keyA], store.meta[keyB])
	}
	// Re-capturar A con su cursor: ya está al día ⇒ 0 nuevos.
	if n, _ := captureCommitsKeyed(store, repoA, nil, nil, memory.ScopeShared, keyA); n != 0 {
		t.Errorf("re-captura de A: esperaba 0 (al día), obtuve %d", n)
	}
	// repoCursorKey es determinista y namespaced bajo la clave global.
	if repoCursorKey("/srv/repos/a.git") != keyA || !strings.HasPrefix(keyA, metaCaptureLastCommit+":") {
		t.Errorf("repoCursorKey no es determinista/namespaced: %q", keyA)
	}
}

// El gemelo del SQUASH-MERGE: GitHub crea en main un commit nuevo con el MISMO mensaje más el
// sufijo `(#123)`, y reescribe el trailer Co-Authored-By → Co-authored-by. Sin normalizar, la
// captura lo guardaba como una memoria NUEVA (el dedup por hash exacto no lo agarra: el texto cambió
// apenas). Encontrado en la memoria real: 3 pares sobre 58 commits.
const commitRama = "feat(dedup): dedup semantico\n\nCuerpo del commit.\n\nCo-Authored-By: Claude <x@y>\n\nArchivos: a.go, b.go"
const commitSquash = "feat(dedup): dedup semantico (#193)\n\nCuerpo del commit.\n\nCo-authored-by: Claude <x@y>\n\nArchivos: a.go, b.go"

// S.a / S.e — el gemelo del squash cae en la MISMA clave: mismo id ⇒ UPSERT, no observación nueva.
func TestCommitIDIgnoresSquashSuffixAndTrailerCase(t *testing.T) {
	if commitObsID(commitRama) != commitObsID(commitSquash) {
		t.Errorf("el gemelo del squash debe caer en el MISMO id que el commit de la rama\n  rama:   %s\n  squash: %s",
			commitObsID(commitRama), commitObsID(commitSquash))
	}
}

// S.d — dos commits con el MISMO subject pero ARCHIVOS distintos NO deben colisionar: la lista de
// archivos entra en la clave, y es lo que los distingue.
func TestCommitIDDistinguishesDifferentFiles(t *testing.T) {
	a := "chore: bump deps\n\nArchivos: go.mod"
	b := "chore: bump deps\n\nArchivos: package.json"
	if commitObsID(a) == commitObsID(b) {
		t.Error("dos commits con el mismo título pero archivos distintos NO deben colisionar")
	}
}

// S.a / S.b — end-to-end: el gemelo del squash NO crea una observación nueva y deja el contenido
// CANÓNICO (el del merge, con el (#193)).
func TestCaptureCommitsUpsertsSquashTwinInsteadOfDuplicating(t *testing.T) {
	store := &recordingStore{}

	// 1) La captura corre sobre la rama.
	g1 := &fakeGit{head: "h1", commits: []commit{{
		SHA: "h1", Subject: "feat(dedup): dedup semantico",
		Body: "Cuerpo del commit.\n\nCo-Authored-By: Claude <x@y>", Files: []string{"a.go", "b.go"},
	}}}
	if n, err := captureCommits(store, g1, nil, nil, memory.ScopeLocal); err != nil || n != 1 {
		t.Fatalf("captura de la rama = (%d, %v)", n, err)
	}
	if len(store.byID) != 1 {
		t.Fatalf("esperaba 1 observación tras la rama, hay %d", len(store.byID))
	}

	// 2) Squash-merge: mismo commit, con (#193) y el trailer reescrito por GitHub.
	g2 := &fakeGit{head: "h2", commits: []commit{{
		SHA: "h2", Subject: "feat(dedup): dedup semantico (#193)",
		Body: "Cuerpo del commit.\n\nCo-authored-by: Claude <x@y>", Files: []string{"a.go", "b.go"},
	}}}
	n, err := captureCommits(store, g2, nil, nil, memory.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}

	if len(store.byID) != 1 {
		t.Errorf("el gemelo del squash NO debe crear una observación nueva: hay %d (esperaba 1)", len(store.byID))
	}
	if n != 0 {
		t.Errorf("un UPSERT no cuenta como memoria nueva: reportó %d guardados", n)
	}
	// S.b — el contenido queda el CANÓNICO (el del merge).
	for _, content := range store.byID {
		if !strings.Contains(content, "(#193)") {
			t.Errorf("tras el UPSERT el contenido debe ser el del merge (con el (#193)), obtuve:\n%s", content)
		}
	}
}

// S.a / R7 — el gate de novedad (M4) NO corre sobre un UPSERT: no hay memoria nueva que relacionar.
func TestCaptureSkipsNoveltyGateOnSquashTwin(t *testing.T) {
	store := &recordingStore{}
	g1 := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "fix: algo importante", Files: []string{"a.go"}}}}
	if _, err := captureCommits(store, g1, nil, nil, memory.ScopeLocal); err != nil {
		t.Fatal(err)
	}

	detected := 0
	g2 := &fakeGit{head: "h2", commits: []commit{{SHA: "h2", Subject: "fix: algo importante (#42)", Files: []string{"a.go"}}}}
	if _, err := captureCommits(store, g2, nil, func(string) { detected++ }, memory.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	if detected != 0 {
		t.Errorf("un UPSERT del gemelo no debe disparar el gate de novedad, corrió %d veces", detected)
	}
}

// M4 — el gate de novedad corre sobre cada commit que REALMENTE se guarda.
func TestCaptureCommitsRunsNoveltyGateOnSavedCommits(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h2", commits: []commit{
		{SHA: "h1", Subject: "feat: algo importante y largo"},
		{SHA: "h2", Subject: "feat: otra cosa importante y larga"},
	}}
	var detected []string
	n, err := captureCommits(store, g, nil, func(id string) { detected = append(detected, id) }, memory.ScopeLocal)
	if err != nil || n != 2 {
		t.Fatalf("captureCommits = (%d, %v)", n, err)
	}
	if len(detected) != 2 {
		t.Errorf("el gate de novedad debe correr sobre los 2 commits guardados, corrió %d veces", len(detected))
	}
}

// M4 / R7-R8 — un commit cuyo id YA EXISTE es un UPSERT (no memoria nueva): no dispara el gate de
// novedad y no cuenta como guardado.
func TestCaptureCommitsUpsertDoesNotCountNorFireGate(t *testing.T) {
	store := &recordingStore{dedupeAll: true} // todo id se considera ya existente
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	detected := 0
	n, err := captureCommits(store, g, nil, func(string) { detected++ }, memory.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("un UPSERT no es memoria nueva: no debe contar como guardado, reportó %d", n)
	}
	if detected != 0 {
		t.Errorf("un UPSERT no debe disparar el gate (no hay observación nueva que relacionar), corrió %d veces", detected)
	}
}

// Sin detect (nil), la captura funciona igual: el gate es opcional (conflicts.enabled: false).
func TestCaptureCommitsWorksWithoutNoveltyGate(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	if n, err := captureCommits(store, g, nil, nil, memory.ScopeLocal); err != nil || n != 1 {
		t.Fatalf("sin gate la captura debe seguir funcionando: (%d, %v)", n, err)
	}
}

// Con embed no-nil, cada commit se guarda CON su vector (participa del recall semántico).
func TestCaptureCommitsEmbeds(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	embed := func(string) []float32 { return []float32{1, 2, 3} }
	n, err := captureCommits(store, g, embed, nil, memory.ScopeLocal)
	if err != nil || n != 1 {
		t.Fatalf("captureCommits = (%d, %v)", n, err)
	}
	if len(store.lastEmbed) != 3 {
		t.Errorf("esperaba el vector de la captura, obtuve %v", store.lastEmbed)
	}
}

// Con embed nil, el guardado es léxico (vector nil): comportamiento histórico.
func TestCaptureCommitsNilEmbedIsLexical(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	n, err := captureCommits(store, g, nil, nil, memory.ScopeLocal)
	if err != nil || n != 1 {
		t.Fatalf("captureCommits = (%d, %v)", n, err)
	}
	if store.lastEmbed != nil {
		t.Errorf("sin embed debería guardar con vector nil, obtuve %v", store.lastEmbed)
	}
}

// fakeGit implementa gitLog para el core sin repo real.
type fakeGit struct {
	head      string
	headErr   error
	commits   []commit
	sinceWith string
}

func (f *fakeGit) Head() (string, error) { return f.head, f.headErr }
func (f *fakeGit) CommitsSince(last string) ([]commit, error) {
	f.sinceWith = last
	return f.commits, nil
}

func newEngine(t *testing.T) *memory.DbEngine {
	t.Helper()
	e, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestCaptureFirstRunSavesHead(t *testing.T) {
	e := newEngine(t)
	g := &fakeGit{head: "abc123", commits: []commit{{SHA: "abc123", Subject: "feat: primera cosa", Files: []string{"a.go"}}}}
	n, err := captureCommits(e, g, nil, nil, memory.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("esperaba 1 guardado, obtuve %d", n)
	}
	if g.sinceWith != "" {
		t.Fatalf("primera corrida sin last: CommitsSince recibió %q", g.sinceWith)
	}
	if v, _, _ := e.GetMeta(metaCaptureLastCommit); v != "abc123" {
		t.Fatalf("meta no avanzó: %q", v)
	}
}

func TestCaptureIncremental(t *testing.T) {
	e := newEngine(t)
	_ = e.SetMeta(metaCaptureLastCommit, "old")
	g := &fakeGit{head: "new", commits: []commit{
		{Subject: "fix: bug uno"},
		{Subject: "feat: cosa dos"},
	}}
	n, err := captureCommits(e, g, nil, nil, memory.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("esperaba 2, obtuve %d", n)
	}
	if g.sinceWith != "old" {
		t.Fatalf("esperaba rango desde 'old', obtuve %q", g.sinceWith)
	}
	if v, _, _ := e.GetMeta(metaCaptureLastCommit); v != "new" {
		t.Fatalf("meta no avanzó a new: %q", v)
	}
}

func TestCaptureNoNewCommits(t *testing.T) {
	e := newEngine(t)
	_ = e.SetMeta(metaCaptureLastCommit, "same")
	g := &fakeGit{head: "same", commits: []commit{{Subject: "feat: no debería leerse"}}}
	n, err := captureCommits(e, g, nil, nil, memory.ScopeLocal)
	if err != nil || n != 0 {
		t.Fatalf("sin commits nuevos debe ser 0 sin error; n=%d err=%v", n, err)
	}
}

func TestCaptureTrivialSkippedButMetaAdvances(t *testing.T) {
	e := newEngine(t)
	g := &fakeGit{head: "h2", commits: []commit{
		{Subject: "chore: bump dependencies"}, // trivial → skip
		{Subject: "fix: real problem here"},   // capturado
	}}
	n, err := captureCommits(e, g, nil, nil, memory.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("esperaba 1 (chore omitido), obtuve %d", n)
	}
	if v, _, _ := e.GetMeta(metaCaptureLastCommit); v != "h2" {
		t.Fatalf("meta debe avanzar aunque se omitan triviales: %q", v)
	}
}

func TestCaptureNotAGitRepo(t *testing.T) {
	e := newEngine(t)
	g := &fakeGit{headErr: errors.New("not a git repository")}
	n, err := captureCommits(e, g, nil, nil, memory.ScopeLocal)
	if err != nil || n != 0 {
		t.Fatalf("sin repo git: no-op silencioso; n=%d err=%v", n, err)
	}
	if v, _, _ := e.GetMeta(metaCaptureLastCommit); v != "" {
		t.Fatalf("sin repo no debe setear meta: %q", v)
	}
}
