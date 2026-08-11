package memory

import (
	"context"
	"fmt"
	"time"
)

// readiness.go calcula el ESTADO DE MADUREZ de una instalación de Musubi a partir de lo que la
// instalación HIZO, no de lo que alguien declara sobre ella.
//
// Las evaluaciones de madurez de la industria son cuestionarios: alguien marca casilleros sobre su
// propio equipo y sale un nivel. El problema no es que mientan — es que miden la INTENCIÓN. Un
// equipo que responde «sí, revisamos todo lo que produce el agente» y otro que efectivamente lo
// revisa sacan el mismo puntaje, y el cuestionario no puede distinguirlos porque nunca miró lo que
// pasó. Musubi tiene algo que un cuestionario no: un ledger de invocaciones, una cola de
// contradicciones, un grafo de código y el estado de su propia memoria. Todo eso es COMPORTAMIENTO
// registrado, y se puede leer sin preguntarle nada a nadie.
//
// La regla que le da los dientes: UNA SEÑAL NO OBSERVADA PUNTÚA CERO. No "N/A", no se saltea, no
// se promedia entre las demás. Si el grafo de código nunca se indexó, esa dimensión vale 0 y el
// puntaje global baja. Sin esa regla el puntaje premia no medir, que es exactamente cómo un
// indicador se convierte en una medalla: quien apaga la instrumentación sube de nivel.
//
// Y la segunda regla, que es la que lo hace auditable: EL PUNTAJE ES OPINABLE, LA EVIDENCIA NO.
// Cada dimensión devuelve los números crudos con los que se calculó. Los umbrales de acá abajo son
// convenciones —discutibles, y la idea es que se discutan— pero «hubo 1.303 invocaciones y 4 de
// ellas fallaron» no es una convención: es lo que pasó. Quien no esté de acuerdo con el umbral
// puede recalcular con el suyo sin tener que confiar en éste.

// Umbrales del cálculo. Son CONVENCIONES, no verdades: están acá arriba, con nombre y motivo,
// justamente para que se puedan discutir sin ir a buscarlas entre el código.
const (
	// readinessVentanaDias es la ventana por defecto del ledger. Un mes cubre el ciclo de trabajo
	// típico sin que una semana floja hunda el puntaje.
	readinessVentanaDias = 30

	// readinessToolsDistintas: un cerebro al que sólo le piden `musubi_recall` está siendo usado
	// como un cajón de notas, no como un sistema. Tres superficies distintas es la señal mínima
	// de integración real.
	readinessToolsDistintas = 3

	// readinessDiasFrescura: actividad en la última semana. Distingue «se usa» de «se usó».
	readinessDiasFrescura = 7

	// readinessFallaMax: por encima de este 5 % de invocaciones con error, el cerebro no está
	// sirviendo — está fallando seguido y alguien lo está tolerando.
	readinessFallaMax = 0.05

	// readinessMantenimientoDias: el mantenimiento (fusión, archivado, poda) corre solo; que no
	// haya corrido en un mes significa que el scheduler no está andando o que nadie lo dispara.
	readinessMantenimientoDias = 30
)

// ReadinessCheck es un hecho binario observado, con lo que se midió y contra qué.
type ReadinessCheck struct {
	Name string `json:"name"`
	Pass bool   `json:"pass"`
	Why  string `json:"why"`
}

// ReadinessDimension es un eje de madurez con su puntaje (fracción de hechos cumplidos) y la
// evidencia cruda con la que se calculó.
//
// Observed=false significa que NO HAY DATO, y entonces Score es 0 por regla. Es distinto de un
// Score 0 con Observed=true, que significa «se midió y está mal». La diferencia importa: el
// primero se arregla instrumentando, el segundo operando.
type ReadinessDimension struct {
	Key      string                 `json:"key"`
	Title    string                 `json:"title"`
	Observed bool                   `json:"observed"`
	Score    float64                `json:"score"`
	Checks   []ReadinessCheck       `json:"checks,omitempty"`
	Evidence map[string]interface{} `json:"evidence"`
}

