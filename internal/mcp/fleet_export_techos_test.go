package mcp

// fleet_export_techos_test.go custodia las TRES mitades que le faltaban a los techos del
// exportador: que se puedan configurar, que el orden que se calculó no se pierda, y que el aviso
// nombre EL TECHO QUE CORTÓ y no el otro.

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// serviciosDePrueba reporta `n` servicios corriendo en `d`, con nombres estables y ordenados.
func serviciosDePrueba(t *testing.T, s *McpServer, d fleet.Device, n int, ahora time.Time) {
	t.Helper()
	lista := make([]fleet.ReporteServicio, 0, n)
	for i := 0; i < n; i++ {
		lista = append(lista, fleet.ReporteServicio{
			Nombre: fmt.Sprintf("svc-%03d", i),
			Salud:  fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo},
		})
	}
	if _, _, err := s.engine.ReportarServicios(d.ID, ahora, lista); err != nil {
		t.Fatalf("no se pudieron reportar los servicios de %q: %v", d.Name, err)
	}
}

// lineasDe devuelve las líneas de la salida que empiezan con `prefijo`.
func lineasDe(salida, prefijo string) []string {
	var out []string
	for _, l := range strings.Split(salida, "\n") {
		if strings.HasPrefix(l, prefijo) {
			out = append(out, l)
		}
	}
	return out
}

// proyectoDeLaLinea saca el valor de la etiqueta project="..." de una línea del exposition format.
func proyectoDeLaLinea(l string) string {
	i := strings.Index(l, `project="`)
	if i < 0 {
		return ""
	}
	resto := l[i+len(`project="`):]
	j := strings.Index(resto, `"`)
	if j < 0 {
		return ""
	}
	return resto[:j]
}

// ── (a) EL TECHO DE SERVICIOS ES UNA PERILLA, NO UN NÚMERO CLAVADO EN EL BINARIO ────────────
//
// El techo por proyecto ya evitaba que un tenant grande dejara ciego a uno chico, pero seguía
// siendo la constante 2000: un proyecto con más servicios legítimos sólo se arreglaba
// RECOMPILANDO el cerebro, y mientras tanto sus servicios de más no tenían serie —así que
// `ServicioCaido` no los cubría, que desde afuera se ve igual que todo bien.
//
// LA PRUEBA MIRA LO QUE DECIDE: cuántas líneas `musubi_fleet_service_up` salen de verdad por
// /metrics, y cuánto vale `musubi_fleet_export_truncated{kind="services"}`. No mira el texto del
// comentario ni el HELP: un HELP que nombre la perilla con el corte hecho a 2000 igual dejaría a
// media flota sin serie.
//
// Y ENTRA POR ConfigurarFlota, que es el camino real (config.yaml → EffectiveServicesPerProject
// Export → s.techoServiciosPorProyecto → las dos bocas). Sabotaje que la pone roja: borrar la
// línea `s.techoServiciosPorProyecto = cfg.EffectiveServicesPerProjectExport()` de
// ConfigurarFlota — el campo se queda en el default 2000, salen los seis servicios y la serie de
// truncado dice 0.
func TestElTechoDeServiciosLoDecideLaConfiguracionYNoLaConstante(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 6, ahora)

	render := func() string {
		var b strings.Builder
		renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora,
			s.sondaIntervalo, versionDePrueba, nil, s.techoServiciosPorProyecto)
		return b.String()
	}

	// CON LA PERILLA EN 3, salen 3 y el exportador lo declara truncado.
	if err := s.ConfigurarFlota(config.FleetConfig{ServicesPerProjectExport: 3}); err != nil {
		t.Fatalf("ConfigurarFlota con techo 3: %v", err)
	}
	if s.techoServiciosPorProyecto != 3 {
		t.Fatalf("la config dijo 3 y el servidor quedó con %d: la perilla no llega al exportador", s.techoServiciosPorProyecto)
	}
	salida := render()
	if n := len(lineasDe(salida, "musubi_fleet_service_up{")); n != 3 {
		t.Errorf("con `services_per_project_export: 3` salieron %d series de servicio y tenían que salir 3: el techo configurado no gobierna el corte", n)
	}
	if !strings.Contains(salida, nombreExportTruncado+`{kind="services"} 1`) {
		t.Errorf("se cortó por el techo de servicios y %s{kind=\"services\"} no vale 1:\n%s", nombreExportTruncado, salida)
	}

	// CON LA PERILLA EN NEGATIVO (sin techo) salen los seis y NO hay truncado. El 0 acá es una
	// medición —«no se cortó»— y no un «no pude medir»: por eso se exige el 0 explícito y no la
	// ausencia de la línea.
	if err := s.ConfigurarFlota(config.FleetConfig{ServicesPerProjectExport: -1}); err != nil {
		t.Fatalf("ConfigurarFlota sin techo: %v", err)
	}
	if s.techoServiciosPorProyecto != 0 {
		t.Fatalf("`services_per_project_export: -1` tenía que dar 0 (sin techo) y dio %d", s.techoServiciosPorProyecto)
	}
	salida = render()
	if n := len(lineasDe(salida, "musubi_fleet_service_up{")); n != 6 {
		t.Errorf("con el techo desactivado salieron %d series de servicio y tenían que salir 6", n)
	}
	if !strings.Contains(salida, nombreExportTruncado+`{kind="services"} 0`) {
		t.Errorf("sin techo la serie tiene que decir 0 (medido y completo), no desaparecer:\n%s", salida)
	}
}

