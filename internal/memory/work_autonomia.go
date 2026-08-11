package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// work_autonomia.go implementa la ESCALERA DE AUTONOMÍA de la pizarra: cuánto puede hacer solo
// el agente que reclame una unidad.
//
// Musubi ya sabía decir QUIÉN opera —el rol del token: reader/writer/admin— y eso es una
// propiedad de la CREDENCIAL. Le faltaba la otra mitad: CUÁNTA AUTONOMÍA TIENE ESTA TAREA. Son
// ejes distintos, y confundirlos sale caro en las dos direcciones. Un mismo agente, con el mismo
// token, recibe «andá y mirá, no toques nada» en una unidad y «arreglalo solo» en la siguiente;
// si el único lugar donde vive esa diferencia es la cabeza del que la dio, el día que el agente
// hace de más nadie puede señalar la regla que rompió, porque no había regla. Y al revés: bajarle
// el rol al token para contener UNA tarea le saca permisos que sí necesita en las otras.
//
// La escalera se declara al POSTEAR la unidad y no cambia después:
//
//	L1  sólo reporta      — puede diagnosticar y dejar hallazgos; NO puede cerrar diciendo que tocó algo
//	L2  arregla asistido  — puede tocar, pero el cierre necesita la firma de OTRO (maker/checker)
//	L3  desatendido       — cierra solo (es lo que la pizarra hacía siempre, y sigue siendo el default)
//
// Del otro lado, quien cierra DECLARA QUÉ HIZO (el efecto):
//
//	report  no cambió nada, produjo hallazgos      — le alcanza L1
//	apply   cambió algo                            — exige L2 (con firma) o L3
//
// LO QUE ESTO NO ES: no es una jaula contra un agente que miente. Un agente que tocó el disco y
// declara `report` cierra igual, del mismo modo que hoy puede escribir cualquier cosa en `result`.
// Contener eso pide un sandbox, no una columna. Lo que sí hace —y no existía— es que el encargo
// deje de ser tácito: queda escrito en la unidad, la regla la aplica el motor y no la memoria del
// humano, y un agente honesto que se pasa de la raya se choca con un error en vez de con nadie.

// Niveles de la escalera de autonomía de una unidad de trabajo.
const (
	AutonomyReport     = "L1" // sólo reporta: no puede cerrar declarando que aplicó cambios
	AutonomyAssisted   = "L2" // arregla, pero el cierre necesita la firma de otro
	AutonomyUnattended = "L3" // cierra solo (comportamiento histórico de la pizarra)
)

// Efectos que un agente puede declarar al cerrar una unidad.
const (
	EffectReport = "report" // no cambió nada: hallazgos, diagnóstico, un informe
	EffectApply  = "apply"  // cambió algo
)

// normalizarAutonomia valida el nivel declarado al postear una unidad. Vacío ⇒ L3, que es
// exactamente lo que la pizarra hacía antes de que el campo existiera: una base vieja, un cliente
// que no sabe del campo y un test escrito ayer siguen comportándose igual.
func normalizarAutonomia(nivel string) (string, error) {
	switch v := strings.ToUpper(strings.TrimSpace(nivel)); v {
	case "":
		return AutonomyUnattended, nil
	case AutonomyReport, AutonomyAssisted, AutonomyUnattended:
		return v, nil
	default:
		return "", fmt.Errorf("autonomy inválida %q (usá L1|L2|L3: L1 sólo reporta, L2 arregla con firma de otro, L3 desatendido)", nivel)
	}
}

// normalizarEfecto valida lo que el agente declara haber hecho al cerrar. Vacío ⇒ apply, el
// efecto MÁS privilegiado: fail-closed. Un llamador que no declara nada es indistinguible de uno
// que cambió el mundo, y frenarlo en una unidad L1 es el error barato; dejarlo pasar es el caro.
func normalizarEfecto(efecto string) (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(efecto)); v {
	case "":
		return EffectApply, nil
	case EffectReport, EffectApply:
		return v, nil
	default:
		return "", fmt.Errorf("effect inválido %q (usá report|apply: report = no cambiaste nada, apply = cambiaste algo)", efecto)
	}
}

// firmaVigente es la condición SQL que hace que la firma de un revisor valga: hay firma, no es la
// del propio dueño, y fue puesta sobre ESTE intento.
//
// El tercer término es el que no se ve venir. Sin él, una unidad aprobada cuyo dueño se cuelga, le
// vence el lease y es retomada por otro agente arrastraría la firma vieja: el trabajo NUEVO, que
// nadie miró, cerraría amparado por una revisión del trabajo VIEJO. Atar la firma al fencing_token
// del momento la invalida sola en cuanto la unidad cambia de manos.
const firmaVigente = `(approved_by <> '' AND approved_by <> owner_id AND approved_token = fencing_token)`

// gateDeAutonomia devuelve la condición SQL extra que el cierre debe cumplir, o "" si no hace
// falta ninguna. Se pega al WHERE del UPDATE de cierre para que la decisión sea ATÓMICA con la
// escritura.
//
// Cerrar como `failed` nunca se frena: reportar que algo salió mal no muta nada, y una unidad L1
// que no pudiera ni declararse fallida quedaría colgada hasta que le venza el lease — se
// escalaría sola como si el agente hubiera desaparecido, que es justo la señal equivocada.
func gateDeAutonomia(status, efecto string) string {
	if status != WorkDone || efecto == EffectReport {
		return ""
	}
	return `(autonomy = '` + AutonomyUnattended + `' OR (autonomy = '` + AutonomyAssisted + `' AND ` + firmaVigente + `))`
}

