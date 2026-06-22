package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// FetchMarketplaceCatalog lee el catálogo ESTÁTICO cosechado desde una URL (el JSON que
// publica el cosechador central). Es lo que sirve al descubrimiento sin pegar a la API en
// vivo (cero rate limit). Mismo patrón que FetchCatalog: timeout por contexto, backstop
// anti-DoS de tamaño, error no-fatal para que el llamador caiga al modo live.
func FetchMarketplaceCatalog(ctx context.Context, url string) (MarketplaceCatalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: construir request del catálogo de marketplace: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "musubi/skillsource")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: GET catálogo de marketplace %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: catálogo de marketplace %s devolvió HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes))
	if err != nil {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: leer catálogo de marketplace %s: %w", url, err)
	}
	var cat MarketplaceCatalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return MarketplaceCatalog{}, fmt.Errorf("skillsource: decodificar catálogo de marketplace %s: %w", url, err)
	}
	return cat, nil
}

// FilterMarketplaceSkills filtra las skills del catálogo por la query (texto libre): una
// skill matchea si ALGÚN término de la query aparece en su nombre, descripción o id (case
// insensitive). Query vacía ⇒ todas. Preserva el orden de entrada (el catálogo ya viene
// ordenado por estrellas) y acota a limit. Es el filtrado local que reemplaza la búsqueda
// del marketplace cuando se sirve desde el catálogo estático.
func FilterMarketplaceSkills(skills []MarketplaceSkill, query string, limit int) []MarketplaceSkill {
	if limit <= 0 {
		limit = defaultMarketplaceLimit
	}
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	out := make([]MarketplaceSkill, 0, limit)
	for _, sk := range skills {
		if len(out) >= limit {
			break
		}
		if len(terms) == 0 || skillMatchesAny(sk, terms) {
			out = append(out, sk)
		}
	}
	return out
}

// skillMatchesAny indica si algún término aparece en nombre+descripción+id (lowercased).
func skillMatchesAny(sk MarketplaceSkill, terms []string) bool {
	hay := strings.ToLower(sk.Name + " " + sk.Description + " " + sk.ID)
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}
