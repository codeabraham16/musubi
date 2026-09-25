package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/cognition"
	"musubi/internal/embedding"
)

// callSharpenUndo llama a musubi_sharpen con `undo` y decodifica su reporte, que no es el de una tanda.
func callSharpenUndo(t *testing.T, s *McpServer, p *Principal, ids []string) (dedupUndoReport, *RpcError) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"undo": ids})
	res, rpcErr := s.toolSharpen(withPrincipal(context.Background(), p), raw)
	if rpcErr != nil {
		return dedupUndoReport{}, rpcErr
	}
	r, ok := res.(CallToolResponse)
	if !ok || len(r.Content) == 0 {
		t.Fatalf("resultado no es CallToolResponse con content: %#v", res)
	}
	var rep dedupUndoReport
	if err := json.Unmarshal([]byte(r.Content[0].Text), &rep); err != nil {
		t.Fatalf("no pude decodificar el reporte de undo: %v (text=%s)", err, r.Content[0].Text)
	}
	return rep, nil
}

// TestSharpenUndoDevuelveLaTarjetaSinMotor — deshacer una fusión, por la herramienta que la hizo.
//
// La herramienta se anunció siempre como «soft-delete REVERSIBLE» y no tenía reversa. `undo` la da:
// la tarjeta vuelve al acervo y el par queda marcado para que el afilado de fondo no la fusione otra
// vez. La prueba fusiona con el juez, APAGA el motor y deshace: si deshacer necesitara al juez, el día
// que el endpoint se cae no se podría devolver ninguna tarjeta, y ése es justo el día en que alguien
// está revisando fusiones malas.
//
// Mira el estado de la BASE, no el reporte: un reporte que dice «restored» sobre una tarjeta que sigue
// archivada es exactamente la clase de verde que no sirve.
//
// Sabotaje que la hace fallar: exigir el motor también para deshacer.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="\tif len(args.Undo) > 0 {"
// arnes: a="\tif len(args.Undo) > 0 && cognition.Enabled(s.cognition) {"
//
// Sabotaje que la hace fallar: reportar la tarjeta como devuelta sin devolverla.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="restored, canonical, err = s.engine.RestoreDuplicate(dedupScope, id, dedupAuthor)"
// arnes: a="restored, canonical, err = true, \"\", nil"
func TestSharpenUndoDevuelveLaTarjetaSinMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	juez := &fakeCognition{answer: `{"verdict":"MERGE"}`}
	s.cognition = juez
	vec := map[string][]float32{"c1": {1, 0, 0, 0}, "c2": {0.98, 0.2, 0, 0}}
	seedCard(t, s, "c1", "grilla-columnas", "4, 8 y 12 columnas según el breakpoint", vec["c1"])
	seedCard(t, s, "c2", "grilla-gutters", "4, 8 y 12 columnas; las columnas se estiran y los gutters quedan fijos", vec["c2"])

	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, rpcErr := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "pairs": 5})
	if rpcErr != nil || rep.Merged != 1 || len(rep.Pairs) != 1 {
		t.Fatalf("precondición: el juez dijo MERGE y no hubo una fusión: %+v err=%+v", rep, rpcErr)
	}
	archivada := rep.Pairs[0].B

	s.cognition = cognition.NoopProvider{} // el motor se cae
	undo, rpcErr := callSharpenUndo(t, s, admin, []string{archivada})
	if rpcErr != nil {
		t.Fatalf("deshacer con el motor caído falló: %+v", rpcErr)
	}
	if undo.Restored != 1 || len(undo.Cards) != 1 || undo.Cards[0].Action != "restored" {
		t.Errorf("el reporte no dice que la devolvió: %+v", undo)
	}
	if got, _, _, err := s.engine.NearestVisibleByVector(dedupScope, dedupCardPrefix, vec[archivada], ""); err != nil || got != archivada {
		t.Errorf("la tarjeta %q no volvió al acervo: la visible más cercana a su vector es %q (err %v)", archivada, got, err)
	}

	s.cognition = juez
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("el par deshecho volvió a proponerse (scanned=%d): el afilado de fondo lo fusionaría de nuevo", rep2.Scanned)
	}

	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "musubi"}
	if _, rpcErr := callSharpenUndo(t, s, writer, []string{archivada}); rpcErr == nil || rpcErr.Code != codeUnauthorized {
		t.Errorf("deshacer escribe en el acervo compartido: un writer no-admin no puede; obtuve %+v", rpcErr)
	}
}

// TestSharpenUndoNoSeMezclaNiSeDesborda — `undo` es un pedido aparte y acotado.
//
// Mezclado con `dry_run` o `pairs`, alguien que quería MIRAR terminaría escribiendo en el acervo
// compartido; y sin tope, un pedido gigante sostiene el candado de escritura tarjeta tras tarjeta.
//
// Sabotaje que la hace fallar: aceptar `undo` junto con los argumentos de una tanda.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="\t\tif args.DryRun || args.Pairs > 0 || args.Floor > 0 {"
// arnes: a="\t\tif false {"
//
// Sabotaje que la hace fallar: sacarle el tope a `undo`.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="\t\tif len(args.Undo) > dedupMaxPairs {"
// arnes: a="\t\tif false {"
func TestSharpenUndoNoSeMezclaNiSeDesborda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	admin := &Principal{Name: "root", Role: RoleAdmin}
	call := func(args map[string]any) *RpcError {
		raw, _ := json.Marshal(args)
		_, rpcErr := s.toolSharpen(withPrincipal(context.Background(), admin), raw)
		return rpcErr
	}
	if rpcErr := call(map[string]any{"undo": []string{"x"}, "dry_run": true}); rpcErr == nil || rpcErr.Code != codeInvalidParams {
		t.Errorf("undo junto con dry_run tiene que rechazarse; obtuve %+v", rpcErr)
	}
	muchos := make([]string, dedupMaxPairs+1)
	for i := range muchos {
		muchos[i] = "id-inexistente"
	}
	if rpcErr := call(map[string]any{"undo": muchos}); rpcErr == nil || rpcErr.Code != codeInvalidParams {
		t.Errorf("undo con %d ids pasa el tope de %d y tiene que rechazarse; obtuve %+v", len(muchos), dedupMaxPairs, rpcErr)
	}
}
