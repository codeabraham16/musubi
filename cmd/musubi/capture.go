package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
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
	CommitsSince(last string) ([]commit, error)
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

func (g realGit) CommitsSince(last string) ([]commit, error) {
	// Separadores de control para un parseo robusto: %x1e entre commits, %x1f entre campos.
	args := []string{"log", "--no-color", "--no-merges", "--reverse", "--name-only",
		"--format=%x1e%H%x1f%s%x1f%b%x1f"}
	if strings.TrimSpace(last) != "" {
		args = append(args, last+"..HEAD")
	} else {
		args = append(args, "-1", "HEAD")
	}
	out, err := g.run(args...)
	if err != nil {
		// El rango last..HEAD puede fallar si `last` ya no existe (rebase/force-push): caer a
		// capturar sólo el HEAD actual en vez de romper.
		if strings.TrimSpace(last) != "" {
			return g.CommitsSince("")
		}
		return nil, err
	}
	return parseCommits(out), nil
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

// captureCommits es el core testeable: captura los commits nuevos desde el último HEAD guardado
// como observaciones LOCALES (nunca shared: C3 no debe filtrar un secreto de un diff; compartir
// pasa por promote, que C2 redacta). Si embed no es nil, cada commit se guarda CON su embedding
// (participa del recall semántico); si es nil, guardado léxico. Devuelve cuántas guardó. No-op
// silencioso si no es repo git o no hay commits nuevos.
// detectFunc corre la detección de relaciones sobre una observación recién guardada. Se inyecta
// (como embed) para que el core siga testeable sin engine real. nil = sin detección.
type detectFunc func(obsID string)

func captureCommits(store captureStore, git gitLog, embed embedFunc, detect detectFunc) (int, error) {
	head, err := git.Head()
	if err != nil || head == "" {
		return 0, nil
	}
	last, _, _ := store.GetMeta(metaCaptureLastCommit)
	if strings.TrimSpace(last) == head {
		return 0, nil
	}
	commits, err := git.CommitsSince(last)
	if err != nil {
		return 0, err
	}
	saved := 0
	for _, c := range commits {
		memType, importance, skip := classifyCommit(c.Subject)
		if skip {
			continue
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
			return saved, err
		}
		if err := store.SaveObservationTyped(id, "git-commit", content, importance, memType, memory.ScopeLocal, vec); err != nil {
			return saved, err
		}
		if existed {
			continue // gemelo del squash: se actualizó lo existente, no hay memoria nueva
		}
		// Gate de novedad (M4): marcar el commit que duplica algo ya guardado. Sólo sobre memoria
		// REALMENTE nueva: un UPSERT no crea observación que relacionar. El detect corre en modo
		// DetectOnly ⇒ jamás auto-oculta un commit anterior.
		if detect != nil {
			detect(id)
		}
		saved++
	}
	_ = store.SetMeta(metaCaptureLastCommit, head)
	return saved, nil
}

// runCapture implementa `musubi capture [--hook-mode]`: en el hook Stop es silencioso; en modo
// normal imprime un resumen. Best-effort: cualquier fallo (sin repo, sin memoria) no rompe el turno.
func runCapture(args []string) {
	hookMode := false
	for _, a := range args {
		if a == "--hook-mode" {
			hookMode = true
		}
	}
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
				DetectOnly:           true,
			}); derr != nil && !hookMode {
				fmt.Fprintf(os.Stderr, "capture: detección de duplicados falló (el commit se guardó igual): %v\n", derr)
			}
		}
	}

	n, err := captureCommits(engine, realGit{dir: root}, embed, detect)
	if err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		}
		return
	}
	if !hookMode {
		fmt.Printf("Capturados %d commit(s) nuevos en memoria local.\n", n)
	}
}
