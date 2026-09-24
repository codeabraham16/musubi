package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// capture.go es la RED DE SEGURIDAD determinista de la captura automática (Fase C3): un hook
// `Stop` que, al cerrar el turno, captura los COMMITS nuevos del repo como memoria LOCAL, sin
// depender del agente ni de un LLM. El mensaje de commit ES el "por qué" destilado por el humano
// (la señal estructurada de mayor valor y menor ruido, según la investigación SOTA).

// metaCaptureLastCommit guarda el último HEAD capturado. Es GLOBAL al repo (no por sesión): el
// HEAD no depende de la sesión, así que scopearlo re-capturaría en cada sesión nueva.
const metaCaptureLastCommit = "capture:last_commit"

// commit es un commit ya parseado, listo para volverse memoria.
type commit struct {
	SHA     string
	Subject string
	Body    string
	Files   []string
}

// gitLog abstrae la lectura del historial, para testear el core con un git falso.
type gitLog interface {
	Head() (string, error) // SHA del HEAD; error si no es un repo git
	// CommitsEntre devuelve los commits de desde..hasta, del más viejo al más nuevo, en orden
	// topológico. Con desde vacío, sólo hasta.
	CommitsEntre(desde, hasta string) ([]commit, error)
}

// captureStore es lo mínimo que el core necesita del motor. *memory.DbEngine lo satisface.
type captureStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	SaveObservationTyped(id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error
	ObservationExists(id string) (bool, error)
}

// prNumSuffix matchea el ` (#123)` que el squash-merge de GitHub le agrega al SUBJECT del commit.
var prNumSuffix = regexp.MustCompile(`\s*\(#\d+\)$`)

// commitKey normaliza el contenido de un commit para deduplicarlo.
//
// EL PROBLEMA: cada PR mergeado con SQUASH deja DOS memorias del mismo commit. La captura guarda el
// commit de la rama; después el squash-merge crea en main un commit NUEVO con el MISMO mensaje más
// el sufijo `(#123)` (y GitHub reescribe el trailer `Co-Authored-By` → `Co-authored-by`). La captura
// lo ve como nuevo y lo guarda otra vez. El dedup por hash EXACTO no lo agarra: el texto cambió
// apenas. Y es redundante POR CONSTRUCCIÓN — tras el squash, el commit de la rama ya no existe en la
// historia de main; el canónico es el del merge.
//
// La normalización: quitar el `(#NNN)` del subject (SÓLO del subject, no del cuerpo) y bajar todo a
// minúsculas (lo que absorbe el reescrito del trailer).
//
// La clave incluye el CUERPO y la LISTA DE ARCHIVOS, no sólo el subject: es lo que evita que dos
// commits genuinamente distintos con el mismo título colisionen.
func commitKey(content string) string {
	subject, rest, _ := strings.Cut(content, "\n")
	subject = prNumSuffix.ReplaceAllString(strings.TrimSpace(subject), "")
	return strings.ToLower(subject + "\n" + rest)
}

// commitObsID deriva un id DETERMINÍSTICO del commit desde su clave normalizada. Como el id ES la
// clave de dedup, el gemelo del squash cae en el MISMO id ⇒ el guardado lo UPSERTEA con el contenido
// canónico (el del merge) en vez de crear una observación nueva.
//
// Es un NOOP seguro por la misma razón que el dedup por hash exacto: no es una interpretación, es un
// hecho ESTRUCTURAL (el mismo commit, reformulado mecánicamente por GitHub). Un duplicado SEMÁNTICO
// —otras palabras, mismo significado— sí requiere juicio, y para eso están el dedup semántico (#193)
// y el gate de novedad (#195), que lo rutean a `pending`.
func commitObsID(content string) string {
	sum := sha256.Sum256([]byte(commitKey(content)))
	return "commit-" + hex.EncodeToString(sum[:])[:16]
}

// embedFunc genera el vector de un texto para la captura (o nil si la semántica está
// apagada / falló). nil ⇒ guardado 100% léxico (comportamiento histórico).
type embedFunc func(text string) []float32

