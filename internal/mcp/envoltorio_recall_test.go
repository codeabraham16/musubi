package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestRecallAdelgazaElEnvoltorio fija lo que musubi_recall EMITE al modelo (specs/recall-envoltorio/).
// El gist y el valor no se tocan; lo que se recorta es envoltorio que el agente re-lee en cada turno
// sin usarlo. Un cambio así regresa callado si nada lo guarda, que es lo que este test impide.
func TestRecallAdelgazaElEnvoltorio(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	// WithConflicts apagado: la siembra son casi-duplicados y el auto-supersede ocultaría parte.
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithConflicts(config.ConflictConfig{}))
	sembrar(t, s, 5)
	exigeSembradas(t, engine, 5)

	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_recall","arguments":{"query":"` + preguntaSembrada + `","token_budget":4000}}`)
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("recall: %+v", rpcErr)
	}
	texto := out.(CallToolResponse).Content[0].Text

	var res memory.RecallResult
	if err := json.Unmarshal([]byte(texto), &res); err != nil {
		t.Fatalf("la respuesta del recall no es JSON: %v\n%s", err, texto)
	}
	if len(res.Items) < 2 {
		t.Fatalf("esperaba varios items para medir el envoltorio, vinieron %d: %s", len(res.Items), texto)
	}

	// 1.1 — score, author y full_tokens NO VIAJAN al modelo. ESTA GUARDA EXIGÍA LO CONTRARIO hasta el
	// 2026-11-13, y el motivo era real: musubi-body (cmd/musubi-body-ui/memoria.go:108) copiaba el
	// score a su Item, así que sacarlo lo dejaba con 0 en silencio. Lo que cambió es el mundo, no el
	// razonamiento: ese repo está EN PAUSA desde el 2026-09-24 —último commit, 5 PRs abiertos sin
	// mergear, y nada corriendo en el server: ni unit, ni proceso, ni contenedor, ni carpeta— y los
	// tres campos tienen CERO lecturas adentro de musubi (el único uso de Score es
	// methods_cognition.go, que lo pone en 0 al reordenar; full_tokens no tuvo lector NUNCA).
	//
	// Eran 24,3 % del payload de cada ítem —score 10,8 %, author 6,6 %, full_tokens 6,9 %— y un recall
	// trae ~78 ítems, medido sobre 1.451 transcripts el 2026-11-13.
	//
	// LA GUARDA QUEDA, INVERTIDA: el invariante no es «viajan» ni «no viajan», es que NO SE MUEVAN EN
	// SILENCIO en ninguna de las dos direcciones. Si se retoma musubi-body, invertirla de nuevo junto
	// con los tags de recall.go —las dos cosas, o el próximo cambio vuelve a ser invisible.
	// Sabotaje: devolverle a RecallItem.Score el tag que lo hace viajar.
	// arnes: archivo="internal/memory/recall.go"
	// arnes: de="Score float64 `json:\"-\"`"
	// arnes: a="Score float64 `json:\"score,omitempty\"`"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	for _, clave := range []string{`"score":`, `"author":`, `"full_tokens":`} {
		if strings.Contains(texto, clave) {
			t.Errorf("la clave %s viajó en el payload del recall y no tiene lector vivo (ver RecallItem en internal/memory/recall.go): %s", clave, texto)
		}
	}

	// 1.1b — LA MITAD ESTRUCTURAL, y no es redundante: tapa un hueco MEDIDO en la de arriba. Un campo
	// con omitempty y sin valor en la siembra no emite su clave NI CUANDO VIAJA, así que el assert
	// textual se lo pierde. Comprobado el 2026-11-13 saboteando los tres: con el tag de Author
	// restaurado a `json:"author,omitempty"` la guarda textual siguió VERDE —las notas que siembra
	// este test no tienen atribución— mientras score y full_tokens sí se pusieron rojos. El TAG no
	// depende de la siembra, así que mide los tres por igual.
	// Sabotaje: devolverle a RecallItem.Author el tag que lo hace viajar.
	// arnes: archivo="internal/memory/recall.go"
	// arnes: de="Author string `json:\"-\"`"
	// arnes: a="Author string `json:\"author,omitempty\"`"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	for _, campo := range []string{"Score", "Author", "FullTokens"} {
		f, ok := reflect.TypeOf(memory.RecallItem{}).FieldByName(campo)
		if !ok {
			t.Errorf("RecallItem ya no tiene el campo %s; si se quitó a propósito, sacalo de esta lista", campo)
			continue
		}
		if tag := f.Tag.Get("json"); tag != "-" {
			t.Errorf("RecallItem.%s declara json:%q y debería declarar json:\"-\": volvería a viajar al modelo sin tener lector vivo", campo, tag)
		}
	}

	// 1.2 — created_at viaja a resolución de DÍA (YYYY-MM-DD), sin hora.
	// Sabotaje: quitar soloFecha() en empaquetar (volver a `CreatedAt: c.createdAt`).
	// arnes: archivo="internal/memory/recall.go"
	// arnes: de="CreatedAt:   soloFecha(c.createdAt),"
	// arnes: a="CreatedAt:   c.createdAt,"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	for _, it := range res.Items {
		if it.CreatedAt == "" {
			continue // una nota sin fecha sale sin la clave: no es lo que este invariante mide
		}
		if len(it.CreatedAt) != 10 || strings.ContainsAny(it.CreatedAt, "T: ") {
			t.Errorf("created_at de %s viajó con hora (%q); esperaba sólo la fecha YYYY-MM-DD", it.ID, it.CreatedAt)
		}
	}

	// 3 — used_tokens informa el COSTO REAL del payload, no la suma de gists. Se fija con DOS cotas:
	//   (a) el envoltorio (id, topic_key, created_at, las claves JSON repetidas) pesa más que el gist,
	//       así que el número honesto SUPERA la suma de gists; el camino viejo informaba sólo esa suma.
	//   (b) y es, salvo redondeo, el tamaño estimado del payload EMITIDO: used_tokens ≈ EstimateTokens(texto).
	//       La (b) es la que mide de verdad: la (a) sola la pasa un subconteo de hasta el 50%.
	// Sabotaje: quitar en toolRecall `res.UsedTokens = memory.EstimateTokens(string(raw))` ⇒ caen (a) y (b);
	// devolver la mitad, `EstimateTokens(string(raw)) / 2` ⇒ (a) sobrevive, (b) cae.
	// arnes: archivo="internal/mcp/methods.go"
	// arnes: de="res.UsedTokens = memory.EstimateTokens(string(raw))"
	// arnes: a="res.UsedTokens = memory.EstimateTokens(string(raw)) / 2"
	// arnes: prueba="TestRecallAdelgazaElEnvoltorio"
	gistSum := 0
	for _, it := range res.Items {
		gistSum += memory.EstimateTokens(it.Gist)
	}
	if res.UsedTokens <= gistSum {
		t.Errorf("used_tokens=%d no supera la suma de gists=%d: informa sólo el gist, no el payload real que entra al contexto",
			res.UsedTokens, gistSum)
	}
	// La medición interna marshalla ANTES de fijar used_tokens a sí mismo, así que difiere del texto
	// final sólo en sus propios dígitos: una tolerancia de ±2 tokens cubre ese desfase y nada más.
	if emitido := memory.EstimateTokens(texto); res.UsedTokens-emitido < -2 || res.UsedTokens-emitido > 2 {
		t.Errorf("used_tokens=%d no refleja el payload emitido EstimateTokens(texto)=%d (dif %d, tolerancia ±2): no mide el costo real",
			res.UsedTokens, emitido, res.UsedTokens-emitido)
	}
}