// ── (b) EL ORDEN QUE SE CALCULA NO SE PUEDE PERDER DESPUÉS ──────────────────────────────────
//
// `devicesVisiblesParaMetricas` ordena las máquinas por (proyecto, nombre) con un `sort.Slice`.
// El barrido de servicios las re-agrupaba en un `map[string][]fleet.Device` y las recorría con un
// `range` sobre el map — que en Go arranca en un bucket al azar en CADA llamada. O sea: se
// ordenaba y después se tiraba el orden. Eso es peor que no ordenar, porque parece hecho: el sort
// está ahí, a la vista, y el bloque de servicios de /metrics salía barajado en cada scrape.
//
// ESTA PRUEBA EJERCITA LA SALIDA FINAL Y NO EL SORT. Mirar `devicesVisiblesParaMetricas` habría
// pasado en verde con el bug puesto, porque el sort nunca estuvo roto: lo que estaba roto era lo
// que pasaba DESPUÉS. Se asserta sobre las líneas `musubi_fleet_service_*` que salen del
// exposition format, que es lo único que ve quien depura un export con un diff.
//
// Sabotaje que la pone roja: volver `for _, proy := range orden` a
// `for proy, devices := range porProyecto` en serviciosVisiblesParaMetricas.
func TestElOrdenDeLosProyectosSobreviveAlReagrupadoYNoSoloAlSort(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Ocho proyectos: con menos, un `range` sobre el map podría salir ordenado de casualidad
	// demasiado seguido y la prueba sería un coin flip. Con 8 hay 40.320 órdenes posibles.
	const proyectos = 8
	for i := 0; i < proyectos; i++ {
		proy := fmt.Sprintf("tenant-%02d", i)
		d := maquinaConMuestra(t, s, proy, "server", *muestraDePrueba(), ahora)
		serviciosDePrueba(t, s, d, 2, ahora)
	}

	render := func() string {
		var b strings.Builder
		renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora,
			s.sondaIntervalo, versionDePrueba, nil, s.techoServiciosPorProyecto)
		return b.String()
	}

	primeraSalida := ""
	// Se repite porque el orden de un map de Go es aleatorio POR RECORRIDO: una sola pasada puede
	// salir ordenada por casualidad. Doce pasadas contra 40.320 órdenes hacen que el falso verde
	// sea imposible en la práctica.
	for intento := 0; intento < 12; intento++ {
		salida := render()
		lineas := lineasDe(salida, "musubi_fleet_service_up{")
		if len(lineas) != proyectos*2 {
			t.Fatalf("intento %d: salieron %d series de servicio y tenían que salir %d", intento, len(lineas), proyectos*2)
		}
		anterior := ""
		for _, l := range lineas {
			proy := proyectoDeLaLinea(l)
			if proy == "" {
				t.Fatalf("intento %d: una serie de servicio salió sin etiqueta project: %q", intento, l)
			}
			if proy < anterior {
				t.Fatalf("intento %d: el bloque de servicios salió con %q después de %q. Se ordena por (proyecto, nombre) y el re-agrupado en el map TIRA ese orden: un diff entre dos /metrics —que es como se depura un export— no sirve para nada.", intento, proy, anterior)
			}
			anterior = proy
		}
		// Y ADEMÁS TIENE QUE SER EL MISMO BLOQUE, byte por byte, entre scrapes.
		bloque := strings.Join(lineas, "\n")
		if primeraSalida == "" {
			primeraSalida = bloque
		} else if bloque != primeraSalida {
			t.Fatalf("intento %d: el bloque de servicios cambió entre dos renders con la MISMA base:\n--- antes ---\n%s\n--- ahora ---\n%s", intento, primeraSalida, bloque)
		}
	}
}