// realGit ejecuta git en un directorio, model-free y determinista (sin pager/color/locale).
type realGit struct{ dir string }

func (g realGit) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	full := append([]string{"-C", g.dir, "--no-pager"}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	return string(out), err
}

func (g realGit) Head() (string, error) {
	out, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g realGit) CommitsEntre(desde, hasta string) ([]commit, error) {
	return g.commitsEntre(desde, hasta, 1)
}

// commitsEntre es CommitsEntre con el tope de commits para el caso sin base: 1 en el camino normal
// (la primera corrida sobre un repo), ventanaBasePerdida cuando la base de la tanda ya no existe.
func (g realGit) commitsEntre(desde, hasta string, ventana int) ([]commit, error) {
	desde, hasta = strings.TrimSpace(desde), strings.TrimSpace(hasta)
	// --topo-order con --reverse: un padre sale SIEMPRE antes que sus hijos. El orden por defecto es
	// por fecha, y con commits del mismo segundo (un rebase en lote) o relojes corridos entre
	// máquinas, un hijo podía salir antes que su padre. Con la tanda congelada no se pierde nada
	// igual —se recorre la lista entera—, pero la memoria de un commit no debería nacer antes que
	// la del cambio del que depende.
	// Separadores de control para un parseo robusto: %x1e entre commits, %x1f entre campos.
	args := []string{"log", "--no-color", "--no-merges", "--reverse", "--topo-order", "--name-only",
		"--format=%x1e%H%x1f%s%x1f%b%x1f"}
	if desde != "" {
		args = append(args, desde+".."+hasta)
	} else {
		if ventana < 1 {
			ventana = 1
		}
		args = append(args, "-n", strconv.Itoa(ventana), hasta)
	}
	out, err := g.run(args...)
	if err != nil {
		// QUÉ FALLÓ decide qué se hace, y la primera versión no lo preguntaba: ante CUALQUIER error del
		// rango caía a capturar sólo `hasta`, y la tanda se cerraba dando por capturado todo lo del
		// medio sin mirarlo. La segunda revisión lo reprodujo: base borrada por un gc, cuatro commits
		// nuevos, se capturó uno. Y un timeout de git con la base viva perdía igual.
		if desde != "" && !g.existe(desde) {
			// La base ya no existe (un rebase seguido de gc): no hay rango que pedir. Se toma una
			// VENTANA hacia atrás desde `hasta` en vez de un solo commit. Repasar lo ya capturado no
			// cuesta memoria: los ids son determinísticos por contenido y repasar es UPSERT.
			return g.commitsEntre("", hasta, ventanaBasePerdida)
		}
		if !g.existe(hasta) {
			return nil, fmt.Errorf("%w (%s): %v", errObjetivoPerdido, hasta, err)
		}
		return nil, err // la base y el objetivo existen: falló otra cosa, y la tanda se conserva
	}
	return parseCommits(out), nil
}

// ventanaBasePerdida es cuántos commits hacia atrás se toman cuando la base de la tanda ya no existe.
const ventanaBasePerdida = 200

// errObjetivoPerdido: el commit objetivo de la tanda ya no existe. Es el ÚNICO error del rango que
// abandona la tanda; cualquier otro la conserva con su progreso.
var errObjetivoPerdido = errors.New("el objetivo de la tanda ya no existe")

