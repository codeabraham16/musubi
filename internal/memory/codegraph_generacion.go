package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// codegraph_generacion.go lleva la cuenta DURABLE de qué foto del grafo tiene el central.
//
// EL AGUJERO QUE TAPA. El scheduler dejó de empujar el grafo en cada tick y pasó a empujar «sólo si
// hay algo nuevo». Mientras la marca de «algo nuevo» vivió en la memoria de UN proceso, se perdía en
// dos casos que en esta PC son el caso normal y no el raro: sobre la misma .musubi/memory.db corren
// varios `musubi daemon` (uno por sesión), y las sesiones se cierran todo el tiempo. Si el cambio lo
// indexaba un daemon sin sync —o uno que moría entre el índice y el push—, el daemon con sync veía
// el árbol limpio y no empujaba NUNCA. Medido con TestRomperDosDaemonsElCambioNoLlegaAlCentral:
// pushes=1 tras 3 ticks, esperaba 2.
//
// POR QUÉ EN LA MISMA TRANSACCIÓN QUE LA ESCRITURA. Si el contador subiera en el llamador, cada
// escritor nuevo del grafo tendría que acordarse de subirlo, y el que se olvida reabre el agujero en
// silencio. Acá sube adentro de las funciones que tocan las tablas de la foto (nodos, aristas y
// gists), así que lo cubren el scheduler, la tool, save_code y cualquier escritor futuro que pase por
// ellas. Y como es la misma transacción, no hay instante en que la base tenga el grafo nuevo con la
// generación vieja.
//
// UNA BASE VIEJA NO TIENE LAS CLAVES y eso vale 0: generación 0, empujada 0, nunca empujado. No hace
// falta migración, y 0 es la lectura segura (ver PushDelGrafo).
const (
	// MetaCodegraphGeneracion sube en 1 con cada escritura de la foto del grafo.
	MetaCodegraphGeneracion = "codegraph_generation"
	// MetaCodegraphEmpujada es la generación más nueva que un push EXITOSO llevó al central. Nunca
	// retrocede.
	MetaCodegraphEmpujada = "codegraph_pushed_generation"
	// MetaCodegraphEmpujadoEn es el instante (RFC3339, UTC) del último push exitoso.
	MetaCodegraphEmpujadoEn = "codegraph_pushed_at"
)

// avanzarGeneracionDelGrafo sube la generación DENTRO de la transacción del escritor. CAST sobre un
// valor que no es número da 0 en SQLite: una clave corrupta vuelve a contar desde 1 en vez de
// abortar la escritura del grafo.
func avanzarGeneracionDelGrafo(tx *sql.Tx) error {
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, '1', CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET
		   value = CAST(CAST(meta.value AS INTEGER) + 1 AS TEXT), updated_at = CURRENT_TIMESTAMP`,
		MetaCodegraphGeneracion,
	); err != nil {
		return fmt.Errorf("error al avanzar la generación del grafo: %w", err)
	}
	return nil
}

// leerGeneracionDelGrafo lee la generación con el mismo consultor que la foto: dentro de la
// transacción de lectura queda en la MISMA instantánea que los nodos, las aristas y los gists.
func leerGeneracionDelGrafo(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (int64, error) {
	var v string
	err := q.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, MetaCodegraphGeneracion).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("error al leer la generación del grafo: %w", err)
	}
	n, _ := strconv.ParseInt(v, 10, 64) // corrupta ⇒ 0, igual que el CAST del escritor
	return n, nil
}

// PushDelGrafo es lo que el scheduler necesita para decidir si empuja: qué generación tiene la base,
// cuál tiene el central y cuándo fue el último push que salió bien.
type PushDelGrafo struct {
	Generacion int64
	Empujada   int64
	EmpujadoEn time.Time // cero si nunca hubo un push exitoso
}

// EstadoDelPushDelGrafo lee las tres claves. Ausentes valen cero.
func (e *DbEngine) EstadoDelPushDelGrafo() (PushDelGrafo, error) {
	var est PushDelGrafo
	rows, err := e.db.Query(`SELECT key, value FROM meta WHERE key IN (?, ?, ?)`,
		MetaCodegraphGeneracion, MetaCodegraphEmpujada, MetaCodegraphEmpujadoEn)
	if err != nil {
		return est, fmt.Errorf("error al leer el estado del push del grafo: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return est, err
		}
		switch k {
		case MetaCodegraphGeneracion:
			est.Generacion, _ = strconv.ParseInt(v, 10, 64)
		case MetaCodegraphEmpujada:
			est.Empujada, _ = strconv.ParseInt(v, 10, 64)
		case MetaCodegraphEmpujadoEn:
			est.EmpujadoEn, _ = time.Parse(time.RFC3339, v)
		}
	}
	return est, rows.Err()
}

// MarcarGrafoEmpujado registra que un push de la foto de generación `generacion` salió bien.
//
// LA GENERACIÓN ES LA DE LA FOTO, NO LA DE AHORA. Entre leer la foto y que el central conteste pasa
// una llamada de red, y en ese rato otro escritor puede subir la generación. Marcar la de ahora
// declararía empujado un cambio que no viajó: es el sabotaje que pone rojo
// TestUnaEscrituraDuranteElPushSeFederaAlTickSiguiente.
//
// NUNCA RETROCEDE: dos daemons pueden terminar sus push en cualquier orden, y el que llega último
// con una foto más vieja no puede bajar la marca del que llevó una más nueva. El instante sí avanza
// siempre que el push salió bien: es el reloj del push de higiene.
func (e *DbEngine) MarcarGrafoEmpujado(generacion int64, en time.Time) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar la marca del push del grafo: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET
		   value = CASE WHEN CAST(excluded.value AS INTEGER) > CAST(meta.value AS INTEGER)
		                THEN excluded.value ELSE meta.value END,
		   updated_at = CURRENT_TIMESTAMP`,
		MetaCodegraphEmpujada, strconv.FormatInt(generacion, 10),
	); err != nil {
		return fmt.Errorf("error al marcar la generación empujada del grafo: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		MetaCodegraphEmpujadoEn, en.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("error al marcar el instante del push del grafo: %w", err)
	}
	return tx.Commit()
}
