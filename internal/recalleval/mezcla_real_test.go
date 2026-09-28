package recalleval

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"

	_ "modernc.org/sqlite"
)

// ¿CUÁNTO DE OTROS PROYECTOS LE LLEGA AL TURNO CUANDO SE PREGUNTA POR EL PROPIO?
//
// Es un INSTRUMENTO, no una compuerta: no hay umbral que haga fallar nada. Existe para que la mezcla
// de proyectos se pueda volver a medir con un comando y no con un script suelto, y para que las tres
// respuestas de loop.recall_otros_proyectos se comparen sobre la MISMA base y las MISMAS consultas.
//
// CÓMO SE CORRE. Sólo con MUSUBI_FIXTURE_DB apuntando a una memory.db real (igual que
// dilucion_real_test.go). Esa base se abre en mode=ro y se copia con VACUUM INTO a t.TempDir(): el
// motor, que migra lo que abre, trabaja sobre la copia, así que la base nombrada no se toca ni aunque
// sea la memoria de trabajo de alguien. El proyecto propio es MUSUBI_FIXTURE_PROPIO o, sin esa
// variable, el project_id con más notas vivas; el informe dice cuál usó.
//
//	MUSUBI_FIXTURE_DB=/ruta/a/copia.db go test -run TestMezclaDeProyectosReal -v ./internal/recalleval/
//
// EL BRAZO ES EL DEL HOOK: ConfigTurnoConAlcance con el modo y el tope de fábrica —las opciones que
// arma buildTurnRecall—, el presupuesto de loop.recall_budget y el motor sin embebedor. No se usa
// rankedIDs porque fuerza un presupuesto infinito: mide el ORDEN, y lo que se mide acá es qué ENTRA
// al bloque, que es justo lo que decide el empaquetado.
//
// LAS CONSULTAS SON SÓLO DEL PROYECTO PROPIO: tópicos cuyas notas vivas tienen todas ese project_id,
// con los filtros del fixture real (de 3 a 50 notas, sin git-commit, sdd/ ni project/profile), y el
// texto de ConsultaDesdeTopico. Se pregunta por lo propio a propósito: lo ajeno que aparece acá es
// ruido para quien trabaja en este repo, no una respuesta que alguien buscó.
//
// LO QUE REPORTA, por modo: el % de notas ajenas por consulta (la media de los porcentajes y el total
// sobre todo lo inyectado), el % de lo ajeno que es un registro histórico (un commit o un artefacto
// SDD de otro repo) y el % de consultas con al menos una ajena; además, cuántas consultas pasaron el
// tope y en cuántas no entró nada propio, con sus tópicos: ésas son las que levantan el tope en
// «aparte», y mirarlas es la forma de saber si lo ajeno que las llenó venía al caso.
func TestMezclaDeProyectosReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	if ruta == "" {
		t.Skip("MUSUBI_FIXTURE_DB no está definida: este banco mide la mezcla de proyectos de una base real y no hay ninguna versionada")
	}
	dir := copiaDeLaBase(t, ruta)
	propio, topicos := topicosDelPropio(t, dir, os.Getenv("MUSUBI_FIXTURE_PROPIO"))
	if len(topicos) == 0 {
		t.Fatalf("ningún tópico del proyecto %q pasa los filtros del fixture: no hay con qué preguntar", propio)
	}

	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	m, l := config.Default().Memory, config.Default().Loop
	ctx := context.Background()
	t.Logf("proyecto propio: %q · %d consultas de sus tópicos · tope %d · presupuesto %d tokens",
		propio, len(topicos), l.RecallOtrosMax, l.RecallBudget)
	for _, modo := range []string{config.OtrosProyectosAparte, config.OtrosProyectosAislado, config.OtrosProyectosMezclado} {
		cfg := ConfigTurnoConAlcance(m, memory.AlcanceDelTurnoSegun(modo, l.RecallOtrosMax, propio))
		// El motor de este banco no tiene embebedor —NewDbEngine no fija procedencia de vectores—,
		// que es el estado del hook de hoy. Un brazo que pidiera vector mediría otro proceso.
		if cfg.UseVector || !cfg.SinEmbebedor {
			t.Fatalf("modo %s: el brazo dejó de ser el del hook (UseVector=%v, SinEmbebedor=%v)", modo, cfg.UseVector, cfg.SinEmbebedor)
		}
		opts := cfg.Opts
		opts.TokenBudget = l.RecallBudget
		opts.NoBump = true // un recall no puede cambiar la frecuencia que rankea al siguiente

		var r mezclaDeProyectos
		for _, tp := range topicos {
			res, err := eng.Recall(ctx, ConsultaDesdeTopico(tp), opts)
			if err != nil {
				t.Fatalf("modo %s, tópico %s: %v", modo, tp, err)
			}
			r.sumar(res.Items, propio, l.RecallOtrosMax, tp)
		}
		t.Log(r.informe(modo, l.RecallOtrosMax))
		if len(r.sinPropiasEn) > 0 {
			t.Logf("%-9s sin nada propio: %s", modo, strings.Join(r.sinPropiasEn, " · "))
		}
	}
}

// mezclaDeProyectos acumula, consulta por consulta, cuánto de lo inyectado es de otro proyecto.
type mezclaDeProyectos struct {
	consultas, items, ajenas, historicas int
	conAjena, sobreElTope, sinPropias    int
	maxAjenas                            int
	sumaPct                              float64
	sinPropiasEn                         []string // los tópicos de las consultas sin nada propio
}

