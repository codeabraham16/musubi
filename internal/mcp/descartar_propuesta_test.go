package mcp

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory/memtest"
)

// musubi_discard_proposal DESCARTA SIN VOLVER VISIBLE, Y UN PEDIDO EQUIVOCADO SE NOTA.
//
// Es el veredicto que faltaba en la cuarentena: la única salida era corroborar, así que una propuesta
// superada no se podía sacar sin hacerla visible. Descartar la archiva, anota la que la reemplaza, la
// saca del listado, y ante un id que no es una propuesta viva falla en vez de reportar éxito.
//
// Sabotaje que la hace fallar: no pasarle al motor la que la reemplaza.
// arnes: archivo="internal/mcp/methods_observation_quarantine.go"
// arnes: de="\tif err := s.engine.DescartarPropuestaCtx(s.scopedCtx(ctx), id, reemplazadaPor); err != nil {\n"
// arnes: a="\tif err := s.engine.DescartarPropuestaCtx(s.scopedCtx(ctx), id, \"\"); err != nil {\n"
//
// Sabotaje que la hace fallar: reportar éxito cuando no era una propuesta viva.
// arnes: archivo="internal/mcp/methods_observation_quarantine.go"
// arnes: de="\t\t\treturn nil, rpcErrorf(codeInvalidParams, \"la observación %q no es una propuesta viva en cuarentena: no hay nada que descartar\", id)\n"
// arnes: a="\t\t\treturn textResult(fmt.Sprintf(\"la observación %q no es una propuesta viva en cuarentena: no hay nada que descartar\", id)), nil\n"
func TestDescartarUnaPropuestaLaArchivaSinVolverlaVisible(t *testing.T) {
	dir := t.TempDir()
	engine := memtest.NuevoEngine(t, dir)
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	proponer := func(topic, contenido string) string {
		t.Helper()
		out, e := call(t, s, "musubi_propose_observation", map[string]interface{}{"topic_key": topic, "content": contenido})
		if e != nil {
			t.Fatalf("proponer: %+v", e)
		}
		return idDeLaPropuesta(t, out)
	}
	vieja := proponer("t/estado", "estado viejo del trabajo de prueba")
	nueva := proponer("t/estado", "estado nuevo del trabajo de prueba")

	out, e := call(t, s, "musubi_discard_proposal", map[string]interface{}{"id": vieja, "superseded_by": nueva})
	if e != nil {
		t.Fatalf("descartar: %+v", e)
	}
	if txt := textOf(t, out); !strings.Contains(txt, "descartada") || !strings.Contains(txt, nueva) {
		t.Errorf("la respuesta no dice que la descartó ni por cuál: %q", txt)
	}
	ps, err := engine.PropuestasEnCuarentena()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != nueva {
		t.Errorf("después de descartar, las vivas son %v: la vieja tenía que salir y la nueva quedar", ps)
	}
	if q, _ := engine.IsQuarantined(vieja); !q {
		t.Error("descartar la sacó de cuarentena: descartar no verifica nada")
	}
	// El motor no expone superseded_by: se lee de la base, en sólo lectura.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, config.DirName, config.DBFile)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sup string
	if err := db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id=?`, vieja).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup != nueva {
		t.Errorf("la descartada quedó reemplazada por %q; quería %s", sup, nueva)
	}

	// Descartar dos veces, o un id que no existe: error de parámetros, no éxito.
	for _, id := range []string{vieja, "no-existe"} {
		if _, e := call(t, s, "musubi_discard_proposal", map[string]interface{}{"id": id}); e == nil || e.Code != codeInvalidParams {
			t.Errorf("descartar %q dio %+v; quería un error de parámetros", id, e)
		}
	}
}
