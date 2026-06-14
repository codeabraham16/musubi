// Package skillsource — merge.go implementa la fusión determinística de catálogos.
// Esta función es pura: no realiza IO, no modifica las entradas de entrada.
package skillsource

import "sort"

// MergeCatalogs fusiona incoming en base, deduplicando por ID (incoming gana).
// Devuelve el catálogo fusionado con Entries ordenadas por ID, y la lista de
// IDs sobreescritos (collisions) en orden ascendente. CatalogVersion se toma de
// base, salvo que base esté vacía (sin entradas y versión 0), en cuyo caso se
// usa la versión de incoming. Las entradas de entrada no se modifican.
func MergeCatalogs(base, incoming Catalog) (merged Catalog, collisions []string) {
	// Construir mapa id→entrada desde base. Usamos un mapa booleano separado
	// para rastrear qué IDs provienen de base (para detectar colisiones).
	fromBase := make(map[string]bool, len(base.Entries))
	byID := make(map[string]CatalogEntry, len(base.Entries)+len(incoming.Entries))

	for _, e := range base.Entries {
		byID[e.ID] = e
		fromBase[e.ID] = true
	}

	// Escanear incoming secuencialmente; last-wins para duplicados intra-incoming.
	// Si el ID ya existe Y vino de base → es colisión.
	for _, e := range incoming.Entries {
		if fromBase[e.ID] {
			// Colisión con base: incoming gana, registrar.
			collisions = append(collisions, e.ID)
			fromBase[e.ID] = false // evitar doble registro si incoming tiene el mismo ID dos veces
		}
		byID[e.ID] = e
	}

	// Materializar entries desde el mapa.
	entries := make([]CatalogEntry, 0, len(byID))
	for _, e := range byID {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	// Ordenar collisions de forma determinista.
	sort.Strings(collisions)

	// Reconciliación de versión: base gana salvo que base esté vacía (sin
	// entradas y versión 0), en cuyo caso se usa la versión de incoming.
	version := base.CatalogVersion
	if base.CatalogVersion == 0 && len(base.Entries) == 0 {
		version = incoming.CatalogVersion
	}

	return Catalog{
		CatalogVersion: version,
		Entries:        entries,
	}, collisions
}