// ReadinessReport es el estado de madurez completo.
type ReadinessReport struct {
	Score      float64              `json:"score"`
	WindowDays int                  `json:"window_days"`
	Dimensions []ReadinessDimension `json:"dimensions"`
	// Unobserved nombra las dimensiones sin dato. Va aparte del promedio A PROPÓSITO: un 0,4
	// compuesto por cuatro dimensiones mediocres y un 0,4 con tres dimensiones sanas y dos que
	// nadie instrumentó piden cosas distintas, y el promedio solo no los distingue.
	Unobserved []string `json:"unobserved,omitempty"`
	Note       string   `json:"note"`
}

// Readiness calcula el estado de madurez acotado al proyecto de la credencial (ctx). days<=0 usa
// la ventana por defecto. Read-only: no escribe nada, no marca nada, no dispara mantenimiento.
func (e *DbEngine) Readiness(ctx context.Context, days int) (ReadinessReport, error) {
	if days <= 0 {
		days = readinessVentanaDias
	}
	rep := ReadinessReport{WindowDays: days}

	uso, conf, err := e.dimensionesDeUso(ctx, days)
	if err != nil {
		return rep, err
	}
	mem, err := e.dimensionMemoria(ctx)
	if err != nil {
		return rep, err
	}
	coh, err := e.dimensionCoherencia(ctx, days)
	if err != nil {
		return rep, err
	}
	graf, err := e.dimensionGrafo(ctx)
	if err != nil {
		return rep, err
	}
	rep.Dimensions = []ReadinessDimension{uso, conf, mem, coh, graf}

	var suma float64
	for _, d := range rep.Dimensions {
		suma += d.Score
		if !d.Observed {
			rep.Unobserved = append(rep.Unobserved, d.Key)
		}
	}
	rep.Score = redondear(suma / float64(len(rep.Dimensions)))
	rep.Note = "Puntaje medido sobre lo que la instalación HIZO, no sobre lo que alguien declara. " +
		"Una dimensión sin dato puntúa 0 y no se saltea: no medir no puede subir el puntaje. " +
		"Los umbrales son convenciones discutibles; la evidencia de cada dimensión son los números crudos."
	return rep, nil
}

// dimensionesDeUso arma las dos dimensiones que salen del ledger: si al cerebro lo invocan, y si
// esas invocaciones salen bien. Van juntas porque comparten la misma consulta — y porque la
// segunda depende de la primera: sin invocaciones no hay tasa de falla que medir, y ahí la
// respuesta correcta es «no observado», no «0 % de errores».
func (e *DbEngine) dimensionesDeUso(ctx context.Context, days int) (uso, conf ReadinessDimension, err error) {
	filas, err := e.ToolUsage(ctx, days)
	if err != nil {
		return uso, conf, fmt.Errorf("readiness: leer el ledger de uso: %w", err)
	}
	var llamadas, errores, rechazos int
	ultima := ""
	for _, f := range filas {
		llamadas += f.Calls
		errores += f.Errors
		rechazos += f.Denied
		if f.LastUsedAt > ultima { // ISO-8601: el orden lexicográfico ES el cronológico
			ultima = f.LastUsedAt
		}
	}
	distintas := len(filas)

	uso = ReadinessDimension{
		Key: "uso", Title: "Lo invocan de verdad",
		Evidence: map[string]interface{}{
			"invocaciones": llamadas, "tools_distintas": distintas,
			"ultima_invocacion": ultima, "ventana_dias": days,
		},
	}
	if llamadas == 0 {
		// El caso que el resto del sistema no sabe ver: un cerebro instalado, sano, migrado, y que
		// nadie llama. Todos los chequeos de integridad dan verde y el valor entregado es cero.
		uso.Evidence["por_que_cero"] = "el ledger no registró ninguna invocación en la ventana"
		conf = ReadinessDimension{Key: "confiabilidad", Title: "Las invocaciones salen bien",
			Evidence: map[string]interface{}{
				"por_que_cero": "sin invocaciones no hay tasa de falla que medir; " +
					"no observado puntúa 0, no 'todo bien'",
			}}
		return uso, conf, nil
	}
	uso.Observed = true
	fresca := ultima != "" && recienteISO(ultima, readinessDiasFrescura)
	uso.Checks = []ReadinessCheck{
		{Name: "hay invocaciones", Pass: true,
			Why: fmt.Sprintf("%d en %d días", llamadas, days)},
		{Name: "más de una superficie", Pass: distintas >= readinessToolsDistintas,
			Why: fmt.Sprintf("%d tools distintas (mínimo %d)", distintas, readinessToolsDistintas)},
		{Name: "se usa ahora, no se usó", Pass: fresca,
			Why: fmt.Sprintf("última invocación %s (frescura: %d días)", oVacio(ultima), readinessDiasFrescura)},
	}
	uso.Score = puntaje(uso.Checks)

	tasaError := float64(errores) / float64(llamadas)
	tasaRechazo := float64(rechazos) / float64(llamadas)
	conf = ReadinessDimension{
		Key: "confiabilidad", Title: "Las invocaciones salen bien", Observed: true,
		Evidence: map[string]interface{}{
			"invocaciones": llamadas, "errores": errores, "rechazos": rechazos,
			"tasa_error": redondear(tasaError), "tasa_rechazo": redondear(tasaRechazo),
		},
		Checks: []ReadinessCheck{
			{Name: "los errores no son la norma", Pass: tasaError <= readinessFallaMax,
				Why: fmt.Sprintf("%d/%d fallaron (%.1f %%, tope %.0f %%)",
					errores, llamadas, tasaError*100, readinessFallaMax*100)},
			{Name: "no se choca contra los permisos", Pass: tasaRechazo <= readinessFallaMax,
				Why: fmt.Sprintf("%d/%d rechazadas (%.1f %%, tope %.0f %%)",
					rechazos, llamadas, tasaRechazo*100, readinessFallaMax*100)},
		},
	}
	conf.Score = puntaje(conf.Checks)
	return uso, conf, nil
}

