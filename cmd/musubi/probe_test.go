package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"

	_ "modernc.org/sqlite"
)

// TestRunDoctorProbeNoMigraBaseVieja: sobre una base MÁS VIEJA que este binario, el probe reporta
// (sale 0, es legible) SIN migrarla. Es el detector de sabotaje central: si runDoctorProbe abriera
// con NewDbEngine en vez de NewDbEngineSoloLectura, migraría la base y el user_version cambiaría.
func TestRunDoctorProbeNoMigraBaseVieja(t *testing.T) {
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	// Fingir una base más vieja: bajar el user_version por debajo del que conoce este binario.
	vieja := memory.EsquemaEsperado() - 1
	db := abrirBaseCrudaParaProbe(t, dir)
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, vieja)); err != nil {
		t.Fatalf("bajar user_version: %v", err)
	}
	db.Close()

	if code := runDoctorProbe(dir, false); code != 0 {
		t.Errorf("runDoctorProbe salió %d; una base migrable es legible y tiene que salir 0", code)
	}

	if uv := leerUserVersionMain(t, dir); uv != vieja {
		t.Errorf("user_version = %d tras el probe; quería %d (el probe MIGRÓ la base: no era sólo lectura)", uv, vieja)
	}
}

// TestRunDoctorProbeSirveBaseMasNueva: sobre una base MÁS NUEVA pero legible —donde el arranque
// normal con NewDbEngine se NIEGA (ErrEsquemaLegible)—, el probe igual abre y sale 0. Segundo
// detector de sabotaje, por otro mecanismo: si abriera con NewDbEngine, no podría abrir y saldría 1.
func TestRunDoctorProbeSirveBaseMasNueva(t *testing.T) {
	dir := baseMasNuevaPeroLegible(t)
	if code := runDoctorProbe(dir, false); code != 0 {
		t.Errorf("runDoctorProbe salió %d sobre una base más nueva pero legible; quería 0", code)
	}
}

// TestFormatProbeTextoYJson: el reporte sale legible en texto y válido en JSON (R3/R5).
func TestFormatProbeTextoYJson(t *testing.T) {
	rep := memory.ProbeReport{
		SchemaActual:   60,
		SchemaEsperado: 55,
		PisoDeLectura:  43,
		PisoGrabado:    true,
		Estado:         memory.EsquemaMasNuevaLegible,
		SoloLectura:    true,
		Conteos:        map[string]int{"observations": 7, "observation_relations": 2},
	}

	var texto bytes.Buffer
	formatProbe(&texto, rep, false)
	for _, quiero := range []string{"mas_nueva_legible", "v60", "v55", "v43", "observations", "7"} {
		if !strings.Contains(texto.String(), quiero) {
			t.Errorf("el texto del probe no contiene %q:\n%s", quiero, texto.String())
		}
	}

	var jsonBuf bytes.Buffer
	formatProbe(&jsonBuf, rep, true)
	var vuelta memory.ProbeReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &vuelta); err != nil {
		t.Fatalf("el --json del probe no es JSON válido: %v\n%s", err, jsonBuf.String())
	}
	if vuelta.Estado != memory.EsquemaMasNuevaLegible || vuelta.SchemaActual != 60 {
		t.Errorf("el JSON no reconstruye el reporte: %+v", vuelta)
	}
}

func abrirBaseCrudaParaProbe(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func leerUserVersionMain(t *testing.T, dir string) int {
	t.Helper()
	db := abrirBaseCrudaParaProbe(t, dir)
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	return v
}
