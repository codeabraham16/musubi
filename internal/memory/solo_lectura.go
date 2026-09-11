package memory

// El engine de SÓLO LECTURA: abrir una base que este binario no puede migrar, sin poder tocarla.
//
// PARA QUÉ. La guarda de esquema tenía una sola respuesta —negarse— para dos situaciones muy
// distintas. Con el piso de lectura grabado en la base (migración 48) una de ellas pasó a ser
// contestable: cuando este binario alcanza el piso, los datos son legibles para él aunque no
// pueda migrarlos. Esto es lo que hace falta para aprovecharlo.
//
// LA GARANTÍA LA DA SQLITE, NO NUESTRA DISCIPLINA. `PRAGMA query_only = 1` va en el DSN, así que
// lo hereda CADA conexión del pool y cualquier INSERT/UPDATE/DELETE/DDL vuelve como error del
// motor. Es a propósito que no sea una lista de funciones que evitamos llamar: una lista hay que
// mantenerla, y el día que alguien agregue una escritura nueva nadie se acuerda. Arriba de esto,
// el servidor MCP además sólo despacha tools declaradas readOnly — cinturón y tiradores, y ninguno
// de los dos depende de que el otro esté bien.

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"musubi/internal/config"
)

// NewDbEngineSoloLectura abre la base SIN migrarla y sin ninguno de los pasos de mantenimiento
// que hace la apertura normal.
//
// LO QUE NO HACE, Y POR QUÉ CADA UNO. No migra (es justamente el caso en que no puede). No
// backfillea digests ni recomputa tokens (son escrituras, y además describen el estado que
// entiende ESTE binario, no el que la base ya tiene). No siembra el outbox (escribe, y encolar
// desde un binario que no entiende el esquema es la forma de propagar el malentendido). No
// entrena el índice vectorial (gasto de arranque para una sesión que es un salvavidas, no un
// puesto de trabajo).
//
// Devuelve error si el archivo no está: acá no se crea nada. Una base ausente en este camino
// significa que alguien lo llamó donde no correspondía, y crear una base vacía lo taparía —el
// llamador vería un engine sano con cero memoria, que es la falla que se lee como un dato.
func NewDbEngineSoloLectura(projectPath string) (*DbEngine, error) {
	dbPath := filepath.Join(projectPath, config.DirName, config.DBFile)

	// journal_mode NO se fija acá. Ponerlo es una escritura al header y con query_only fallaría;
	// además no hace falta: el modo ya está grabado en el archivo desde que alguien lo migró.
	// busy_timeout sí, porque el escritor de verdad puede estar trabajando al mismo tiempo.
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=query_only(1)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir la base en sólo lectura: %w", err)
	}

	// Un Ping no alcanza para saber que la base sirve: sql.Open es perezoso y Ping abre la
	// conexión, pero una lectura real es la que dice que el archivo tiene forma de base.
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al leer el esquema de la base en sólo lectura: %w", err)
	}

	// VERIFICAR QUE query_only DE VERDAD ESTÁ PUESTO, y no confiar en que el DSN se haya
	// aplicado. Si el driver ignorara ese `_pragma` —por un typo, por un cambio de versión— el
	// engine saldría de acá escribible y todo lo de arriba sería decorativo. Es barato y es la
	// diferencia entre una garantía y una intención.
	var soloLectura int
	if err := db.QueryRow(`PRAGMA query_only`).Scan(&soloLectura); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al verificar query_only: %w", err)
	}
	if soloLectura != 1 {
		db.Close()
		return nil, fmt.Errorf("la base abrió SIN query_only (=%d): no se puede garantizar que sea sólo lectura", soloLectura)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	// outboxEnabled en false explícito: el cero de Go ya es false, pero acá el default histórico
	// del engine normal es true y dejarlo implícito invitaría a que alguien "arregle" la omisión.
	engine := &DbEngine{db: db, path: dbPath, outboxEnabled: false, soloLectura: true}

	// LOS DIVISORES CALIBRADOS DE **ESTA** BASE, que acá faltaban. NewDbEngine los aplica
	// (database.go) y este camino no, así que un engine de sólo lectura estimaba tokens con los
	// defaults de fábrica (4.0/3.4/2.6) aunque su base tuviera una calibración guardada.
	//
	// Y el modo degradado es justo donde más duele: a este escalón se llega cuando el esquema es
	// más nuevo que el binario, o sea cuando el operador ya está mirando una base que no controla.
	// Que además le mienta el presupuesto de tokens —packByBudget corta donde no corresponde—
	// convierte un diagnóstico en dos.
	//
	// Es el hermano exacto del que documenta database.go, y no era latente como el otro: acá el
	// camino REAL de producción lo ejercita.
	if err := engine.applyCalibratedDivisors(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al aplicar los divisores calibrados de %s: %w", dbPath, err)
	}
	return engine, nil
}

// EsSoloLectura dice si este engine se abrió en el escalón de sólo lectura. Lo usa el servidor MCP
// para declararlo en el handshake y para no despachar tools que muten.
func (e *DbEngine) EsSoloLectura() bool { return e.soloLectura }