// dimensionMemoria mide si hay conocimiento vivo y si el mantenimiento está corriendo.
//
// El mantenimiento sale de `meta`, que NO está acotada por proyecto: en un cerebro central la
// marca es de la instalación entera. Se declara en la evidencia en vez de fingir un dato
// per-proyecto que la tabla no tiene.
//
// LO QUE ESTA DIMENSIÓN NO MIDE, dicho acá para que no se lea de más: la marca `last_maintenance`
// la refresca el SCHEDULER del propio binario, no una persona. En una instalación viva siempre está
// fresca, así que el chequeo detecta «el scheduler está muerto» y NO «alguien cuida la memoria».
// Son cosas distintas y la primera versión las confundía en el nombre — medía bien, pero el rótulo
// prometía otra cosa, que es la manera más silenciosa de que un indicador mienta. El nombre del
// chequeo y su `why` ahora dicen exactamente lo que se observó. Detectado el 2026-08-11 mirando el
// primer resultado real del central: la marca estaba a un minuto de la consulta porque acababa de
// reiniciarse el servicio.
func (e *DbEngine) dimensionMemoria(ctx context.Context) (ReadinessDimension, error) {
	d := ReadinessDimension{Key: "memoria", Title: "Hay conocimiento y se lo mantiene",
		Evidence: map[string]interface{}{}}

	ins, err := e.InsightsCtx(ctx)
	if err != nil {
		return d, fmt.Errorf("readiness: leer el estado de la memoria: %w", err)
	}
	ultimo, _, _ := e.GetMeta(metaLastMaintenance)
	d.Evidence["observaciones_activas"] = ins.Observations.Active
	d.Evidence["observaciones_totales"] = ins.Observations.Total
	d.Evidence["ultimo_mantenimiento"] = oVacio(ultimo)
	d.Evidence["alcance_del_mantenimiento"] = "de la instalación, no del proyecto (la marca vive en `meta`, que es global)"

	if ins.Observations.Total == 0 {
		d.Evidence["por_que_cero"] = "la memoria está vacía: no hay conocimiento que mantener ni que consultar"
		return d, nil
	}
	d.Observed = true
	d.Checks = []ReadinessCheck{
		{Name: "hay memoria viva", Pass: ins.Observations.Active > 0,
			Why: fmt.Sprintf("%d activas de %d", ins.Observations.Active, ins.Observations.Total)},
		{Name: "el mantenimiento corrió alguna vez", Pass: ultimo != "",
			Why: "marca `last_maintenance`: " + oVacio(ultimo)},
		{Name: "el scheduler de mantenimiento está vivo", Pass: ultimo != "" && recienteISO(ultimo, readinessMantenimientoDias),
			Why: fmt.Sprintf("último %s (se espera cada %d días). OJO: esta marca la refresca el "+
				"scheduler del binario, no una persona — en una instalación viva siempre está fresca, "+
				"así que verde acá significa «el scheduler corre», no «alguien curó la memoria»",
				oVacio(ultimo), readinessMantenimientoDias)},
	}
	d.Score = puntaje(d.Checks)
	return d, nil
}

