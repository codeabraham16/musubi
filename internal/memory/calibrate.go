package memory

import (
	"encoding/json"
	"fmt"
	"math"

	"musubi/internal/redact"
)

// calibrate.go implementa la calibración OPT-IN del estimador de tokens contra
// conteos reales (p. ej. del endpoint count_tokens de Anthropic). Es opt-in y
// fuera del camino del server: el server sigue 100% offline/model-free. La parte
// de red vive en el comando CLI 'musubi calibrate'; aquí están las piezas puras
// (ajuste de divisores, reporte) y la persistencia de los divisores calibrados.

const metaTokenDivisors = "token_divisors"

// TextCount asocia un texto con su conteo de tokens REAL (ground truth). Es la
// entrada pública de la calibración: el tipo de contenido se infiere internamente.
type TextCount struct {
	Text   string
	Actual int
}

// TokenSample asocia un texto, su tipo (ya clasificado) y su conteo real. Uso interno.
type TokenSample struct {
	Text   string
	Kind   contentKind
	Actual int
}

// KindCalibration resume la precisión del estimador para un tipo de contenido.
type KindCalibration struct {
	Kind             string  `json:"kind"`
	Samples          int     `json:"samples"`
	EstimatedTokens  int     `json:"estimated_tokens"`
	ActualTokens     int     `json:"actual_tokens"`
	ErrorPct         float64 `json:"error_pct"`
	CurrentDivisor   float64 `json:"current_divisor"`
	SuggestedDivisor float64 `json:"suggested_divisor"`
}

// CalibrationReport es el resultado de comparar el estimador contra ground truth.
type CalibrationReport struct {
	PerKind         []KindCalibration `json:"per_kind"`
	OverallErrorPct float64           `json:"overall_error_pct"`
}

// kindName y kindFromName mapean contentKind <-> string (para reporte y persistencia).
func kindName(k contentKind) string {
	switch k {
	case kindCode:
		return "code"
	case kindJSON:
		return "json"
	default:
		return "prose"
	}
}

// fitDivisorForSamples calcula el divisor chars/token que mejor ajusta el conjunto
// SOBRE LA PORCIÓN ASCII, agregando todos los samples: divisor = sum(ascii) /
// sum(max(1, actual - cjk - nonascii/divNonASCII)). Descuenta las contribuciones fijas
// de CJK (~1 tok/char) y no-ASCII (divNonASCII) para que el ajuste no las absorba en el
// divisor del tipo. Se acota a un rango sano.
func fitDivisorForSamples(samples []TokenSample) float64 {
	var asciiTotal, tokenTotal float64
	for _, s := range samples {
		ascii, nonascii, cjk := countChars(s.Text)
		eff := float64(s.Actual) - float64(cjk) - float64(nonascii)/divNonASCII
		if eff < 1 {
			eff = 1
		}
		asciiTotal += float64(ascii)
		tokenTotal += eff
	}
	if tokenTotal <= 0 || asciiTotal <= 0 {
		return 0
	}
	d := asciiTotal / tokenTotal
	if d < 1.5 {
		d = 1.5
	}
	if d > 6.0 {
		d = 6.0
	}
	return d
}

// BuildCalibrationReport clasifica cada texto, los agrupa por tipo y compara el
// estimador actual contra los conteos reales, sugiriendo un divisor ajustado por
// tipo. Es puro (la clasificación es model-free).
func BuildCalibrationReport(counts []TextCount) CalibrationReport {
	byKind := map[contentKind][]TokenSample{}
	for _, c := range counts {
		k := classifyContent(c.Text)
		byKind[k] = append(byKind[k], TokenSample{Text: c.Text, Kind: k, Actual: c.Actual})
	}

	var rep CalibrationReport
	var sumAbs, sumActual float64
	for _, k := range []contentKind{kindProse, kindCode, kindJSON} {
		ss := byKind[k]
		if len(ss) == 0 {
			continue
		}
		est, act := 0, 0
		for _, s := range ss {
			est += estimateTokensFor(s.Text, k)
			act += s.Actual
		}
		errPct := 0.0
		if act > 0 {
			errPct = math.Abs(float64(est-act)) / float64(act) * 100
		}
		rep.PerKind = append(rep.PerKind, KindCalibration{
			Kind:             kindName(k),
			Samples:          len(ss),
			EstimatedTokens:  est,
			ActualTokens:     act,
			ErrorPct:         errPct,
			CurrentDivisor:   divisorFor(k),
			SuggestedDivisor: fitDivisorForSamples(ss),
		})
		sumAbs += math.Abs(float64(est - act))
		sumActual += float64(act)
	}
	if sumActual > 0 {
		rep.OverallErrorPct = sumAbs / sumActual * 100
	}
	return rep
}