func (r *mezclaDeProyectos) sumar(items []memory.RecallItem, propio string, tope int, topico string) {
	r.consultas++
	ajenas, historicas := 0, 0
	for _, it := range items {
		// El criterio de la muralla, el mismo que marca la viñeta: sin atribución no es ajena.
		if memory.MismoProyecto(propio, it.ProjectID) {
			continue
		}
		ajenas++
		if esRegistroHistorico(it.TopicKey) {
			historicas++
		}
	}
	r.items += len(items)
	r.ajenas += ajenas
	r.historicas += historicas
	if len(items) > 0 {
		r.sumaPct += float64(ajenas) / float64(len(items))
		if ajenas == len(items) {
			r.sinPropias++
			r.sinPropiasEn = append(r.sinPropiasEn, topico)
		}
	}
	if ajenas > 0 {
		r.conAjena++
	}
	if ajenas > tope {
		r.sobreElTope++
	}
	if ajenas > r.maxAjenas {
		r.maxAjenas = ajenas
	}
}

func (r mezclaDeProyectos) informe(modo string, tope int) string {
	pct := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return 100 * float64(a) / float64(b)
	}
	media := 0.0
	if r.consultas > 0 {
		media = 100 * r.sumaPct / float64(r.consultas)
	}
	return fmt.Sprintf("%-9s ajeno por consulta %5.1f %% (media) · %d de %d notas inyectadas (%.1f %%) · "+
		"históricas %d (%.1f %% de lo ajeno) · consultas con ≥1 ajena %d de %d (%.1f %%) · "+
		"máx %d por consulta · %d pasaron el tope de %d · %d sin nada propio",
		modo, media, r.ajenas, r.items, pct(r.ajenas, r.items),
		r.historicas, pct(r.historicas, r.ajenas), r.conAjena, r.consultas, pct(r.conAjena, r.consultas),
		r.maxAjenas, r.sobreElTope, tope, r.sinPropias)
}

// esRegistroHistorico es el criterio de memory.historicalRecord —un commit o un artefacto SDD—, que
// no se exporta. Se repite acá, y no se exporta para un banco que no es compuerta, con la misma
// definición: `git-commit`, o `sdd/<cambio>/<fase>` con el cambio no vacío.
func esRegistroHistorico(topic string) bool {
	if topic == memory.CommitTopicKey {
		return true
	}
	resto, ok := strings.CutPrefix(topic, "sdd/")
	if !ok {
		return false
	}
	cambio, _, ok := strings.Cut(resto, "/")
	return ok && cambio != ""
}

// copiaDeLaBase copia la base con VACUUM INTO —una foto consistente aunque la base esté viva y en
// WAL— a la carpeta de un proyecto temporal, que es lo que NewDbEngine abre. La original se lee en
// mode=ro y nada más.
func copiaDeLaBase(t *testing.T, ruta string) string {
	t.Helper()
	src, err := sql.Open("sqlite", "file:"+ruta+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.ToSlash(filepath.Join(dir, config.DirName, config.DBFile))
	// VACUUM INTO no admite parámetros enlazados; el destino lo arma este test en su t.TempDir().
	if _, err := src.Exec(`VACUUM INTO '` + strings.ReplaceAll(dest, "'", "''") + `'`); err != nil {
		t.Fatalf("copiar %s: %v", ruta, err)
	}
	return dir
}

// topicosDelPropio devuelve el proyecto propio (el pedido o, si viene vacío, el de más notas vivas) y
// sus tópicos que pasan los filtros del fixture real, en orden.
func topicosDelPropio(t *testing.T, dir, propio string) (string, []string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, config.DirName, config.DBFile))+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// El predicado de visibilidad del fixture real (fixture_real.go), el mismo que usa el recall.
	filas, err := db.Query(`
		SELECT COALESCE(topic_key,''), COALESCE(project_id,'')
		FROM observations
		WHERE COALESCE(archived,0) = 0 AND superseded_by IS NULL AND COALESCE(quarantined,0) = 0`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	notasPorProyecto := map[string]int{}
	type tema struct {
		notas    int
		proyecto string
		mezclado bool
	}
	temas := map[string]*tema{}
	for filas.Next() {
		var topic, proyecto string
		if err := filas.Scan(&topic, &proyecto); err != nil {
			t.Fatal(err)
		}
		notasPorProyecto[proyecto]++
		if topic == "" {
			continue
		}
		tm, ok := temas[topic]
		if !ok {
			tm = &tema{proyecto: proyecto}
			temas[topic] = tm
		}
		tm.notas++
		if proyecto != tm.proyecto {
			tm.mezclado = true
		}
	}
	if err := filas.Err(); err != nil {
		t.Fatal(err)
	}
	if propio == "" {
		mejor := 0
		for p, n := range notasPorProyecto {
			if p != "" && (n > mejor || (n == mejor && p < propio)) {
				propio, mejor = p, n
			}
		}
	}
	o := OpcionesFixtureReal{}.conDefaults()
	var topicos []string
	for topic, tm := range temas {
		if tm.mezclado || tm.proyecto != propio || tm.notas < o.MinPorTopico || tm.notas > o.MaxPorTopico ||
			tieneAlgunPrefijo(topic, o.PrefijosExcluidos) {
			continue
		}
		topicos = append(topicos, topic)
	}
	sort.Strings(topicos)
	return propio, topicos
}