// motivoDeCierreBloqueado explica un cierre que no tocó ninguna fila, cuando la causa es la
// autonomía y no el lease. hay=false ⇒ que el llamador use su mensaje genérico (la unidad no
// existe, no está reclamada, o el dueño/token no coinciden).
func (e *DbEngine) motivoDeCierreBloqueado(id, efecto string) (string, bool) {
	if efecto != EffectApply {
		return "", false
	}
	var (
		autonomy   string
		owner      string
		approvedBy string
		approvedTk int64
		fencingTk  int64
	)
	err := e.db.QueryRow(`
		SELECT COALESCE(autonomy,'`+AutonomyUnattended+`'), COALESCE(owner_id,''),
		       COALESCE(approved_by,''), COALESCE(approved_token,0), COALESCE(fencing_token,0)
		  FROM work_units WHERE id=? AND status=?`, id, WorkClaimed).
		Scan(&autonomy, &owner, &approvedBy, &approvedTk, &fencingTk)
	if err != nil {
		return "", false // no existe, no está claimed: el mensaje genérico dice la verdad
	}
	switch autonomy {
	case AutonomyReport:
		return fmt.Sprintf("la unidad %q declara autonomía L1 (sólo reporta): no puede cerrarse "+
			"declarando effect=apply. Cerrala con effect=report dejando tus hallazgos en 'result', "+
			"o pedile a quien la posteó que la reponga con autonomy=L2 si hay que tocar algo", id), true
	case AutonomyAssisted:
		switch {
		case approvedBy == "":
			return fmt.Sprintf("la unidad %q declara autonomía L2 (arregla asistido): falta la firma "+
				"de un revisor. Que OTRO agente/persona la apruebe (action=approve) antes de cerrarla", id), true
		case approvedBy == owner:
			return fmt.Sprintf("la unidad %q está firmada por su propio dueño (%q): en L2 el que hace "+
				"no puede ser el que aprueba", id, owner), true
		case approvedTk != fencingTk:
			return fmt.Sprintf("la unidad %q tiene una firma VENCIDA (aprobada sobre el intento #%d, "+
				"va por el #%d): la unidad cambió de manos después de la revisión, así que el trabajo "+
				"actual no está revisado. Pedí una firma nueva", id, approvedTk, fencingTk), true
		}
	}
	return "", false
}

// ApproveWorkUnit firma el intento EN CURSO de una unidad L2: deja constancia de quién la revisó
// y sobre qué intento. Es el checker del par maker/checker.
//
// Reglas, todas fail-closed:
//   - sólo sobre una unidad `claimed`: aprobar una que nadie está haciendo, o una ya cerrada, no
//     revisa nada — sería una firma en blanco esperando a que alguien la use;
//   - el revisor no puede ser el dueño, que es el punto entero de la separación;
//   - sólo tiene sentido en L2. En L3 la firma no cambia nada y en L1 no hay nada que firmar
//     (no se puede aplicar), así que se rechaza en vez de aceptar un gesto vacío.
//
// La firma queda atada al fencing_token vigente: si la unidad cambia de manos, se invalida sola.
//
// No guarda una nota del revisor A PROPÓSITO: el único campo de texto de la unidad es `result`, y
// pisarlo con el comentario de la revisión borraría lo que el agente dejó escrito — el revisor
// tapando justo la evidencia que se revisó. Quién firmó, cuándo, y sobre qué intento es lo que el
// gate necesita; el fundamento de la revisión es material del reporte, no de esta fila.
func (e *DbEngine) ApproveWorkUnit(id, reviewer string) error {
	id, reviewer = strings.TrimSpace(id), strings.TrimSpace(reviewer)
	if id == "" || reviewer == "" {
		return fmt.Errorf("approve requiere 'id' (la unidad) y 'reviewer' (quién firma)")
	}
	var (
		autonomy string
		owner    string
	)
	err := e.db.QueryRow(
		`SELECT COALESCE(autonomy,'`+AutonomyUnattended+`'), COALESCE(owner_id,'')
		   FROM work_units WHERE id=? AND status=?`, id, WorkClaimed).Scan(&autonomy, &owner)
	if err == sql.ErrNoRows {
		return fmt.Errorf("la unidad %q no existe o no está reclamada: sólo se firma trabajo en curso", id)
	}
	if err != nil {
		return fmt.Errorf("error al leer la unidad a firmar: %w", err)
	}
	if autonomy != AutonomyAssisted {
		return fmt.Errorf("la unidad %q declara autonomía %s: la firma sólo aplica en L2 (en L3 no hace "+
			"falta y en L1 no hay nada que aplicar)", id, autonomy)
	}
	if reviewer == owner {
		return fmt.Errorf("%q es el dueño de la unidad %q: el que hace no puede ser el que aprueba", reviewer, id)
	}
	// La firma se estampa contra el fencing_token de ESTA fila, en el mismo UPDATE. El WHERE
	// repite las guardas de estado y dueño: si entre la lectura de arriba y esta escritura otro
	// agente expropió la unidad, no se firma nada — la revisión miró el trabajo del dueño viejo.
	res, err := e.db.Exec(`
		UPDATE work_units
		   SET approved_by=?, approved_at=datetime('now'), approved_token=fencing_token,
		       updated_at=datetime('now')
		 WHERE id=? AND status=? AND owner_id=?`,
		reviewer, id, WorkClaimed, owner)
	if err != nil {
		return fmt.Errorf("error al firmar la unidad: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al verificar la firma: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("la unidad %q cambió de manos mientras se la firmaba: volvé a revisarla", id)
	}
	return nil
}
