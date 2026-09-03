package mcp

// fleet_export_techos_test.go custodia LOS TECHOS DEL EXPORT (A80) y, sobre todo, que dejen de
// ser silenciosos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ SE ESTÁ PROBANDO, Y POR QUÉ NINGUNA PRUEBA VIEJA LO CAZABA
//
// El techo de servicios era 2000 EN TOTAL por scrape. Un total es un techo COMPARTIDO: el
// proyecto que se barría primero se lo comía y los siguientes salían del export ENTEROS. Y sin
// series, `ServicioCaido` no tiene a qué matchear — así que esas máquinas no quedaban vigiladas
// en amarillo: quedaban sin vigilar y en verde, que se ve exactamente igual que todo bien.
//
// La suite pasaba en verde con eso porque ninguna prueba tenía más de un puñado de servicios: el
// techo no se tocaba nunca. Y el aviso de truncado era una línea `#`, que Prometheus DESCARTA al
// parsear — o sea que ni con el techo tocado había nada que alertar.
//
// Las tres cosas que se custodian acá, entonces, son: que el techo sea POR PROYECTO, que el
// truncado salga como SERIE, y que esa serie salga por LAS DOS BOCAS.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// sembrarFlotaConServicios enrola `maquinas` máquinas en `proyecto`, cada una con `porMaquina`
// servicios corriendo, y devuelve cuántos servicios quedaron en total.
//
// Va DIRECTO AL MOTOR y no por las tools a propósito: por `musubi_fleet_service_declare` una
// siembra de 2400 servicios son 2400 llamadas MCP y la prueba tarda minutos. Lo que se está
// midiendo es el exportador, no la puerta de entrada.
func sembrarFlotaConServicios(t *testing.T, s *McpServer, proyecto string, maquinas, porMaquina int) int {
	t.Helper()
	ahora := time.Now()
	total := 0
	for i := 0; i < maquinas; i++ {
		nombre := fmt.Sprintf("maq-%03d", i)
		d, err := s.engine.AltaDevice(fleet.Device{
			Name: nombre, ProjectID: proyecto, Tier: fleet.TierAgente,
			Caps: []fleet.Cap{fleet.CapMetrics}, OS: "linux",
		}, fmt.Sprintf("token-%s-%s", proyecto, nombre))
		if err != nil {
			t.Fatalf("alta de %s/%s: %v", proyecto, nombre, err)
		}
		reportes := make([]fleet.ReporteServicio, 0, porMaquina)
		for j := 0; j < porMaquina; j++ {
			reportes = append(reportes, fleet.ReporteServicio{
				Nombre: fmt.Sprintf("svc-%03d", j),
				Salud:  fleet.SaludServicio{Estado: fleet.EstadoCorriendo, Tomada: ahora},
			})
		}
		nuevos, _, err := s.engine.ReportarServicios(d.ID, ahora, reportes)
		if err != nil {
			t.Fatalf("reporte de servicios de %s: %v", nombre, err)
		}
		if nuevos != porMaquina {
			t.Fatalf("se sembraron %d servicios de %d en %s: la siembra no mide lo que dice", nuevos, porMaquina, nombre)
		}
		total += nuevos
	}
	return total
}

// contarLineas cuenta las líneas de UNA serie en el exposition format (las de datos, no el HELP
// ni el TYPE).
func contarLineas(salida, serie string) int {
	n := 0
	for _, l := range strings.Split(salida, "\n") {
		if strings.HasPrefix(l, serie+"{") {
			n++
		}
	}
	return n
}

// CUARENTA MÁQUINAS CON SESENTA SERVICIOS NO SE CAEN DEL EXPORT EN SILENCIO.
//
// Es el escenario que motivó el slice: ya hay dos máquinas con 57 y 64 servicios, y a ~35
// máquinas × 60 el techo de 2000 se pasa. El invariante es una disyunción a propósito —o salen
// todas las series, o el export DICE que truncó— porque lo inadmisible no es truncar: es truncar
// sin que nadie se pueda enterar.
//
// Sabotaje que la hace fallar: sacar la serie de seriesDeExport (o el `escribirSerieDeExport` de
// renderFlota) dejando el techo activo. El export sigue truncando 400 servicios y la prueba se
// pone roja porque ni están todas las series ni hay quien lo diga.
func TestCuarentaMaquinasConSesentaServiciosNoSeCaenDelExportEnSilencio(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrados := sembrarFlotaConServicios(t, s, "casa", 40, 60)
	if sembrados != 2400 {
		t.Fatalf("se sembraron %d servicios y la prueba necesita 2400 para pasar el techo", sembrados)
	}
	if s.techoServiciosPorProyecto != 2000 {
		t.Fatalf("el techo por proyecto arrancó en %d y no en su default de 2000: la prueba estaría "+
			"midiendo otra cosa", s.techoServiciosPorProyecto)
	}

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), time.Now(), s.sondaIntervalo,
		versionDePrueba, s.techoServiciosPorProyecto)
	salida := b.String()

	exportadas := contarLineas(salida, "musubi_fleet_service_up")
	if exportadas == sembrados {
		return // no se truncó nada: están todas, y no hay nada que avisar
	}
	if !strings.Contains(salida, `musubi_fleet_export_truncated{kind="services"} 1`) {
		t.Errorf("se exportaron %d de %d servicios y el export NO lo dice: los %d que quedaron "+
			"afuera no tienen serie, así que ServicioCaido no los cubre y el hueco es invisible.\n"+
			"Falta musubi_fleet_export_truncated{kind=\"services\"} en 1.",
			exportadas, sembrados, sembrados-exportadas)
	}
}

