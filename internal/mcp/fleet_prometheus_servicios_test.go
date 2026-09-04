package mcp

// fleet_prometheus_servicios_test.go custodia la exportación de servicios a métricas (A43).
//
// Lo que se prueba no es «salen las series» —eso lo diría cualquier cosa— sino las cuatro
// decisiones que, incumplidas, no rompen nada visible: que la compuerta no se evalúe dos veces,
// que lo desconocido no viaje como cero, que ninguna etiqueta rote, y que el scrape y el empuje
// exporten LO MISMO.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// TestLosServiciosDeUnaMaquinaQueNoVeoNoSeExportan — la compuerta, del lado de los servicios.
//
// Es el invariante más importante del archivo: un servicio se exporta SÓLO si su máquina pasó
// PuedeSobreDevice. Y no se cuenta ni se nombra lo ajeno — contarlo sería decir cuántas máquinas
// hay del otro lado de la compuerta.
//
// Sabotaje que la hace fallar: en serviciosVisiblesParaMetricas, exportar el servicio cuando su
// device no está en `porID` en vez de saltearlo.
func TestLosServiciosDeUnaMaquinaQueNoVeoNoSeExportan(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarServicios(t, s, "nas", "samba")
	sembrarServicios(t, s, "pc-gio", "postgres", "redis")

	// bob ve SÓLO `nas`.
	bob := principalDeFlota("bob", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"nas"}})
	var b strings.Builder
	renderFlota(&b, s.engine, bob, time.Now(), s.sondaIntervalo, versionDePrueba, nil)
	salida := b.String()

	if !strings.Contains(salida, `service="samba"`) {
		t.Error("no se exportó el servicio de la máquina que SÍ puede ver")
	}
	for _, ajeno := range []string{"postgres", "redis", "pc-gio"} {
		if strings.Contains(salida, ajeno) {
			t.Errorf("la salida nombra %q, que está en una máquina que esta credencial no ve", ajeno)
		}
	}
}

// TestLoQueNoSeSabeDeUnServicioNoViajaComoCero.
//
// Un servicio declarado a mano y todavía sin muestras no tiene antigüedad; el SCM de Windows no
// da el contador de reinicios. Emitir 0 en cualquiera de los dos casos es una AFIRMACIÓN falsa:
// «recién reportado» y «nunca se reinició». La serie se omite.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// `up` ESTABA DEL LADO EQUIVOCADO DE SU PROPIA REGLA
//
// Esta prueba exigía que `up` SÍ saliera para este mismo servicio, con el argumento de que «no
// está corriendo es un dato, no una ausencia». El argumento es cierto para `detenido` y para
// `fallado`, y el fixture no es ninguno de los dos: es Salud nil, o sea `desconocido`. Para él,
// «no está corriendo» no es el dato — es la suposición. La prueba pedía, en su rama de `up`,
// exactamente lo que su nombre prohíbe, y sus otras dos ramas ya negaban por el mismo motivo.
//
// El contrato del dominio lo dice sin ambigüedad en la descripción de `musubi_fleet_services`:
// «`desconocido` NO ES `detenido`» y «un servicio declarado y todavía sin medir es un estado
// legítimo y distinto de «caído»». El exportador lo contradecía, y `ServicioCaido` anunciaba como
// caída algo que nadie vio caer — la misma forma de A70, un estado más allá.
//
// Verificado contra la flota antes de cambiarlo: 182 `corriendo` y 2 `fallado`, ningún
// `desconocido`. Las dos alertas vivas son reales y siguen sonando; esto no calla ninguna.
//
// LA AUSENCIA NO ES INVISIBILIDAD: `musubi_fleet_service_declared` sale igual y en 1, así que el
// servicio existe en las métricas con sus etiquetas y `declared unless up` nombra exactamente a
// los declarados de los que no se sabe nada.
//
// Sabotaje que la hace fallar: devolver (0, true) para lo desconocido en cualquiera de las tres.
func TestLoQueNoSeSabeDeUnServicioNoViajaComoCero(t *testing.T) {
	ahora := time.Now()
	sinNada := fleet.Servicio{Nombre: "declarado-y-sin-medir"} // UltimoReporte cero, Salud nil

	var declarado bool
	for _, s := range seriesDeServicio() {
		v, hay := s.Valor(sinNada, ahora)
		switch s.Nombre {
		case "musubi_fleet_service_last_report_seconds", "musubi_fleet_service_restarts_total",
			"musubi_fleet_service_up":
			if hay {
				t.Errorf("%s emitió %v para un servicio del que no se sabe nada: un 0 ahí es una afirmación", s.Nombre, v)
			}
		case "musubi_fleet_service_declared":
			// Y ÉSTA SÍ, siempre: es lo que impide que esconder lo no medido lo vuelva invisible.
			if !hay || v != 1 {
				t.Errorf("musubi_fleet_service_declared dio (%v, %v): sin ella, un servicio declarado y nunca reportado no aparece en NINGUNA serie y deja de poder verse", v, hay)
			}
			declarado = true
		}
	}
	if !declarado {
		t.Error("no existe musubi_fleet_service_declared: esconder lo desconocido sin ella borra el servicio del exportador")
	}
}

