package main

import (
	"errors"
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
}

func (r *recordingStore) GetMeta(k string) (string, bool, error) { v, ok := r.meta[k]; return v, ok, nil }
func (r *recordingStore) SetMeta(k, v string) error {
	if r.meta == nil {
		r.meta = map[string]string{}
	}
	r.meta[k] = v
	return nil
}
func (r *recordingStore) SaveObservationDedupedTyped(_, _ string, _ float64, _, _ string, emb []float32) (string, bool, error) {
	r.lastEmbed = emb
	r.saved++
	return "id", false, nil
}

// Con embed no-nil, cada commit se guarda CON su vector (participa del recall semántico).
func TestCaptureCommitsEmbeds(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "h1", commits: []commit{{SHA: "h1", Subject: "feat: algo importante y largo"}}}
	embed := func(string) []float32 { return []float32{1, 2, 3} }
	n, err := captureCommits(store, g, embed)
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
	n, err := captureCommits(store, g, nil)
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
	n, err := captureCommits(e, g, nil)
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
	n, err := captureCommits(e, g, nil)
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
	n, err := captureCommits(e, g, nil)
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
	n, err := captureCommits(e, g, nil)
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
	n, err := captureCommits(e, g, nil)
	if err != nil || n != 0 {
		t.Fatalf("sin repo git: no-op silencioso; n=%d err=%v", n, err)
	}
	if v, _, _ := e.GetMeta(metaCaptureLastCommit); v != "" {
		t.Fatalf("sin repo no debe setear meta: %q", v)
	}
}
