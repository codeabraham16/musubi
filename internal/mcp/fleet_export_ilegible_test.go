package mcp

// fleet_export_ilegible_test.go custodia que EL CERO DEL EXPORTADOR SIGNIFIQUE «MEDÍ Y NO CORTÉ»
// y nunca «no pude medir».
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, MEDIDO
//
// Los cuatro barridos del exportador tenían la misma forma:
//
//	x, err := engine.Loquesea(proy, …)
//	if err != nil {
//	    continue // un proyecto ilegible no puede tumbar el scrape entero
//	}
//
// El comentario es correcto y la consecuencia no estaba escrita en ningún lado: ese proyecto NO
// SE EXPORTA, así que sus máquinas y sus servicios se quedan sin serie, así que `ServicioCaido` y
// `MaquinaCaida` no tienen a qué matchear — y mientras tanto la única serie que habla de lo que
// quedó afuera (`musubi_fleet_export_truncated`) seguía emitiendo 0, que es una AFIRMACIÓN: «no
// se recortó nada». Medido antes del arreglo, con el proyecto «casa» ilegible y sus 4 servicios:
//
//	musubi_fleet_export_truncated{kind="projects"} 0
//	musubi_fleet_export_truncated{kind="services"} 0
//
// «Fallé» y «no hubo corte» eran el mismo valor, y el que se lee en un panel es el segundo.
//
// EL ARREGLO NO ES ROMPER EL SCRAPE: es un tercer `kind`. `kind="unreadable"` vale 1 mientras
// haya una parte de la flota que no se pudo leer, y la alerta que ya existe (`ExportacionTruncada`,
// `musubi_fleet_export_truncated == 1`, sin filtro de kind) lo levanta sin tocar la regla.
//
// LOS CUATRO HERMANOS ESTÁN CUBIERTOS ACÁ, uno por caso: la lista de proyectos, las máquinas de
// un proyecto, sus servicios y sus aprobaciones. Tres de los cuatro mentían con un cero; el de
// aprobaciones omitía la línea, que es la otra cara de lo mismo —una serie ausente tampoco
// dispara nada—.

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// almacenQueNoSeDejaLeer hace fallar UNA de las cuatro lecturas del exportador y deja pasar el
// resto. Es la única forma de ejercitar estos caminos: son ramas de error de I/O.
type almacenQueNoSeDejaLeer struct {
	memory.StorageBackend
	rompe    string // "proyectos" | "devices" | "servicios" | "aprobaciones"
	proyecto string // para las tres que son por proyecto
}

var errAlmacenSimulado = fmt.Errorf("simulado: la base no contestó")

func (a almacenQueNoSeDejaLeer) ProyectosConDevices(tope int) ([]string, error) {
	if a.rompe == "proyectos" {
		return nil, errAlmacenSimulado
	}
	return a.StorageBackend.ProyectosConDevices(tope)
}

func (a almacenQueNoSeDejaLeer) ListarDevices(projectID string, incluirRevocados bool) ([]fleet.Device, error) {
	if a.rompe == "devices" && projectID == a.proyecto {
		return nil, errAlmacenSimulado
	}
	return a.StorageBackend.ListarDevices(projectID, incluirRevocados)
}

func (a almacenQueNoSeDejaLeer) ListarServicios(projectID, deviceID string, incluirRevocados bool) ([]fleet.Servicio, error) {
	if a.rompe == "servicios" && projectID == a.proyecto {
		return nil, errAlmacenSimulado
	}
	return a.StorageBackend.ListarServicios(projectID, deviceID, incluirRevocados)
}

func (a almacenQueNoSeDejaLeer) AprobacionesPendientes(projectID string, ahora time.Time, tope int) ([]fleet.SolicitudDeAprobacion, error) {
	if a.rompe == "aprobaciones" && projectID == a.proyecto {
		return nil, errAlmacenSimulado
	}
	return a.StorageBackend.AprobacionesPendientes(projectID, ahora, tope)
}

