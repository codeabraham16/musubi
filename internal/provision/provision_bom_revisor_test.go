package provision

import "testing"

// Un BOM UTF-8 al principio (PowerShell 5.1 lo escribe con -Encoding utf8, y yaml.v3 lo acepta)
// esconde la cabecera de la PRIMERA línea: el editor no ve ese bloque, agrega otro al final, y la
// clave queda duplicada. Falla cerrado, pero niega el alta de un config válido.
func TestProvisionConBOMEncuentraLosBloques(t *testing.T) {
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	for _, contenido := range []string{
		bom + `sync:
  enabled: false
memory:
  recall_token_budget: 400
`,
		bom + `memory:
  recall_token_budget: 400
`,
	} {
		r, got := provisionar(t, contenido)
		if r.Status != StatusDone {
			t.Errorf("%q: un BOM no puede hacer que el alta se niegue; estado %q: %s", contenido, r.Status, r.Detail)
			continue
		}
		if cfg := cargarDe(t, got); !cfg.Sync.Enabled || !cfg.Memory.TeamMode || cfg.Memory.RecallTokenBudget != 400 {
			t.Errorf("quedó mal:\n%s", got)
		}
	}
}