// EL TECHO ES POR PROYECTO, NO DE LA FLOTA ENTERA. Es la prueba que separa el arreglo del bug.
//
// Con un techo TOTAL, un tenant grande se come la cuota y el siguiente no exporta NADA — y cuál
// es «el siguiente» lo decidía el orden de un map de Go, así que el proyecto que se quedaba a
// ciegas cambiaba entre scrapes. Con el techo por proyecto, cada uno se corta a sí mismo.
//
// El techo se baja a 5 en vez de sembrar 4000 servicios: lo que se mide es la ARITMÉTICA del
// corte, y para eso 5 y 2000 son el mismo número. Que el default sea 2000 lo custodia la prueba
// de arriba y la de internal/config.
//
// Sabotaje que la hace fallar: en serviciosVisiblesParaMetricas, volver el corte a un total —
// cambiar `deEsteProyecto` por `len(out)` y el `break` por un `return`. Uno de los dos proyectos
// se queda sin una sola serie y esta prueba lo nombra.
func TestElTechoDeServiciosEsPorProyectoYNoDeLaFlotaEntera(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	const techo = 5
	s.techoServiciosPorProyecto = techo
	sembrarFlotaConServicios(t, s, "casa", 2, 4)         // 8 servicios: pasa el techo
	sembrarFlotaConServicios(t, s, "cliente-acme", 2, 4) // 8 más, otro tenant

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), time.Now(), s.sondaIntervalo,
		versionDePrueba, techo)
	salida := b.String()

	// CADA PROYECTO APORTA LO SUYO. Es el corazón de la prueba: ninguno queda en cero por culpa
	// del vecino.
	for _, proy := range []string{"casa", "cliente-acme"} {
		n := 0
		for _, l := range strings.Split(salida, "\n") {
			if strings.HasPrefix(l, "musubi_fleet_service_up{") && strings.Contains(l, `project="`+proy+`"`) {
				n++
			}
		}
		if n != techo {
			t.Errorf("el proyecto %q aportó %d servicios y el techo POR PROYECTO es %d: con 0 se "+
				"quedó a ciegas porque el otro se comió la cuota, que es el bug que este techo "+
				"vino a cerrar", proy, n, techo)
		}
	}
	if total := contarLineas(salida, "musubi_fleet_service_up"); total != 2*techo {
		t.Errorf("salieron %d series de servicio en total y tenían que ser %d (%d por proyecto): "+
			"el techo se está aplicando sobre la flota y no sobre cada tenant", total, 2*techo, techo)
	}
	if !strings.Contains(salida, `musubi_fleet_export_truncated{kind="services"} 1`) {
		t.Error("se cortaron servicios en los dos proyectos y la serie de truncado no lo dice")
	}
}

