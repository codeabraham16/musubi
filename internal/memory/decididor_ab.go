package memory

// decididor_ab.go — EL INSTRUMENTO DEL A/B DEL DECIDIDOR DE SKILLS (spec skills-decididor, R10).
//
// El decididor de skills (la conciencia que, dado el texto de la tarea, ELIGE una skill y se la
// exige al agente) se enciende detrás de un A/B default-off. La pregunta que el A/B contesta es
// barata de enunciar y cara de medir: ¿exigir una skill hace que se CARGUE más seguido que
// dejar que el agente decida solo? Sin medirlo, prender el motor es gastar la suscripción a ciegas.
//
// LA ARITMÉTICA ES MODEL-FREE (musubi-rules #3). El lift se calcula con dos divisiones y una resta,
// no con un juicio. Por brazo:
//
//	tasa  = cargadas / oportunidades          (cargas por ocasión de actuar)
//	lift  = tasa(on) − tasa(off)              (cuánto sube la carga el tratamiento)
//
//   - oportunidades: el DENOMINADOR. Cuántas veces el gate del decididor pasó —tuvo ocasión de
//     actuar—. Se cuenta en el central (RegistrarOportunidadDecididor), SÓLO con el A/B encendido.
//   - exigencias: de esas ocasiones, cuántas el motor terminó exigiendo una skill. No entra en el
//     lift; es telemetría para leerlo (si es ~0, el motor casi no exige y no hay lift que esperar).
//   - cargadas: el NUMERADOR. Cuántas skills se cargaron en ese brazo. Se cuenta donde corre el
//     hook de carga (RecordSkillEvents → bumpCargadasDecididorTx), que es el eje `loaded`.
//
// EL BRAZO SALE DE BrazoAB(project_id, día): crossover por día, cohorte = tenant. Las dos puntas
// (central y local) caen en el mismo brazo sin coordinarse porque dependen sólo de algo que las dos
// conocen igual. Ver brazo.go.
//
// EL LÍMITE DEFERIDO YA ESTÁ CERRADO. oportunidades nace en el central y cargadas en la máquina
// local; un lift real exige que las dos magnitudes convivan en una base. Eso lo cierra
// MergeDecididorABDesdeCentral (más abajo): el sync ENTRANTE baja el snapshot de oportunidades del
// central y lo mergea en la base local con autoridad por columna, así el UPDATE-only de
// bumpCargadasDecididorTx tiene filas sobre las que sumar y CalcularLift ve las dos juntas. El
// substrato (la tabla), los dos registradores y la aritmética ya estaban cubiertos con fixture
// sembrado; ahora el flujo cruzando el sync también.

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DecididorABRow es una celda del libro mayor: un (proyecto, brazo) con sus tres contadores.
// Lleva tags JSON explícitos porque cruza el borde MCP en el handler de telemetría (musubi-rules #6):
// sin ellos saldría "ProjectID"/"Oportunidades" y un consumidor que parsea en minúscula guardaría
// ceros —una falla que se lee como un dato.
type DecididorABRow struct {
	ProjectID     string `json:"project_id"`
	Arm           string `json:"arm"`
	Oportunidades int    `json:"oportunidades"`
	Exigencias    int    `json:"exigencias"`
	Cargadas      int    `json:"cargadas"`
	FirstAt       string `json:"first_at,omitempty"`
	LastAt        string `json:"last_at,omitempty"`
}

// RegistrarOportunidadDecididor cuenta UNA ocasión en que el decididor tuvo chance de actuar, en el
// brazo dado. Suma 1 a oportunidades SIEMPRE, y 1 a exigencias sólo si el motor terminó exigiendo
// una skill. Es el DENOMINADOR del A/B, y nace la fila (project_id, arm): que la fila exista es lo
// que después deja que bumpCargadasDecididorTx cuente las cargas (ver su nota).
//
// Se cuenta ANTES de saber si el brazo es de tratamiento o control: una ocasión en el brazo `off`
// también es una oportunidad —es el control contra el que se mide el tratamiento—, y si no se
// contara, el denominador del control quedaría en cero y el lift sería indefinido justo del lado
// que lo hace honesto.
func (e *DbEngine) RegistrarOportunidadDecididor(ctx context.Context, projectID, arm string, exigio bool) error {
	if arm != BrazoOn && arm != BrazoOff {
		return fmt.Errorf("decididor A/B: brazo inválido %q (esperaba %q o %q)", arm, BrazoOn, BrazoOff)
	}
	inc := 0
	if exigio {
		inc = 1
	}
	if _, err := e.db.ExecContext(ctx, `
		INSERT INTO decididor_ab (project_id, arm, oportunidades, exigencias, first_at, last_at)
		VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(project_id, arm)
		DO UPDATE SET oportunidades = oportunidades + 1, exigencias = exigencias + ?, last_at = CURRENT_TIMESTAMP`,
		projectID, arm, inc, inc); err != nil {
		return fmt.Errorf("decididor A/B: registrar oportunidad: %w", err)
	}
	return nil
}

