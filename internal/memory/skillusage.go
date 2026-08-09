package memory

// skillusage.go — LOS CONTADORES DEL ARSENAL (§7 del track «Forja global»).
//
// POR QUÉ EXISTE. Nadie podía decir qué skill vale la pena. `skill_decisions` guarda «acepté o
// rechacé INSTALARLA», que es otra pregunta; y `tool_invocations` no guarda argumentos —a propósito,
// es una garantía de privacidad que no se toca— así que ni siquiera indirectamente se sabía qué
// skill se activó. Toda decisión sobre el arsenal se tomaba con opinión.
//
// QUÉ SE PUEDE MEDIR SIN UN MODELO, Y QUÉ NO. Se puede contar que una skill matcheó, por qué
// evidencia matcheó, si su cuerpo viajó, y si alguien lo pidió. NO se puede medir si sirvió: eso es
// juicio. Este archivo cuenta lo primero y no le pone el nombre de lo segundo — un campo «utilidad»
// acá sería opinión con un número al lado para que parezca medición.
//
// EL INSTRUMENTO LO CREÓ EL SPEC DE NIVELES. Mientras cada resolución entregaba todos los cuerpos no
// había ninguna decisión que observar. Ahora el llamador ve el nivel 1 y decide si pide el cuerpo:
// esa decisión es gratis de mirar y es lo más cerca de «sirvió» que se llega sin inferencia.

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Evidencias con las que una skill puede entrar en una resolución. Taxonomía CERRADA, como
// `outcome` en el ledger de tools.
//
// Los valores son los mismos strings que `skills.ComoMatcheo` — y hay una prueba que lo verifica.
// No se importa el paquete `skills` desde acá a propósito: la capa de memoria no tiene por qué saber
// cómo se resuelve una skill, sólo cómo se cuenta.
const (
	EvidenciaAlcance = "alcance"
	EvidenciaGlob    = "glob"
	EvidenciaComodin = "comodin"
)

// Tipos de conteo.
const (
	// UsoResuelta — la skill matcheó. Se cuenta con su evidencia.
	UsoResuelta = "resolved"
	// UsoCuerpoEnviado — su cuerpo viajó en la respuesta. Va SEPARADO de UsoResuelta: si se
	// contaran juntos, la lectura «ocupa contexto en cada resolución y nadie la abrió» sería
	// imposible de escribir, porque es exactamente la diferencia entre los dos.
	UsoCuerpoEnviado = "body_sent"
	// UsoCuerpoPedido — alguien pidió su cuerpo por nombre. Sin evidencia: el pedido no viene de
	// una resolución.
	UsoCuerpoPedido = "body_requested"
)

// SkillEvent es un conteo pendiente de bajar a la base.
type SkillEvent struct {
	Skill     string
	ProjectID string
	Evidence  string // vacío para UsoCuerpoPedido
	Kind      string
}

func kindValido(k string) bool {
	return k == UsoResuelta || k == UsoCuerpoEnviado || k == UsoCuerpoPedido
}

func evidenciaValida(e string) bool {
	return e == "" || e == EvidenciaAlcance || e == EvidenciaGlob || e == EvidenciaComodin
}