// existe dice si un commit existe en el repo.
func (g realGit) existe(sha string) bool {
	_, err := g.run("cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// incapturable dice si un error de capturarUnCommit es del COMMIT y no del momento: un payload que
// una guarda rechaza mirándolo (ErrPayloadInvalido; p. ej. un cuerpo que termina en `</content>`) o
// un id que ya es de otro proyecto en el central (ErrCrossTenant; dos repos con el mismo primer
// commit). Reintentarlos da lo mismo siempre.
func incapturable(err error) bool {
	return errors.Is(err, memory.ErrPayloadInvalido) || errors.Is(err, memory.ErrCrossTenant)
}

// sinSHARepetidos saca de la lista los registros con un SHA que ya apareció. En la salida de git un
// commit sale una vez; un repetido sólo lo fabrica un mensaje que trae los separadores del parseo
// adentro. Si el progreso (`:hecho`) caía en esa segunda aparición, la corrida siguiente lo
// encontraba en la PRIMERA y rehacía el mismo tramo para siempre.
func sinSHARepetidos(commits []commit) []commit {
	vistos := make(map[string]bool, len(commits))
	out := commits[:0:0]
	for _, c := range commits {
		if c.SHA != "" && vistos[c.SHA] {
			continue
		}
		vistos[c.SHA] = true
		out = append(out, c)
	}
	return out
}

// parseCommits parsea la salida de `git log` con separadores %x1e/%x1f + --name-only.
func parseCommits(out string) []commit {
	var res []commit
	for _, rec := range strings.Split(out, "\x1e") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 4)
		if len(parts) < 3 {
			continue
		}
		c := commit{
			SHA:     strings.TrimSpace(parts[0]),
			Subject: strings.TrimSpace(parts[1]),
			Body:    strings.TrimSpace(parts[2]),
		}
		if len(parts) == 4 {
			for _, f := range strings.Split(parts[3], "\n") {
				if f = strings.TrimSpace(f); f != "" {
					c.Files = append(c.Files, f)
				}
			}
		}
		res = append(res, c)
	}
	return res
}

// classifyCommit deduce (model-free, por keyword del subject) el tipo/importancia de un commit, o
// si es trivial y hay que omitirlo. fix/bug/security → alto; feat/refactor/perf → medio;
// chore/docs/style/test/build/ci y merge/wip/subjects cortos → skip.
func classifyCommit(subject string) (memType string, importance float64, skip bool) {
	s := strings.ToLower(strings.TrimSpace(subject))
	if len(s) < 10 || strings.HasPrefix(s, "merge") || strings.HasPrefix(s, "wip") {
		return "", 0, true
	}
	typ := s
	if i := strings.IndexAny(s, ":("); i > 0 {
		typ = strings.TrimSpace(s[:i])
	}
	switch {
	case hasWord(s, "fix", "bug", "hotfix", "security", "cve", "vuln", "revert"):
		return "episodic", 0.7, false
	case hasWord(s, "feat", "refactor", "perf"):
		return "episodic", 0.5, false
	case typ == "chore" || typ == "docs" || typ == "doc" || typ == "style" || typ == "test" || typ == "build" || typ == "ci":
		return "", 0, true
	default:
		return "episodic", 0.4, false
	}
}

// hasWord matchea w como palabra/prefijo de tipo (fix, fix:, fix(scope), "... fix ..."), evitando
// falsos positivos por substring (prefix, suffix).
func hasWord(s string, words ...string) bool {
	for _, w := range words {
		if s == w ||
			strings.HasPrefix(s, w+":") || strings.HasPrefix(s, w+"(") || strings.HasPrefix(s, w+" ") ||
			strings.Contains(s, " "+w+" ") || strings.Contains(s, " "+w+":") {
			return true
		}
	}
	return false
}

// captureCommits es el core testeable: captura los commits nuevos desde el último HEAD guardado.
// El SCOPE lo decide el caller (C5.2): 'local' en un proyecto personal, 'shared' en team mode —
// donde la captura es CENTRAL por naturaleza y los commits deben llegar a las demás máquinas del
// equipo, no quedarse en la que los hizo. Compartir es seguro: la redacción de secretos corre en el
// BORDE a 'shared' dentro de saveObservation (C2), por cualquier ruta, no sólo vía promote.
// Si embed no es nil, cada commit se guarda CON su embedding (participa del recall semántico); si
// es nil, guardado léxico. Devuelve cuántas guardó. No-op silencioso si no es repo git o no hay
// commits nuevos.
// detectFunc corre la detección de relaciones sobre una observación recién guardada. Se inyecta
// (como embed) para que el core siga testeable sin engine real. nil = sin detección.
type detectFunc func(obsID string)

func captureCommits(store captureStore, git gitLog, embed embedFunc, detect detectFunc, scope string) (int, error) {
	return captureCommitsKeyed(store, git, embed, detect, scope, metaCaptureLastCommit, 0)
}