// bumpCargadasDecididorTx suma al NUMERADOR del A/B las cargas de este lote, dentro de la MISMA
// transacción que los contadores de skill_usage. Lo llama RecordSkillEvents —el único choke point de
// las cargas— para que NO exista una segunda vía que alguien olvide enganchar: la trampa de la guarda
// definida y desconectada (verde y cero callers) es justo la que se evita contando acá.
//
// ES UPDATE-ONLY, Y ESO ES LA DECISIÓN CLAVE. No crea la fila (project_id, arm): sólo suma si YA
// existe, y la fila sólo nace cuando RegistrarOportunidadDecididor cuenta una oportunidad, cosa que
// pasa SÓLO con el A/B encendido. Dos cosas se siguen de eso:
//
//   - una carga de un período SIN A/B (el default, el motor apagado) no infla el numerador: no hay
//     fila contra la cual sumar, así que no cuenta. Si en cambio esto hiciera un upsert, nacería una
//     fila con oportunidades=0 y cargadas>0, y CalcularLift tendría que descartar ese brazo por
//     división-por-cero — o peor, reportar un lift fabricado.
//   - el conteo cruza el sync solo: hasta que las filas de oportunidad del central lleguen a esta
//     base (el límite deferido del encabezado), el UPDATE no afecta filas y la carga no cuenta acá.
//
// El brazo sale de BrazoAB(project_id, now): como `now` es uno solo para todo el flush, todas las
// cargas del mismo proyecto caen en el mismo brazo, así que un proyecto toca UNA fila.
func bumpCargadasDecididorTx(ctx context.Context, tx *sql.Tx, batch []SkillEvent, now time.Time) error {
	porProyecto := map[string]int{}
	for _, ev := range batch {
		// Mismo criterio de validez que el lote de skill_usage: una carga es un UsoCargada con skill
		// no vacía. project_id vacío es legítimo (el default de la columna), no se filtra.
		if ev.Kind != UsoCargada || strings.TrimSpace(ev.Skill) == "" {
			continue
		}
		porProyecto[ev.ProjectID]++
	}
	for pid, n := range porProyecto {
		arm := BrazoAB(pid, now)
		if _, err := tx.ExecContext(ctx, `
			UPDATE decididor_ab SET cargadas = cargadas + ?, last_at = CURRENT_TIMESTAMP
			WHERE project_id = ? AND arm = ?`, n, pid, arm); err != nil {
			return fmt.Errorf("decididor A/B: contar cargas del proyecto %q: %w", pid, err)
		}
	}
	return nil
}

