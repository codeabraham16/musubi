package memory

import (
	"database/sql"
	"fmt"
	"testing"
)

// TestProbeBaseSanaDiceAlDia: sobre una base recién migrada, el probe dice «al día», publica el
// esquema real, declara que abrió en sólo lectura, y —lo central— NO ESCRIBE. Esto último se mide de
// dos formas independientes: query_only hace que un INTENTO de escritura sea un error del motor, y el
// user_version queda idéntico después del probe (una migración lo habría movido).
func TestProbeBaseSanaDiceAlDia(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	// Una nota, para que los conteos no sean todos cero y el informe signifique algo.
	if err := eng.SaveObservation("", "probe/prueba", "una nota para contar", nil); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	eng.Close()

	uvAntes := leerUserVersion(t, dir)

	ro, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer ro.Close()

	rep, err := ro.Probe()
	if err != nil {
		t.Fatalf("Probe devolvió error: %v", err)
	}

	if rep.Estado != EsquemaAlDia {
		t.Errorf("Estado = %q; quería %q", rep.Estado, EsquemaAlDia)
	}
	if rep.SchemaActual != latestSchemaVersion() || rep.SchemaEsperado != latestSchemaVersion() {
		t.Errorf("esquema actual=%d esperado=%d; quería ambos = %d", rep.SchemaActual, rep.SchemaEsperado, latestSchemaVersion())
	}
	if !rep.SoloLectura {
		t.Error("SoloLectura = false; un probe sobre un engine solo-lectura tiene que decir true")
	}
	if n, ok := rep.Conteos["observations"]; !ok || n < 1 {
		t.Errorf("Conteos[observations] = %d (ok=%v); quería >=1 (sembramos una nota)", n, ok)
	}
	if _, ok := rep.Conteos["observation_relations"]; !ok {
		t.Error("Conteos no trae observation_relations")
	}

	// Garantía read-only #1: query_only de verdad rechaza una escritura por este engine.
	if _, werr := ro.db.Exec(`CREATE TABLE _probe_no_deberia (x INTEGER)`); werr == nil {
		t.Error("una escritura por el engine del probe NO falló: query_only no está protegiendo la base")
	}

	// Garantía read-only #2: el probe no movió el esquema.
	if uvDespues := leerUserVersion(t, dir); uvDespues != uvAntes {
		t.Errorf("user_version cambió por correr el probe: antes=%d después=%d (el probe migró la base)", uvAntes, uvDespues)
	}
}

// TestProbeClasificaElEstado ejercita las tres clases restantes fabricando el escenario en crudo
// (user_version / read_floor), sin migrar de verdad: lo que se mide es la REGLA DE DECISIÓN del
// probe, que tiene que coincidir con la de applyMigrations.
func TestProbeClasificaElEstado(t *testing.T) {
	latest := latestSchemaVersion()
	casos := []struct {
		nombre string
		mutar  func(t *testing.T, db *sql.DB)
		quiero EstadoEsquema
	}{
		{
			nombre: "mas vieja -> migrable",
			mutar:  func(t *testing.T, db *sql.DB) { fijarUserVersion(t, db, latest-1) },
			quiero: EsquemaMigrable,
		},
		{
			nombre: "mas nueva con piso alcanzable -> legible",
			// El piso lo grabó NewDbEngine en su valor real (<= latest); sólo adelantamos el esquema.
			mutar:  func(t *testing.T, db *sql.DB) { fijarUserVersion(t, db, latest+5) },
			quiero: EsquemaMasNuevaLegible,
		},
		{
			nombre: "mas nueva con piso por encima -> ilegible",
			mutar: func(t *testing.T, db *sql.DB) {
				fijarUserVersion(t, db, latest+5)
				if _, err := db.Exec(`UPDATE schema_floor SET read_floor = ? WHERE id = 1`, latest+1); err != nil {
					t.Fatalf("subir el piso: %v", err)
				}
			},
			quiero: EsquemaMasNuevaIlegible,
		},
		{
			nombre: "mas nueva sin piso grabado -> ilegible",
			mutar: func(t *testing.T, db *sql.DB) {
				fijarUserVersion(t, db, latest+5)
				if _, err := db.Exec(`DELETE FROM schema_floor`); err != nil {
					t.Fatalf("borrar el piso: %v", err)
				}
			},
			quiero: EsquemaMasNuevaIlegible,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			eng, err := NewDbEngine(dir)
			if err != nil {
				t.Fatalf("NewDbEngine: %v", err)
			}
			eng.Close()

			db := abrirCruda(t, dir)
			c.mutar(t, db)
			db.Close()

			ro, err := NewDbEngineSoloLectura(dir)
			if err != nil {
				t.Fatalf("NewDbEngineSoloLectura: %v", err)
			}
			defer ro.Close()

			rep, err := ro.Probe()
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if rep.Estado != c.quiero {
				t.Errorf("Estado = %q; quería %q (actual=%d esperado=%d piso=%d grabado=%v)",
					rep.Estado, c.quiero, rep.SchemaActual, rep.SchemaEsperado, rep.PisoDeLectura, rep.PisoGrabado)
			}
		})
	}
}

// fijarUserVersion escribe el PRAGMA user_version en crudo. Va con Sprintf porque PRAGMA no admite
// placeholder, y el valor es un int del test, no input.
func fijarUserVersion(t *testing.T, db *sql.DB, v int) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
		t.Fatalf("fijar user_version=%d: %v", v, err)
	}
}

// leerUserVersion abre la base en crudo y devuelve su user_version.
func leerUserVersion(t *testing.T, dir string) int {
	t.Helper()
	db := abrirCruda(t, dir)
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	return v
}
