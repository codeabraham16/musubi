package mcp

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// EL RECORTE DEL EXPORTADOR TIENE QUE SER UNA SERIE, NO UN COMENTARIO.
//
// Ola 0 del plan empresa. El exportador tiene tres techos y los tres avisaban con una línea que
// empieza con `#`, o sea que Prometheus la DESCARTA al parsear. Estaban escritas para una persona
// que abriera /metrics a mano, y nadie abre /metrics a mano: el sistema sabía que había recortado
// la cobertura y no había forma de que ese hecho llegara a una alerta.
//
// Y el recorte no es cosmético: lo que queda afuera NO tiene serie, así que `ServicioCaido` no
// puede dispararse sobre esos servicios. Es cobertura que desaparece en silencio justo cuando la
// flota crece.
//
// Sabotaje que la hace fallar: sacar el `renderTruncado` de renderFlota; o emitir la serie sólo
// cuando vale 1 (una serie que sólo existe cuando hay problema no se distingue de «el exportador
// no corrió»).
func TestElTruncadoDelExportadorSaleComoSerieYNoComoComentario(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, nil)
	salida := b.String()

	// SIN recorte las dos series existen y valen 0. El 0 es un hecho medido —«no se truncó»—, no
	// un «no sé»: por eso acá sí corresponde emitirlo, al revés que en las series de máquina.
	for _, kind := range []string{"projects", "services"} {
		linea := nombreExportTruncado + `{kind="` + kind + `"} 0`
		if !strings.Contains(salida, linea) {
			t.Errorf("falta %q en el export: una serie que sólo aparece cuando hay problema no se distingue de que el exportador no corrió", linea)
		}
	}
	if !strings.Contains(salida, "# TYPE "+nombreExportTruncado+" gauge") {
		t.Error("la serie sale sin TYPE: Prometheus la toma como untyped y el runbook la nombra como gauge")
	}
}

// Y EL TECHO DE SERVICIOS SE CUENTA POR PROYECTO, NO SOBRE LA FLOTA ENTERA.
//
// Con un techo total, el primer tenant barrido se comía el cupo y los demás quedaban sin series
// —cobertura perdida en un cliente por culpa del tamaño de OTRO cliente, y sin que ninguno de los
// dos se enterara—. Medido el 2026-09-03: dos máquinas reales ya reportan 121 servicios entre las
// dos, así que el total de 2000 se cruzaba a ~35 máquinas.
//
// Sabotaje que la hace fallar: volver el contador a uno global (contar sobre `out` en vez de
// sobre `contadoPorProyecto[d.ProjectID]`).
func TestElTechoDeServiciosEsPorProyectoYNoDejaCiegoAlOtroTenant(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// LOS NOMBRES IMPORTAN Y NO ES CASUALIDAD: los proyectos se barren ORDENADOS, así que el
	// grande tiene que ir PRIMERO para que un techo global le coma el cupo al chico. Con los
	// nombres al revés esta prueba pasaba en verde aunque el techo fuera global —el chico entraba
	// antes de que se llenara— y no custodiaba nada. Lo cazó el sabotaje.
	maquinaConMuestra(t, s, "aaa-grande", "server-grande", *muestraDePrueba(), ahora)
	maquinaConMuestra(t, s, "zzz-chico", "server-chico", *muestraDePrueba(), ahora)

	dGrande, _, _ := s.engine.DevicePorNombre("aaa-grande", "server-grande")
	dChico, _, _ := s.engine.DevicePorNombre("zzz-chico", "server-chico")

	// El proyecto grande satura su propio techo; el chico reporta uno solo.
	var muchos []fleet.ReporteServicio
	for i := 0; i < serviciosPorExportar+5; i++ {
		muchos = append(muchos, fleet.ReporteServicio{
			Nombre: "svc-" + string(rune('a'+i%26)) + "-" + itoaCorto(i),
			Salud:  fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo},
		})
	}
	if _, _, err := s.engine.ReportarServicios(dGrande.ID, ahora, muchos); err != nil {
		t.Fatalf("no se pudieron reportar los servicios del proyecto grande: %v", err)
	}
	if _, _, err := s.engine.ReportarServicios(dChico.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "el-unico-del-chico", Salud: fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoCorriendo}},
	}); err != nil {
		t.Fatalf("no se pudieron reportar los servicios del proyecto chico: %v", err)
	}

	svs, truncado := serviciosVisiblesParaMetricas(s.engine, devicesDeTodos(t, s))
	if !truncado {
		t.Fatal("el proyecto grande pasó su techo y el exportador no lo declaró truncado")
	}
	var vioAlChico bool
	for _, e := range svs {
		if e.sv.Nombre == "el-unico-del-chico" {
			vioAlChico = true
		}
	}
	if !vioAlChico {
		t.Error("el único servicio del proyecto chico quedó afuera porque otro tenant llenó el cupo: con un techo compartido, un cliente grande deja ciego al chico y ninguno de los dos se entera")
	}
}

func itoaCorto(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func devicesDeTodos(t *testing.T, s *McpServer) []fleet.Device {
	t.Helper()
	vistos, _ := devicesVisiblesParaMetricas(s.engine, ptrPrincipal(principalDePrometheus()))
	return vistos
}