// ── (c) CADA TECHO AVISA POR SÍ MISMO, Y NOMBRA SU PROPIA PERILLA ───────────────────────────
//
// El empuje fusionaba las dos mitades (`truncado = truncado || truncadoSvs`) y su único aviso
// decía «se barrieron los primeros proyectos y hay más», imprimiendo `proyectosParaExportar`.
// Cuando el que cortaba era el techo de SERVICIOS —el que se cruza de verdad, y el único de los
// dos que tiene perilla— el operador leía la causa equivocada: miraba cuántos tenants tenía, veía
// tres, y descartaba el aviso. Un aviso que nombra el techo equivocado es PEOR que no avisar.
//
// LA ASERCIÓN ES SOBRE CUÁL TECHO NOMBRA EL MENSAJE, no sobre que haya un mensaje: se exige que
// diga `services_per_project_export` y se PROHÍBE que hable de proyectos por scrape. Con el bug
// puesto hay mensaje, y por eso una prueba de «¿avisó?» quedaba en verde.
//
// Sabotaje que la pone roja: volver el aviso a uno solo con el texto de proyectos, o volver a
// fusionar las dos mitades en un bool.
func TestElAvisoDelEmpujeNombraElTechoQueCortoYNoElOtro(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	// UN SOLO proyecto: el techo de PROYECTOS no se cruza ni de casualidad, así que lo único que
	// puede cortar es el de servicios.
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 5, ahora)
	s.techoServiciosPorProyecto = 2

	_, _, truncado, err := armarPayloadOTLP(s.engine, ptrPrincipal(principalDePrometheus()), ahora,
		s.sondaIntervalo, versionDePrueba, s.techoServiciosPorProyecto)
	if err != nil {
		t.Fatal(err)
	}
	if !truncado.Servicios {
		t.Fatal("cinco servicios con techo 2 tenían que dar truncado por SERVICIOS")
	}
	if truncado.Proyectos {
		t.Fatal("con un solo proyecto el techo de PROYECTOS no se cruzó, y el barrido dice que sí: las dos mitades siguen fusionadas")
	}

	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	s.empujarUnaVez(context.Background(), ahora)
	restaurar()
	texto := log.String()

	if !strings.Contains(texto, "services_per_project_export") {
		t.Errorf("cortó el techo de SERVICIOS y el aviso no nombra su perilla (`fleet.services_per_project_export`); quien lo lea no sabe qué subir:\n%s", texto)
	}
	if strings.Contains(texto, "proyectos_por_scrape") || strings.Contains(texto, "primeros PROYECTOS") {
		t.Errorf("cortó el techo de SERVICIOS y el aviso habla del de PROYECTOS: manda a la perilla equivocada, que es peor que no avisar:\n%s", texto)
	}
	if _, avisado := s.avisosDados.Load("empuje_truncado_servicios"); !avisado {
		t.Error("no quedó registrado el aviso del techo de servicios")
	}
	if _, avisado := s.avisosDados.Load("empuje_truncado_proyectos"); avisado {
		t.Error("se registró el aviso del techo de PROYECTOS sin que ese techo se cruzara: una sola clave para los dos techos hace que el primero en cortar deje mudo al segundo")
	}

	// Y AL REVÉS: con el techo de servicios desactivado y más proyectos que `proyectosParaExportar`,
	// el aviso tiene que ser el de proyectos y NO el de servicios.
	s2 := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	s2.techoServiciosPorProyecto = 0 // sin techo de servicios: sólo puede cortar el de proyectos
	for i := 0; i < proyectosParaExportar+1; i++ {
		maquinaConMuestra(t, s2, fmt.Sprintf("tenant-%03d", i), "server", *muestraDePrueba(), ahora)
	}
	log.Reset()
	restaurar = logx.Capturar(&log)
	s2.empujarUnaVez(context.Background(), ahora)
	restaurar()
	texto = log.String()

	if !strings.Contains(texto, "proyectos_por_scrape") {
		t.Errorf("cortó el techo de PROYECTOS y el aviso no lo nombra:\n%s", texto)
	}
	if strings.Contains(texto, "services_per_project_export") {
		t.Errorf("cortó el techo de PROYECTOS y el aviso ofrece la perilla de SERVICIOS, que no arregla nada:\n%s", texto)
	}
	if _, avisado := s2.avisosDados.Load("empuje_truncado_servicios"); avisado {
		t.Error("se registró el aviso del techo de servicios con ese techo desactivado")
	}
}