// TestRecallEmiteIdCortoResolvible fija el Tier 2a (specs/recall-envoltorio/tier2-id-corto.md): el
// `id` del payload del tool viaja ACORTADO a 8 hex, y ese id corto SIGUE SIENDO EXPANDIBLE. Las dos
// mitades son el invariante: (a) el acortado ocurrió —el UUID entero no viaja— y (b) cero pérdida
// —el handle corto resuelve la MISMA observación por musubi_memory_expand—. La siembra usa
// save_observation, que genera UUIDs REALES de 36 chars: sin el acortado el id mediría 36 y (a) caería.
// El acortado vive en el LÍMITE del tool (toolRecall), no en empaquetar: adentro de Recall el id
// entero lo necesitan la hidratación del grounding y el marcado de ranciedad (markStaleOrigins).
//
// Sabotaje: sacar el loop que acorta en toolRecall ⇒ el id viaja como UUID entero ⇒ (a) en ROJO.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\tfor i := range res.Items {\n\t\tres.Items[i].ID = memory.IDCorto(res.Items[i].ID)\n\t}"
// arnes: a=""
// arnes: prueba="TestRecallEmiteIdCortoResolvible"
func TestRecallEmiteIdCortoResolvible(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithConflicts(config.ConflictConfig{}))
	sembrar(t, s, 5)
	exigeSembradas(t, engine, 5)

	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_recall","arguments":{"query":"` + preguntaSembrada + `","token_budget":4000}}`)
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("recall: %+v", rpcErr)
	}
	var res memory.RecallResult
	if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &res); err != nil {
		t.Fatalf("la respuesta del recall no es JSON: %v", err)
	}
	if len(res.Items) < 2 {
		t.Fatalf("esperaba varios items para medir el id, vinieron %d", len(res.Items))
	}

	// (a) cada id del payload es una ref corta de 8 hex: PRUEBA que el acortado ocurrió. Con el UUID
	// entero (36 chars) este assert cae, que es lo que el sabotaje provoca.
	for _, it := range res.Items {
		if !esHex8(it.ID) {
			t.Errorf("el id %q del payload no es una ref corta de 8 hex: el UUID entero viajó al agente", it.ID)
		}
	}

	// (b) round-trip con IDENTIDAD, por CADA item: su id corto RESUELVE por musubi_memory_expand y trae
	// LA MISMA observación, no otra. Antes esto miraba sólo Items[0] y exigía una frase COMPARTIDA por
	// todas las notas sembradas, así que dejaba pasar dos sabotajes: (S4) acortar bien sólo el item 0 y
	// darle al resto una ref de 8 hex que NO resuelve, y (S6) darle a cada item la ref corta del VECINO
	// —resuelve, pero a OTRA observación—. sembrar() distingue cada nota por un marcador único «caso X»;
	// verificar ese marcador POR ITEM cierra los dos huecos. Se expande con el MISMO ctx del recall para
	// que el scope sea idéntico y no falle por aislamiento de proyecto.
	for i := range res.Items {
		marcador := casoSembrado(res.Items[i].Gist)
		if marcador == "" {
			t.Fatalf("item %d: no pude leer el marcador «caso X» en su gist %q (¿cambió sembrar()?); sin marcador no se prueba identidad", i, res.Items[i].Gist)
		}
		corto := res.Items[i].ID
		expParams := json.RawMessage(`{"name":"musubi_memory_expand","arguments":{"ids":["` + corto + `"]}}`)
		expOut, expErr := s.handleToolsCall(ctx, expParams)
		if expErr != nil {
			t.Fatalf("memory_expand de la ref corta %q (item %d): %+v", corto, i, expErr)
		}
		expTexto := expOut.(CallToolResponse).Content[0].Text
		if !strings.Contains(expTexto, marcador) {
			t.Errorf("el id corto %q del item %d no resolvió a SU observación (esperaba %q en el contenido): la mitad «cero pérdida» no se cumple; expand=%s",
				corto, i, marcador, expTexto)
		}
	}
}

// casoSembrado extrae el marcador único «caso X» que sembrar() le pone a cada nota (…, caso a|b|c…),
// o "" si no está. Es lo único que distingue una nota sembrada de otra, y por eso sirve para probar
// IDENTIDAD en el round-trip del id corto: que resuelva a la MISMA observación, no sólo a alguna.
func casoSembrado(s string) string {
	i := strings.Index(s, "caso ")
	if i < 0 || i+6 > len(s) {
		return ""
	}
	return s[i : i+6]
}

// esHex8 dice si s es EXACTAMENTE 8 hex minúscula, la forma de una ref corta. Inline y no
// memory.esRefCorta porque ésa es privada del paquete memory; acá sólo hace falta la forma.
func esHex8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