// maxCommitsPorCorrida acota cuántos commits mira UNA corrida del hook Stop. El hook tiene un
// presupuesto de 10 s y cada commit paga embedding más detección de duplicados contra toda la
// memoria, así que un rango largo no entra nunca. Con la tanda congelada un tope no pierde nada: la
// corrida siguiente sigue desde el último commit hecho. 0 = sin tope, que es lo que usa el modo
// origin-side del cerebro central, porque corre en un timer y no tiene ese presupuesto encima.
const maxCommitsPorCorrida = 25

// Claves de la TANDA en curso, colgadas de la clave del cursor (así cada repo lleva la suya).
const (
	sufijoTandaObjetivo = ":objetivo" // el HEAD que se fijó al empezar la tanda
	sufijoTandaHecho    = ":hecho"    // el último commit ya procesado de esa tanda
)

// captureCommitsKeyed es captureCommits con la CLAVE DEL CURSOR explícita. Capturar varios repos en
// la MISMA memoria (el cerebro central, origin-side) exige que cada repo lleve su propio cursor
// `capture:last_commit:<repo>`: con la clave global compartida, capturar el repo B pisaría el HEAD
// del repo A y ninguno avanzaría bien. captureCommits usa la clave global histórica (un repo por
// workspace, el caso del hook en la máquina de dev). limite acota cuántos commits procesa esta
// corrida (0 = todos); ver maxCommitsPorCorrida.
//
// EL AVANCE ES UNA TANDA CONGELADA, NO UN COMMIT SUELTO, y la diferencia es todo el arreglo.
//
// El defecto original: el cursor se guardaba UNA vez, al final del bucle. El hook tiene 10 s; con
// 546 commits pendientes cada corrida moría a mitad y la siguiente arrancaba por el mismo commit.
// Veinte días con progreso cero.
//
// La primera versión de este arreglo guardaba el cursor en el SHA de cada commit procesado, y una
// revisión adversarial lo refutó antes del merge, con tres simulaciones independientes: la historia
// de este repo tiene más de cien merges, y `cursor..HEAD` excluye sólo los ANCESTROS del cursor. Si
// el último procesado queda en una rama, los commits ya procesados de la rama paralela vuelven a
// entrar al rango; el rango no se achica de forma monótona y, sobre la historia real desde el cursor
// huérfano e928e67, a la corrida 15 caía en un ciclo de tres cursores y no llegaba nunca al HEAD.
// Agregar --topo-order no alcanzaba: ciclaba con período dos.
//
// Ahora: al empezar una tanda se FIJA su objetivo —el HEAD de ese momento— y se recorre la lista
// determinística base..objetivo, guardando el último commit hecho. La corrida siguiente recalcula la
// MISMA lista (mismos extremos, mismo orden), saltea hasta el último hecho y sigue. Recién cuando la
// lista se termina, la base salta al objetivo. El avance es una posición en una lista fija, no un
// commit suelto en un grafo que no es una línea. Los commits que lleguen mientras tanto entran en la
// tanda siguiente, objetivo..HEAD.
func captureCommitsKeyed(store captureStore, git gitLog, embed embedFunc, detect detectFunc, scope, cursorKey string, limite int) (int, error) {
	head, err := git.Head()
	if err != nil || head == "" {
		return 0, nil
	}
	objKey, hechoKey := cursorKey+sufijoTandaObjetivo, cursorKey+sufijoTandaHecho
	base := metaLimpia(store, cursorKey)
	objetivo := metaLimpia(store, objKey)
	if objetivo == "" {
		if base == head {
			return 0, nil // al día
		}
		objetivo = head
		_ = store.SetMeta(objKey, objetivo)
		_ = store.SetMeta(hechoKey, "")
	}
	commits, err := git.CommitsEntre(base, objetivo)
	if err != nil {
		// Sólo si el OBJETIVO ya no existe (un rebase seguido de gc) se abandona la tanda, y la corrida
		// siguiente arranca una nueva hacia el HEAD de ese momento. Cualquier otro error —un timeout de
		// git, un lock— la CONSERVA con su progreso: tirarla obligaba a repasarla desde el principio.
		if errors.Is(err, errObjetivoPerdido) {
			_ = store.SetMeta(objKey, "")
			_ = store.SetMeta(hechoKey, "")
		}
		return 0, err
	}
	commits = sinSHARepetidos(commits)
	desde := 0
	if hecho := metaLimpia(store, hechoKey); hecho != "" {
		for i, c := range commits {
			if c.SHA == hecho {
				desde = i + 1
				break
			}
		}
		// Si el último hecho no aparece, la lista cambió: se recorre entera. El id determinístico
		// hace que repasar lo ya guardado lo actualice, no lo duplique.
	}
	saved, procesados, avanzo := 0, 0, false
	for _, c := range commits[desde:] {
		// El tope sólo se aplica si esta corrida YA pudo guardar su progreso (un commit con SHA).
		// Cortar sin haberlo guardado dejaría a la corrida siguiente repitiendo el mismo tramo para
		// siempre: sería cambiar una captura trabada por otra.
		if limite > 0 && procesados >= limite && avanzo {
			return saved, nil // tanda a medias: la próxima corrida sigue desde el último hecho
		}
		nuevo, err := capturarUnCommit(store, c, embed, detect, scope)
		if err != nil {
			if !incapturable(err) {
				return saved, err // transitorio (BUSY, E/S): se reintenta desde acá en la corrida siguiente
			}
			// INCAPTURABLE POR CONSTRUCCIÓN: fallaría igual en cada corrida, y antes eso congelaba la
			// tanda para siempre —la base no se movía y nada posterior entraba nunca—. Se avisa y se
			// saltea, y el progreso avanza como con cualquier otro commit.
			fmt.Fprintf(os.Stderr, "musubi capture: el commit %.7s no se puede guardar como memoria y se saltea: %v\n", c.SHA, err)
			nuevo = 0
		}
		saved += nuevo
		procesados++
		// Progreso DURABLE, commit por commit: si el hook se queda sin sus 10 s a mitad, lo hecho
		// queda hecho y la corrida siguiente no lo repite.
		if c.SHA != "" {
			_ = store.SetMeta(hechoKey, c.SHA)
			avanzo = true
		}
	}
	// Tanda completa: recién ahora la base salta al objetivo. Hacerlo antes —al cortar por el tope—
	// se tragaría en silencio lo que quedó sin mirar.
	_ = store.SetMeta(cursorKey, objetivo)
	_ = store.SetMeta(objKey, "")
	_ = store.SetMeta(hechoKey, "")
	return saved, nil
}

