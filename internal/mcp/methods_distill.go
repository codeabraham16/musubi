package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"musubi/internal/cognition"
	"musubi/internal/memory"
)

// methods_distill.go implementa musubi_distill: el DESTILADOR del acervo de diseño (pilar 'Musubi
// Renaissance'). Es el pase que vuelve el acervo INFINITO — no basta con ingerir material crudo, hay que
// DESTILARLO en conocimiento reusable, o los blobs quedan como cabos sueltos que el brief nunca surfacea
// (las tarjetas cortas pierden en similitud contra los artículos de miles de tokens).
//
// Qué hace: recorre los blobs `ingested/*` del tenant `musubi-design` que TODAVÍA no fueron destilados,
// y por cada uno el motor de cognición extrae 1-6 TARJETAS cortas, genéricas y accionables bajo
// `design-corpus/<slug>`, embebidas para búsqueda. preferCuratedSources (methods_design.go) ya prioriza
// esas tarjetas sobre los blobs, así que destilar es lo que hace que el conocimiento ingerido SE USE.
//
// Disciplina del pilar: es OFFLINE (usa el LLM, jamás en el camino caliente del recall), OPT-IN (sin
// motor configurado falla explícito, igual que musubi_ask), ADMIN (escribe en el acervo compartido) e
// IDEMPOTENTE + reanudable (marca cada blob con una arista de procedencia `derived_from`; "sin destilar"
// = blob sin esa arista entrante). Se corre de a tandas hasta drenar el backlog; re-correr no duplica.

const (
	// distillScope es el tenant del acervo (el mismo que lee musubi_design). Se destila SÓLO ahí.
	distillScope = designCorpusScope
	// distillRawPrefix es el prefijo de topic de los blobs crudos ingeridos que hay que destilar.
	distillRawPrefix = "ingested/"
	// distillCardPrefix es el prefijo de las tarjetas curadas que produce el destilador. Coincide con
	// lo que preferCuratedSources trata como CURADO (todo lo que no empieza con "ingested/").
	distillCardPrefix = "design-corpus/"
	// distillMarker es la relación de procedencia tarjeta→blob que marca un blob como ya destilado.
	distillMarker = memory.RelDerivedFrom
	// distillDefaultBatch / distillMaxBatch acotan cuántos blobs procesa una llamada. Cada blob es una
	// llamada al motor (lenta): la tanda chica mantiene la latencia manejable y se corre en bucle.
	distillDefaultBatch = 6
	distillMaxBatch     = 25
	// distillInputCap acota el texto crudo que entra al prompt (bound de costo/latencia): las lecciones
	// de un artículo suelen estar en los primeros miles de tokens.
	distillInputCap = 14000
	// distillMaxCards es el techo de tarjetas por blob (evita que un texto largo explote en decenas).
	distillMaxCards = 6
	// distillAuthor es el autor que se estampa en las tarjetas y en la arista: procedencia legible del
	// pase automático (no un humano ni "legacy").
	distillAuthor = "destilador"
	// distillTwinFloor es el piso de coseno del chequeo ANTI-GEMELO (causa raíz del loop de afilado): si
	// una tarjeta por escribir se parece a una que YA existe por encima de este umbral, NO se escribe la
	// gemela — el blob se absorbe como CORROBORACIÓN de la existente. A diferencia del afilador
	// (musubi_sharpen), acá NO hay juez LLM, así que el umbral es ALTO: sólo se saltea lo casi idéntico.
	distillTwinFloor = 0.93
)

// distillCard es una tarjeta destilada tal como la devuelve el motor: un slug temático + la lección.
type distillCard struct {
	Slug    string `json:"slug"`
	Content string `json:"content"`
}

// distillPending es una tarjeta ya lista para persistir (id determinístico + embedding precomputado
// FUERA del candado de escritura).
type distillPending struct {
	id, topic, content string
	emb                []float32
}

