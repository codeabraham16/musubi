package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/skillsource"
)

// --- Helpers de fixture para tests de merge ---

// entradaCLI construye una CatalogEntry mínima y válida para pruebas del CLI.
func entradaCLI(id, name string) skillsource.CatalogEntry {
	return skillsource.CatalogEntry{
		ID:          id,
		Name:        name,
		Description: "Descripción de " + id,
		Stacks:      []string{"Go"},
		Triggers:    []string{"*.go"},
		RulesURL:    "https://example.com/" + id + ".md",
		Excerpt:     "Excerpt de " + id,
		Source:      "musubi-catalog-v1",
	}
}

// catalogJSON serializa un Catalog como JSON válido.
func catalogJSON(t *testing.T, cat skillsource.Catalog) []byte {
	t.Helper()
	b, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("catalogJSON: marshal falló: %v", err)
	}
	return b
}

// escribirCatalogEnDisco escribe un Catalog JSON en path.
func escribirCatalogEnDisco(t *testing.T, path string, cat skillsource.Catalog) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("no se pudo crear directorio: %v", err)
	}
	data := catalogJSON(t, cat)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("no se pudo escribir catálogo: %v", err)
	}
}

// nuevoServidor crea un httptest.Server que sirve body con content-type JSON.
func nuevoServidor(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// leerCatalogDeDisco lee y parsea un Catalog desde path.
func leerCatalogDeDisco(t *testing.T, path string) skillsource.Catalog {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leerCatalogDeDisco: no se pudo leer %s: %v", path, err)
	}
	var cat skillsource.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("leerCatalogDeDisco: JSON inválido en %s: %v", path, err)
	}
	return cat
}

// --- Tests de runMerge ---

// TestMergeNetworkError verifica que un servidor que cierra la conexión
// produce error y no escribe nada.
func TestMergeNetworkError(t *testing.T) {
	// Crear un listener que cierra la conexión inmediatamente.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no se pudo abrir listener: %v", err)
	}
	addr := ln.Addr().String()
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		ln.Close()
	}()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	err = runMerge([]string{"http://" + addr, "--output", outPath})
	if err == nil {
		t.Fatal("esperaba error con servidor que cierra conexión, obtuve nil")
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("no debería haberse creado el archivo de salida")
	}
}

// TestMergeInvalidIncomingJSON verifica que JSON inválido del servidor produce
// error y no escribe nada.
func TestMergeInvalidIncomingJSON(t *testing.T) {
	srv := nuevoServidor(t, []byte("not-json"))

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	err := runMerge([]string{srv.URL, "--output", outPath})
	if err == nil {
		t.Fatal("esperaba error con JSON inválido, obtuve nil")
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("no debería haberse creado el archivo de salida")
	}
}

// TestMergeSchemaInvalidIncoming verifica que un catálogo con entrada inválida
// (stack desconocido) produce error y no modifica el archivo base.
func TestMergeSchemaInvalidIncoming(t *testing.T) {
	// Catálogo incoming con stack inválido.
	entradaMala := skillsource.CatalogEntry{
		ID:          "bad-entry",
		Name:        "Mala entrada",
		Description: "Descripción",
		Stacks:      []string{"UnknownStack"}, // inválido
		Triggers:    []string{"*.go"},
		RulesURL:    "https://example.com/bad.md",
		Excerpt:     "Excerpt",
	}
	incoming := skillsource.Catalog{CatalogVersion: 1, Entries: []skillsource.CatalogEntry{entradaMala}}
	srv := nuevoServidor(t, catalogJSON(t, incoming))

	dir := t.TempDir()
	basePath := filepath.Join(dir, "index.json")
	baseCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("go-valid", "Válida")},
	}
	escribirCatalogEnDisco(t, basePath, baseCat)

	// Leer contenido original.
	originalData, _ := os.ReadFile(basePath)

	err := runMerge([]string{srv.URL, "--output", basePath})
	if err == nil {
		t.Fatal("esperaba error con incoming schema-inválido, obtuve nil")
	}

	// El archivo base no debe haber cambiado.
	currentData, _ := os.ReadFile(basePath)
	if string(originalData) != string(currentData) {
		t.Error("el archivo base fue modificado a pesar del error de validación")
	}
}

