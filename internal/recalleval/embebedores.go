package recalleval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// embebedores.go es lo que el banco necesita para comparar EMBEBEDORES entre sí, que no es lo que
// hace Run: Run compara configuraciones del ranker con un embebedor fijo. Acá se re-embebe el
// corpus por el camino de producción, se embeben las consultas con el prefijo que pida cada modelo
// y se cronometran las dos cosas.
//
// POR QUÉ LOS PREFIJOS VIVEN ACÁ Y NO EN MUSUBI. Hay modelos que piden marcar la consulta (una
// instrucción, «query: ») y dejar el documento pelado, o marcar los dos. Musubi HOY no lo soporta:
// embedding.Provider tiene un solo Embed(texto), y un texto se embebe igual se esté indexando o
// consultando. Medir uno de esos modelos sin su prefijo lo mide en condiciones que su model card
// desaconseja, y la comparación queda sesgada en su contra. Por eso el banco lo agrega por fuera, y
// quien mide declara las dos variantes: con el prefijo (lo que el modelo pide) y sin él (lo que
// Musubi haría hoy si lo adoptara tal cual).

// Cronometro junta las duraciones de las llamadas a un embebedor. Es seguro para uso concurrente.
type Cronometro struct {
	mu         sync.Mutex
	durs       []time.Duration
	textos     int
	caracteres int
}

// Anotar registra UNA llamada que embebió `textos` textos con `caracteres` bytes en total.
func (c *Cronometro) Anotar(d time.Duration, textos, caracteres int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.durs = append(c.durs, d)
	c.textos += textos
	c.caracteres += caracteres
}

// ResumenLatencia resume un Cronometro. Media, P50 y P95 son POR LLAMADA; TotalMs es la suma.
type ResumenLatencia struct {
	Llamadas   int     `json:"llamadas"`
	Textos     int     `json:"textos"`
	Caracteres int     `json:"caracteres"`
	TotalMs    float64 `json:"total_ms"`
	MediaMs    float64 `json:"media_ms"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
}

// Resumen calcula el resumen con percentiles por rango más cercano (sin interpolar: con pocas
// llamadas, un percentil interpolado inventa una duración que ninguna llamada tuvo).
func (c *Cronometro) Resumen() ResumenLatencia {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := ResumenLatencia{Llamadas: len(c.durs), Textos: c.textos, Caracteres: c.caracteres}
	if len(c.durs) == 0 {
		return r
	}
	ms := make([]float64, len(c.durs))
	for i, d := range c.durs {
		ms[i] = float64(d) / float64(time.Millisecond)
		r.TotalMs += ms[i]
	}
	sort.Float64s(ms)
	rango := func(p float64) float64 {
		i := int(math.Ceil(p*float64(len(ms)))) - 1
		return ms[max(0, min(i, len(ms)-1))]
	}
	r.MediaMs = r.TotalMs / float64(len(ms))
	r.P50Ms = rango(0.50)
	r.P95Ms = rango(0.95)
	return r
}

// EmbedDeConsultas arma el EmbedFunc de las CONSULTAS: antepone el prefijo, embebe por el Provider
// y cronometra.
//
// Memoiza por texto porque Evaluate corre la misma consulta una vez por config, y lo que se quiere
// medir es cuánto cuesta embeber una consulta, no cuánto cuesta leer un mapa: sólo la primera
// llamada de cada texto entra al cronómetro. crono puede ser nil.
func EmbedDeConsultas(ctx context.Context, prov embedding.Provider, prefijo string, crono *Cronometro) EmbedFunc {
	var mu sync.Mutex
	memo := make(map[string][]float32)
	return func(texto string) ([]float32, error) {
		mu.Lock()
		v, ok := memo[texto]
		mu.Unlock()
		if ok {
			return v, nil
		}
		t0 := time.Now()
		v, err := prov.Embed(ctx, prefijo+texto)
		if err != nil {
			return nil, err
		}
		if crono != nil {
			crono.Anotar(time.Since(t0), 1, len(prefijo)+len(texto))
		}
		mu.Lock()
		memo[texto] = v
		mu.Unlock()
		return v, nil
	}
}

// LoteDeDocumentos arma el embebedor por LOTES de los DOCUMENTOS con la forma que pide
// DbEngine.EmbedBackfill: antepone el prefijo a cada texto y pide el lote por embedding.EmbedBatch,
// que es el único punto por el que producción pide lotes (y el que verifica que vuelvan tantos
// vectores como textos). crono puede ser nil.
func LoteDeDocumentos(ctx context.Context, prov embedding.Provider, prefijo string, crono *Cronometro) func([]string) ([][]float32, error) {
	return func(textos []string) ([][]float32, error) {
		con := make([]string, len(textos))
		caracteres := 0
		for i, x := range textos {
			con[i] = prefijo + x
			caracteres += len(con[i])
		}
		t0 := time.Now()
		vecs, err := embedding.EmbedBatch(ctx, prov, con)
		if err != nil {
			return nil, err
		}
		if crono != nil {
			crono.Anotar(time.Since(t0), len(textos), caracteres)
		}
		return vecs, nil
	}
}

// Reembebido es lo que costó re-embeber el corpus de un fixture.
type Reembebido struct {
	Docs      int     `json:"docs"`      // documentos del fixture
	Sembrados int     `json:"sembrados"` // los que el motor aceptó (SeedEngine saltea los históricos que no re-entran)
	Embebidos int     `json:"embebidos"`
	Fallidos  int     `json:"fallidos"`
	Salteados int     `json:"salteados"`
	Segundos  float64 `json:"segundos"` // de pared: lotes + escritura + reconstrucción del índice
}

// SembrarYReembeber siembra el fixture SIN vectores y después lo re-embebe con
// DbEngine.EmbedBackfill, que es el camino por el que producción re-embebe una base entera: lotes
// del tamaño de producción, reintento de a uno cuando un lote falla y el índice vectorial
// reconstruido una sola vez al final. Los vectores quedan con la procedencia ModeloDelBanco, como
// los de SeedEngine, y el motor leyendo con ella.
//
// Sembrar con vector (SeedEngine con un EmbedFunc) daría vectores equivalentes pero pedidos de a
// uno, y ése no es el costo que paga quien cambia de modelo: el tiempo que devuelve esta función sí
// lo es. El caller es dueño del motor (Close).
func SembrarYReembeber(dir string, fx *Fixture, lote func([]string) ([][]float32, error)) (*memory.DbEngine, Reembebido, error) {
	eng, err := SeedEngine(dir, fx, nil)
	if err != nil {
		return nil, Reembebido{}, err
	}
	eng.SetVectorModelID(ModeloDelBanco)
	t0 := time.Now()
	res, err := eng.EmbedBackfill(lote)
	r := Reembebido{
		Docs:      len(fx.Docs),
		Sembrados: res.Scanned,
		Embebidos: res.Embedded,
		Fallidos:  res.Failed,
		Salteados: res.Skipped,
		Segundos:  time.Since(t0).Seconds(),
	}
	if err != nil {
		eng.Close()
		return nil, r, fmt.Errorf("re-embeber el corpus: %w", err)
	}
	return eng, r, nil
}