// distillBlobResult reporta qué pasó con UN blob de la tanda.
type distillBlobResult struct {
	BlobID  string `json:"blob_id"`
	Topic   string `json:"topic"`
	Cards   int    `json:"cards,omitempty"`
	Twins   int    `json:"twins,omitempty"`   // tarjetas que NO se escribieron por ser gemelas de una existente (anti-gemelo)
	Skipped string `json:"skipped,omitempty"` // motivo si no se destiló (motor falló, sin tarjetas, dry-run)
}

// distillReport es la respuesta de musubi_distill.
type distillReport struct {
	Distilled int                 `json:"distilled"` // blobs efectivamente destilados (con >=1 tarjeta)
	Cards     int                 `json:"cards"`     // tarjetas escritas en total
	Remaining int                 `json:"remaining"` // blobs que quedan por destilar tras esta tanda
	Motor     string              `json:"motor"`     // el modelo que destiló
	Note      string              `json:"note,omitempty"`
	Blobs     []distillBlobResult `json:"blobs"`
}

func (s *McpServer) toolDistill(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	// Escribe en el acervo COMPARTIDO (no acotado al tenant del caller): sólo admin, igual que maintain.
	if !principalFrom(ctx).isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_distill es una operación de mantenimiento del acervo: requiere un principal admin")
	}
	// Opt-in: sin motor, no hay destilación. Falla explícito (no degrada en silencio), igual que musubi_ask.
	if !cognition.Enabled(s.cognition) {
		return nil, rpcErrorf(codeInvalidParams, "cognición no disponible: musubi_distill es un pase offline con LLM y necesita un motor (cognition.provider en .musubi/config.yaml)")
	}
	var args struct {
		Limit  int  `json:"limit"`
		DryRun bool `json:"dry_run"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	rep, err := s.runDistillBatch(ctx, args.Limit, args.DryRun)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	return jsonResult(rep)
}

// runDistillBatch es el NÚCLEO del destilador, compartido por la tool musubi_distill y el auto-drain del
// scheduler (RunDistillScheduler). NO hace control de acceso ni parseo de args —eso es del caller— y asume
// que el motor de cognición está disponible. Acota su propia sección crítica —lee bajo RLock, escribe bajo
// Lock, con el LLM y el embedder AFUERA—, así que el caller NO debe tener tomado el candado del despacho.
// dryRun lista sin escribir. Sólo devuelve error ante un fallo de LECTURA del backlog; los fallos por blob
// van en el reporte (Skipped) y no abortan la tanda.
func (s *McpServer) runDistillBatch(ctx context.Context, limit int, dryRun bool) (distillReport, error) {
	batch := limit
	if batch <= 0 {
		batch = distillDefaultBatch
	}
	if batch > distillMaxBatch {
		batch = distillMaxBatch
	}

	report := distillReport{Motor: s.cognition.Name(), Blobs: []distillBlobResult{}}

	// 1) Leer los blobs sin destilar (read, bajo RLock acotado).
	var blobs []memory.ObsLite
	var readErr error
	s.withReadLock(func() {
		blobs, readErr = s.engine.ObservationsMissingRelation(distillScope, distillRawPrefix, distillMarker, batch)
	})
	if readErr != nil {
		return report, fmt.Errorf("no pude listar los blobs sin destilar: %w", readErr)
	}
	if len(blobs) == 0 {
		report.Note = "el acervo está destilado al día: no hay blobs `ingested/*` pendientes."
		return report, nil
	}

	// dry-run: informa qué se destilaría, sin tocar el motor ni escribir.
	if dryRun {
		for _, b := range blobs {
			report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Skipped: "dry_run"})
		}
		s.withReadLock(func() {
			report.Remaining, _ = s.engine.CountObservationsMissingRelation(distillScope, distillRawPrefix, distillMarker)
		})
		report.Note = fmt.Sprintf("dry-run: %d blobs listos para destilar (no se escribió nada); quedan %d en total.", len(blobs), report.Remaining)
		return report, nil
	}

	// 2) Por cada blob: destilar con el motor (SIN candado) → embeber (SIN candado) → persistir (write-lock).
	for _, b := range blobs {
		// El freno de gasto del motor se consulta ANTES de cada llamada y la CUENTA si la deja pasar.
		// Sin presupuesto se CORTA limpio (no se rechaza toda la tool): lo hecho hasta acá ya quedó
		// persistido y marcado, así que reanudar más tarde sigue justo donde quedó.
		if !s.hayPresupuestoDeMotor(ctx) {
			if s.metrics != nil {
				s.metrics.motorDenied.Add(1)
			}
			report.Note = fmt.Sprintf("presupuesto del motor agotado tras destilar %d blobs (máx %d llamadas/hora por principal); corré de nuevo más tarde para seguir.", report.Distilled, s.motorQuota.max)
			break
		}
		cards, derr := s.distillOne(ctx, b)
		if derr != nil {
			report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Skipped: derr.Error()})
			continue
		}
		if len(cards) == 0 {
			// Sin tarjetas parseables NO se marca el blob: se reintenta en la próxima corrida (un hipo
			// transitorio del motor no debe enterrar un blob bueno para siempre).
			report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Skipped: "el motor no devolvió tarjetas parseables"})
			continue
		}

		// Embeber FUERA del candado (I/O externa). El id es determinístico por (blob, slug) ⇒ UPSERT
		// idempotente: re-destilar el mismo blob no duplica tarjetas.
		toWrite := make([]distillPending, 0, len(cards))
		for _, c := range cards {
			content := s.redactIfForced(c.Content)
			id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("musubi-design:distill:"+b.ID+"|"+c.Slug)).String()
			toWrite = append(toWrite, distillPending{
				id:      id,
				topic:   distillCardPrefix + c.Slug,
				content: content,
				emb:     s.embedIfEnabled(content),
			})
		}

		// Persistir bajo write-lock EXCLUSIVO. El MARCADOR del blob (las aristas derived_from) se escribe AL
		// FINAL, recién cuando TODAS las tarjetas se guardaron OK: si una escritura falla a mitad de camino,
		// el blob queda SIN marcar y se reintenta ENTERO en la próxima corrida (idempotente por el id
		// determinístico de cada tarjeta ⇒ UPSERT, no duplica). Antes las aristas iban intercaladas con los
		// saves, así que un fallo tardío dejaba el blob marcado por una tarjeta previa y las tarjetas que
		// seguían se perdían para siempre —el blob nunca se reprocesaba— (revisión adversarial 2026-08-20;
		// withWriteLock es un mutex, no una transacción, y cada save/arista autocommitea por separado). El
		// motor y el embedder ya quedaron afuera del candado.
		var writeErr error
		var written, twins int
		var edges []memory.ObsRelation
		s.withWriteLock(func() {
			for _, p := range toWrite {
				// Anti-gemelo (causa raíz del loop de afilado): si la tarjeta por escribir se parece a una
				// que YA existe por encima de distillTwinFloor, NO se escribe una gemela — el blob se
				// absorbe como CORROBORACIÓN de la existente (arista derived_from tarjeta_existente→blob).
				// Sin embedder (emb vacío) el chequeo se saltea y degrada blando. excludeID=p.id descarta la
				// propia tarjeta (id determinístico) en un re-destilado, para que no se tome a sí misma por gemela.
				if len(p.emb) > 0 {
					twinID, _, cos, nerr := s.engine.NearestVisibleByVector(distillScope, distillCardPrefix, p.emb, p.id)
					if nerr == nil && twinID != "" && cos >= distillTwinFloor {
						edges = append(edges, memory.ObsRelation{
							SourceID: twinID, TargetID: b.ID, Relation: distillMarker,
							Status: memory.RelStatusResolved, ResolvedBy: distillAuthor,
							Reason: "el blob corrobora una tarjeta existente (gemela); no se duplicó", Confidence: 1.0,
						})
						twins++
						continue
					}
				}
				if err := s.engine.SaveObservationTypedFrom(distillScope, distillAuthor, p.id, p.topic, p.content, 1.0, "semantic", s.defaultScope(), p.emb); err != nil {
					writeErr = err
					return
				}
				edges = append(edges, memory.ObsRelation{
					SourceID: p.id, TargetID: b.ID, Relation: distillMarker,
					Status: memory.RelStatusResolved, ResolvedBy: distillAuthor,
					Reason: "tarjeta destilada del blob crudo", Confidence: 1.0,
				})
				written++
			}
			// Todas las tarjetas OK: RECIÉN AHORA se marca el blob (las aristas). Si algo falló arriba, se
			// volvió con writeErr y no se escribió NINGUNA arista ⇒ el blob sigue pendiente y se reintenta.
			for _, rel := range edges {
				if _, err := s.engine.UpsertObsRelation(rel); err != nil {
					writeErr = err
					return
				}
			}
		})
		if writeErr != nil {
			report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Skipped: "error al persistir: " + writeErr.Error()})
			continue
		}
		// Un blob cuyas tarjetas eran TODAS gemelas queda marcado (aristas de corroboración) pero no suma
		// tarjetas nuevas: se reporta como absorbido, no como destilado, y NO se reprocesa (ya tiene arista).
		if written == 0 {
			report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Twins: twins, Skipped: "todas las tarjetas ya existían (gemelas): blob absorbido como corroboración"})
			continue
		}
		report.Distilled++
		report.Cards += written
		report.Blobs = append(report.Blobs, distillBlobResult{BlobID: b.ID, Topic: b.TopicKey, Cards: written, Twins: twins})
	}

	s.withReadLock(func() {
		report.Remaining, _ = s.engine.CountObservationsMissingRelation(distillScope, distillRawPrefix, distillMarker)
	})
	if report.Note == "" {
		report.Note = fmt.Sprintf("destilé %d blobs → %d tarjetas; quedan %d por destilar.", report.Distilled, report.Cards, report.Remaining)
	}
	return report, nil
}

// distillOne destila UN blob: llama al motor con el texto crudo (acotado) y devuelve las tarjetas
// parseadas. El timeout es el mismo backstop generoso de la cognición a-demanda (askTimeout).
func (s *McpServer) distillOne(ctx context.Context, b memory.ObsLite) ([]distillCard, error) {
	text := b.Content
	if len(text) > distillInputCap {
		text = text[:distillInputCap]
	}
	user := "ARTÍCULO CRUDO (tema: " + b.TopicKey + "):\n\n" + text
	dctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	answer, err := s.cognition.Ask(dctx, distillSystemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("el motor falló: %v", err)
	}
	return parseDistillCards(answer), nil
}

// parseDistillCards extrae el array JSON de tarjetas de la respuesta del motor, tolerando el envoltorio
// ```json y prosa alrededor: recorta al primer '[' … último ']', parsea, y conserva las tarjetas con
// slug + content no vacíos. Sanea el slug a kebab-case y corta a distillMaxCards. Devuelve nil si no hay
// nada parseable (el caller lo trata como "sin tarjetas", NO como error duro: el blob se reintenta).
func parseDistillCards(answer string) []distillCard {
	i := strings.IndexByte(answer, '[')
	j := strings.LastIndexByte(answer, ']')
	if i < 0 || j <= i {
		return nil
	}
	var raw []distillCard
	if err := json.Unmarshal([]byte(answer[i:j+1]), &raw); err != nil {
		return nil
	}
	out := make([]distillCard, 0, len(raw))
	for _, c := range raw {
		slug := slugify(c.Slug)
		content := strings.TrimSpace(c.Content)
		if slug == "" || content == "" {
			continue
		}
		out = append(out, distillCard{Slug: slug, Content: content})
		if len(out) >= distillMaxCards {
			break
		}
	}
	return out
}

var (
	slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)
	// slugAccentFold baja los acentos del castellano ANTES de tirar lo no-alfanumérico, así "jerarquía"
	// no queda como "jerarqu-a". No es un fold unicode completo: cubre lo que aparece en el acervo.
	slugAccentFold = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u",
	)
)

// slugify normaliza un slug a kebab-case ascii: minúsculas, acentos folded, todo lo no-alfanumérico → '-',
// sin guiones a los bordes, tope de largo. Un slug que queda vacío tras sanear devuelve "" (la tarjeta se
// descarta). La clave de topic es una etiqueta de agrupación; el texto real vive en el content.
func slugify(s string) string {
	s = slugAccentFold.Replace(strings.ToLower(strings.TrimSpace(s)))
	s = slugStripRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

const distillSystemPrompt = `Sos el DESTILADOR del acervo de diseño de Musubi (pilar 'Musubi Renaissance'). Recibís UN artículo o transcripción CRUDO sobre diseño y extraés sus lecciones REUSABLES como TARJETAS cortas y densas: el conocimiento que un diseñador de producto senior aplicaría en OTRO proyecto, separado de la paja del texto original.

REGLAS:
- Cada tarjeta es UNA idea accionable, de 1 a 4 frases, CONCRETA: no "usá buena jerarquía" sino CÓMO se logra.
- GENÉRICA y transferible: sin la marca de ningún proyecto, sin nombres propios del artículo, sin "según el video/el autor". El conocimiento queda limpio; la identidad de marca la pone otra capa del motor.
- Entre 1 y 6 tarjetas por artículo. Si el texto casi no tiene diseño reusable, devolvé 1 sola con lo poco que haya.
- "slug": 2 a 5 palabras en kebab-case que nombren el tema (ej. "contraste-por-peso", "espaciado-4pt", "estado-vacio-util").
- "content": la lección, AUTOCONTENIDA (se entiende sin leer el artículo).

SALIDA: SÓLO un array JSON, sin ` + "```" + `json, sin prosa antes ni después:
[{"slug":"...","content":"..."}, ...]`

// distillToolEntry registra musubi_distill. Va en el central (donde vive el acervo `musubi-design` y hay
// motor de cognición); en stdio local sin motor, la tool falla explícito (opt-in). NO es readOnly:
// escribe tarjetas + aristas. Es lockSelf porque hace I/O externa (motor + embedder) por cada blob —
// sostener el candado del despacho durante minutos congelaría el servidor.
func (s *McpServer) distillToolEntry() toolEntry {
	return toolEntry{
		Tool: Tool{
			Name:        "musubi_distill",
			Description: "DESTILADOR del acervo de diseño (pilar 'Musubi Renaissance'): pase OFFLINE que convierte los artículos crudos ingeridos (blobs `ingested/*` del acervo `musubi-design`) en TARJETAS curadas y cortas (`design-corpus/*`) que el motor de diseño surfacea ANTES que los blobs. Cada blob se destila con el motor de cognición en 1-6 lecciones genéricas y reusables, embebidas para búsqueda. IDEMPOTENTE y reanudable: marca cada blob con una arista de procedencia `derived_from`, procesa de a tandas (default 6) y no reprocesa lo ya hecho — corré en bucle hasta que 'remaining' llegue a 0. Requiere admin y un motor de cognición configurado (opt-in: sin motor, falla explícito). Pasá `dry_run:true` para ver qué se destilaría sin escribir, o `limit` para el tamaño de la tanda (máx 25).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit":   {Type: "number", Description: "Cuántos blobs destilar en esta tanda (default 6, máximo 25). Cada blob es una llamada al motor; tandas grandes tardan más."},
					"dry_run": {Type: "boolean", Description: "Si es true, lista los blobs que se destilarían SIN llamar al motor ni escribir nada."},
				},
			},
		},
		handler: s.toolDistill,
		lock:    lockSelf,
	}
}
