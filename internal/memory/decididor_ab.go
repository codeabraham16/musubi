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
// ⚠️ LÍMITE CONOCIDO, DEFERIDO A UN PR POSTERIOR (flaggeado en verify): oportunidades nace en el
// central y cargadas en la máquina local. Un lift real exige que las dos convivan en una base, cosa
// que hoy sólo pasa post-merge del sync. El substrato (la tabla), los dos registradores y la
// aritmética son correctos y están cubiertos con fixture sembrado; el flujo cruzando el sync es
// trabajo siguiente.

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