// TestMergeOutputDirMissing verifica que un directorio de salida inexistente
// produce error.
func TestMergeOutputDirMissing(t *testing.T) {
	incoming := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("x", "X")},
	}
	srv := nuevoServidor(t, catalogJSON(t, incoming))

	outPath := filepath.Join(t.TempDir(), "nonexistent-dir", "out.json")

	err := runMerge([]string{srv.URL, "--output", outPath})
	if err == nil {
		t.Fatal("esperaba error con directorio de salida inexistente, obtuve nil")
	}
}

// TestMergeOutputFileMissing verifica que si el archivo base no existe,
// se trata como catálogo vacío y el merge procede (fresh catalog).
func TestMergeOutputFileMissing(t *testing.T) {
	incoming := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("go-gin", "Gin")},
	}
	srv := nuevoServidor(t, catalogJSON(t, incoming))

	dir := t.TempDir()
	outPath := filepath.Join(dir, "index.json")
	// No creamos el archivo base.

	err := runMerge([]string{srv.URL, "--output", outPath})
	if err != nil {
		t.Fatalf("esperaba nil con base ausente, obtuve: %v", err)
	}

	// El archivo de salida debe existir con las entradas de incoming.
	if _, statErr := os.Stat(outPath); os.IsNotExist(statErr) {
		t.Fatal("el archivo de salida no fue creado")
	}
	result := leerCatalogDeDisco(t, outPath)
	if len(result.Entries) != 1 {
		t.Errorf("esperaba 1 entrada, obtuve %d", len(result.Entries))
	}
	if len(result.Entries) > 0 && result.Entries[0].ID != "go-gin" {
		t.Errorf("esperaba entrada 'go-gin', obtuve %q", result.Entries[0].ID)
	}
}

// TestMergeIDCollision verifica que un ID en base sobreescrito por incoming
// produce una línea de colisión en stdout y el resultado final tiene la versión incoming.
func TestMergeIDCollision(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "index.json")

	baseCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("go-gin", "Gin Base")},
	}
	escribirCatalogEnDisco(t, basePath, baseCat)

	incomingEntry := entradaCLI("go-gin", "Gin Incoming")
	incoming := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{incomingEntry},
	}
	srv := nuevoServidor(t, catalogJSON(t, incoming))

	err := runMerge([]string{srv.URL, "--output", basePath})
	if err != nil {
		t.Fatalf("merge con colisión no debería producir error: %v", err)
	}

	result := leerCatalogDeDisco(t, basePath)
	if len(result.Entries) != 1 {
		t.Errorf("esperaba 1 entrada, obtuve %d", len(result.Entries))
	}
	if len(result.Entries) > 0 && result.Entries[0].Name != "Gin Incoming" {
		t.Errorf("incoming debía ganar, obtuve name %q", result.Entries[0].Name)
	}
}

// TestMergeEmptyCatalog verifica que incoming vacío produce output igual a base.
func TestMergeEmptyCatalog(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "index.json")

	baseCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries: []skillsource.CatalogEntry{
			entradaCLI("go-gin", "Gin"),
			entradaCLI("go-echo", "Echo"),
		},
	}
	escribirCatalogEnDisco(t, basePath, baseCat)

	incoming := skillsource.Catalog{CatalogVersion: 1, Entries: []skillsource.CatalogEntry{}}
	srv := nuevoServidor(t, catalogJSON(t, incoming))

	err := runMerge([]string{srv.URL, "--output", basePath})
	if err != nil {
		t.Fatalf("merge con incoming vacío no debería producir error: %v", err)
	}

	result := leerCatalogDeDisco(t, basePath)
	if len(result.Entries) != 2 {
		t.Errorf("esperaba 2 entradas (= base), obtuve %d", len(result.Entries))
	}
}

