package mcp

// Pruebas de la puerta del RENDIMIENTO (fase 4): salud para un servicio DECLARADO que ninguna
// máquina enumera — un bot, un puente.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// declararBot da de alta a mano el servicio que ninguna máquina va a enumerar.
func declararBot(t *testing.T, s *McpServer, device, nombre string) {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_service_declare", map[string]any{
		"device": device, "nombre": nombre, "project": "casa"}); e != nil {
		t.Fatalf("service_declare(%q): %+v", nombre, e)
	}
}

func cuerpoDeSalud(reportes ...fleet.ReporteServicio) string {
	b, _ := json.Marshal(map[string]any{"servicios": reportes})
	return string(b)
}

func rendimientoDelBot() *fleet.Rendimiento {
	p95, max := 820, 1940
	return &fleet.Rendimiento{
		VentanaSeg: 60, Atendidas: 47, Fallidas: 3,
		Desglose:      map[string]int{"ok": 41, "no_puedo": 3, "vacio": 3},
		LatenciaP95Ms: &p95, LatenciaMaxMs: &max,
	}
}

// EL HUECO QUE ESTA PUERTA CIERRA: `service_declare` existe —lo dice su descripción— para declarar
// «un Tier B que no enumera solo, un bot, un puente», y hasta acá un bot declarado se quedaba en
// `desconocido` PARA SIEMPRE. Su salud «pasa a tener estado cuando la máquina lo reporte», y a un
// bot que vive en una base gestionada en la nube no lo reporta ninguna máquina.
//
// Sabotaje que la hace fallar: no registrar fleetSaludPath en el mux.
// Sabotaje que la hace fallar: no llamar a ReportarSaludDeServicios desde el handler.
func TestUnBotDeclaradoAlFinPuedeRecibirSalud(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	declararBot(t, s, "pc-gio", "alturito20")

	// Antes del reporte: declarado y SIN medir, que es un estado legítimo y no una falla.
	svs, err := s.engine.ServiciosDeDevice(idDeDevice(t, s, "pc-gio"))
	if err != nil {
		t.Fatal(err)
	}
	if len(svs) != 1 || svs[0].Salud != nil {
		t.Fatalf("el bot recién declarado ya tenía salud: %+v", svs)
	}

	cuerpo := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "alturito20",
		Salud: fleet.SaludServicio{
			Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot(),
		},
	})
	code, body := postCon(t, ts.URL+fleetSaludPath, tokenDevice, cuerpo)
	if code != http.StatusOK {
		t.Fatalf("la puerta devolvió %d: %s", code, body)
	}
	var res respuestaSalud
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("respuesta ilegible: %s", body)
	}
	if res.Actualizados != 1 {
		t.Errorf("actualizados = %d, esperaba 1 (desconocidos: %v)", res.Actualizados, res.Desconocidos)
	}

	svs, _ = s.engine.ServiciosDeDevice(idDeDevice(t, s, "pc-gio"))
	if len(svs) != 1 || svs[0].Salud == nil {
		t.Fatalf("el bot sigue sin salud después del reporte: %+v", svs)
	}
	r := svs[0].Salud.Rendimiento
	if r == nil {
		t.Fatal("la salud llegó SIN rendimiento: el campo se perdió en el viaje o en la base")
	}
	if r.Atendidas != 47 || r.Fallidas != 3 {
		t.Errorf("rendimiento = %d atendidas / %d fallidas, esperaba 47/3", r.Atendidas, r.Fallidas)
	}
	if r.LatenciaP95Ms == nil || *r.LatenciaP95Ms != 820 {
		t.Errorf("el p95 no sobrevivió: %v", r.LatenciaP95Ms)
	}
	if r.Desglose["ok"] != 41 {
		t.Errorf("el desglose no sobrevivió: %#v", r.Desglose)
	}
}

