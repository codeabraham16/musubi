package memory

// probe.go: la foto de salud de SÓLO LECTURA del esquema de la base.
//
// PARA QUÉ. `musubi doctor` normal abre con NewDbEngine, que MIGRA la base al abrirla (ver
// runMigrations). En una flota mixta —laptop, PC y central con binarios de distinta versión— eso
// tiene un costo que no se ve: correr el doctor nuevo sobre una base migra el esquema hacia adelante
// y DEJA AFUERA al binario viejo, que ya no puede reabrirla (gotcha medido). El probe contesta la
// pregunta previa —«¿en qué esquema está esta base y este binario la puede leer?»— SIN TOCARLA.
//
// EL VEREDICTO ES EL MISMO QUE ACTÚA EL DAEMON, no uno nuevo. Al arrancar, main.go abre con
// NewDbEngine y, si falla con ErrEsquemaLegible, cae a sólo lectura; si falla con otra cosa, a
// degradado. El probe reproduce ESA decisión con el MISMO criterio de applyMigrations
// (`hay && latest >= piso`), pero leyendo —no abriendo para escribir—, así que reusa las mismas
// funciones (latestSchemaVersion, leerPisoDeLectura) en vez de copiar el umbral: si la regla cambia,
// no hay dos versiones que diverjan.

import "fmt"

// EstadoEsquema clasifica la relación entre el esquema grabado en la base y el que conoce este
// binario, sin migrar. Es el estado que el daemon actúa al arrancar, nombrado.
type EstadoEsquema string

const (
	// EsquemaAlDia: la base está en la versión a la que apunta este binario. Sana.
	EsquemaAlDia EstadoEsquema = "al_dia"
	// EsquemaMigrable: la base es MÁS VIEJA; un arranque normal la migraría hacia adelante (escribe).
	EsquemaMigrable EstadoEsquema = "migrable"
	// EsquemaMasNuevaLegible: la base es MÁS NUEVA pero su piso de lectura está a la altura de este
	// binario. El daemon la sirve en SÓLO LECTURA (main.go, rama ErrEsquemaLegible).
	EsquemaMasNuevaLegible EstadoEsquema = "mas_nueva_legible"
	// EsquemaMasNuevaIlegible: la base es MÁS NUEVA y su piso queda por encima de este binario (o no
	// hay piso grabado). El daemon cae a MODO DEGRADADO: no puede ni leerla a salvo.
	EsquemaMasNuevaIlegible EstadoEsquema = "mas_nueva_ilegible"
)

// ProbeReport es la foto que devuelve Probe: estado del esquema y unos conteos, todo leído sin
// escribir. Los campos están pensados para imprimirse tal cual o serializarse a JSON.
type ProbeReport struct {
	SchemaActual   int            `json:"schema_actual"`   // user_version de la base
	SchemaEsperado int            `json:"schema_esperado"` // al que apunta este binario (latestSchemaVersion)
	PisoDeLectura  int            `json:"piso_de_lectura"` // piso grabado en la base (0 si no hay)
	PisoGrabado    bool           `json:"piso_grabado"`    // distingue «piso 0» de «no hay piso»
	Estado         EstadoEsquema  `json:"estado"`
	SoloLectura    bool           `json:"solo_lectura"` // si el engine se abrió en el escalón de sólo lectura
	Conteos        map[string]int `json:"conteos"`      // filas por tabla clave; -1 = la tabla no se pudo contar
}

// conteoTablas son las tablas que el probe cuenta. Son las dos del libro mayor de la memoria; existen
// en cualquier base migrada, y si una faltara (base rara) el conteo anota -1 y el informe sigue.
var conteoTablas = []string{"observations", "observation_relations"}

// Probe arma el informe de salud SIN tocar la base. Sólo lee: PRAGMA user_version (vía schemaVersion),
// el piso grabado (leerPisoDeLectura) y un COUNT(*) por tabla.
//
// Pensado para correr sobre un engine abierto con NewDbEngineSoloLectura: ahí el DSN lleva
// query_only, así que un intento de escritura sería un ERROR del motor —no una corrupción
// silenciosa— y la garantía read-only no depende de la disciplina de esta función.
func (e *DbEngine) Probe() (ProbeReport, error) {
	actual, err := e.schemaVersion()
	if err != nil {
		return ProbeReport{}, fmt.Errorf("probe: no se pudo leer el esquema de la base: %w", err)
	}
	esperado := latestSchemaVersion()
	piso, hay := leerPisoDeLectura(e.db)

	rep := ProbeReport{
		SchemaActual:   actual,
		SchemaEsperado: esperado,
		PisoDeLectura:  piso,
		PisoGrabado:    hay,
		SoloLectura:    e.soloLectura,
		Conteos:        make(map[string]int, len(conteoTablas)),
	}

	// La MISMA regla de decisión que applyMigrations (migrations.go), no una copia del umbral.
	switch {
	case actual == esperado:
		rep.Estado = EsquemaAlDia
	case actual < esperado:
		rep.Estado = EsquemaMigrable
	case hay && esperado >= piso:
		rep.Estado = EsquemaMasNuevaLegible
	default:
		rep.Estado = EsquemaMasNuevaIlegible
	}

	for _, tabla := range conteoTablas {
		// El nombre de tabla es una constante del binario (conteoTablas), no entra input del usuario:
		// no hay inyección que prevenir y por eso se concatena.
		var n int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			// Una tabla que no se puede contar no tumba el informe: se anota -1 y se sigue. Un probe
			// que aborta por una tabla ausente diagnostica menos que uno que dice cuál faltó.
			rep.Conteos[tabla] = -1
			continue
		}
		rep.Conteos[tabla] = n
	}

	return rep, nil
}