// metaLimpia lee una clave de meta sin espacios alrededor ("" si no está).
func metaLimpia(store captureStore, key string) string {
	v, _, _ := store.GetMeta(key)
	return strings.TrimSpace(v)
}

// capturarUnCommit guarda un commit. Devuelve 1 si nació memoria nueva y 0 si no hubo nada que
// guardar: el commit era trivial, o el UPSERT actualizó el gemelo que ya existía.
func capturarUnCommit(store captureStore, c commit, embed embedFunc, detect detectFunc, scope string) (int, error) {
	memType, importance, skip := classifyCommit(c.Subject)
	if skip {
		return 0, nil
	}
	content := c.Subject
	if c.Body != "" {
		content += "\n\n" + c.Body
	}
	if len(c.Files) > 0 {
		content += "\n\nArchivos: " + strings.Join(c.Files, ", ")
	}
	var vec []float32
	if embed != nil {
		vec = embed(content)
	}
	// Id DETERMINÍSTICO desde la clave normalizada (ver commitObsID): si ya existe, este "commit
	// nuevo" es el mismo commit reformulado por el squash-merge ⇒ el guardado lo UPSERTEA con el
	// contenido canónico en vez de crear un gemelo. No se oculta ni se descarta nada: se
	// ACTUALIZA. SaveObservationTyped preserva created_at y las stats de acceso en el update.
	id := commitObsID(content)
	existed, err := store.ObservationExists(id)
	if err != nil {
		return 0, err
	}
	if err := store.SaveObservationTyped(id, memory.CommitTopicKey, content, importance, memType, scope, vec); err != nil {
		return 0, err
	}
	if existed {
		return 0, nil // gemelo del squash: se actualizó lo existente, no hay memoria nueva
	}
	// Gate de novedad (M4): marcar el commit que duplica algo ya guardado. Sólo sobre memoria
	// REALMENTE nueva: un UPSERT no crea observación que relacionar. El detect corre en modo
	// DetectOnly ⇒ jamás auto-oculta un commit anterior.
	if detect != nil {
		detect(id)
	}
	return 1, nil
}