// ESTA PUERTA NO PODA, Y ES LA MITAD QUE MÁS FÁCIL SE ROMPE.
//
// Un colector que manda UN servicio por el camino del latido borraría los otros de esa máquina: la
// poda por ausencia es correcta para un INVENTARIO —«esto es todo lo que corre acá»— y es una
// afirmación que el colector de un bot no está en condiciones de hacer.
//
// Sabotaje que la hace fallar: llamar a ReportarServicios (el del latido) desde el handler.
// Sabotaje que la hace fallar: podar por ausencia en ReportarSaludDeServicios.
func TestReportarLaSaludDeUnBotNoBorraLosOtrosServiciosDeLaMaquina(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	// La máquina enumera lo suyo por el latido, como siempre.
	inventario := cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "postgresql.service", Clase: "systemd", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "nginx.service", Clase: "systemd", Salud: saludViva(fleet.EstadoCorriendo)},
	)
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, inventario); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	declararBot(t, s, "pc-gio", "alturito20")

	// Y el colector reporta SÓLO el bot.
	cuerpo := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "alturito20",
		Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot()},
	})
	if code, body := postCon(t, ts.URL+fleetSaludPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("la puerta devolvió %d: %s", code, body)
	}

	svs, err := s.engine.ServiciosDeDevice(idDeDevice(t, s, "pc-gio"))
	if err != nil {
		t.Fatal(err)
	}
	vivos := map[string]bool{}
	for _, sv := range svs {
		if !sv.Revocado {
			vivos[sv.Nombre] = true
		}
	}
	for _, n := range []string{"postgresql.service", "nginx.service", "alturito20"} {
		if !vivos[n] {
			t.Errorf("%q desapareció tras el reporte de salud del bot: la puerta podó un "+
				"inventario que el colector no estaba en condiciones de afirmar", n)
		}
	}
}

// ESTA PUERTA NO ESTAMPA SEÑAL DE VIDA. Si lo hiciera, un host cuyo AGENTE murió pero cuyo colector
// sigue corriendo figuraría sano — y el colector es justamente lo que menos se cae, porque es un
// cron de un minuto. La vida de la máquina la afirma quien la mide.
//
// Sabotaje que la hace fallar: llamar a LatirDevice desde ReportarSaludDeServicios o del handler.
func TestReportarSaludNoResucitaAUnaMaquinaCaida(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	declararBot(t, s, "pc-gio", "alturito20")

	// La máquina nunca latió: figura caída, que es lo que hay que preservar.
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if d.EnLinea(time.Now(), umbralEnLineaDefault) {
		t.Fatal("la máquina ya figuraba viva sin haber latido: el escenario no es el que dice")
	}

	cuerpo := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "alturito20",
		Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot()},
	})
	if code, _ := postCon(t, ts.URL+fleetSaludPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("la puerta devolvió %d", code)
	}

	d, _, _ = s.engine.DevicePorNombre("casa", "pc-gio")
	if d.EnLinea(time.Now(), umbralEnLineaDefault) {
		t.Error("un reporte de salud marcó VIVA a la máquina: un host cuyo agente murió y cuyo " +
			"colector sigue corriendo figuraría sano, que es el peor de los dos mundos")
	}
}

