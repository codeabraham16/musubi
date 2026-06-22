package skillsource

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"musubi/internal/logx"
)

// harvest.go implementa el COSECHADOR del marketplace (Track 8 / T8.3): consulta el
// marketplace por un conjunto de "seeds" (stacks/keywords), deduplica y cura un
// subconjunto, y produce un catálogo ESTÁTICO. La idea del trayecto: en vez de que cada
// usuario pegue a la API en vivo (y choque con el rate limit), un cosechador central corre
// de vez en cuando y publica este JSON; el descubrimiento lo lee de un archivo (cero rate
// limit). skillsmp es solo el índice de DESCUBRIMIENTO; no se mirrorea su 1.7M, se cura un
// subconjunto por relevancia (seeds) y popularidad (estrellas).

// marketplaceCatalogVersion es la versión del esquema del catálogo cosechado.
const marketplaceCatalogVersion = 1

// MarketplaceCatalog es el catálogo cosechado de Agent Skills: un subconjunto curado y
// deduplicado del marketplace, servible desde un archivo estático.
type MarketplaceCatalog struct {
	// Version identifica el esquema del catálogo cosechado.
	Version int `json:"version"`
	// Generated es el instante de la cosecha (RFC3339). Lo setea el caller (CLI), no la
	// función de cosecha, para que el núcleo sea determinista y testeable sin reloj.
	Generated string `json:"generated,omitempty"`
	// Seeds son las queries (stacks/keywords) que efectivamente produjeron resultados.
	Seeds []string `json:"seeds"`
	// Skills son las skills curadas, ordenadas por estrellas desc (desempate por id).
	Skills []MarketplaceSkill `json:"skills"`
}

// MarketplaceFetchFunc obtiene skills del marketplace para una query. Es inyectable para
// testear la cosecha sin red; en producción es un wrapper de FetchMarketplaceSkills con la
// baseURL y la API key ya fijadas.
type MarketplaceFetchFunc func(ctx context.Context, query string, limit int) ([]MarketplaceSkill, error)

// HarvestMarketplace cosecha skills para cada seed y arma un catálogo curado: deduplica por
// id (gana la de más estrellas), descarta las de menos de minStars y ordena por estrellas
// desc. perSeed acota cuántas pedir por seed (≤0 ⇒ 50). Best-effort: si una seed falla, se
// omite con warn y la cosecha sigue (una caída parcial no aborta todo). NO setea Generated.
func HarvestMarketplace(ctx context.Context, fetch MarketplaceFetchFunc, seeds []string, perSeed, minStars int) (MarketplaceCatalog, error) {
	if fetch == nil {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: HarvestMarketplace requiere un fetch no nulo")
	}
	if perSeed <= 0 {
		perSeed = 50
	}

	byID := make(map[string]MarketplaceSkill)
	var usedSeeds []string
	for _, seed := range seeds {
		seed = strings.TrimSpace(seed)
		if seed == "" {
			continue
		}
		skills, err := fetch(ctx, seed, perSeed)
		if err != nil {
			logx.Warn("skillsource: seed omitida en la cosecha", "seed", seed, "error", err)
			continue
		}
		usedSeeds = append(usedSeeds, seed)
		for _, sk := range skills {
			if sk.Stars < minStars {
				continue
			}
			// Dedup entre seeds: una misma skill puede aparecer en varias; gana la de más
			// estrellas (o la primera si empatan, da igual: mismo id, mismos datos).
			if prev, ok := byID[sk.ID]; !ok || sk.Stars > prev.Stars {
				byID[sk.ID] = sk
			}
		}
	}

	out := make([]MarketplaceSkill, 0, len(byID))
	for _, sk := range byID {
		out = append(out, sk)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stars != out[j].Stars {
			return out[i].Stars > out[j].Stars
		}
		return out[i].ID < out[j].ID // desempate estable: salida determinista
	})

	return MarketplaceCatalog{
		Version: marketplaceCatalogVersion,
		Seeds:   usedSeeds,
		Skills:  out,
	}, nil
}
