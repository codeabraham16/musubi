package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// marketplace.go implementa el DESCUBRIMIENTO de Agent Skills desde un marketplace
// externo compatible con SKILL.md (por defecto skillsmp.com, ~1.7M skills indexadas de
// GitHub público). Es deliberadamente un canal SEPARADO del catálogo curado de Musubi:
//
//   - NO se mergea al gate de aplicabilidad (Hermes) ni al modelo de skills local. El
//     marketplace no expone triggers/capabilities, solo metadatos + estrellas, así que no
//     se puede filtrar con la misma precisión; Musubi aporta la pieza que al marketplace
//     le falta —saber el stack del proyecto— construyendo la query desde el stack detectado.
//   - Es SOLO descubrimiento: devuelve metadatos y el githubUrl de cada skill para que el
//     usuario los REVISE e instale por su cuenta. Musubi nunca baja, ejecuta ni instala el
//     SKILL.md (contenido no confiable de GitHub arbitrario: el propio marketplace avisa
//     "revisá el código antes de instalar"). Esto preserva el modelo de confianza.
//   - Opt-in y con degradación graciosa: si está deshabilitado o la red cae, el llamador
//     degrada a una guía textual, nunca a un error fatal (invariante local-first).

// defaultMarketplaceURL es el host del marketplace de Agent Skills por defecto.
const defaultMarketplaceURL = "https://skillsmp.com"

// marketplaceSearchPath es el endpoint REST de búsqueda de skills del marketplace.
const marketplaceSearchPath = "/api/v1/skills/search"

// maxMarketplaceLimit es el tope de resultados por página que acepta el marketplace.
const maxMarketplaceLimit = 100

// defaultMarketplaceLimit es la cantidad de resultados pedida cuando no se especifica.
const defaultMarketplaceLimit = 20

// MarketplaceSkill es una Agent Skill descubierta en el marketplace. Los nombres de campo
// JSON coinciden con la respuesta de skillsmp.com (camelCase). Es metadatos puros: NO
// incluye el contenido del SKILL.md (eso vive en githubUrl, para que el usuario lo revise).
type MarketplaceSkill struct {
	// ID es el identificador del marketplace (deriva del path del SKILL.md en su repo).
	ID string `json:"id"`
	// Name es el nombre legible de la skill.
	Name string `json:"name"`
	// Author es el dueño del repositorio de GitHub que la publica.
	Author string `json:"author"`
	// Description describe la skill; suele incluir pistas de aplicabilidad en prosa
	// (ej. "Use when optimizing Go data structures...").
	Description string `json:"description"`
	// GithubURL apunta al SKILL.md en GitHub: la fuente a revisar antes de instalar.
	GithubURL string `json:"githubUrl"`
	// SkillURL es la página de la skill en el marketplace.
	SkillURL string `json:"skillUrl"`
	// Stars es la cantidad de estrellas del repo (señal de popularidad, no de calidad).
	Stars int `json:"stars"`
	// UpdatedAt es el timestamp Unix (string) de la última actualización.
	UpdatedAt string `json:"updatedAt"`
}

// marketplaceResponse refleja el sobre JSON del endpoint de búsqueda del marketplace.
type marketplaceResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Skills     []MarketplaceSkill `json:"skills"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	} `json:"data"`
}

// FetchMarketplaceSkills consulta el endpoint de búsqueda del marketplace y devuelve las
// skills que matchean query, ordenadas por estrellas (las más populares primero). baseURL
// es el host (ej. https://skillsmp.com); si apiKey no está vacío, se envía como
// Authorization: Bearer (sube el rate limit), si no se usa el tier anónimo. limit se acota
// a [1, 100]. Devuelve error no-fatal en: red caída, HTTP ≠ 200, JSON inválido o sobre con
// success=false. El llamador decide cómo degradar.
func FetchMarketplaceSkills(ctx context.Context, baseURL, apiKey, query string, limit int) ([]MarketplaceSkill, error) {
	if baseURL == "" {
		baseURL = defaultMarketplaceURL
	}
	if limit <= 0 {
		limit = defaultMarketplaceLimit
	}
	if limit > maxMarketplaceLimit {
		limit = maxMarketplaceLimit
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("skillsource: marketplace URL inválida %q: %w", baseURL, err)
	}
	u.Path = marketplaceSearchPath
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("sortBy", "stars") // priorizar las más populares en descubrimiento
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("skillsource: construir request del marketplace: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "musubi/skillsource")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillsource: GET marketplace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skillsource: el marketplace devolvió HTTP %d", resp.StatusCode)
	}

	// Mismo backstop anti-DoS que el catálogo: tope de tamaño del body.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes))
	if err != nil {
		return nil, fmt.Errorf("skillsource: leer respuesta del marketplace: %w", err)
	}

	var mr marketplaceResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, fmt.Errorf("skillsource: decodificar respuesta del marketplace: %w", err)
	}
	if !mr.Success {
		return nil, fmt.Errorf("skillsource: el marketplace respondió success=false")
	}

	// Omitir entradas sin id o sin enlace de revisión (githubUrl): un descubrimiento sin
	// fuente que revisar no le sirve al usuario.
	out := mr.Data.Skills[:0]
	for _, sk := range mr.Data.Skills {
		if sk.ID == "" || sk.GithubURL == "" {
			continue
		}
		out = append(out, sk)
	}
	return out, nil
}