// ESTA PUERTA NO CREA SERVICIOS, y no es prudencia: es un bug evitado.
//
// El camino del latido crea con `declared = 0` para que la poda por ausencia pueda sacarlos. Si
// ésta creara igual, el bot nacería podable y el SIGUIENTE latido del agente lo borraría —el agente
// enumera systemd y contenedores, y el bot no está en ninguno—. El colector lo recrearía un minuto
// después, el agente lo podaría de nuevo, y el servicio parpadearía en el panel para siempre.
//
// Sabotaje que la hace fallar: agregarle el INSERT a ReportarSaludDeServicios.
func TestReportarLaSaludDeUnServicioQueNadieDeclaroNoLoCrea(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	cuerpo := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "bot-que-nadie-declaro",
		Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot()},
	})
	code, body := postCon(t, ts.URL+fleetSaludPath, tokenDevice, cuerpo)
	if code != http.StatusOK {
		t.Fatalf("la puerta devolvió %d: %s", code, body)
	}
	var res respuestaSalud
	_ = json.Unmarshal([]byte(body), &res)
	if res.Actualizados != 0 {
		t.Errorf("actualizados = %d: se creó un servicio que nadie declaró", res.Actualizados)
	}
	// Y SE DICE CUÁL. El error más probable de este camino es un typo en el nombre, y su síntoma
	// sin esto sería un panel que nunca cambia y un colector convencido de que está reportando.
	if len(res.Desconocidos) != 1 || !strings.Contains(res.Desconocidos[0], "bot-que-nadie-declaro") {
		t.Errorf("la respuesta no nombra el servicio desconocido: %#v", res.Desconocidos)
	}
	svs, _ := s.engine.ServiciosDeDevice(idDeDevice(t, s, "pc-gio"))
	for _, sv := range svs {
		if sv.Nombre == "bot-que-nadie-declaro" {
			t.Fatal("la fila se creó igual: el próximo latido del agente la podaría, y el " +
				"colector la recrearía — el servicio parpadearía en el panel para siempre")
		}
	}
	// Un nombre desconocido NO es un error HTTP: el reporte llegó y se aplicó lo que se pudo.
	// Devolver 4xx haría que un colector con un typo en UN servicio deje de reportar los otros.
	if !res.OK {
		t.Error("un nombre desconocido puso ok=false: un typo en un servicio no puede apagar el resto")
	}
}

// EL TOKEN DECIDE LA MÁQUINA, Y EL CUERPO NO TIENE POR DÓNDE DECIRLO. Es la misma garantía del
// latido y del resultado: no es disciplina, es que el tipo no tiene el campo.
//
// Sabotaje que la hace fallar: agregarle un campo `device` a cuerpoSalud y hacerle caso.
func TestLaPuertaDeSaludNoDejaReportarSobreLaMaquinaDeOtro(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	// Una segunda máquina, con un servicio propio.
	enrolarDePrueba(t, s, "casa", "servidor-ajeno")
	declararBot(t, s, "servidor-ajeno", "postgres-de-produccion")

	// El token es de pc-gio. Se intenta nombrar el servicio del otro, y de todas las formas que
	// un atacante probaría.
	for _, nombre := range []string{"postgres-de-produccion", "servidor-ajeno/postgres-de-produccion"} {
		cuerpo := cuerpoDeSalud(fleet.ReporteServicio{
			Nombre: nombre,
			Salud:  fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoFallado},
		})
		code, body := postCon(t, ts.URL+fleetSaludPath, tokenDevice, cuerpo)
		if code != http.StatusOK {
			continue
		}
		var res respuestaSalud
		_ = json.Unmarshal([]byte(body), &res)
		if res.Actualizados != 0 {
			t.Errorf("con %q se escribió sobre la máquina de otro: cualquier máquina de la flota "+
				"podría reportar que el postgres de producción está caído", nombre)
		}
	}
	// Y el servicio ajeno sigue sin salud.
	svs, _ := s.engine.ServiciosDeDevice(idDeDevice(t, s, "servidor-ajeno"))
	if len(svs) != 1 {
		t.Fatalf("la máquina ajena tiene %d servicios", len(svs))
	}
	if svs[0].Salud != nil {
		t.Errorf("le escribieron la salud al servicio de la máquina ajena: %+v", svs[0].Salud)
	}
}

