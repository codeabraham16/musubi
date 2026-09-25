package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// CADA FILA DE LA CRONOLOGÍA DICE EL PLANO DE SU TIPO, en la superficie donde se lee.
//
// Las guardas de la tool miraban el plano de UNA clase de fila: TestUnaOperacionDePantalla… exige
// `entrar` sólo para `canal_pantalla`, y el fixture de la cronología (sembrarLosTresPlanos) no
// encola ningún aviso. Con el plano de un hecho calculado desde el argv —la forma de C1-m6 en la
// auditoría A131—, los avisos llegaban con `plano: ""` y las guardas de mcp seguían verdes. Acá se
// encolan los cuatro avisos por el MISMO encolador que usan pantalla, shell, exec y las políticas,
// más la operación de pantalla, y se exige para CADA fila que el plano que se muestra sea el del
// tipo que decidió quién la ve.
//
// EXPOSICIÓN medida por la auditoría: 22 `musubi:avisar` del exec en producción, el último del
// 2026-09-21, que con esa mutación se habrían mostrado con `plano: ""` a quien tiene `exec`.
//
// Sabotaje: que la superficie arme el plano desde el argv y no desde el tipo del hecho → los avisos
// pierden el plano de quien los encoló y las sesiones, que no tienen argv, caen en `actuar`.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\t\t\"plano\":      string(h.Plano()),"
// arnes: a="\t\t\"plano\":      string(fleet.Hecho{Tipo: fleet.TipoDeArgv(h.Argv)}.Plano()),"
func TestCadaFilaDeLaCronologiaDiceElPlanoDeSuTipo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	for _, a := range []avisoDeAcceso{avisoPantalla, avisoShell, avisoExec, avisoPolitica} {
		if !s.encolarAvisoDeAcceso(d, conCaps("infra", nil), a) {
			t.Fatalf("no se encoló el aviso de %s: la prueba no mediría su fila", a.operacion)
		}
	}
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "mirador",
		Argv: []string{fleet.OpPantalla, "ses-9", "clave", "30m0s"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}

	todo := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	res, e := callAsPrincipal(t, s, todo, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "limite": 100})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	vistos := map[string]int{}
	for _, h := range jsonOf(t, res)["hechos"].([]any) {
		fila := h.(map[string]any)
		tipo, _ := fila["tipo"].(string)
		quiero, clasificado := fleet.PlanoDeHecho(fleet.TipoDeHecho(tipo))
		if !clasificado {
			t.Errorf("se mostró una fila de tipo %q, que no tiene plano: %v", tipo, fila)
			continue
		}
		if fila["plano"] != string(quiero) {
			t.Errorf("una fila %q se mostró en el plano %q y su tipo es del plano %q: quién la ve y cómo "+
				"se lee dicen dos cosas distintas (%v)", tipo, fila["plano"], quiero, fila)
		}
		vistos[tipo]++
	}
	// EL PISO: cada clase de fila que se puede MOSTRAR llegó a la respuesta. Sin esto, una
	// cronología que dejara afuera los avisos pasaría la comparación de arriba sin haberlos mirado.
	//
	// Las clases salen del enum (fleet.TiposDeHecho, que TestTiposDeHechoEsElEnumEntero ata al bloque
	// const) y no de una lista: eran seis escritas a mano, y un tipo nuevo habría quedado sin su plano
	// medido sin que nada se pusiera rojo. Ahora un tipo nuevo pide que el sembrado de arriba lo
	// produzca. Queda afuera sólo el que por definición no se muestra.
	medibles := 0
	for _, tipo := range fleet.TiposDeHecho {
		if tipo == fleet.HechoSinClasificar {
			continue
		}
		medibles++
		if vistos[string(tipo)] == 0 {
			t.Errorf("no llegó ninguna fila %q a la cronología: la prueba no midió su plano (vistos: %v)", tipo, vistos)
		}
	}
	if medibles < 6 {
		t.Fatalf("fleet.TiposDeHecho trae %d tipos mostrables y cuando se escribió esta prueba eran 6: "+
			"el piso de arriba no está midiendo lo que dice", medibles)
	}
}
