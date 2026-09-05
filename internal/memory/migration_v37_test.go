package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// LA MIGRACIÓN 37 SEPARA LO DECLARADO A MANO DE LO QUE REPORTA LA MÁQUINA, SIN PERDER NADA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTA PRUEBA CUSTODIA DE VERDAD ES EL BACKFILL, NO LA COLUMNA
//
// Agregar `declared` es trivial. Lo delicado es qué pasa con las filas QUE YA ESTÁN, porque un
// `DEFAULT 0` a ciegas deja podable todo lo que alguien declaró antes del despliegue — y como la
// poda es irreversible y redeclarar chocaba contra el índice único, eso se pierde en el primer
// latido con inventario y no vuelve.
//
// El backfill usa la firma exacta de AltaServicio: `last_report IS NULL`. Es el único camino que
// inserta con esa columna en NULL; el agente siempre escribe la fecha del latido. Así, la máquina
// vieja que declaró alguien queda protegida y la que enumeró el agente queda podable, sin que
// nadie tenga que acordarse de nada.
//
// Sabotaje que la hace fallar: cambiar el backfill por `UPDATE services SET declared = 1` a secas
// (la fila reportada queda protegida y la poda deja de podar), o borrarlo (la fila declarada queda
// desprotegida y el primer latido se la lleva).
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestMigracionV37MarcaDeclaradoLoQueNuncaReporto(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := applyMigrations(db, migrationsUpTo(36)); err != nil {
		t.Fatalf("migrar hasta v36: %v", err)
	}
	if _, err := db.Exec(`SELECT declared FROM services LIMIT 1`); err == nil {
		t.Fatal("la columna `declared` ya existía en la v36: la migración 37 está declarada de menos")
	}

	// Dos filas de una base vieja: la DECLARADA a mano (nunca reportó) y la que trajo un latido.
	mustExec(t, db, `INSERT INTO services (id, name, project_id, device_id, kind, registered_at, last_report, last_health, revoked)
	                 VALUES (?,?,?,?,?,?,NULL,'',0)`,
		"sv-declarado", "bot-telegram", "casa", "pc-gio", "docker", "2026-08-01T00:00:00Z")
	mustExec(t, db, `INSERT INTO services (id, name, project_id, device_id, kind, registered_at, last_report, last_health, revoked)
	                 VALUES (?,?,?,?,?,?,?,'',0)`,
		"sv-reportado", "sshd.service", "casa", "pc-gio", "systemd", "2026-08-01T00:00:00Z", "2026-08-02T00:00:00Z")

	if err := applyMigrations(db, migrationsUpTo(37)); err != nil {
		t.Fatalf("aplicar v37: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 37 {
		t.Fatalf("user_version=%d tras la v37, esperaba 37", v)
	}

	for _, c := range []struct {
		id      string
		quiero  int
		porque  string
		nombre  string
		podable string
	}{
		{"sv-declarado", 1, "nunca reportó: sólo AltaServicio inserta con last_report en NULL, así que lo declaró una persona", "bot-telegram", "quedaría a merced de la poda del primer latido con inventario"},
		{"sv-reportado", 0, "tiene last_report: lo trajo un latido, y la poda por ausencia es justo lo que le corresponde", "sshd.service", "la poda dejaría de podar lo que la máquina dejó de reportar"},
	} {
		var declarado int
		if err := db.QueryRow(`SELECT declared FROM services WHERE id = ?`, c.id).Scan(&declarado); err != nil {
			t.Fatalf("la fila %q no sobrevivió la migración: %v", c.nombre, err)
		}
		if declarado != c.quiero {
			t.Errorf("%q quedó con declared=%d y esperaba %d (%s): %s", c.nombre, declarado, c.quiero, c.porque, c.podable)
		}
	}

	// IDEMPOTENTE: una prueba que rebobina user_version la corre dos veces, y lo sufrieron la v21 y
	// la v22. `agregarColumnaSiFalta` mira el PRAGMA y el UPDATE marca las mismas filas.
	mustExec(t, db, `PRAGMA user_version = 36`)
	if err := applyMigrations(db, migrationsUpTo(37)); err != nil {
		t.Fatalf("la v37 no es idempotente: %v", err)
	}
}