// repoCursorKey deriva una clave de cursor estable y única por repo desde su ruta absoluta, para que
// la captura multi-repo no mezcle los HEAD. Ruta no resoluble ⇒ se usa la cruda (degradación segura).
func repoCursorKey(repoPath string) string {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	sum := sha256.Sum256([]byte(abs))
	return metaCaptureLastCommit + ":" + hex.EncodeToString(sum[:])[:12]
}

// runCapture implementa `musubi capture [--hook-mode] [--repo DIR --project ID --scope shared --fetch]`.
// Sin flags de repo: es el hook Stop en la máquina de dev (silencioso, captura el workspace). Con
// --repo captura OTRO repo hacia esta misma memoria — el modo ORIGIN-SIDE del cerebro central: un
// timer corre `capture --repo <mirror> --project <p> --scope shared --fetch` por cada repo de Forgejo,
// así el cerebro aprende de CADA push de cualquiera (no solo de lo que tocó una sesión de Claude).
// Best-effort: cualquier fallo (sin repo, sin memoria) no rompe nada.
func runCapture(args []string) {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	hookModeF := fs.Bool("hook-mode", false, "modo hook Stop (silencioso)")
	repoF := fs.String("repo", "", "capturar desde este repo en vez del workspace (ruta a un repo git, incl. bare/mirror)")
	projectF := fs.String("project", "", "estampar este project_id en los commits capturados (aislamiento por tenant)")
	scopeF := fs.String("scope", "", "scope de guardado: local | shared (default: según team_mode)")
	fetchF := fs.Bool("fetch", false, "git fetch en el repo antes de capturar (para mantener frescos los mirror clones)")
	_ = fs.Parse(args)
	hookMode := *hookModeF

	if hookMode {
		_, _ = io.Copy(io.Discard, os.Stdin) // drenar el payload del Stop (no lo necesitamos)
	}

	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: workspace no disponible: %v\n", err)
		}
		return
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: memoria no disponible: %v\n", err)
		}
		return
	}
	defer engine.Close()

	// De DÓNDE se leen los commits: el repo indicado, o el workspace (hook). La MEMORIA es siempre
	// la del workspace (el cerebro central en el modo origin-side); solo cambia el árbol git de origen.
	gitDir := root
	if strings.TrimSpace(*repoF) != "" {
		gitDir = *repoF
	}

	// Embeddings en la captura (16.2e): si la semántica está encendida (auto-detección de la
	// tabla + degradación elegante, igual que serve/daemon), cada commit capturado se guarda CON
	// su vector, estampando la MISMA procedencia (F2.2) que el daemon para que sean homogéneos.
	// Best-effort: un error de embedding devuelve nil (ese commit queda léxico), no rompe el turno.
	var embed embedFunc
	cfg, _ := config.Load(root)
	embedder := resolveEmbedder(cfg, root)
	if embedding.Enabled(embedder) {
		engine.SetVectorModelID(embedder.Name())
		embed = func(text string) []float32 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			v, eerr := embedder.Embed(ctx, text)
			if eerr != nil {
				return nil
			}
			return v
		}
	}

	// Gate de novedad (M4): la memoria que Musubi captura SOLA también pasa por la detección de
	// duplicados. En modo DetectOnly: detecta y MARCA como `pending` para que lo juzgue el agente,
	// pero NUNCA auto-oculta — acá todos los commits comparten topic_key="git-commit" (un balde, no
	// un tema), así que un auto-supersede taparía un commit anterior por parecerse en el mensaje.
	// Best-effort: si la detección falla, el commit YA quedó guardado; la captura no rompe el turno.
	var detect detectFunc
	if cfg.Conflicts.Enabled {
		detect = func(obsID string) {
			if _, derr := engine.DetectRelations(obsID, memory.ConflictOptions{
				SimilarityFloor:      cfg.Conflicts.SimilarityFloor,
				AutoResolveThreshold: cfg.Conflicts.AutoResolveThreshold,
				CandidatePool:        cfg.Conflicts.CandidatePool,
				CosineFloor:          cfg.Conflicts.CosineFloor,
				CosineAutoThreshold:  cfg.Conflicts.CosineAutoThreshold,
				LedgerPrefixes:       cfg.Conflicts.LedgerPrefixes,
				DetectOnly:           true,
			}); derr != nil && !hookMode {
				fmt.Fprintf(os.Stderr, "capture: detección de duplicados falló (el commit se guardó igual): %v\n", derr)
			}
		}
	}

	// Atribución por proyecto (aislamiento por tenant): con --project se estampa ese project_id en
	// cada commit capturado, así el recall del central los acota al tenant correcto. Sin --project, el
	// del workspace (comportamiento del hook). Debe fijarse ANTES de capturar (afecta el guardado).
	projectID := strings.TrimSpace(*projectF)
	if projectID == "" {
		projectID = resolveProjectID(cfg, root)
	}
	engine.SetProjectID(projectID)
	engine.SetLedgerPrefixes(cfg.Conflicts.LedgerPrefixes)

	// Scope (C5.2): --scope lo fija explícito; si no, team mode ⇒ shared, si no local. En el modo
	// origin-side del central se pasa --scope shared (la captura es central por naturaleza: los
	// commits deben poder llegar por inbound-sync a las máquinas del equipo). El id del commit es
	// DETERMINÍSTICO desde su contenido: si dos orígenes capturan el mismo commit, se UPSERTEA, no duplica.
	scope := memory.ScopeLocal
	switch strings.ToLower(strings.TrimSpace(*scopeF)) {
	case "shared":
		scope = memory.ScopeShared
	case "local":
		scope = memory.ScopeLocal
	default:
		if cfg.Memory.TeamMode {
			scope = memory.ScopeShared
		}
	}

	// Cursor por repo: capturar VARIOS repos en la misma memoria exige un cursor por repo para que no
	// se pisen el HEAD. El hook (sin --repo) mantiene la clave global histórica.
	cursorKey := metaCaptureLastCommit
	if strings.TrimSpace(*repoF) != "" {
		cursorKey = repoCursorKey(*repoF)
	}

	// --fetch: refresca el mirror clone antes de leer (Forgejo es localhost ⇒ instantáneo). Sólo tiene
	// sentido con --repo. Best-effort: un fetch fallido no aborta la captura de lo que ya está local.
	if *fetchF && strings.TrimSpace(*repoF) != "" {
		if _, ferr := (realGit{dir: gitDir}).run("fetch", "--quiet"); ferr != nil && !hookMode {
			fmt.Fprintf(os.Stderr, "capture: git fetch falló en %s (capturo lo que haya): %v\n", gitDir, ferr)
		}
	}

	// El tope por corrida es SÓLO del hook: el modo origin-side corre en un timer, sin el
	// presupuesto de 10 s encima, y ahí cortar un lote largo sólo lo haría tardar más días.
	limite := 0
	if hookMode {
		limite = maxCommitsPorCorrida
	}

	n, err := captureCommitsKeyed(engine, realGit{dir: gitDir}, embed, detect, scope, cursorKey, limite)
	if err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		}
		return
	}
	if !hookMode {
		destino := "memoria local"
		if scope == memory.ScopeShared {
			destino = "memoria compartida (van al cerebro central)"
		}
		origen := "el workspace"
		if strings.TrimSpace(*repoF) != "" {
			origen = *repoF
		}
		fmt.Printf("Capturados %d commit(s) nuevos de %s en %s (proyecto: %s).\n", n, origen, destino, projectID)
	}
}
