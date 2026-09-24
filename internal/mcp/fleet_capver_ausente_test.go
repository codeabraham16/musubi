package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// LO QUE CUSTODIA: que `musubi_fleet_device_capver` se OMITA cuando el agente no declara contrato,
// en los dos transportes — y que la alerta que vive de esa ausencia siga escrita para aprovecharla.
//
// SALE DE AUDITAR A131 SOBRE `fleet_otlp_test.go`. La guarda que cuida esta familia hoy,
// `TestUnValorDesconocidoNoViajaComoCeroEnElPayload`, lleva una lista de CUATRO nombres escrita a
// mano; con una muestra mínima el payload omite unas quince métricas, así que hay once ausencias
// que nadie custodia. `capver` es una de ellas, y es la que tiene consecuencia medible.
//
// LA MUTACIÓN QUE DEJA TODO EN VERDE —`if d.Capver <= 0 { return 0, true }`— hace pasar
// `./internal/mcp` entero. Una sonda propia dio PASS en el árbol sano y FAIL en el mutado, en los
// DOS transportes, así que no era vacua.
//
// POR QUÉ IMPORTA, Y SON DOS EFECTOS OPUESTOS. `AgenteSinContratoDeclarado` usa
// `unless on(project, device) musubi_fleet_device_capver` SIN comparación: suprime por PRESENCIA
// de la serie. Si la serie existiera valiendo 0, esa alerta quedaría muda para toda la flota. Y su
// hermana `AgenteConContratoViejo` (`capver < 2`) pasaría a disparar sobre cada máquina que no
// declara. O sea: una se calla y la otra acusa, y las dos se equivocan por el mismo cambio.
//
// LA EXPOSICIÓN NO ERA CERO. Medido el 2026-09-23: de cuatro máquinas, tres declaran `capver=3` y
// `altura-db` (tier B) lo tiene AUSENTE — que es el estado correcto, y su propia alerta lo dice en
// `ausente_en`: «un Tier B no corre nuestro agente, así que no declara ninguna capver». Con la
// mutación, esa máquina emitiría 0 y se la acusaría de contrato viejo sin tener agente.
//
// POR QUÉ NO SALIÓ UNA GUARDA ESTRUCTURAL, que fue el primer intento. La idea era derivar la lista
// de la DOC —«toda métrica que promete AUSENTE tiene que poder omitirse»— pero medida contra el
// árbol sano dio TRECE hallazgos y casi todos eran del instrumento: unas métricas delegan en un
// helper (`return valorDe(m.CPUPct)`, un solo resultado) y otras condicionan con un `> 0`
// defensivo que no es una ausencia semántica. Un lector que acusa a doce métricas sanas no es una
// guarda. Y la otra ancla candidata —«las series de las que una alerta depende por presencia»—
// resultó tener UN solo miembro, que es éste: enumerar uno no es peor que enumerar uno.
//
// Sabotaje: emitir `capver=0` cuando el agente no declara contrato, en vez de omitir la serie.
// arnes: archivo="internal/mcp/fleet_prometheus.go"
// arnes: de="\t\t\t\tif d.Capver <= 0 {\n\t\t\t\t\treturn 0, false\n\t\t\t\t}"
// arnes: a="\t\t\t\tif d.Capver <= 0 {\n\t\t\t\t\treturn 0, true\n\t\t\t\t}"
func TestCapverAusenteSeQuedaAusenteEnLosDosTransportes(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Una máquina cuyo agente NO declara capver — el caso de `altura-db` hoy.
	maquinaConMuestra(t, s, "casa", "pc-gio", fleet.Muestra{
		Tomada: ahora, NumCPU: 4, MemTotal: 100, MemUsada: 25,
		DiscoTotal: 1000, DiscoUsado: 100, DiscoDisponible: 850,
	}, ahora)

	// ── el empuje (OTLP) ──
	cuerpo, _, _, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora, 0, versionDePrueba, serviciosPorProyectoDefault)
	if err != nil {
		t.Fatal(err)
	}
	puntos := puntosDelPayload(t, cuerpo)
	if len(puntos) == 0 {
		t.Fatal("control: el payload salió vacío, esta guarda no probaría nada")
	}
	for _, p := range puntos {
		if p.Metrica == "musubi_fleet_device_capver" {
			t.Errorf("EMPUJE: `capver` viajó con valor %v para un agente que no declara contrato.\n"+
				"`AgenteSinContratoDeclarado` suprime por PRESENCIA de esta serie, así que un 0 la "+
				"deja muda para toda la flota — y `AgenteConContratoViejo` (`capver < 2`) pasa a "+
				"acusar a cada máquina sin agente.", p.Valor)
		}
	}

	// ── el scrape (/metrics) ──
	out := exportar(t, s, nil)
	if !strings.Contains(out, "musubi_fleet_device_up{") {
		t.Fatal("control: el scrape no exportó ni `up`, esta guarda no probaría nada")
	}
	if strings.Contains(out, "musubi_fleet_device_capver{") {
		t.Error("SCRAPE: `capver` salió en /metrics para un agente que no declara contrato. " +
			"Los dos transportes tienen que omitirla igual, o Prometheus ve una cosa distinta " +
			"según por dónde haya llegado la serie.")
	}

	// ── LA OTRA MITAD: que la alerta siga contando con la ausencia ──
	//
	// Sin esto, la guarda de arriba custodia un invariante que quizá ya no le sirva a nadie: si
	// alguien reescribe la regla con `musubi_fleet_device_capver == 0`, la supresión deja de ser
	// por presencia y toda la explicación de este archivo pasa a ser folklore. Que se entere acá.
	ruta := filepath.Join("..", "..", "deploy", "musubi-alerts-flota.yml")
	reglas, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	// La forma que importa: `unless on(...) musubi_fleet_device_capver` y NADA más en la línea.
	re := regexp.MustCompile(`(?m)unless on\([^)]*\)\s+musubi_fleet_device_capver\s*$`)
	if !re.MatchString(string(reglas)) {
		t.Errorf("`AgenteSinContratoDeclarado` ya no suprime por PRESENCIA de `capver`.\n"+
			"Buscaba en %s una línea `unless on(…) musubi_fleet_device_capver` sin comparación, y no está.\n"+
			"O la regla cambió de forma —y entonces hay que revisar si la ausencia sigue importando—\n"+
			"o se borró, y esta guarda quedó cuidando un invariante que ya no usa nadie.",
			filepath.Base(ruta))
	}
}
