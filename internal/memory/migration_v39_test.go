package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// LA MIGRACIÓN 39 RECREA LA TABLA DE COOLDOWNS SIN PERDER LOS QUE YA ESTÁN.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTA PRUEBA CUSTODIA ES LA COPIA, Y HAY QUE PARTIR DE UNA BASE VIEJA PARA VERLA
//
// Una prueba que arranca con una base nueva no puede cazar la pérdida: no hay nada que perder, la
// tabla se crea vacía y todo pasa. (Esa fue, literalmente, la primera versión — el sabotaje de
// borrar el `INSERT ... SELECT` la dejó en verde.) Así que hay que migrar HASTA LA 38, escribir
// filas como las que existen en producción, y recién ahí correr la 39.
//
// Y el costo de perderlas es concreto: un cooldown que desaparece es una política que puede
// actuar antes de tiempo, una vez, justo después de un despliegue — que es cuando más cosas
// están disparando y menos ganas hay de una acción de más.
//
// Sabotaje que la hace fallar: borrar el `INSERT OR IGNORE ... SELECT` de la migración.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestMigracionV39ConservaLosCooldownsDeUnaBaseVieja(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := applyMigrations(db, migrationsUpTo(38)); err != nil {
		t.Fatalf("migrar hasta v38: %v", err)
	}
	// En la v38 la tabla NO tiene `alcance`: si la tuviera, la 39 estaría declarada de menos.
	if _, err := db.Exec(`SELECT alcance FROM fleet_policy_state LIMIT 1`); err == nil {
		t.Fatal("la columna `alcance` ya existía en la v38")
	}

	// Dos cooldowns como los que hay en una base real: políticas de host sobre dos máquinas.
	mustExec(t, db, `INSERT INTO fleet_policy_state (policy, device_id, last_fired) VALUES (?,?,?)`,
		"purgar-journal", "dev-1", "2026-08-01T10:00:00Z")
	mustExec(t, db, `INSERT INTO fleet_policy_state (policy, device_id, last_fired) VALUES (?,?,?)`,
		"purgar-journal", "dev-2", "2026-08-01T11:00:00Z")

	if err := applyMigrations(db, migrationsUpTo(39)); err != nil {
		t.Fatalf("migrar a v39: %v", err)
	}

	// LOS DOS SIGUEN ESTANDO, con alcance vacío — que es lo que corresponde a una política de host.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM fleet_policy_state WHERE alcance = ''`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("sobrevivieron %d cooldowns de 2: recrear la tabla se los llevó, y eso es una "+
			"política que actúa antes de tiempo justo después de un despliegue", n)
	}
	var cuando string
	if err := db.QueryRow(
		`SELECT last_fired FROM fleet_policy_state WHERE policy = ? AND device_id = ?`,
		"purgar-journal", "dev-2").Scan(&cuando); err != nil {
		t.Fatalf("se perdió el cooldown de dev-2: %v", err)
	}
	if cuando != "2026-08-01T11:00:00Z" {
		t.Errorf("la fecha del cooldown cambió al copiar: %q", cuando)
	}

	// Y la tabla vieja NO quedó dando vueltas: dos tablas con el mismo propósito es cómo una
	// consulta termina leyendo la que nadie escribe.
	var sobrante int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='fleet_policy_state_v2'`).Scan(&sobrante); err != nil {
		t.Fatal(err)
	}
	if sobrante != 0 {
		t.Error("quedó la tabla intermedia `fleet_policy_state_v2`: dos tablas con el mismo " +
			"propósito es cómo una consulta termina leyendo la que nadie escribe")
	}
}