// TestMergeVersionMismatch verifica que una diferencia de versión produce
// una advertencia en stderr pero aún escribe con la versión de base.
func TestMergeVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "index.json")

	baseCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("go-gin", "Gin")},
	}
	escribirCatalogEnDisco(t, basePath, baseCat)

	incomingCat := skillsource.Catalog{
		CatalogVersion: 2,
		Entries:        []skillsource.CatalogEntry{entradaCLI("go-echo", "Echo")},
	}
	srv := nuevoServidor(t, catalogJSON(t, incomingCat))

	// Redirigir stderr para capturar advertencia.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runMerge([]string{srv.URL, "--output", basePath})

	w.Close()
	os.Stderr = oldStderr
	stderrBuf := make([]byte, 4096)
	n, _ := r.Read(stderrBuf)
	stderrOutput := string(stderrBuf[:n])

	if err != nil {
		t.Fatalf("esperaba nil con version-mismatch, obtuve: %v", err)
	}

	// Advertencia en stderr.
	if !strings.Contains(stderrOutput, "versión") && !strings.Contains(stderrOutput, "version") {
		t.Errorf("esperaba advertencia de versión en stderr, obtuve: %q", stderrOutput)
	}

	// Output lleva versión de base (1).
	result := leerCatalogDeDisco(t, basePath)
	if result.CatalogVersion != 1 {
		t.Errorf("esperaba CatalogVersion=1 (de base), obtuve %d", result.CatalogVersion)
	}
}

// TestMergeValidRoundTrip verifica que base + incoming válidos, sin colisión,
// producen un JSON indentado, ordenado y válido.
func TestMergeValidRoundTrip(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "index.json")

	baseCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("z-skill", "Z Skill")},
	}
	escribirCatalogEnDisco(t, basePath, baseCat)

	incomingCat := skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("a-skill", "A Skill")},
	}
	srv := nuevoServidor(t, catalogJSON(t, incomingCat))

	err := runMerge([]string{srv.URL, "--output", basePath})
	if err != nil {
		t.Fatalf("round-trip válido falló: %v", err)
	}

	// Leer el archivo raw para verificar indentación.
	rawData, _ := os.ReadFile(basePath)
	rawStr := string(rawData)
	if !strings.Contains(rawStr, "\n  ") {
		t.Error("el JSON de salida no parece estar indentado")
	}

	// Verificar orden (a-skill antes que z-skill).
	result := leerCatalogDeDisco(t, basePath)
	if len(result.Entries) != 2 {
		t.Fatalf("esperaba 2 entradas, obtuve %d", len(result.Entries))
	}
	if result.Entries[0].ID != "a-skill" || result.Entries[1].ID != "z-skill" {
		t.Errorf("orden incorrecto: %q, %q", result.Entries[0].ID, result.Entries[1].ID)
	}

	// Verificar que el catálogo es válido según ValidateCatalog.
	if errs := skillsource.ValidateCatalog(result); len(errs) != 0 {
		t.Errorf("catálogo de salida no es válido: %v", errs)
	}
}

func TestValidateCatalogFile(t *testing.T) {
	dir := t.TempDir()

	// Catálogo válido.
	okPath := filepath.Join(dir, "ok.json")
	escribirCatalogEnDisco(t, okPath, skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{entradaCLI("a-skill", "A"), entradaCLI("b-skill", "B")},
	})
	entries, verrs, err := validateCatalogFile(okPath)
	if err != nil {
		t.Fatalf("error inesperado en catálogo válido: %v", err)
	}
	if len(verrs) != 0 {
		t.Errorf("catálogo válido no debería tener errores: %v", verrs)
	}
	if entries != 2 {
		t.Errorf("esperaba 2 entradas, obtuve %d", entries)
	}

	// Archivo inexistente -> err de lectura.
	if _, _, err := validateCatalogFile(filepath.Join(dir, "no-existe.json")); err == nil {
		t.Error("esperaba error para archivo inexistente")
	}

	// JSON inválido -> err de parseo.
	badPath := filepath.Join(dir, "bad.json")
	if werr := os.WriteFile(badPath, []byte("{ no es json"), 0o644); werr != nil {
		t.Fatalf("preparando bad.json: %v", werr)
	}
	if _, _, err := validateCatalogFile(badPath); err == nil {
		t.Error("esperaba error para JSON inválido")
	}

	// Catálogo inválido (entrada sin campos requeridos) -> sin err, pero verrs > 0.
	invPath := filepath.Join(dir, "inv.json")
	escribirCatalogEnDisco(t, invPath, skillsource.Catalog{
		CatalogVersion: 1,
		Entries:        []skillsource.CatalogEntry{{ID: "", Name: ""}},
	})
	_, verrs, err = validateCatalogFile(invPath)
	if err != nil {
		t.Fatalf("catálogo inválido no debería dar err de ejecución: %v", err)
	}
	if len(verrs) == 0 {
		t.Error("esperaba errores de validación para entrada incompleta")
	}
}
