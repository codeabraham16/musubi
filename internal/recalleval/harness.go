package recalleval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"musubi/internal/memory"
)

// Doc es un documento del corpus de evaluación: una observación con id estable.
type Doc struct {
	ID      string `json:"id"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

// Query es una consulta etiquetada: el texto que busca el agente y el conjunto de docs
// que un humano considera RELEVANTES (la verdad de referencia contra la que se mide).
type Query struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Relevant []string `json:"relevant"`
	// Note documenta POR QUÉ estos docs son relevantes (p. ej. "hueco de vocabulario
	// deploy↔despliegue": el léxico no debería encontrarlo, la semántica sí). Solo
	// informativo; no afecta el cálculo.
	Note string `json:"note,omitempty"`
}

// Fixture es el corpus + queries etiquetadas. Vive versionado en testdata/ para que la
// evaluación sea reproducible y revisable en el diff.
type Fixture struct {
	Docs    []Doc   `json:"docs"`
	Queries []Query `json:"queries"`
}

// EmbedFunc genera el vector de un texto (el StaticProvider real, o uno sintético en
// tests). nil ⇒ evaluación 100% léxica (los docs se siembran sin vector).
type EmbedFunc func(text string) ([]float32, error)

// Config es una variante de recall a evaluar. UseVector activa el recall híbrido
// (rellena QueryVector con embed(query)); requiere un EmbedFunc no-nil en Run.
type Config struct {
	Name      string
	Opts      memory.RecallOptions
	UseVector bool
}

// Scores son las métricas agregadas (promedio sobre queries con ≥1 relevante) de una
// configuración. RecallAtK y NDCGAtK están indexados por k.
type Scores struct {
	Config    string          `json:"config"`
	Queries   int             `json:"queries"`
	MRR       float64         `json:"mrr"`
	RecallAtK map[int]float64 `json:"recall_at_k"`
	NDCGAtK   map[int]float64 `json:"ndcg_at_k"`
}

// LoadFixture lee un fixture JSON del disco.
func LoadFixture(path string) (*Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx Fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, fmt.Errorf("fixture %s: %w", path, err)
	}
	if len(fx.Docs) == 0 || len(fx.Queries) == 0 {
		return nil, fmt.Errorf("fixture %s: necesita al menos 1 doc y 1 query", path)
	}
	return &fx, nil
}

// SeedEngine crea un motor de memoria en dir y guarda todos los docs del fixture. Si
// embed no es nil, cada doc lleva su embedding (activa la señal vectorial); si es nil,
// se siembra 100% léxico. El caller es dueño del engine (debe Close()).
func SeedEngine(dir string, fx *Fixture, embed EmbedFunc) (*memory.DbEngine, error) {
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		return nil, err
	}
	for _, d := range fx.Docs {
		var vec []float32
		if embed != nil {
			vec, err = embed(d.Content)
			if err != nil {
				eng.Close()
				return nil, fmt.Errorf("embed doc %s: %w", d.ID, err)
			}
		}
		if err := eng.SaveObservation(d.ID, d.Topic, d.Content, vec); err != nil {
			eng.Close()
			return nil, fmt.Errorf("guardar doc %s: %w", d.ID, err)
		}
	}
	return eng, nil
}

// rankedIDs corre un recall y devuelve solo los ids en orden de score (mejor primero).
// Fuerza un presupuesto de tokens enorme y un pool amplio para que el ranking NO se
// recorte por presupuesto: el harness mide CALIDAD DE ORDEN, no empaquetado. NoBump evita
// que un recall contamine las stats de acceso del siguiente (reproducibilidad).
func rankedIDs(ctx context.Context, eng *memory.DbEngine, query string, cfg Config, embed EmbedFunc, pool int) ([]string, error) {
	opts := cfg.Opts
	opts.NoBump = true
	opts.TokenBudget = 1 << 30
	if opts.CandidatePool < pool {
		opts.CandidatePool = pool
	}
	if cfg.UseVector {
		if embed == nil {
			return nil, fmt.Errorf("config %q usa vector pero no se pasó EmbedFunc", cfg.Name)
		}
		vec, err := embed(query)
		if err != nil {
			return nil, fmt.Errorf("embed query %q: %w", query, err)
		}
		opts.QueryVector = vec
	}
	res, err := eng.Recall(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(res.Items))
	for i, it := range res.Items {
		ids[i] = it.ID
	}
	return ids, nil
}

// Evaluate corre todas las queries del fixture bajo una configuración y agrega las
// métricas en los k pedidos. Omite del promedio las queries sin relevantes.
func Evaluate(ctx context.Context, eng *memory.DbEngine, fx *Fixture, cfg Config, embed EmbedFunc, ks []int) (Scores, error) {
	pool := len(fx.Docs) // pool = corpus entero: el recorte lo hacen las métricas @k, no el pool
	recallByK := make(map[int][]float64, len(ks))
	ndcgByK := make(map[int][]float64, len(ks))
	var rr []float64
	for _, q := range fx.Queries {
		if len(q.Relevant) == 0 {
			continue
		}
		relevant := make(map[string]bool, len(q.Relevant))
		for _, id := range q.Relevant {
			relevant[id] = true
		}
		ranked, err := rankedIDs(ctx, eng, q.Text, cfg, embed, pool)
		if err != nil {
			return Scores{}, fmt.Errorf("query %s: %w", q.ID, err)
		}
		rr = append(rr, ReciprocalRank(ranked, relevant))
		for _, k := range ks {
			recallByK[k] = append(recallByK[k], RecallAtK(ranked, relevant, k))
			ndcgByK[k] = append(ndcgByK[k], NDCGAtK(ranked, relevant, k))
		}
	}
	s := Scores{
		Config:    cfg.Name,
		Queries:   len(rr),
		MRR:       mean(rr),
		RecallAtK: make(map[int]float64, len(ks)),
		NDCGAtK:   make(map[int]float64, len(ks)),
	}
	for _, k := range ks {
		s.RecallAtK[k] = mean(recallByK[k])
		s.NDCGAtK[k] = mean(ndcgByK[k])
	}
	return s, nil
}

// Run es el orquestador: siembra un engine en dir con el fixture (embeddings si embed
// no es nil) y evalúa cada configuración sobre el MISMO corpus, devolviendo un Scores por
// config. Comparar léxico vs híbrido con el mismo embed aísla el aporte de la semántica.
func Run(ctx context.Context, dir string, fx *Fixture, embed EmbedFunc, configs []Config, ks []int) ([]Scores, error) {
	eng, err := SeedEngine(dir, fx, embed)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	out := make([]Scores, 0, len(configs))
	for _, cfg := range configs {
		s, err := Evaluate(ctx, eng, fx, cfg, embed, ks)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// FormatReport arma una tabla legible de los Scores (una fila por config), ordenada por
// los k ascendentes. Es la salida que un humano lee para decidir si la semántica gana.
func FormatReport(scores []Scores, ks []int) string {
	sorted := append([]int(nil), ks...)
	sort.Ints(sorted)
	var b strings.Builder
	fmt.Fprintf(&b, "%-22s %7s", "config", "MRR")
	for _, k := range sorted {
		fmt.Fprintf(&b, "  R@%-4d nDCG@%-2d", k, k)
	}
	b.WriteByte('\n')
	for _, s := range scores {
		fmt.Fprintf(&b, "%-22s %7.3f", s.Config, s.MRR)
		for _, k := range sorted {
			fmt.Fprintf(&b, "  %5.3f  %6.3f", s.RecallAtK[k], s.NDCGAtK[k])
		}
		b.WriteByte('\n')
	}
	return b.String()
}