// LA SERIE DE TRUNCADO SALE POR LAS DOS BOCAS, con el mismo nombre y el mismo valor.
//
// El empuje OTLP no es un segundo camino de export: es el MISMO con otra boca, y la tabla de
// series es una sola justamente para que no puedan discrepar. Si esta serie viviera sólo en el
// scrape, en el despliegue real —donde el tirón descarta `musubi_fleet_(device|service)_.*` y la
// flota entra por el push— seguiría existiendo, pero contando la verdad de la boca equivocada:
// las dos exportan con credenciales distintas y pueden truncar distinto.
//
// Sabotaje que la hace fallar: borrar el bloque de seriesDeExport de armarPayloadOTLP (o dejarlo
// detrás del `if len(metricas) == 0`, que lo apaga cuando importa).
func TestLaSerieDeTruncadoSalePorElScrapeYPorElEmpuje(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	const techo = 3
	s.techoServiciosPorProyecto = techo
	sembrarFlotaConServicios(t, s, "casa", 2, 4) // 8 > 3: se trunca
	ahora := time.Now()
	p := ptrPrincipal(principalDePrometheus())

	var b strings.Builder
	renderFlota(&b, s.engine, p, ahora, s.sondaIntervalo, versionDePrueba, techo)
	delScrape := b.String()
	if !strings.Contains(delScrape, `musubi_fleet_export_truncated{kind="services"} 1`) {
		t.Fatalf("el scrape no exportó el truncado; la comparación con el empuje sería vacía y verde:\n%s", delScrape)
	}

	cuerpo, _, truncado, err := armarPayloadOTLP(s.engine, p, ahora, s.sondaIntervalo, versionDePrueba, techo)
	if err != nil {
		t.Fatal(err)
	}
	if !truncado {
		t.Error("el empuje no devolvió truncado con un techo pasado: el log de A50 no avisaría")
	}
	if !strings.Contains(string(cuerpo), "musubi_fleet_export_truncated") {
		t.Fatalf("la serie sale por el scrape y NO por el empuje: los dos caminos discreparon\n%s", cuerpo)
	}
	// Y con el MISMO valor: una serie que existe en las dos bocas con valores distintos es peor
	// que no tenerla, porque `max by(kind)` de la alerta se quedaría con el optimista.
	var vistas int
	for _, pt := range puntosDelPayload(t, cuerpo) {
		if pt.Metrica != "musubi_fleet_export_truncated" {
			continue
		}
		vistas++
		if strings.Contains(pt.Labels, `kind="services"`) && pt.Valor != 1 {
			t.Errorf("el empuje dice que los servicios NO se truncaron (%v) y el scrape dice que sí", pt.Valor)
		}
	}
	if vistas != 2 {
		t.Errorf("el empuje llevó %d puntos de truncado y tenían que ser 2 (services y projects): "+
			"la mitad que falta no se puede alertar", vistas)
	}
}

// EL EXPORT COMPLETO DICE QUE ESTÁ COMPLETO, EN VEZ DE CALLARSE.
//
// Es la única serie del exportador que emite 0, y contra la regla general del archivo (lo
// desconocido no viaja como cero) por una razón: acá el 0 es una MEDICIÓN. Sin él, la serie sólo
// aparece cuando hay problema y no se distingue de un export que se murió; y `ExportacionTruncada`
// no se apagaría al arreglarse, porque Prometheus CONGELA el último valor de una serie que
// desaparece en vez de borrarla.
//
// Sabotaje que la hace fallar: emitir el punto sólo cuando el booleano es true (un `if !truncado
// { continue }` adentro de seriesDeExport).
func TestUnExportCompletoDiceQueNoSeTrunco(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarFlotaConServicios(t, s, "casa", 2, 3) // muy por debajo de cualquier techo

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), time.Now(), s.sondaIntervalo,
		versionDePrueba, s.techoServiciosPorProyecto)
	salida := b.String()

	for _, quiero := range []string{
		`musubi_fleet_export_truncated{kind="services"} 0`,
		`musubi_fleet_export_truncated{kind="projects"} 0`,
	} {
		if !strings.Contains(salida, quiero) {
			t.Errorf("falta %q: una serie que sólo aparece cuando hay problema no se distingue de "+
				"un export muerto, y la alerta no se apaga al arreglarse", quiero)
		}
	}
	// Y el TYPE una sola vez, no uno por punto: dos bloques con el mismo nombre de métrica es un
	// error de formato que Prometheus rechaza.
	if n := strings.Count(salida, "# TYPE musubi_fleet_export_truncated gauge"); n != 1 {
		t.Errorf("el TYPE de musubi_fleet_export_truncated salió %d veces y tiene que salir 1", n)
	}
}

// UN TECHO DESACTIVADO EXPORTA TODO, Y NO MIENTE DICIENDO QUE TRUNCÓ.
//
// `services_per_project_export: -1` es el apagado explícito (el mismo vocabulario que
// `probe_minutes`), y EffectiveServicesPerProjectExport lo traduce a 0. Un 0 que se leyera como
// «techo cero» dejaría el export sin una sola serie de servicio y la serie de truncado en 1 para
// siempre: el apagado sería el modo más destructivo de la perilla.
//
// Sabotaje que la hace fallar: cambiar `techo > 0` por `techo >= 0` en
// serviciosVisiblesParaMetricas.
func TestElTechoDesactivadoNoCortaNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrados := sembrarFlotaConServicios(t, s, "casa", 3, 4)

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), time.Now(), s.sondaIntervalo,
		versionDePrueba, config.FleetConfig{ServicesPerProjectExport: -1}.EffectiveServicesPerProjectExport())
	salida := b.String()

	if n := contarLineas(salida, "musubi_fleet_service_up"); n != sembrados {
		t.Errorf("con el techo desactivado salieron %d de %d servicios: el apagado se está leyendo "+
			"como «techo cero»", n, sembrados)
	}
	if !strings.Contains(salida, `musubi_fleet_export_truncated{kind="services"} 0`) {
		t.Error("con el techo desactivado el export dice que truncó: la alerta quedaría firing para siempre")
	}
}
