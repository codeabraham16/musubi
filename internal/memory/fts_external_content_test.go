package memory

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Tests adversariales de la FTS EXTERNAL-CONTENT (v17). El riesgo del cambio es la corrupción
// LÓGICA silenciosa: un índice que apunta a rowids/contenido viejos y devuelve basura sin fallar
// ruidosamente. Cada test fija un invariante que, de romperse, produciría justo eso.

// EL invariante crítico: VACUUM renumera los rowids de observations (no tiene INTEGER PRIMARY KEY)
// y la FTS external-content indexa por rowid. Compact DEBE rebuildear tras VACUUM; sin eso, la
// búsqueda devolvería filas equivocadas o vacío. Este test lo prueba end-to-end.
func TestFTSSurvivesVacuum(t *testing.T) {
	e := newTestEngine(t)
	seeds := map[string]string{"o0": "alfa unico", "o1": "beta unico", "o2": "gamma unico", "o3": "delta unico"}
	for id, c := range seeds {
		if err := e.SaveObservation(id, "t", c, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Borrar una fila crea un hueco de rowid que VACUUM va a renumerar (mueve los rowids de las
	// filas posteriores). Es exactamente el escenario que rompería la FTS sin el rebuild.
	if _, err := e.db.Exec(`DELETE FROM observations WHERE id='o1'`); err != nil {
		t.Fatal(err)
	}

	if err := e.Compact(true); err != nil { // VACUUM + rebuild de la FTS
		t.Fatalf("Compact(vacuum): %v", err)
	}

	// El índice quedó íntegro y sincronizado con el contenido (rank=1 lo verifica).
	if err := ftsIntegrityErr(e); err != nil {
		t.Fatalf("la FTS quedó desincronizada tras VACUUM (¿falta el rebuild en Compact?): %v", err)
	}
	// Y la búsqueda encuentra el contenido correcto por el rowid NUEVO.
	res, err := e.SearchObservationsFTS(context.Background(), "gamma", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "o2" {
		t.Errorf("tras VACUUM, 'gamma' debe seguir devolviendo o2, obtuve %+v", res)
	}
	// La fila borrada no aparece.
	if r, _ := e.SearchObservationsFTS(context.Background(), "beta", 10); len(r) != 0 {
		t.Errorf("la fila borrada no debe encontrarse tras VACUUM, obtuve %+v", r)
	}
}

// Un UPDATE de contenido re-indexa (patrón external-content: 'delete' del viejo + insert del nuevo):
// el término viejo deja de encontrarse y el nuevo sí.
func TestFTSUpdateReindexes(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t", "manzana verde", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("x", "t", "banana amarilla", nil); err != nil { // UPSERT → UPDATE
		t.Fatal(err)
	}
	if r, _ := e.SearchObservationsFTS(context.Background(), "manzana", 10); len(r) != 0 {
		t.Errorf("el término viejo no debe encontrarse tras el update, obtuve %+v", r)
	}
	if r, _ := e.SearchObservationsFTS(context.Background(), "banana", 10); len(r) != 1 || r[0].ID != "x" {
		t.Errorf("el término nuevo debe encontrarse tras el update, obtuve %+v", r)
	}
	if err := ftsIntegrityErr(e); err != nil {
		t.Fatalf("la FTS quedó inconsistente tras el update: %v", err)
	}
}

// Un DELETE saca la fila del índice (trigger AD external-content 'delete').
func TestFTSDeleteRemovesFromIndex(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t", "zanahoria naranja", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("y", "t", "zanahoria morada", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`DELETE FROM observations WHERE id='x'`); err != nil {
		t.Fatal(err)
	}
	res, err := e.SearchObservationsFTS(context.Background(), "zanahoria", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "y" {
		t.Errorf("tras borrar x, sólo y debe quedar buscable, obtuve %+v", res)
	}
	if err := ftsIntegrityErr(e); err != nil {
		t.Fatalf("la FTS quedó inconsistente tras el delete: %v", err)
	}
}

// La migración v17 convierte una FTS REGULAR (con datos) a external-content, y la fila migrada
// queda buscable por el join de rowid. Se construye el estado pre-v17 a mano en una base cruda.
func TestV17MigrationConvertsRegularFTS(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Estado pre-v17: observations + FTS REGULAR (guarda su copia, columna id) + una fila indexada.
	for _, stmt := range []string{
		`CREATE TABLE observations (id TEXT PRIMARY KEY, topic_key TEXT NOT NULL, content TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE VIRTUAL TABLE observations_fts USING fts5(id UNINDEXED, topic_key UNINDEXED, content)`,
		`INSERT INTO observations(id, topic_key, content) VALUES('m','t','contenido migrado unico')`,
		`INSERT INTO observations_fts(id, topic_key, content) VALUES('m','t','contenido migrado unico')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup pre-v17: %v", err)
		}
	}

	// Correr SÓLO la migración v17 sobre esa base.
	var v17 migration
	for _, m := range schemaMigrations() {
		if m.version == 17 {
			v17 = m
		}
	}
	if v17.up == nil {
		t.Fatal("no se encontró la migración v17")
	}
	if err := v17.up(db); err != nil {
		t.Fatalf("v17.up: %v", err)
	}

	// Ahora es external-content.
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='observations_fts'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "content=") {
		t.Errorf("la FTS debe ser external-content tras v17, DDL=%q", ddl)
	}
	// La fila migrada es buscable por el join de rowid (no por id).
	var id string
	err = db.QueryRow(`SELECT o.id FROM observations_fts f JOIN observations o ON o.rowid=f.rowid WHERE observations_fts MATCH 'migrado'`).Scan(&id)
	if err != nil || id != "m" {
		t.Errorf("la fila migrada debe ser buscable por rowid, id=%q err=%v", id, err)
	}
	// Índice íntegro y consistente con el contenido.
	if _, err := db.Exec(`INSERT INTO observations_fts(observations_fts, rank) VALUES('integrity-check', 1)`); err != nil {
		t.Errorf("integrity-check tras la migración: %v", err)
	}
}

// La conversión v17 es idempotente y segura sobre una base pre-FTS (observations con filas pero
// SIN índice poblado): el 'rebuild' que corre SIEMPRE puebla esas filas, evitando que un UPDATE
// posterior dispare el 'delete' sobre una entrada inexistente y corrompa el índice.
func TestV17RebuildsWhenAlreadyExternalButUnpopulated(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "prefts.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE observations (id TEXT PRIMARY KEY, topic_key TEXT NOT NULL, content TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO observations(id, topic_key, content) VALUES('p','t','fila sin indexar')`,
		// FTS external-content YA creada (como haría la baseline) pero VACÍA: la fila 'p' no está indexada.
		ftsTableDDL,
		fmt.Sprintf(`%s %s %s`, ftsTriggerAI, ftsTriggerAD, ftsTriggerAU),
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	var v17 migration
	for _, m := range schemaMigrations() {
		if m.version == 17 {
			v17 = m
		}
	}
	if err := v17.up(db); err != nil {
		t.Fatalf("v17.up: %v", err)
	}
	// La fila quedó indexada por el rebuild.
	var id string
	if err := db.QueryRow(`SELECT o.id FROM observations_fts f JOIN observations o ON o.rowid=f.rowid WHERE observations_fts MATCH 'indexar'`).Scan(&id); err != nil || id != "p" {
		t.Errorf("el rebuild de v17 debe indexar la fila pre-existente, id=%q err=%v", id, err)
	}
	// Y un UPDATE ya no corrompe (la entrada existe, el 'delete' del trigger la encuentra).
	if _, err := db.Exec(`UPDATE observations SET content='fila actualizada' WHERE id='p'`); err != nil {
		t.Fatalf("update tras rebuild: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO observations_fts(observations_fts, rank) VALUES('integrity-check', 1)`); err != nil {
		t.Errorf("integrity-check tras update: %v", err)
	}
}