// UN RENDIMIENTO IMPOSIBLE NO PISA LA ÚLTIMA SALUD BUENA.
//
// Acá NO vale la asimetría del latido —guardar el servicio con la salud vacía— porque el servicio
// YA EXISTE: no hay nada que crear, y pisar una salud buena con una vacía sería perder el último
// dato bueno por culpa de un reporte roto.
//
// Sabotaje que la hace fallar: guardar la salud sin validarla en ReportarSaludDeServicios.
func TestUnReporteImposibleNoPisaLaUltimaSaludBuena(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	declararBot(t, s, "pc-gio", "alturito20")

	bueno := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "alturito20",
		Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot()},
	})
	if code, _ := postCon(t, ts.URL+fleetSaludPath, tokenDevice, bueno); code != http.StatusOK {
		t.Fatal("el reporte bueno no entró")
	}

	// Ahora uno imposible: más fallidas que atendidas.
	malo := cuerpoDeSalud(fleet.ReporteServicio{
		Nombre: "alturito20",
		Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoFallado,
			Rendimiento: &fleet.Rendimiento{VentanaSeg: 60, Atendidas: 3, Fallidas: 7}},
	})
	code, body := postCon(t, ts.URL+fleetSaludPath, tokenDevice, malo)
	if code != http.StatusOK {
		t.Fatalf("la puerta devolvió %d: %s", code, body)
	}
	var res respuestaSalud
	_ = json.Unmarshal([]byte(body), &res)
	if res.Actualizados != 0 {
		t.Errorf("se aplicó un rendimiento imposible (actualizados=%d)", res.Actualizados)
	}
	if len(res.Desconocidos) == 0 || !strings.Contains(strings.Join(res.Desconocidos, " "), "subconjunto") {
		t.Errorf("la respuesta no dice POR QUÉ se rechazó: %#v", res.Desconocidos)
	}

	svs, _ := s.engine.ServiciosDeDevice(idDeDevice(t, s, "pc-gio"))
	if svs[0].Salud == nil || svs[0].Salud.Rendimiento == nil {
		t.Fatal("el reporte roto borró la salud que ya había")
	}
	if svs[0].Salud.Rendimiento.Atendidas != 47 {
		t.Errorf("la última salud buena se pisó: atendidas quedó en %d, esperaba 47",
			svs[0].Salud.Rendimiento.Atendidas)
	}
	if svs[0].Salud.Estado != fleet.EstadoCorriendo {
		t.Errorf("el estado se pisó con el del reporte roto: %q", svs[0].Salud.Estado)
	}
}

// idDeDevice resuelve el id interno de una máquina por su nombre.
func idDeDevice(t *testing.T, s *McpServer, nombre string) string {
	t.Helper()
	d, ok, err := s.engine.DevicePorNombre("casa", nombre)
	if err != nil || !ok {
		t.Fatalf("no se pudo resolver la máquina %q: %v", nombre, err)
	}
	return d.ID
}

// ── La tool y el exportador tienen que coincidir, igual que en A39 ───────────────────────────

