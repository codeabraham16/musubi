//go:build sonda

package mcp

// sonda_recall_test.go — cuánto pesa la respuesta de musubi_recall sobre una COPIA de la base real,
// antes y después del techo de la respuesta entera (RecallOptions.PresupuestoDeRespuesta).
//
//	MUSUBI_SONDA_RECALL_DIR=<dir con .musubi/memory.db copiada> \
//	  go test -tags sonda ./internal/mcp -run TestSondaRecallLiviano -v -count=1
//
// La base se abre en SÓLO LECTURA (query_only): ni el bump de accesos la toca, así que el antes y el
// después ven exactamente el mismo estado. «Antes» es el mismo recall con la opción apagada y el
// default viejo de 400; «después» es el handler real, el que recibe el agente.
//
// Falla sólo por lo que el cambio promete: que lo de después sea un PREFIJO de lo de antes (sólo se
// corta la cola) y que used_tokens no pase el techo, holguraMedida veces el presupuesto. El resto
// lo reporta.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// defaultDeAntes es el recall_token_budget que traía config.Default() antes del cambio.
const defaultDeAntes = 400

var consultasDeLaSonda = []string{
	"ahorro de tokens",
	"deploy del cerebro central",
	"gotcha nordvpn curl",
	"compactación SessionStart",
	"worktree anidado sucio",
	"rotación de credenciales",
	"recibo pre-push receipt",
	"fichaje F18 raspberry kiosko",
	"juez de pertinencia rerank latencia",
	"sabotaje de tests y guardas",
	"grafo de código impact callers",
	"puente whatsapp reinicio",
	"go test cacheado count",
	"panel del cerebro picking GPU",
}

func TestSondaRecallLiviano(t *testing.T) {
	dir := strings.TrimSpace(os.Getenv("MUSUBI_SONDA_RECALL_DIR"))
	if dir == "" {
		t.Skip("MUSUBI_SONDA_RECALL_DIR vacío: la sonda necesita una copia de la base real")
	}
	eng, err := memory.NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("abrir la copia: %v", err)
	}
	defer eng.Close()
	s := NewMcpServer(eng, dir, embedding.NoopProvider{}, WithMemory(config.Default().Memory))
	ctx := context.Background()

	antes := func(q string, budget int) memory.RecallResult {
		opts := s.opcionesDeRecall(ctx, budget)
		opts.PresupuestoDeRespuesta = false
		opts.NoBump = true
		if budget == 0 {
			opts.TokenBudget = defaultDeAntes
		}
		consulta, corr := s.corregirConsulta(ctx, q, opts)
		res, err := s.engine.Recall(ctx, consulta, opts)
		if err != nil {
			t.Fatalf("recall de antes %q: %v", q, err)
		}
		res.Correcciones = corr
		return res
	}
	despues := func(q string, budget int) (string, memory.RecallResult) {
		args := map[string]any{"query": q}
		if budget > 0 {
			args["token_budget"] = budget
		}
		raw, _ := json.Marshal(args)
		out, rerr := s.toolRecall(ctx, raw)
		if rerr != nil {
			t.Fatalf("recall de después %q: %+v", q, rerr)
		}
		txt := textOf(t, out)
		var res memory.RecallResult
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatalf("respuesta ilegible %q: %v", q, err)
		}
		return txt, res
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n| consulta | presupuesto | chars antes | chars después | items antes | items después | top-10 iguales | prefijo | used_tokens | chars/used |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|---|---|---|---|\n")
	type total struct{ antes, despues, itemsA, itemsD, consultas, top10 int }
	totales := map[int]*total{}
	for _, budget := range []int{0, 1500, 2500} {
		totales[budget] = &total{}
		for _, q := range consultasDeLaSonda {
			ra := antes(q, budget)
			ja, _ := json.Marshal(ra)
			txt, rd := despues(q, budget)
			ca, cd := utf8.RuneCount(ja), utf8.RuneCountInString(txt)

			n := 10
			if len(ra.Items) < n {
				n = len(ra.Items)
			}
			iguales := 0
			for i := 0; i < n && i < len(rd.Items); i++ {
				if ra.Items[i].ID == rd.Items[i].ID {
					iguales++
				}
			}
			prefijo := len(rd.Items) <= len(ra.Items)
			for i := 0; prefijo && i < len(rd.Items); i++ {
				prefijo = ra.Items[i].ID == rd.Items[i].ID
			}
			// Con el default no aplica: antes y después empaquetan con presupuestos distintos
			// (defaultDeAntes y el de ahora), y el de ahora deja entrar gists grandes que el de antes
			// salteaba. Ahí se reporta en la columna, y el top-10 dice si cambió la cabeza.
			if !prefijo && budget > 0 {
				t.Errorf("%q @%d: lo de después NO es un prefijo de lo de antes", q, budget)
			}
			techo := holguraMedida * rd.Budget
			if len(rd.Items) > 1 && rd.UsedTokens > techo {
				t.Errorf("%q @%d: used_tokens=%d pasa el techo %d", q, budget, rd.UsedTokens, techo)
			}
			etiqueta := fmt.Sprintf("%d", budget)
			if budget == 0 {
				etiqueta = fmt.Sprintf("default (%d→%d)", defaultDeAntes, rd.Budget)
			}
			ratio := 0.0
			if rd.UsedTokens > 0 {
				ratio = float64(cd) / float64(rd.UsedTokens)
			}
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d/%d | %v | %d | %.2f |\n",
				q, etiqueta, ca, cd, len(ra.Items), len(rd.Items), iguales, n, prefijo, rd.UsedTokens, ratio)
			tt := totales[budget]
			tt.antes += ca
			tt.despues += cd
			tt.itemsA += len(ra.Items)
			tt.itemsD += len(rd.Items)
			tt.consultas++
			if iguales == n && n == 10 {
				tt.top10++
			}
		}
	}
	fmt.Fprintf(&b, "\n| presupuesto | chars antes | chars después | baja | items/consulta antes | items/consulta después | consultas con top-10 idéntico |\n|---|---|---|---|---|---|---|\n")
	for _, budget := range []int{0, 1500, 2500} {
		tt := totales[budget]
		fmt.Fprintf(&b, "| %d | %d | %d | %.1f×| %.1f | %.1f | %d/%d |\n", budget, tt.antes, tt.despues,
			float64(tt.antes)/float64(tt.despues), float64(tt.itemsA)/float64(tt.consultas),
			float64(tt.itemsD)/float64(tt.consultas), tt.top10, tt.consultas)
	}
	t.Log(b.String())
}