// RecordSkillEvents suma un LOTE de conteos en una sola transacción.
//
// Por lote y no de a uno porque el caller la llama desde una goroutine de flush: agrupar N upserts
// en una transacción convierte N fsync en uno. El caller NO debe tener tomado el lock de dispatch.
//
// Un evento con taxonomía inválida se SALTEA en vez de escribirse o de romper el lote: la telemetría
// no acepta texto arbitrario en una columna que después se lee sin escrutinio, pero tampoco puede
// ser el motivo por el que se pierdan los conteos buenos que venían al lado.
func (e *DbEngine) RecordSkillEvents(ctx context.Context, batch []SkillEvent) error {
	if len(batch) == 0 {
		return nil
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("contadores de skills: abrir transacción: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO skill_usage (skill, project_id, evidence, kind, n, first_at, last_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(skill, project_id, evidence, kind)
		DO UPDATE SET n = n + 1, last_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("contadores de skills: preparar upsert: %w", err)
	}
	defer stmt.Close()

	for _, ev := range batch {
		if strings.TrimSpace(ev.Skill) == "" || !kindValido(ev.Kind) || !evidenciaValida(ev.Evidence) {
			continue
		}
		if _, err := stmt.ExecContext(ctx, ev.Skill, ev.ProjectID, ev.Evidence, ev.Kind); err != nil {
			return fmt.Errorf("contadores de skills: contar %q: %w", ev.Skill, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("contadores de skills: commit: %w", err)
	}
	return nil
}

// SkillUsageRow es lo que se sabe de una skill. Los nombres dicen lo que se midió y nada más:
// no hay `utilidad`, no hay `score`, no hay ranking.
type SkillUsageRow struct {
	Skill string `json:"skill"`
	// Resolved es el TOTAL HISTÓRICO de veces que matcheó, no una ventana. Los contadores no
	// guardan serie de tiempo —son contadores— así que la recencia se lee en LastAt y no en el
	// número. Decirlo importa: un `resolved: 200` con `last_at` de hace medio año no es actividad.
	Resolved      int `json:"resolved"`
	PorAlcance    int `json:"por_alcance"`
	PorGlob       int `json:"por_glob"`
	PorComodin    int `json:"por_comodin"`
	BodySent      int `json:"body_sent"`
	BodyRequested int `json:"body_requested"`
	// LastAt es lo último que le pasó a esta skill, de cualquier tipo. Vacío si nunca pasó nada.
	LastAt string `json:"last_at,omitempty"`
	// Candidata marca un PATRÓN, no un veredicto: nada se retira ni se apaga solo. Retirar es del
	// dueño del arsenal, igual que musubi_promote_skill es explícita a propósito.
	Candidata string `json:"candidata,omitempty"`
	Porque    string `json:"porque,omitempty"`
}

// SkillUsage devuelve los contadores del proyecto de la credencial.
//
// ACOTADA AL PROYECTO (Track 19). Sin el scope, un miembro del cerebro central vería qué skills usa
// otro equipo y con qué frecuencia. La tabla tiene project_id justamente para esto.
//
// No recibe una ventana de días: los contadores no tienen serie de tiempo, así que un filtro por
// fecha daría un total histórico presentado como si fuera de la ventana. La recencia va en LastAt.
func (e *DbEngine) SkillUsage(ctx context.Context) ([]SkillUsageRow, error) {
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")

	rows, err := e.db.QueryContext(ctx, `
		SELECT skill, evidence, kind, n, last_at
		FROM skill_usage
		WHERE 1=1`+scopeSQL+`
		ORDER BY skill ASC`, scopeArgs...)
	if err != nil {
		return nil, fmt.Errorf("contadores de skills: consultar: %w", err)
	}
	defer rows.Close()

	porSkill := map[string]*SkillUsageRow{}
	for rows.Next() {
		var skill, evidencia, kind, lastAt string
		var n int
		if err := rows.Scan(&skill, &evidencia, &kind, &n, &lastAt); err != nil {
			return nil, fmt.Errorf("contadores de skills: escanear fila: %w", err)
		}
		r := porSkill[skill]
		if r == nil {
			r = &SkillUsageRow{Skill: skill}
			porSkill[skill] = r
		}
		switch kind {
		case UsoResuelta:
			r.Resolved += n
			switch evidencia {
			case EvidenciaAlcance:
				r.PorAlcance += n
			case EvidenciaGlob:
				r.PorGlob += n
			case EvidenciaComodin:
				r.PorComodin += n
			}
		case UsoCuerpoEnviado:
			r.BodySent += n
		case UsoCuerpoPedido:
			r.BodyRequested += n
		}
		if lastAt > r.LastAt {
			r.LastAt = lastAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("contadores de skills: iterar: %w", err)
	}

	out := make([]SkillUsageRow, 0, len(porSkill))
	for _, r := range porSkill {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Skill < out[j].Skill })
	return out, nil
}

// MarcarCandidata escribe el patrón que le corresponde a la fila, si alguno.
//
// LOS TRES PATRONES son lecturas de los contadores, no juicios sobre la skill:
//
//   - nunca matcheó ⇒ está muerta;
//   - matcheó y su cuerpo no viajó NI se lo pidieron ⇒ ocupa contexto y nadie la abrió;
//   - matcheó SIEMPRE por comodín y aun así le piden el cuerpo ⇒ aplica de verdad y no tiene cómo
//     decir cuándo. Es el más valioso: es la única forma de descubrir un '*' que merece volverse
//     `applies_to` sin que alguien lo adivine.
func MarcarCandidata(r *SkillUsageRow) {
	switch {
	case r.Resolved == 0 && r.BodyRequested == 0:
		r.Candidata = "muerta"
		r.Porque = "nunca matcheó ni se pidió su cuerpo"
	case r.PorComodin == r.Resolved && r.Resolved > 0 && r.BodyRequested > 0:
		r.Candidata = "alcance"
		r.Porque = fmt.Sprintf("matcheó siempre por comodín (%d) y le pidieron el cuerpo %d veces: aplica, pero no puede decir cuándo",
			r.PorComodin, r.BodyRequested)
	case r.Resolved > 0 && r.BodySent == 0 && r.BodyRequested == 0:
		r.Candidata = "retiro"
		r.Porque = fmt.Sprintf("matcheó %d veces y nadie abrió su cuerpo", r.Resolved)
	}
}

// FormatSkillUsage arma la tabla que ve una persona. Vive acá, como FormatToolUsage, porque el
// cuerpo va a querer exactamente el mismo formato en su panel.
// `huerfanas` son las que tienen contadores pero ya NO están instaladas. Van POR NOMBRE y no como
// un número: es actividad real, y decir «hay 3» sin decir cuáles no deja hacer nada con el dato.
func FormatSkillUsage(rows, huerfanas []SkillUsageRow) string {
	if len(rows) == 0 && len(huerfanas) == 0 {
		return "No hay skills en el arsenal de este proyecto."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Uso del arsenal · %d skills instaladas\n\n", len(rows))
	fmt.Fprintf(&b, "%-24s %8s %7s %7s %8s %8s %9s  %s\n",
		"SKILL", "MATCHEÓ", "alcance", "glob", "comodín", "CUERPO", "PEDIDOS", "CANDIDATA")
	var candidatas int
	for _, r := range rows {
		if r.Candidata != "" {
			candidatas++
		}
		fmt.Fprintf(&b, "%-24s %8d %7d %7d %8d %8d %9d  %s\n",
			r.Skill, r.Resolved, r.PorAlcance, r.PorGlob, r.PorComodin, r.BodySent, r.BodyRequested, r.Candidata)
	}
	fmt.Fprintf(&b, "\n%d con un patrón que amerita mirarlas. Nada se retira solo.\n", candidatas)
	if len(huerfanas) > 0 {
		b.WriteString("\nCon contadores pero YA NO INSTALADAS:\n")
		for _, h := range huerfanas {
			fmt.Fprintf(&b, "  %-24s matcheó %d · cuerpo %d · pedidos %d\n",
				h.Skill, h.Resolved, h.BodySent, h.BodyRequested)
		}
	}
	// Se dice lo que NO se midió, para que nadie lea estos números como utilidad.
	b.WriteString("Se cuenta activación y pedido de cuerpo. Si una skill SIRVIÓ no se puede medir sin juicio.\n")
	return b.String()
}
