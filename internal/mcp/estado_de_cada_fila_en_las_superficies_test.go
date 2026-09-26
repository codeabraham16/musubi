package mcp

import (
	"sort"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// CADA SUPERFICIE MUESTRA, DE CADA FILA, EL ESTADO QUE DERIVA EL DOMINIO — NI EL GUARDADO NI UNA
// REGLA PROPIA.
//
// La guarda vieja (TestLasDosSuperficiesMuestranVencidoUnComandoQueNadieLevanto) clava el estado:
// siembra un pendiente de diez horas y espera `expirado` en la bitácora y en la cronología. La
// auditoría A131 (C4-m5) devolvió hechosDeComandos al código de antes de A60 —deriva `expirado` y
// nunca `perdido`— y todo quedó verde: el único rojo fue el censo, porque el `de` pisaba el ancla de
// esa guarda. La del dominio (TestUnEntregadoQueNuncaReportaSeMuestraPerdido) cuida la regla adentro
// de EstadoActual; nada ataba esa regla a las superficies que tienen que usarla.
//
// Acá la referencia ES EstadoActual y no un estado escrito a mano. Se siembra, por las mismas puertas
// del motor que usan las tools, una fila por cada rama —pendiente vivo, pendiente vencido, expirado
// estampado por la toma, entregado vivo, entregado que no volvió, terminado de recién y terminado
// viejo—, y cada superficie tiene que mostrar de cada fila lo que el dominio deriva de ESA fila
// releída de la base. El piso exige que la siembra cubra fleet.EstadosDeComando entero: un estado
// nuevo del enum pone esto en rojo hasta que alguien le siembre su fila.
//
// musubi_fleet_contexto no entra: lee la misma cronología pero devuelve cuántos hechos hay de cada
// tipo, no el estado de cada uno.
//
// musubi_fleet_exec TAMPOCO SE RECORRE, y ésa sí dibuja un estado sin pasar por EstadoActual: cuando
// su espera vence sin resultado devuelve el estado GUARDADO (methods_exec.go, la rama «todavía sin
// resultado»), y las ramas que encolan y vuelven dicen `pendiente`. Hoy no puede mentir, porque la
// fila que muestra nace adentro de esa misma llamada, que espera a lo sumo esperaMaxExec, y lo
// guardado sólo se aparta de lo derivado pasado fleet.ComandoVidaMax sin que nadie la levante o
// pasada fleet.EsperaMaxDeEntregado desde la entrega. Ninguna siembra hace que esa rama muestre una
// fila que diverja, así que recorrerla —o agregarle ahí la derivación— daría una guarda verde contra
// su propio sabotaje. LA PREMISA SÍ SE MIDE, abajo: si la espera de exec alcanza alguna de las dos
// cotas, esto se pone rojo, y esa rama tiene que pasar por EstadoActual y entrar a esta prueba.
//
// EXPOSICIÓN medida por la auditoría: hoy hay 4 comandos `entregado` en producción, de davantis-1, con
// 4,5 a 19 días sin reporte; las dos superficies los muestran `perdido`. Con la mutación, la
// cronología los dibujaría `entregado`: corriendo.
//
// Sabotaje: que la cronología derive sólo `expirado`, como antes de A60 (C4-m5) → el entregado que no
// volvió se dibuja corriendo.
// arnes: archivo="internal/memory/cronologia.go"
// arnes: de="\t\tc.Estado = c.EstadoActual(ahora)\n\t\tout = append(out, fleet.HechoDeComando(c, nombre))"
// arnes: a="\t\tif c.Vencido(ahora) {\n\t\t\tc.Estado = fleet.EstadoExpirado\n\t\t}\n\t\tout = append(out, fleet.HechoDeComando(c, nombre))"
// arnes: colision_ok="TestLasDosSuperficiesMuestranVencidoUnComandoQueNadieLevanto"
// Sabotaje: que la bitácora muestre el estado GUARDADO → `pendiente` sobre lo vencido y `entregado`
// sobre lo que no volvió.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tfila[\"estado\"] = string(c.EstadoActual(ahora))"
// arnes: a="\tfila[\"estado\"] = string(c.Estado)"
func TestCadaSuperficieMuestraElEstadoQueDerivaElDominioDeCadaFila(t *testing.T) {
	// LA PREMISA DE LO QUE NO SE RECORRE: musubi_fleet_exec muestra el estado guardado de una fila que
	// nació en la misma llamada, y eso vale sólo mientras su espera no alcance ninguna cota del dominio.
	for _, cota := range []struct {
		nombre string
		valor  time.Duration
	}{{"fleet.ComandoVidaMax", fleet.ComandoVidaMax}, {"fleet.EsperaMaxDeEntregado", fleet.EsperaMaxDeEntregado}} {
		if esperaMaxExec >= cota.valor {
			t.Errorf("musubi_fleet_exec espera hasta %s y %s es %s: la fila que devuelve «todavía sin resultado» "+
				"ya puede estar vencida o perdida, y esa rama muestra el estado GUARDADO. Pasala por EstadoActual "+
				"y sumala a las superficies de esta prueba", esperaMaxExec, cota.nombre, cota.valor)
		}
	}
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": "pc-gio", "tier": "A", "project": "infra", "caps": []string{"metrics", "exec"}, "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, existe, err := s.engine.DevicePorNombre("infra", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("no quedó la máquina: %v %v", existe, err)
	}

	ahora := time.Now().UTC().Truncate(time.Second)
	// VIEJO queda adentro de la ventana más larga que acepta la cronología (fleet.VentanaMax) y lejos
	// de cualquier cota: ni la vida de un pendiente ni la espera de un entregado llegan a días.
	viejo := ahora.Add(-20 * 24 * time.Hour)

	ids := map[string]string{} // marca → id
	encolar := func(marca string, creado time.Time) {
		t.Helper()
		c, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Origen: fleet.OrigenPersona,
			Creado: creado, Argv: []string{"echo", marca}, Timeout: 30 * time.Second,
		})
		if err != nil {
			t.Fatalf("encolar %s: %v", marca, err)
		}
		ids[marca] = c.ID
	}
	tomar := func(cuando time.Time, quiero int) {
		t.Helper()
		entregados, err := s.engine.TomarComandos(d.ID, cuando, fleet.ComandosPorEntregaMax)
		if err != nil || len(entregados) != quiero {
			t.Fatalf("la toma de las %s entregó %d comandos y la siembra esperaba %d (%v): la prueba no arma "+
				"las filas que dice", cuando.Format(time.RFC3339), len(entregados), quiero, err)
		}
	}
	terminar := func(marca string, cuando time.Time) {
		t.Helper()
		cero := 0
		if err := s.engine.GuardarResultado(d.ID, ids[marca], &cero, "", "", "", cuando); err != nil {
			t.Fatalf("resultado de %s: %v", marca, err)
		}
	}

	// La tanda vieja: uno que nadie levantó a tiempo (la toma lo ESTAMPA `expirado`), uno que se llevó
	// el agente y no volvió nunca, y uno que volvió.
	encolar("MARCA-EXPIRADO-ESTAMPADO", viejo.Add(-fleet.ComandoVidaMax-time.Minute))
	encolar("MARCA-ENTREGADO-QUE-NO-VOLVIO", viejo.Add(-time.Minute))
	encolar("MARCA-TERMINADO-VIEJO", viejo.Add(-time.Minute))
	tomar(viejo, 2)
	terminar("MARCA-TERMINADO-VIEJO", viejo.Add(time.Minute))
	// La tanda de recién: uno corriendo y uno que ya volvió.
	encolar("MARCA-ENTREGADO-CORRIENDO", ahora.Add(-3*time.Minute))
	encolar("MARCA-TERMINADO-RECIEN", ahora.Add(-3*time.Minute))
	tomar(ahora.Add(-2*time.Minute), 2)
	terminar("MARCA-TERMINADO-RECIEN", ahora.Add(-time.Minute))
	// Los que nadie levantó: uno que ya venció sin que ninguna toma lo estampara, y uno vivo.
	encolar("MARCA-PENDIENTE-VENCIDO", viejo)
	encolar("MARCA-PENDIENTE-VIVO", ahora.Add(-time.Minute))

	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	estadosEn := func(tool string, args map[string]any, filas, clave string) map[string]string {
		t.Helper()
		res, e := callAsPrincipal(t, s, p, tool, args)
		if e != nil {
			t.Fatalf("%s: %+v", tool, e)
		}
		lista, _ := jsonOf(t, res)[filas].([]any)
		out := map[string]string{}
		for _, f := range lista {
			fila, _ := f.(map[string]any)
			id, _ := fila[clave].(string)
			estado, _ := fila["estado"].(string)
			out[id] = estado
		}
		return out
	}
	superficies := []struct {
		nombre  string
		estados map[string]string
	}{
		{"la BITÁCORA (musubi_fleet_log)", estadosEn("musubi_fleet_log",
			map[string]any{"device": "pc-gio", "limite": 50}, "comandos", "command_id")},
		{"la CRONOLOGÍA (musubi_fleet_cronologia)", estadosEn("musubi_fleet_cronologia",
			map[string]any{"device": "pc-gio", "horas": fleet.VentanaMax.Hours(), "limite": 100}, "hechos", "referencia")},
	}

	// LA REFERENCIA: lo que el dominio deriva de cada fila tal como quedó en la base, a esta hora. Las
	// edades están lejos de todo borde, así que el segundo que pasó entre las lecturas no mueve nada.
	despues := time.Now()
	quiero := map[string]fleet.EstadoComando{}
	cubiertos := map[fleet.EstadoComando]bool{}
	for marca, id := range ids {
		c, ok, err := s.engine.ComandoPorID(id)
		if err != nil || !ok {
			t.Fatalf("releer %s: ok=%v err=%v", marca, ok, err)
		}
		quiero[marca] = c.EstadoActual(despues)
		cubiertos[quiero[marca]] = true
	}
	// EL PISO: la siembra cubre el enum entero. Sin esto, una rama que ninguna fila ejerce pasaría en
	// verde por no haberse mirado — que es exactamente cómo `perdido` quedó sin guarda.
	for _, e := range fleet.EstadosDeComando {
		if !cubiertos[e] {
			t.Errorf("ninguna fila sembrada deriva %q: la prueba no mide cómo lo muestran las superficies", e)
		}
	}

	marcas := make([]string, 0, len(ids))
	for m := range ids {
		marcas = append(marcas, m)
	}
	sort.Strings(marcas)
	for _, sup := range superficies {
		for _, marca := range marcas {
			got, esta := sup.estados[ids[marca]]
			if !esta {
				t.Errorf("%s no devolvió la fila %s: la prueba no midió esa superficie", sup.nombre, marca)
				continue
			}
			if got != string(quiero[marca]) {
				t.Errorf("%s muestra %q para %s y el dominio deriva %q de esa misma fila: una superficie que "+
					"dibuja su propia regla cuenta otra historia —`entregado` sobre lo que no volvió manda a "+
					"esperar un resultado que no llega; `pendiente` sobre lo vencido, a esperar que corra",
					sup.nombre, got, marca, quiero[marca])
			}
		}
	}
}