// LA MISMA CLASE DE GUARDA QUE A39, UN NIVEL MÁS ABAJO. La tool `musubi_fleet_services` muestra el
// rendimiento y el exportador lo emite: las dos leen la MISMA fila. Si una se olvida, el gráfico
// muestra un hueco y la tabla un número, y no hay forma de saber cuál miente.
//
// Sabotaje que la hace fallar: sacar una de las series de rendimiento de seriesDeServicio.
// Sabotaje que la hace fallar: sacar `fila["rendimiento"]` de filaDeServicio.
func TestLaToolYElExportadorCoincidenSobreElRendimiento(t *testing.T) {
	ahora := time.Now().UTC()
	d := fleet.Device{Name: "pc-gio", Tier: fleet.TierAgente, OS: "linux", ProjectID: "casa", LastSeen: ahora}

	casos := []struct {
		nombre string
		salud  *fleet.SaludServicio
		espera bool // ¿tiene que haber rendimiento en las dos superficies?
	}{
		{"un servicio de systemd: no mide trabajo",
			&fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo}, false},
		{"un servicio sin salud ninguna", nil, false},
		{"un bot con trabajo",
			&fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo, Rendimiento: rendimientoDelBot()}, true},
		{"un bot CALLADO: el cero es una medición",
			&fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo,
				Rendimiento: &fleet.Rendimiento{VentanaSeg: 60, Atendidas: 0}}, true},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			sv := fleet.Servicio{Nombre: "alturito20", DeviceID: d.ID, ProjectID: "casa",
				UltimoReporte: ahora, Salud: c.salud, Declarado: true}

			fila := filaDeServicio(sv, d, ahora)
			_, enLaTool := fila["rendimiento"]

			emite := map[string]bool{}
			for _, serie := range seriesDeServicio() {
				_, ok := serie.Valor(sv, ahora)
				emite[serie.Nombre] = ok
			}
			// Las cuatro series del rendimiento tienen que estar TODAS o ninguna. Una que falte
			// deja el par colgado sin que nada se ponga rojo.
			for _, n := range []string{"musubi_fleet_service_handled", "musubi_fleet_service_failed",
				"musubi_fleet_service_window_seconds"} {
				si, conocida := emite[n]
				if !conocida {
					t.Fatalf("la serie %q no existe: el par quedó colgado y esta prueba dejó de mirarlo", n)
				}
				if si != c.espera {
					t.Errorf("%q: el exportador %s y se esperaba lo contrario", n,
						map[bool]string{true: "SÍ emite", false: "NO emite"}[si])
				}
				if si != enLaTool {
					t.Errorf("desacuerdo: el exportador %s %q y la tool %s el rendimiento. "+
						"Un gráfico con hueco y una tabla con número no se pueden reconciliar.",
						map[bool]string{true: "emite", false: "no emite"}[si], n,
						map[bool]string{true: "trae", false: "no trae"}[enLaTool])
				}
			}
			// El p95 es la excepción DECLARADA: sobre cero atendidas no hay percentil, así que la
			// serie está ausente aunque el rendimiento exista. Es la única asimetría y por eso se
			// prueba aparte, en vez de dejarla caer en el lazo de arriba como si fuera igual.
			p95 := emite["musubi_fleet_service_latency_p95_ms"]
			quiereP95 := c.salud != nil && c.salud.Rendimiento != nil && c.salud.Rendimiento.LatenciaP95Ms != nil
			if p95 != quiereP95 {
				t.Errorf("el p95 %s y se esperaba lo contrario: sobre cero unidades no hay percentil",
					map[bool]string{true: "SÍ salió", false: "NO salió"}[p95])
			}
		})
	}
}

// EL DESGLOSE NO SE EXPORTA A PROMETHEUS, y es una decisión que hay que poder defender.
//
// Sus claves las elige quien reporta, con el vocabulario de SU dominio. Una etiqueta cuyos valores
// decide un tercero es cardinalidad sin techo: el dominio la acota a DesgloseMax POR SERVICIO,
// pero por flota no hay tope. Se mira en la tool y en el panel, donde una clave nueva cuesta una
// columna y no una serie por máquina.
//
// Sabotaje que la hace fallar: agregar una serie por clave del desglose.
func TestElDesgloseNoViajaAPrometheusPorCardinalidad(t *testing.T) {
	for _, s := range seriesDeServicio() {
		for _, clave := range []string{"ok", "no_puedo", "vacio", "desglose"} {
			if strings.Contains(s.Nombre, clave) {
				t.Errorf("la serie %q lleva una clave del desglose en su nombre: las claves las "+
					"elige quien reporta, y una etiqueta así es cardinalidad sin techo", s.Nombre)
			}
		}
	}
	// Y la tool SÍ lo trae: es donde una clave nueva cuesta una columna, no una serie.
	ahora := time.Now().UTC()
	d := fleet.Device{Name: "pc-gio", Tier: fleet.TierAgente, OS: "linux", ProjectID: "casa"}
	fila := filaDeServicio(fleet.Servicio{Nombre: "alturito20", UltimoReporte: ahora,
		Salud: &fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo,
			Rendimiento: rendimientoDelBot()}}, d, ahora)
	rend, _ := fila["rendimiento"].(map[string]interface{})
	if rend == nil || rend["desglose"] == nil {
		t.Error("la tool tampoco trae el desglose: entonces el dato no se ve en ningún lado")
	}
}
