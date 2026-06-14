package skillsource

import (
	"fmt"
	"regexp"
)

// slugPattern valida el id de una entrada del catálogo.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// KnownStacks son los ecosistemas que el detector de Musubi reconoce.
// Deben coincidir con detector.StackResult.Ecosystem (capitalización exacta);
// si no coinciden, el gate de aplicabilidad nunca matchea.
var KnownStacks = map[string]bool{
	"Go": true, "Node.js": true, "Python": true, "Rust": true, "Docker": true,
	"Java": true, "Ruby": true, "PHP": true, ".NET": true,
	"C/C++": true, "Dart": true, "Elixir": true,
}

// ValidateCatalog valida un catálogo de forma determinística: campos requeridos,
// formato slug del id, ids únicos y stacks conocidos. Devuelve la lista de
// errores encontrados (vacía si el catálogo es válido).
func ValidateCatalog(cat Catalog) []error {
	var errs []error

	if cat.CatalogVersion < 1 {
		errs = append(errs, fmt.Errorf("catalog_version debe ser >= 1 (es %d)", cat.CatalogVersion))
	}

	seen := make(map[string]bool)
	for i, e := range cat.Entries {
		ref := e.ID
		if ref == "" {
			ref = fmt.Sprintf("entrada #%d", i)
		}
		if !slugPattern.MatchString(e.ID) {
			errs = append(errs, fmt.Errorf("%s: id inválido (debe ser slug [a-z0-9-])", ref))
		}
		if seen[e.ID] {
			errs = append(errs, fmt.Errorf("%s: id duplicado", ref))
		}
		seen[e.ID] = true

		if e.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name vacío", ref))
		}
		if e.Description == "" {
			errs = append(errs, fmt.Errorf("%s: description vacía", ref))
		}
		if len(e.Stacks) == 0 {
			errs = append(errs, fmt.Errorf("%s: stacks vacío", ref))
		}
		for _, s := range e.Stacks {
			if !KnownStacks[s] {
				errs = append(errs, fmt.Errorf("%s: stack desconocido %q (¿capitalización?)", ref, s))
			}
		}
		if len(e.Triggers) == 0 {
			errs = append(errs, fmt.Errorf("%s: triggers vacío", ref))
		}
		if e.RulesURL == "" {
			errs = append(errs, fmt.Errorf("%s: rules_url vacío", ref))
		}
		if e.Excerpt == "" {
			errs = append(errs, fmt.Errorf("%s: excerpt vacío", ref))
		}
	}

	return errs
}