// Sabotaje que la pone roja: en cualquiera de los cuatro barridos, volver el `ilegible = true` a
// un `continue` mudo.
func TestUnPedazoDeLaFlotaQueNoSePudoLeerNoSeInformaComoSinRecorte(t *testing.T) {
	for _, rompe := range []string{"proyectos", "devices", "servicios", "aprobaciones"} {
		t.Run(rompe, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			ahora := time.Now()
			d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
			serviciosDePrueba(t, s, d, 4, ahora)
			// Un segundo proyecto SANO: el scrape tiene que seguir exportándolo. Un exportador
			// que se cae entero ante un error es peor que uno que recorta.
			otra := maquinaConMuestra(t, s, "oficina", "pc-otra", *muestraDePrueba(), ahora)
			serviciosDePrueba(t, s, otra, 2, ahora)

			eng := almacenQueNoSeDejaLeer{StorageBackend: s.engine, rompe: rompe, proyecto: "casa"}

			var log bytes.Buffer
			restaurar := logx.Capturar(&log)
			var b strings.Builder
			renderFlota(&b, eng, ptrPrincipal(principalDePrometheus()), ahora,
				s.sondaIntervalo, versionDePrueba, nil, s.techoServiciosPorProyecto)
			restaurar()
			salida := b.String()

			// (a) LA SERIE LO DICE. Es lo único que llega a una alerta: un comentario `#` lo
			//     descarta el parser y un log no lo mira nadie a las 3 de la mañana.
			if !strings.Contains(salida, nombreExportTruncado+`{kind="unreadable"} 1`) {
				t.Errorf("no se pudo leer %q del proyecto «casa» y el export no lo declara. Sus series no salen, así que ninguna alerta las cubre, y lo único que se ve desde afuera es un cero que dice «no hubo recorte»:\n%s",
					rompe, bloqueDeTruncado(salida))
			}

			// (b) Y NO SE DISFRAZA DE OTRA COSA. Los otros dos `kind` son techos, se arreglan con
			//     un número, y este problema no: mandarlo por ahí manda a subir una perilla que
			//     no cambia nada.
			if strings.Contains(salida, nombreExportTruncado+`{kind="services"} 1`) ||
				strings.Contains(salida, nombreExportTruncado+`{kind="projects"} 1`) {
				t.Errorf("una lectura que falló se está informando como un TECHO cruzado:\n%s", bloqueDeTruncado(salida))
			}

			// (c) EL LOG DICE QUÉ PASÓ. La serie avisa que algo no se leyó; el log es lo único
			//     que dice qué proyecto y con qué error.
			if !strings.Contains(log.String(), "export de flota:") {
				t.Errorf("la lectura de %q falló y el log no dice nada:\n%s", rompe, log.String())
			}

			// (d) EL RESTO DE LA FLOTA SIGUE SALIENDO. Salvo cuando lo que falla es la LISTA DE
			//     PROYECTOS, que no deja nada en pie — y por eso ese caso es el más grave de los
			//     cuatro: un export entero en cero se lee igual que una flota apagada.
			if rompe != "proyectos" && !strings.Contains(salida, `project="oficina"`) {
				t.Errorf("un proyecto ilegible se llevó puesto el scrape del proyecto sano:\n%s", salida)
			}
		})
	}
}

// EL HERMANO ES LA OTRA BOCA. El empuje comparte los barridos, así que tiene que llegar al mismo
// hecho; si no, el operador ve el problema o no según por dónde mire, que es exactamente lo que
// el tipo compartido `truncadoDeExport` existe para impedir.
func TestElEmpujeTambienDeclaraLoQueNoSePudoLeer(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 4, ahora)

	eng := almacenQueNoSeDejaLeer{StorageBackend: s.engine, rompe: "servicios", proyecto: "casa"}
	_, _, truncado, err := armarPayloadOTLP(eng, ptrPrincipal(principalDePrometheus()), ahora,
		s.sondaIntervalo, versionDePrueba, s.techoServiciosPorProyecto)
	if err != nil {
		t.Fatal(err)
	}
	if !truncado.Ilegible {
		t.Error("el empuje no se enteró de que un proyecto no se pudo leer: las dos bocas comparten el barrido, así que el hecho tiene que llegar a las dos")
	}
	if truncado.Servicios {
		t.Error("el empuje informó un TECHO de servicios cruzado cuando lo que hubo fue un error de lectura: manda a subir una perilla que no arregla nada")
	}
}

// Y EL CERO TIENE QUE SEGUIR SIENDO UN CERO CUANDO SE MIDIÓ. Sin este caso, «emitir siempre 1»
// pasaría las dos pruebas de arriba y el kind nuevo no distinguiría nada.
func TestConTodoLegibleElExportDeclaraQueNoHuboNadaIlegible(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 4, ahora)

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora,
		s.sondaIntervalo, versionDePrueba, nil, s.techoServiciosPorProyecto)
	if !strings.Contains(b.String(), nombreExportTruncado+`{kind="unreadable"} 0`) {
		t.Errorf("con todo legible la serie tiene que decir 0 —una medición— y no desaparecer:\n%s", bloqueDeTruncado(b.String()))
	}
}

// LA ALERTA QUE YA EXISTE TIENE QUE LEVANTAR EL KIND NUEVO. Agregar un punto que ninguna regla
// mira es agregar un dato a un archivo: `ExportacionTruncada` compara la serie ENTERA contra 1,
// sin filtrar por kind, y esta guarda impide que alguien la acote a los dos kinds viejos «para
// ser más preciso» y deje al tercero mudo.
func TestLaAlertaDeTruncadoNoFiltraPorKindYPorEsoCubreAlIlegible(t *testing.T) {
	crudo, err := os.ReadFile("../../deploy/musubi-alerts.yml")
	if err != nil {
		t.Fatalf("no se pudo leer las reglas: %v", err)
	}
	reglas := string(crudo)
	i := strings.Index(reglas, "alert: ExportacionTruncada")
	if i < 0 {
		t.Fatal("no está la alerta ExportacionTruncada en deploy/musubi-alerts.yml")
	}
	expr := ""
	for _, l := range strings.Split(reglas[i:], "\n") {
		if s := strings.TrimSpace(l); strings.HasPrefix(s, "expr:") {
			expr = strings.TrimSpace(strings.TrimPrefix(s, "expr:"))
			break
		}
	}
	if expr == "" {
		t.Fatal("ExportacionTruncada no tiene expr")
	}
	if !strings.Contains(expr, nombreExportTruncado) {
		t.Fatalf("la alerta ya no mira %s: %q", nombreExportTruncado, expr)
	}
	if strings.Contains(expr, "kind=") {
		t.Errorf("la alerta filtra por kind (%q): el día que se agregue un cuarto motivo de recorte va a nacer mudo, que es como nació `kind=\"unreadable\"` en el diseño original", expr)
	}
}
