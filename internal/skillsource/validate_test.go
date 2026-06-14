package skillsource

import (
	"strings"
	"testing"
)

func entradaValida() CatalogEntry {
	return CatalogEntry{
		ID:       "go-effective",
		Name:     "Go — Effective Go",
		Stacks:   []string{"Go"},
		Triggers: []string{"*.go"},
		RulesURL: "https://go.dev/doc/effective_go",
		Excerpt:  "Manejá errores explícitamente.",
		// Description se completa abajo donde se necesita.
		Description: "Convenciones de Go.",
	}
}

func TestValidateCatalogValido(t *testing.T) {
	cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{entradaValida()}}
	if errs := ValidateCatalog(cat); len(errs) != 0 {
		t.Fatalf("esperaba 0 errores, obtuve %v", errs)
	}
}

func TestValidateCatalogIdDuplicado(t *testing.T) {
	e := entradaValida()
	cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e, e}}
	if errs := ValidateCatalog(cat); len(errs) == 0 {
		t.Fatal("esperaba error por id duplicado")
	}
}

func TestValidateCatalogStackDesconocido(t *testing.T) {
	e := entradaValida()
	e.Stacks = []string{"golang"} // capitalización/valor incorrecto
	cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e}}
	if errs := ValidateCatalog(cat); len(errs) == 0 {
		t.Fatal("esperaba error por stack desconocido")
	}
}

func TestValidateCatalogExcerptLargoEnRunas(t *testing.T) {
	// El esquema enforce maxLength: 600, contado en CARACTERES (runas Unicode),
	// no en bytes. Estos casos prueban que el validador Go cuenta runas.
	t.Run("exactamente 600 runas ascii es válido", func(t *testing.T) {
		e := entradaValida()
		e.Excerpt = strings.Repeat("a", 600)
		cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e}}
		if errs := ValidateCatalog(cat); len(errs) != 0 {
			t.Fatalf("esperaba 0 errores con 600 runas, obtuve %v", errs)
		}
	})

	t.Run("601 runas es error", func(t *testing.T) {
		e := entradaValida()
		e.Excerpt = strings.Repeat("a", 601)
		cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e}}
		if errs := ValidateCatalog(cat); len(errs) == 0 {
			t.Fatal("esperaba error por excerpt de 601 runas")
		}
	})

	t.Run("600 runas multibyte es válido (cuenta runas, no bytes)", func(t *testing.T) {
		// 600 'é' = 1200 bytes en UTF-8. Si el validador usara len() (bytes),
		// rechazaría falsamente esta entrada válida.
		e := entradaValida()
		e.Excerpt = strings.Repeat("é", 600)
		cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e}}
		if errs := ValidateCatalog(cat); len(errs) != 0 {
			t.Fatalf("esperaba 0 errores con 600 runas multibyte, obtuve %v", errs)
		}
	})
}

func TestValidateCatalogCamposFaltantes(t *testing.T) {
	cases := map[string]func(*CatalogEntry){
		"sin id":       func(e *CatalogEntry) { e.ID = "" },
		"id no slug":   func(e *CatalogEntry) { e.ID = "Go Effective!" },
		"sin name":     func(e *CatalogEntry) { e.Name = "" },
		"sin triggers": func(e *CatalogEntry) { e.Triggers = nil },
		"sin rulesurl": func(e *CatalogEntry) { e.RulesURL = "" },
		"sin excerpt":  func(e *CatalogEntry) { e.Excerpt = "" },
		"sin stacks":   func(e *CatalogEntry) { e.Stacks = nil },
	}
	for nombre, mut := range cases {
		t.Run(nombre, func(t *testing.T) {
			e := entradaValida()
			mut(&e)
			cat := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{e}}
			if errs := ValidateCatalog(cat); len(errs) == 0 {
				t.Fatalf("esperaba error para caso %q", nombre)
			}
		})
	}
}