// dimensionCoherencia mide si la cola de contradicciones SE ATIENDE.
//
// El detector encuentra pares que se contradicen y los deja `pending`; resolverlos es un juicio que
// hace un humano o un agente. Una cola que sólo crece es memoria que se contradice a sí misma y
// nadie arbitra — la forma en que un cerebro «empieza a fallar» sin que ningún chequeo de integridad
// se ponga en rojo, porque las dos memorias en conflicto son individualmente válidas.
//
// SE COMPARA FLUJO CONTRA FLUJO, y ésa es la corrección que trajo el primer dato real. La primera
// versión medía «pendientes AHORA contra resueltas DE TODA LA VIDA»: dos escalas distintas, y la
// comparación se gana sola con historia. El central lo mostró crudo — 372 pendientes contra 780
// resueltas daba verde, pero las 780 son de años y las 372 son de hoy. Un cerebro que arbitró mucho
// hace un año y hace seis meses no arbitra nada seguiría dando verde hasta que la cola pasara las
// 780, o sea justo cuando ya no hace falta ningún indicador para darse cuenta.
//
// Ahora las dos preguntas se responden con datos de la MISMA ventana:
//
//	¿alguien arbitra todavía?  → resoluciones dentro de la ventana > 0
//	¿la cola crece o se achica? → resoluciones >= detecciones nuevas, ambas en la ventana
//
// El backlog acumulado sigue en la evidencia, pero ya no como umbral: es un número que el que lee
// interpreta, no una vara que se pasa sola con el tiempo.
func (e *DbEngine) dimensionCoherencia(ctx context.Context, days int) (ReadinessDimension, error) {
	d := ReadinessDimension{Key: "coherencia", Title: "Las contradicciones se arbitran",
		Evidence: map[string]interface{}{}}

	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("o")
	// `-N days` se interpola con %d sobre un int del caller ya normalizado (>0): SQLite no admite
	// parámetros enlazados dentro del modificador de datetime().
	desde := fmt.Sprintf(`datetime('now','-%d days')`, days)
	var pendientes, resueltasTotal, resueltasVentana, nuevasVentana int
	err := e.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN r.status = ? THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN r.status <> ? THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN r.status <> ? AND r.updated_at >= `+desde+` THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN r.created_at >= `+desde+` THEN 1 ELSE 0 END), 0)
		  FROM observation_relations r
		  JOIN observations o ON o.id = r.source_id
		 WHERE 1=1`+scopeSQL,
		append([]interface{}{RelStatusPending, RelStatusPending, RelStatusPending}, scopeArgs...)...).
		Scan(&pendientes, &resueltasTotal, &resueltasVentana, &nuevasVentana)
	if err != nil {
		return d, fmt.Errorf("readiness: contar relaciones de conflicto: %w", err)
	}
	d.Evidence["pendientes"] = pendientes
	d.Evidence["resueltas_historico"] = resueltasTotal
	d.Evidence["resueltas_en_ventana"] = resueltasVentana
	d.Evidence["detectadas_en_ventana"] = nuevasVentana
	d.Evidence["ventana_dias"] = days

	if pendientes+resueltasTotal == 0 {
		// Sin detecciones no hay nada que arbitrar. Puntúa 0 igual, y el motivo lo dice: puede ser
		// una memoria chica y sana, o el detector que nunca corrió. Desde acá no se distinguen, y
		// suponer la versión buena es justo lo que un cuestionario haría.
		d.Evidence["por_que_cero"] = "no hay ninguna relación de conflicto registrada: o la memoria es " +
			"demasiado chica para contradecirse, o el detector nunca corrió. No se puede distinguir desde acá"
		return d, nil
	}
	d.Observed = true
	d.Checks = []ReadinessCheck{
		{Name: "alguien arbitra todavía", Pass: resueltasVentana > 0,
			Why: fmt.Sprintf("%d resueltas en %d días (histórico: %d, backlog actual: %d)",
				resueltasVentana, days, resueltasTotal, pendientes)},
		{Name: "la cola no crece", Pass: resueltasVentana >= nuevasVentana,
			Why: fmt.Sprintf("%d resueltas contra %d detectadas nuevas, ambas en %d días",
				resueltasVentana, nuevasVentana, days)},
	}
	d.Score = puntaje(d.Checks)
	return d, nil
}

// dimensionGrafo mide si el código está indexado. Un grafo con nodos y sin aristas es una lista de
// símbolos: no sabe responder quién llama a qué, que es lo único para lo que existe.
func (e *DbEngine) dimensionGrafo(ctx context.Context) (ReadinessDimension, error) {
	d := ReadinessDimension{Key: "grafo", Title: "El código está indexado",
		Evidence: map[string]interface{}{}}

	nodos, porTipo, err := e.GraphStatsCtx(ctx)
	if err != nil {
		return d, fmt.Errorf("readiness: leer el grafo de código: %w", err)
	}
	aristas := 0
	for _, n := range porTipo {
		aristas += n
	}
	d.Evidence["nodos"] = nodos
	d.Evidence["aristas"] = aristas
	d.Evidence["aristas_por_tipo"] = porTipo

	if nodos == 0 && aristas == 0 {
		d.Evidence["por_que_cero"] = "el grafo de código nunca se indexó (musubi_codegraph_index)"
		return d, nil
	}
	d.Observed = true
	d.Checks = []ReadinessCheck{
		{Name: "hay símbolos indexados", Pass: nodos > 0, Why: fmt.Sprintf("%d nodos", nodos)},
		{Name: "hay relaciones, no sólo una lista", Pass: aristas > 0, Why: fmt.Sprintf("%d aristas", aristas)},
	}
	d.Score = puntaje(d.Checks)
	return d, nil
}

// puntaje es la fracción de chequeos cumplidos. Sin chequeos ⇒ 0 (no observado).
func puntaje(cs []ReadinessCheck) float64 {
	if len(cs) == 0 {
		return 0
	}
	ok := 0
	for _, c := range cs {
		if c.Pass {
			ok++
		}
	}
	return redondear(float64(ok) / float64(len(cs)))
}

// recienteISO responde si una marca de tiempo cae dentro de los últimos `dias`.
//
// Acepta DOS formatos porque el cerebro guarda las fechas de dos maneras y las dos llegan acá: la
// marca de mantenimiento vive en `meta` en RFC3339, y el `last_used_at` del ledger sale del
// `CURRENT_TIMESTAMP` de SQLite, que es `2006-01-02 15:04:05` en UTC. Reconocer una sola dejaba el
// chequeo de frescura imposible de aprobar — un chequeo que no puede dar verde no mide, castiga.
// Lo encontró el test, no la lectura del código.
//
// Una marca ilegible cuenta como NO reciente: un dato que no se puede leer no es evidencia de nada,
// y darlo por bueno sería regalar puntaje por tener basura en la tabla.
func recienteISO(marca string, dias int) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, marca); err == nil {
			return time.Since(t) <= time.Duration(dias)*24*time.Hour
		}
	}
	return false
}

// oVacio evita que un campo ausente se lea como un dato.
func oVacio(s string) string {
	if s == "" {
		return "(nunca)"
	}
	return s
}

// redondear deja los puntajes en dos decimales: fingir más precisión sobre una escala de tres
// chequeos binarios sería ruido con aire de medición.
func redondear(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