// MergeDecididorABDesdeCentral fusiona en la base LOCAL un snapshot de filas decididor_ab bajadas del
// central (sync ENTRANTE del A/B del decididor). Es el cable que cierra el LÍMITE DEFERIDO del
// encabezado de este archivo: trae el DENOMINADOR (oportunidades/exigencias), que nace sólo en el
// central, a la base donde se cuenta el NUMERADOR (cargadas). Hasta acá las dos magnitudes nunca
// convivían en una base y el lift era inmedible; con las filas de oportunidad presentes, el
// UPDATE-only de bumpCargadasDecididorTx ya tiene sobre qué sumar y CalcularLift ve las dos juntas.
//
// AUTORIDAD POR COLUMNA, Y ES LA DECISIÓN CLAVE. Cada contador tiene UN dueño y el merge lo respeta:
//
//   - oportunidades y exigencias: dueño el CENTRAL (nacen de RegistrarOportunidadDecididor, que sólo
//     corre con motor real, o sea en el central). Se REEMPLAZAN con el valor del central, NO se suman:
//     así bajar el MISMO snapshot dos veces deja los mismos números (idempotente). Sumar duplicaría el
//     denominador en cada bajada y fabricaría un lift falso.
//   - cargadas: dueño la máquina LOCAL (nace de bumpCargadasDecididorTx, el eje `loaded`). El DO
//     UPDATE NO la nombra: una bajada PRESERVA el numerador local aunque el central mande otro valor.
//     Una fila NUEVA nace con cargadas=0 —esta máquina todavía no contó cargas para ese brazo— y NO
//     hereda el cargadas del central, que son cargas de OTRA máquina y contaminarían el lift local.
//
// first_at/last_at se amplían a la unión (min/max): la ventana del experimento es la de las dos
// puntas. OJO CON EL FORMATO, Y ES SUTIL: el central NO llega con el layout de CURRENT_TIMESTAMP sino
// en RFC3339 ('…T…Z'), porque allá la columna DATETIME se escanea a un string de Go y database/sql
// formatea el time.Time con RFC3339Nano. Una punta vacía, en cambio, cae a CURRENT_TIMESTAMP
// ('YYYY-MM-DD HH:MM:SS', con espacio). Comparar esos dos layouts como TEXTO NO es cronológico —el
// espacio (0x20) ordena antes que la 'T' (0x54), así que una marca con espacio gana siempre el MIN y
// pierde siempre el MAX, tenga la fecha que tenga—. Por eso cada lado va envuelto en datetime(), que
// normaliza los dos a un layout único antes del MIN/MAX.
//
// SÓLO BAJADA: no encola nada en el outbox ni abre vía de subida. El contador del A/B no sube nunca,
// así que no hay eco posible («re-sube lo que baja»). Es el espejo mínimo de IngestShared para una
// tabla de contadores, sin su maquinaria anti-loop, porque sin subida no hay loop que cerrar.
//
// Una sola transacción para todo el snapshot (atómico, y el DSN lleva _txlock=immediate). Un brazo
// inválido aborta el merge entero: el central sólo guarda 'on'/'off' (CHECK de la migración v59), así
// que recibir otra cosa es una anomalía, no un caso borde que valga tragarse.
func (e *DbEngine) MergeDecididorABDesdeCentral(rows []DecididorABRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("decididor A/B: abrir la transacción del merge de bajada: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range rows {
		if r.Arm != BrazoOn && r.Arm != BrazoOff {
			return fmt.Errorf("decididor A/B: merge de bajada con brazo inválido %q (esperaba %q o %q)", r.Arm, BrazoOn, BrazoOff)
		}
		// first_at/last_at vacíos (un central viejo que los omite, u omitempty en el JSON) caen al
		// default de la columna vía CURRENT_TIMESTAMP, igual que RegistrarOportunidadDecididor.
		if _, err := tx.Exec(`
			INSERT INTO decididor_ab (project_id, arm, oportunidades, exigencias, cargadas, first_at, last_at)
			VALUES (?, ?, ?, ?, 0, COALESCE(NULLIF(?, ''), CURRENT_TIMESTAMP), COALESCE(NULLIF(?, ''), CURRENT_TIMESTAMP))
			ON CONFLICT(project_id, arm) DO UPDATE SET
				oportunidades = excluded.oportunidades,
				exigencias    = excluded.exigencias,
				first_at = MIN(datetime(decididor_ab.first_at), datetime(excluded.first_at)),
				last_at  = MAX(datetime(decididor_ab.last_at),  datetime(excluded.last_at))`,
			r.ProjectID, r.Arm, r.Oportunidades, r.Exigencias, r.FirstAt, r.LastAt); err != nil {
			return fmt.Errorf("decididor A/B: mergear la fila (%q, %q) bajada del central: %w", r.ProjectID, r.Arm, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("decididor A/B: commitear el merge de bajada: %w", err)
	}
	return nil
}

// DecididorAB devuelve las celdas del libro mayor visibles para el proyecto de la credencial.
//
// ACOTADA AL PROYECTO con el MISMO scopeClause que SkillUsage: sin el scope, un miembro del central
// vería el experimento de otro tenant. Federado (admin / stdio) ⇒ ve todo, que es lo que quiere un
// operador leyendo el A/B global.
func (e *DbEngine) DecididorAB(ctx context.Context) ([]DecididorABRow, error) {
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	rows, err := e.db.QueryContext(ctx, `
		SELECT project_id, arm, oportunidades, exigencias, cargadas, first_at, last_at
		FROM decididor_ab
		WHERE 1=1`+scopeSQL+`
		ORDER BY project_id ASC, arm ASC`, scopeArgs...)
	if err != nil {
		return nil, fmt.Errorf("decididor A/B: consultar: %w", err)
	}
	defer rows.Close()

	out := make([]DecididorABRow, 0)
	for rows.Next() {
		var r DecididorABRow
		if err := rows.Scan(&r.ProjectID, &r.Arm, &r.Oportunidades, &r.Exigencias, &r.Cargadas, &r.FirstAt, &r.LastAt); err != nil {
			return nil, fmt.Errorf("decididor A/B: escanear fila: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("decididor A/B: iterar: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProjectID != out[j].ProjectID {
			return out[i].ProjectID < out[j].ProjectID
		}
		return out[i].Arm < out[j].Arm
	})
	return out, nil
}

// LiftDecididor es el resultado model-free del A/B, agregado por brazo (sumando todos los proyectos
// que entraron en la lectura). Tags JSON explícitos por el mismo motivo que DecididorABRow.
type LiftDecididor struct {
	OportunidadesOn  int     `json:"oportunidades_on"`
	OportunidadesOff int     `json:"oportunidades_off"`
	CargadasOn       int     `json:"cargadas_on"`
	CargadasOff      int     `json:"cargadas_off"`
	TasaOn           float64 `json:"tasa_on"`
	TasaOff          float64 `json:"tasa_off"`
	Lift             float64 `json:"lift"`
}

// CalcularLift agrega las filas por brazo y devuelve el lift = tasa(on) − tasa(off). El segundo
// valor es `ok`: FALSE cuando alguno de los dos brazos no tiene oportunidades.
//
// POR QUÉ ok=false Y NO lift=0 cuando falta un denominador (R10/E9). Sin oportunidades en un brazo no
// hay una tasa que restar: el lift no es cero, es INDEFINIDO. Devolver 0 afirmaría «el tratamiento no
// tuvo efecto» cuando lo cierto es «todavía no se puede medir» —y esa es exactamente la mentira que
// un A/B default-off produce al principio, cuando el brazo `off` o el `on` aún no acumuló ocasiones.
// El caller distingue «no hay efecto» (ok=true, lift≈0) de «no hay dato» (ok=false) por este bool.
//
// Aritmética pura sobre las filas que le pasan: NO lee el corpus vivo. Quien lo pruebe SIEMBRA las
// filas (si leyera una magnitud auto-consumible del corpus, el test se mediría a sí mismo).
func CalcularLift(rows []DecididorABRow) (LiftDecididor, bool) {
	var l LiftDecididor
	for _, r := range rows {
		switch r.Arm {
		case BrazoOn:
			l.OportunidadesOn += r.Oportunidades
			l.CargadasOn += r.Cargadas
		case BrazoOff:
			l.OportunidadesOff += r.Oportunidades
			l.CargadasOff += r.Cargadas
		}
	}
	if l.OportunidadesOn == 0 || l.OportunidadesOff == 0 {
		return l, false
	}
	l.TasaOn = float64(l.CargadasOn) / float64(l.OportunidadesOn)
	l.TasaOff = float64(l.CargadasOff) / float64(l.OportunidadesOff)
	l.Lift = l.TasaOn - l.TasaOff
	return l, true
}

// FormatLiftDecididor rinde el lift del A/B para la tabla de skill_usage, model-free. Lo consume
// musubi_skill_usage: quien mantiene el arsenal necesita ver si ENTREGAR la skill como exigencia mueve
// la carga respecto del control. Vive acá, al lado de FormatSkillUsage, porque el cuerpo va a querer
// el mismo formato en su panel.
//
// `ok=false` se dice «INDEFINIDO», NUNCA lift=0 (R10/E9). Un A/B default-off recién encendido tiene un
// brazo sin ocasiones: imprimir 0 ahí leería como «no hubo efecto» cuando la verdad es «todavía no se
// puede medir». Por eso el número del lift sólo se imprime con ok=true; con ok=false se muestran los
// crudos de cada brazo y la palabra que lo marca indefinido —el dato honesto es «faltan ocasiones»—.
func FormatLiftDecididor(l LiftDecididor, ok bool) string {
	var b strings.Builder
	b.WriteString("\nA/B del decididor de skills · carga por brazo\n")
	if l.OportunidadesOn == 0 && l.OportunidadesOff == 0 {
		b.WriteString("  sin ocasiones todavía: el decididor está apagado o recién encendido.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  on : %d cargas / %d ocasiones\n", l.CargadasOn, l.OportunidadesOn)
	fmt.Fprintf(&b, "  off: %d cargas / %d ocasiones\n", l.CargadasOff, l.OportunidadesOff)
	if ok {
		fmt.Fprintf(&b, "  lift = %+.1f puntos porcentuales (%.1f%% on − %.1f%% off)\n",
			l.Lift*100, l.TasaOn*100, l.TasaOff*100)
	} else {
		b.WriteString("  lift = INDEFINIDO: un brazo todavía no acumuló ocasiones; no es 0, es que aún no se puede medir.\n")
	}
	return b.String()
}
