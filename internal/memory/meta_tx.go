package memory

import (
	"database/sql"
	"errors"
	"fmt"
)

// MetaTx es la meta vista desde adentro de una transacción (MetaEnTransaccion): lo que se lee ahí
// adentro ya incluye lo que la misma transacción escribió, y nadie más escribe hasta que termina.
// *DbEngine también la satisface, por fuera de toda transacción.
type MetaTx interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
}

// ErrMetaSoloLectura es lo que devuelve MetaEnTransaccion en un engine abierto en sólo lectura.
var ErrMetaSoloLectura = errors.New("la base está abierta en sólo lectura: la meta no se escribe")

// consultorDeMeta es lo que tienen en común *sql.DB y *sql.Tx para leer y escribir la meta: la
// misma consulta corre por fuera o por dentro de una transacción.
type consultorDeMeta interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// metaEnTx es la MetaTx de una transacción abierta.
type metaEnTx struct{ tx *sql.Tx }

func (m metaEnTx) GetMeta(key string) (string, bool, error) { return leerMeta(m.tx, key) }
func (m metaEnTx) SetMeta(key, value string) error          { return escribirMeta(m.tx, key, value) }

// MetaEnTransaccion corre fn con la meta adentro de UNA transacción: si fn devuelve un error no queda
// escrito nada de lo que hizo, y si no, queda todo junto.
//
// ES PARA LEER-MODIFICAR-ESCRIBIR UNA CLAVE QUE ESCRIBEN VARIOS PROCESOS, como el índice del delta
// de cmd/musubi, que escriben los hooks de todas las sesiones abiertas a la vez. Con GetMeta y SetMeta
// sueltos, dos escritores leían el mismo valor y el segundo pisaba lo que había anotado el primero.
// El DSN lleva `_txlock=immediate` (dsnEscribible), así que el candado de escritura se toma al nacer
// la transacción, antes de la primera lectura: nadie puede colarse entre leer y escribir. Es el mismo
// mecanismo que LedgerAdd.
//
// EN SÓLO LECTURA NO CORRE fn Y DEVUELVE ErrMetaSoloLectura, sin abrir la transacción: fn existe para
// escribir. El DSN de sólo lectura no lleva `_txlock=immediate`, así que la transacción se abriría
// diferida, fn haría sus lecturas y la primera escritura rebotaría en `query_only`; no quedaría nada
// escrito, pero por un error del motor a mitad de camino en vez de uno con nombre, y después de
// trabajar para nada.
func (e *DbEngine) MetaEnTransaccion(fn func(MetaTx) error) error {
	if e.soloLectura {
		return ErrMetaSoloLectura
	}
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al abrir la transacción de meta: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(metaEnTx{tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar la transacción de meta: %w", err)
	}
	return nil
}

// leerMeta es GetMeta sobre una conexión o una transacción.
func leerMeta(q consultorDeMeta, key string) (string, bool, error) {
	var v string
	err := q.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("error al leer meta %q: %w", key, err)
	}
	return v, true, nil
}

// escribirMeta es SetMeta sobre una conexión o una transacción.
func escribirMeta(q consultorDeMeta, key, value string) error {
	_, err := q.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("error al guardar meta %q: %w", key, err)
	}
	return nil
}
