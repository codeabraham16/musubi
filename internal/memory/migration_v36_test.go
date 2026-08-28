package memory

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// LA MIGRACIÓN 36 CORRE SOBRE UNA BASE EN 35, NO PIERDE NADA Y ES IDEMPOTENTE.
//
// Las tres cosas juntas, porque las tres se rompen distinto:
//   - correr sobre la 35 es el caso real (la base de producción está ahí);
//   - no perder nada es lo que hace que se pueda desplegar sin una ventana;
//   - la idempotencia importa porque una migración SÍ corre dos veces cuando una prueba rebobina
//     user_version — lo sufrieron la v21 y la v22.
//
// Sabotaje que la hace fallar: declararla con `version: 35` (versión repetida: no se aplica sobre
// una base que ya está en 35 y la tabla nunca se crea), o sacarle los `IF NOT EXISTS` a la tabla y
// a los tres índices (la segunda corrida revienta).
func TestMigracionV36CreaLosServiciosSinPerderLaFlota(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Hasta la 35: `devices` ya existe con todas sus columnas, `services` todavía no.
	if err := applyMigrations(db, migrationsUpTo(35)); err != nil {
		t.Fatalf("migrar hasta v35: %v", err)
	}
	if _, err := db.Exec(`SELECT 1 FROM services LIMIT 1`); err == nil {
		t.Fatal("la tabla `services` ya existía en la v35: la migración 36 está declarada de menos")
	}
	mustExec(t, db, `INSERT INTO devices (id, name, project_id, tier, caps, os, arch, address, agent_version, tags, enrolled_at, revoked, last_sample, rustdesk_id, token_sha256)
	                 VALUES (?,?,?,?,?,?,?,?,?,?,?,0,'','','')`,
		"dev-viejo", "nas", "casa", "A", "metrics", "linux", "amd64", "", "", "", "2026-01-01T00:00:00Z")

	if err := applyMigrations(db, migrationsUpTo(36)); err != nil {
		t.Fatalf("aplicar v36: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 36 {
		t.Fatalf("user_version=%d tras la v36, esperaba 36", v)
	}

	// La máquina vieja sobrevivió: la migración es aditiva.
	var nombre string
	if err := db.QueryRow(`SELECT name FROM devices WHERE id='dev-viejo'`).Scan(&nombre); err != nil {
		t.Fatalf("la máquina de la v35 no sobrevivió: %v", err)
	}
	if nombre != "nas" {
		t.Errorf("la máquina quedó como %q", nombre)
	}

	// Los TRES índices existen, con sus nombres.
	quiero := map[string]bool{"idx_services_project": false, "idx_services_device": false, "idx_services_nombre": false}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='services'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		if _, hay := quiero[n]; hay {
			quiero[n] = true
		}
	}
	rows.Close()
	for n, hay := range quiero {
		if !hay {
			t.Errorf("falta el índice %s: sin él, listar los servicios de un proyecto recorre la tabla entera", n)
		}
	}

	// IDEMPOTENCIA: se rebobina user_version y se vuelve a correr, que es lo que hacen las
	// pruebas y lo que pasó de verdad con la v21.
	mustExec(t, db, `PRAGMA user_version = 35`)
	if err := applyMigrations(db, migrationsUpTo(36)); err != nil {
		t.Fatalf("la v36 no es idempotente: %v", err)
	}
}

// NO EXISTE UNA COLUMNA DE ESTADO EN `services`, y esta prueba de FORMA custodia esa ausencia.
//
// Un booleano de salud guardado se queda en `true` para siempre cuando la cosa muere de golpe,
// que es justo cuando querés saber que se cayó. Es la misma lección que `devices` no tiene columna
// `online` (custodiada por su gemela en devices_test.go): el estado se DERIVA de `last_health` y
// de la EDAD de `last_report`, al servir.
//
// Sabotaje que la hace fallar: agregarle a la migración 36 una columna `healthy INTEGER NOT NULL
// DEFAULT 0` (o `status`, `up`, `activo`, `online`).
func TestNoExisteColumnaDeEstadoEnServices(t *testing.T) {
	e := newTestEngine(t)
	rows, err := e.db.Query(`PRAGMA table_info(services)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	prohibidas := map[string]string{
		"estado":  "el estado se deriva de last_health al servir",
		"status":  "el estado se deriva de last_health al servir",
		"up":      "un booleano guardado se queda en true cuando la cosa muere de golpe",
		"healthy": "un booleano guardado se queda en true cuando la cosa muere de golpe",
		"activo":  "el frescor se deriva de la edad de last_report",
		"online":  "el frescor se deriva de la edad de last_report",
	}
	columnas := 0
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        any
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		columnas++
		if porque, mal := prohibidas[strings.ToLower(name)]; mal {
			t.Errorf("`services` tiene una columna %q: %s", name, porque)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// Si el PRAGMA no devolvió NADA, la tabla no existe y el barrido pasaría vacío y en verde —
	// el modo de fallo más peligroso que puede tener una prueba de forma.
	if columnas == 0 {
		t.Fatal("PRAGMA table_info(services) no devolvió columnas: la tabla no existe y este barrido no está mirando nada")
	}
}