// SampleContents devuelve hasta limit contenidos de observaciones para usar como muestras de
// calibración. Determinista (orden por id).
//
// LO QUE SALE DE ACÁ CRUZA LA RED. Su único consumidor es `musubi calibrate`, que manda cada
// muestra al endpoint count_tokens de Anthropic. Por eso el filtro NO es "no archivadas": es el
// predicado canónico de visibilidad MÁS la exclusión del scope local.
//
// Acá había un `archived = 0` escrito a mano, y es el defecto que este repo ya conoce: cada vez
// que alguien reimplementa el predicado en vez de interpolar visibleObsPredicate, nace un bug.
// Medido sobre la base real antes del fix: de las primeras 20 filas que este SELECT devolvía, 3
// no tenían que salir de la máquina (superseded o scope local). Una observación en cuarentena
// —o sea, marcada como no confiable— también calificaba.
//
// El scope local no es visibilidad, es DESTINO: `local` significa "no sale de esta máquina", y
// una muestra de calibración que cruza a un tercero es precisamente salir. Los 70 locales de la
// base real no aportan nada al estimador que no aporten los shared, así que el costo es cero.
func (e *DbEngine) SampleContents(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := e.db.Query(
		`SELECT content FROM observations
		 WHERE `+visibleObsPredicate+` AND COALESCE(scope, '') <> 'local'
		 ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("error al muestrear contenidos: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		// EL PORTERO, SEGUNDA LÍNEA. Medido: hoy el filtro de arriba ya es suficiente, porque
		// SaveObservation tapa el contenido al guardar cuando el scope es 'shared'
		// (operations.go, guarda C2) y las dos formas que quedan crudas —'local' y el scope
		// vacío, que normaliza a local— las excluye el WHERE. O sea que por las vías públicas
		// de escritura de HOY no hay fila alcanzable con un secreto crudo que pase por acá.
		//
		// Se pone igual, y el motivo es concreto, no ceremonial: la guarda C2 no existió siempre,
		// y una fila 'shared' escrita antes de ella conserva su texto crudo. El repo ya tiene
		// evidencia de contenido que entró sin pasar por donde debía (las 66-73 filas con marcado
		// MCP adentro del content). Un punto de egreso no puede depender de una invariante que
		// mantiene otra capa tres archivos más allá.
		//
		// Va ACÁ y no en el caller: éste es el borde por el que el texto cruza a un tercero, es
		// el único lugar donde se puede probar contra una fila cruda, y tener la misma regla en
		// dos sitios la deja sin dueño.
		clean, _ := redact.Redact(c)
		out = append(out, clean)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar contenidos para calibración: %w", err)
	}
	return out, nil
}

// divisorSet es la forma persistida de los divisores calibrados.
type divisorSet struct {
	Prose float64 `json:"prose"`
	Code  float64 `json:"code"`
	JSON  float64 `json:"json"`
}

// SaveDivisors persiste los divisores calibrados en meta.
func (e *DbEngine) SaveDivisors(prose, code, jsn float64) error {
	data, err := json.Marshal(divisorSet{Prose: prose, Code: code, JSON: jsn})
	if err != nil {
		return fmt.Errorf("error al serializar divisores: %w", err)
	}
	return e.SetMeta(metaTokenDivisors, string(data))
}

// LoadDivisors lee los divisores calibrados (ok=false si no hay calibración).
func (e *DbEngine) LoadDivisors() (prose, code, jsn float64, ok bool, err error) {
	v, has, err := e.GetMeta(metaTokenDivisors)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if !has || v == "" {
		return 0, 0, 0, false, nil
	}
	var d divisorSet
	if err := json.Unmarshal([]byte(v), &d); err != nil {
		return 0, 0, 0, false, nil // calibración corrupta: usar defaults
	}
	return d.Prose, d.Code, d.JSON, true, nil
}

// applyCalibratedDivisors normaliza los divisores activos al valor guardado en
// esta DB (o a los defaults si no hay calibración). Se llama al abrir el motor,
// para que el estado por proceso sea siempre el de la base que se está usando.
func (e *DbEngine) applyCalibratedDivisors() error {
	p, c, j, ok, err := e.LoadDivisors()
	if err != nil {
		return err
	}
	if !ok {
		ResetDivisors()
		return nil
	}
	ResetDivisors()
	ConfigureDivisors(p, c, j)
	return nil
}

// RecomputeTokens recomputa gist/tokens de TODAS las filas con los divisores
// activos. Lo usa la calibración al aplicar nuevos divisores.
func (e *DbEngine) RecomputeTokens() error {
	rows, err := e.db.Query(`SELECT id, content FROM observations`)
	if err != nil {
		return fmt.Errorf("error al consultar filas para recompute: %w", err)
	}
	type fila struct{ id, content string }
	var filas []fila
	for rows.Next() {
		var f fila
		if err := rows.Scan(&f.id, &f.content); err != nil {
			rows.Close()
			return fmt.Errorf("error al escanear recompute: %w", err)
		}
		filas = append(filas, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("error al iterar filas para recompute: %w", err)
	}
	rows.Close()

	for _, f := range filas {
		if _, err := e.db.Exec(
			`UPDATE observations SET gist=?, tokens=? WHERE id=?`,
			Gist(f.content, defaultGistMaxTokens), EstimateTokens(f.content), f.id,
		); err != nil {
			return fmt.Errorf("error al recomputar tokens de %s: %w", f.id, err)
		}
	}
	return nil
}