// TestNingunaEtiquetaDeServicioROTA — la guarda de cardinalidad.
//
// Una etiqueta cuyo valor cambia crea una serie nueva cada vez, y las viejas no se borran. El pid
// rota en cada reinicio: con él adentro, un servicio que se reinicia 400 veces deja 400 series.
// Es la forma más común de matar un Prometheus, y no da ningún error mientras pasa.
//
// Sabotaje que la hace fallar: agregar {"pid", ...} a labelsDeServicio.
func TestNingunaEtiquetaDeServicioRota(t *testing.T) {
	pid := 4242
	sv := fleet.Servicio{Nombre: "postgres", Clase: "systemd",
		Salud: &fleet.SaludServicio{Estado: fleet.EstadoCorriendo, PID: &pid}}
	d := fleet.Device{Name: "nas", ProjectID: "casa", Tier: fleet.TierAgente, OS: "linux"}

	permitidas := map[string]bool{"device": true, "project": true, "tier": true, "os": true, "service": true, "class": true}
	for _, kv := range labelsDeServicio(sv, d) {
		if !permitidas[kv[0]] {
			t.Errorf("la etiqueta %q no está en la lista de estables: si su valor rota, cada cambio deja una serie muerta que no se borra nunca", kv[0])
		}
		if strings.Contains(kv[1], "4242") {
			t.Errorf("la etiqueta %q lleva el pid (%s): rota en cada reinicio", kv[0], kv[1])
		}
	}
}

// TestElScrapeYElEmpujeExportanLosMismosServicios — la misma guarda que ya existe para las
// máquinas, ahora para los servicios.
//
// Los dos caminos comparten la tabla de series y el juego de labels a propósito. Dos copias
// discrepan el día que alguien agrega un campo, y la discrepancia se descubre semanas después
// cuando dos dashboards muestran cosas distintas.
//
// Sabotaje que la hace fallar: sacar el bloque de servicios de armarPayloadOTLP, o el
// renderServicios de renderFlota.
func TestElScrapeYElEmpujeExportanLosMismosServicios(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarServicios(t, s, "nas", "samba", "postgres")
	ahora := time.Now()
	p := ptrPrincipal(principalDePrometheus())

	var b strings.Builder
	renderFlota(&b, s.engine, p, ahora, s.sondaIntervalo, versionDePrueba, nil)
	delScrape := b.String()

	cuerpo, _, _, err := armarPayloadOTLP(s.engine, p, ahora, s.sondaIntervalo, versionDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	delPush := string(cuerpo)

	if !strings.Contains(delScrape, "musubi_fleet_service_up") {
		t.Fatal("el scrape no exportó ninguna serie de servicio: la comparación sería vacía y verde")
	}
	for _, quiero := range []string{"musubi_fleet_service_up", "samba", "postgres"} {
		if !strings.Contains(delPush, quiero) {
			t.Errorf("el scrape exporta %q y el EMPUJE no: los dos caminos discreparon", quiero)
		}
	}
	for _, serie := range seriesDeServicio() {
		if strings.Contains(delScrape, serie.Nombre) && !strings.Contains(delPush, serie.Nombre) {
			t.Errorf("la serie %s sale por el scrape y no por el empuje", serie.Nombre)
		}
	}
}

// TestElNombreDeUnServicioSeCitaYNoSeMutila.
//
// El nombre lo produce la MÁQUINA, y va a una línea del exposition format. Dos cosas tienen que
// pasar y son distintas:
//
//	· una comilla o un salto de línea DEBEN escaparse, o la línea se parte en dos y Prometheus
//	  descarta la respuesta ENTERA — no esa serie, la respuesta;
//	· un acento debe pasar TAL CUAL. Los valores de label son UTF-8 y no se escapan.
//
// SOBRE EL SABOTAJE DECLARADO, porque el primero que escribí era falso y lo comprobé corriéndolo:
// puse `%q` en lugar de `citarLabel` esperando que rompiera, y la prueba siguió verde. Son
// byte a byte idénticos para estas entradas — `%q` de Go deja los caracteres imprimibles Unicode
// como están y escapa la comilla y el salto igual. No es una regresión, así que no sirve como
// sabotaje. El que SÍ rompe es el de abajo, y está ejecutado.
//
// Sabotaje que la hace fallar: escribir el valor crudo (`b.WriteString(kv[1])`) sin citar.
func TestElNombreDeUnServicioSeCitaYNoSeMutila(t *testing.T) {
	d := fleet.Device{Name: "nas", ProjectID: "casa"}

	conAcento := etiquetasDeServicio(fleet.Servicio{Nombre: "gestión-de-turnos", Clase: "systemd"}, d)
	if !strings.Contains(conAcento, "gestión-de-turnos") {
		t.Errorf("el acento se mutiló: %s\n  Los valores de label son UTF-8: escaparlos deja un nombre que no matchea el de nadie", conAcento)
	}

	rompible := etiquetasDeServicio(fleet.Servicio{Nombre: "mal\"nombre", Clase: "systemd"}, d)
	if !strings.Contains(rompible, `mal\"nombre`) {
		t.Errorf("la comilla no se escapó: %s", rompible)
	}
	conSalto := etiquetasDeServicio(fleet.Servicio{Nombre: "dos\nlineas", Clase: "systemd"}, d)
	if strings.Contains(conSalto, "\n") {
		t.Errorf("un salto de línea sobrevivió sin escapar y parte la respuesta entera: %q", conSalto)
	}
}
