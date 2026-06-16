// Package skillsource implementa la obtención y filtrado de skills desde un
// catálogo remoto. Proporciona tipos para el catálogo, funciones de fetch HTTP
// y el gate de aplicabilidad. No tiene conocimiento de MCP ni de memory.
package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"musubi/internal/detector"
	"musubi/internal/logx"
)

// defaultMaxCandidates es la cantidad máxima de candidatos retornados cuando
// maxCandidates es ≤ 0 en FilterCatalog.
const defaultMaxCandidates = 20

// catalogVersionSoportada es la versión de catálogo que este código soporta plenamente.
const catalogVersionSoportada = 1

// maxCatalogBytes es el tope de tamaño del catálogo remoto (backstop anti-DoS).
const maxCatalogBytes = 32 << 20

// CatalogEntry representa una entrada del catálogo de skills.
type CatalogEntry struct {
	// ID es el identificador único de la skill (slug: [a-z0-9][a-z0-9-]{0,63}).
	ID string `json:"id"`
	// Name es el nombre legible de la skill.
	Name string `json:"name"`
	// Description describe brevemente la skill.
	Description string `json:"description"`
	// Stacks son los ecosistemas compatibles (ej. "Go", "Node.js"). Deben coincidir
	// exactamente con los valores Ecosystem de detector.StackResult.
	Stacks []string `json:"stacks"`
	// Deps son dependencias del proyecto requeridas (al menos una debe estar presente).
	// Si está vacío, la skill aplica a nivel de ecosistema sin dep específica.
	Deps []string `json:"deps"`
	// Triggers son patrones glob que deben coincidir con algún archivo del proyecto.
	Triggers []string `json:"triggers"`
	// Capabilities son herramientas que deben estar en PATH (ej. "go", "cargo").
	Capabilities []string `json:"capabilities"`
	// Tags son etiquetas informativas de la skill.
	Tags []string `json:"tags"`
	// RulesURL es la URL del archivo de reglas completo (fetching delegado al cliente).
	RulesURL string `json:"rules_url"`
	// Excerpt son los primeros ~200 caracteres de las reglas, devueltos sin network call.
	Excerpt string `json:"excerpt"`
	// Source identifica el origen del catálogo (ej. "musubi-catalog-v1").
	Source string `json:"source"`
}

// Catalog es el objeto raíz del índice de catálogo JSON.
type Catalog struct {
	// CatalogVersion identifica la versión del esquema del catálogo.
	CatalogVersion int `json:"catalog_version"`
	// Entries contiene las entradas válidas del catálogo.
	Entries []CatalogEntry `json:"entries"`
}

// ApplicabilityEvidence contiene la evidencia del gate de aplicabilidad.
type ApplicabilityEvidence struct {
	// MatchedStack es el ecosistema detectado que coincidió con la entrada.
	MatchedStack string
	// MatchedDeps son las deps del proyecto que coincidieron con las requeridas.
	MatchedDeps []string
	// MatchedFileCount es la cantidad de archivos que coincidieron con algún trigger.
	MatchedFileCount int
	// MissingCaps son las capabilities declaradas que no se encontraron en PATH.
	MissingCaps []string
}

// Candidate agrupa una entrada del catálogo con la evidencia de aplicabilidad.
type Candidate struct {
	// Entry es la entrada del catálogo.
	Entry CatalogEntry
	// Evidence contiene la evidencia del gate.
	Evidence ApplicabilityEvidence
}

// FetchCatalog realiza un GET HTTP al url dado usando el contexto ctx para timeout.
// Decodifica el JSON del catálogo y valida la versión. Entradas malformadas se omiten
// con logx.Warn; las válidas se devuelven en best-effort.
// Devuelve error no-fatal en: red caída, código HTTP ≠ 200, JSON inválido.
// El llamador decide cómo degradar ante un error.
func FetchCatalog(ctx context.Context, url string) (Catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Catalog{}, fmt.Errorf("skillsource: construir request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "musubi/skillsource")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Catalog{}, fmt.Errorf("skillsource: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("skillsource: GET %s devolvió HTTP %d", url, resp.StatusCode)
	}

	// Tope de tamaño: backstop ante un endpoint que intente agotar memoria.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes))
	if err != nil {
		return Catalog{}, fmt.Errorf("skillsource: leer body de %s: %w", url, err)
	}

	var cat Catalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return Catalog{}, fmt.Errorf("skillsource: decodificar catálogo de %s: %w", url, err)
	}

	// Advertir si la versión del catálogo no es la soportada, pero parsear de todas formas.
	if cat.CatalogVersion != catalogVersionSoportada {
		logx.Warn("skillsource: versión de catálogo desconocida, se parsea en best-effort",
			"url", url, "version", cat.CatalogVersion, "soportada", catalogVersionSoportada)
	}

	// Filtrar entradas malformadas (sin id).
	validas := cat.Entries[:0]
	for _, e := range cat.Entries {
		if e.ID == "" {
			logx.Warn("skillsource: entrada sin id omitida", "name", e.Name)
			continue
		}
		validas = append(validas, e)
	}
	cat.Entries = validas

	return cat, nil
}

// FilterCatalog aplica el gate de aplicabilidad sobre todas las entradas del catálogo
// y devuelve los candidatos aplicables, limitados a maxCandidates (si ≤ 0 usa defaultMaxCandidates).
func FilterCatalog(cat Catalog, root string, deps map[string][]string, stacks []detector.StackResult, maxCandidates int) []Candidate {
	if maxCandidates <= 0 {
		maxCandidates = defaultMaxCandidates
	}

	var candidatos []Candidate
	for _, entrada := range cat.Entries {
		if len(candidatos) >= maxCandidates {
			break
		}
		if ok, ev := IsApplicable(entrada, root, deps, stacks); ok {
			candidatos = append(candidatos, Candidate{
				Entry:    entrada,
				Evidence: ev,
			})
		}
	}
	return candidatos
}
