package mcp

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// sembrarPendientes abre `n` solicitudes pendientes sobre una máquina, la más vieja primero.
func sembrarPendientes(t *testing.T, s *McpServer, proyecto, deviceID string, n int, ahora time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
			DeviceID: deviceID, ProjectID: proyecto, Solicitante: "mirador", Capacidad: fleet.CapScreen,
			Creada: ahora.Add(time.Duration(i-n) * time.Minute), Vence: ahora.Add(fleet.VentanaDeAprobacion),
		}); err != nil {
			t.Fatalf("sembrar la solicitud %d de %d: %v", i+1, n, err)
		}
	}
}

// tieneKind dice si `xs` contiene `k`. Local a propósito: el homónimo del repo vive en
// `internal/codeintel`, que este paquete no importa.
func tieneKind(xs []string, k string) bool {
	for _, x := range xs {
		if x == k {
			return true
		}
	}
	return false
}

// kindsEnUno devuelve los `kind` de `musubi_fleet_export_truncated` que valen 1.
func kindsEnUno(salida string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`musubi_fleet_export_truncated\{kind="([a-z_]+)"\} 1`).FindAllStringSubmatch(salida, -1) {
		out = append(out, m[1])
	}
	return out
}

// UNA PÁGINA LLENA DE SOLICITUDES QUE LA CREDENCIAL NO VE NO PUEDE SALIR COMO UN CERO MUDO (A124).
//
// Es el TERCER techo del exportador y el único que no recorta: HACE DESAPARECER EL HECHO. El tope
// lo aplica el almacén sobre el PROYECTO ENTERO (`ORDER BY creada ASC LIMIT ?`) y la compuerta por
// máquina corre después, ya en el exportador. Así que una credencial acotada a pocas máquinas de
// un proyecto con más pendientes que el techo se lleva una página donde no ve NINGUNA, y emitía
// `musubi_fleet_approval_pending 0` — con el HELP de esa misma serie diciendo, con todas las
// letras, «0 = no hay ninguna esperando». Los otros dos techos dejan menos series; éste deja una
// serie que AFIRMA algo falso.
//
// SE MIDE EN LAS DOS DIRECCIONES, y la segunda es la que hace que la guarda valga: sin ella la
// satisfaría un `kind="approvals"` clavado en 1, que avisaría siempre y por lo tanto nunca.
//
// Sabotaje que la hace fallar: no encender la dimensión cuando la página vuelve llena.
// arnes: archivo="internal/mcp/fleet_prometheus.go"
// arnes: de="if techo > 0 && len(pendientes) >= techo {"
// arnes: a="if false {"
func TestUnaPaginaLlenaDeAprobacionesInvisiblesNoSaleComoUnCeroMudo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now().UTC()
	maquinaConMuestra(t, s, "casa", "visible", *muestraDePrueba(), ahora)
	maquinaConMuestra(t, s, "casa", "reservada", *muestraDePrueba(), ahora)
	reservada := devicePorNombreEnPrueba(t, s, "casa", "reservada")

	// Un principal que SÓLO ve `visible`, como el de la guarda de la compuerta.
	acotado := &Principal{
		Name: "panel", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"visible"}},
	}
	const techo = 3
	sembrarPendientes(t, s, "casa", reservada.ID, techo+1, ahora)

	var b strings.Builder
	renderFlota(&b, s.engine, acotado, ahora, s.sondaIntervalo, versionDePrueba, nil, serviciosPorProyectoDefault, techo)
	salida := b.String()

	// El 0 sigue siendo lo correcto para esta credencial: no ve ninguna de esas solicitudes. Lo
	// que cambia es que ahora tiene al lado quien diga que ese 0 no es una medición.
	if !strings.Contains(salida, nombreAprobPendientes+`{project="casa"} 0`) {
		t.Fatalf("se esperaba el conteo en 0 (la credencial no ve esas máquinas); salió:\n%s",
			strings.Join(lineasDe(salida, nombreAprobPendientes+"{"), "\n"))
	}
	unos := kindsEnUno(salida)
	if !tieneKind(unos, "approvals") {
		t.Errorf("la página volvió LLENA (%d pendientes con techo %d) y ninguna de esas solicitudes es visible, "+
			"pero el export no levanta `kind=\"approvals\"`: el 0 de %s se lee como «no hay nadie esperando» "+
			"cuando significa «no sé». kinds en 1: %v", techo+1, techo, nombreAprobPendientes, unos)
	}

	// ── LA OTRA DIRECCIÓN: con la página a medio llenar, la dimensión NO se enciende.
	var holgado strings.Builder
	renderFlota(&holgado, s.engine, acotado, ahora, s.sondaIntervalo, versionDePrueba, nil, serviciosPorProyectoDefault, techo+10)
	if u := kindsEnUno(holgado.String()); tieneKind(u, "approvals") {
		t.Errorf("con techo %d y sólo %d pendientes la página NO volvió llena, y el export igual dice que recortó: "+
			"una dimensión que avisa siempre no avisa nunca. kinds en 1: %v", techo+10, techo+1, u)
	}
}

// TODA DIMENSIÓN DEL TRUNCADO TIENE SU PROPIO PUNTO, Y ESO SE DERIVA DEL TIPO — NO SE ENUMERA.
//
// La guarda vecina lista los `kind` a mano (`[]string{"projects", "services"}`) y por eso cubría
// DOS DE CUATRO: `unreadable` se agregó sin que nadie la tocara, y `approvals` tampoco la habría
// movido. Enumerar formas no converge: la lista se queda corta justo cuando aparece la forma
// nueva, que es exactamente cuando hace falta.
//
// Acá el conjunto sale de `truncadoDeExport` por reflexión —un campo es una dimensión recortable—
// y además se exige que encender UN campo encienda EXACTAMENTE UN punto. Eso último es lo que
// prueba que ninguna dimensión comparte serie con otra, que es como empezó todo esto: los dos
// techos fusionados en un solo bool, con el aviso nombrando el que no había cortado.
//
// Sabotaje que la hace fallar: que dos dimensiones compartan punto.
// arnes: archivo="internal/mcp/fleet_prometheus.go"
// arnes: de="unoSi(t.Aprobaciones))"
// arnes: a="unoSi(t.Servicios))"
func TestCadaDimensionDelTruncadoTieneSuPropioPuntoYSeDerivaDelTipo(t *testing.T) {
	tipo := reflect.TypeOf(truncadoDeExport{})
	if tipo.NumField() < 3 {
		t.Fatalf("truncadoDeExport declara %d campos: esta guarda está midiendo el vacío", tipo.NumField())
	}

	var apagado strings.Builder
	renderTruncado(&apagado, truncadoDeExport{}, serviciosPorProyectoDefault, aprobacionesPorProyectoDefault)
	puntos := regexp.MustCompile(`musubi_fleet_export_truncated\{kind="([a-z_]+)"\}`).FindAllStringSubmatch(apagado.String(), -1)
	if len(puntos) != tipo.NumField() {
		var kinds []string
		for _, p := range puntos {
			kinds = append(kinds, p[1])
		}
		t.Fatalf("`truncadoDeExport` declara %d dimensiones recortables y la serie emite %d puntos (%v): "+
			"una dimensión sin punto es un recorte que ninguna alerta puede ver",
			tipo.NumField(), len(puntos), kinds)
	}

	for i := 0; i < tipo.NumField(); i++ {
		v := reflect.New(tipo).Elem()
		v.Field(i).SetBool(true)
		var b strings.Builder
		renderTruncado(&b, v.Interface().(truncadoDeExport), serviciosPorProyectoDefault, aprobacionesPorProyectoDefault)
		if u := kindsEnUno(b.String()); len(u) != 1 {
			t.Errorf("con sólo `%s` en true el export levanta %d puntos (%v) y tiene que levantar exactamente 1: "+
				"dos dimensiones que comparten punto mandan a mover la perilla equivocada",
				tipo.Field(i).Name, len(u), u)
		}
	}
}

// EL TECHO QUE CORTA ES EL DE LA PERILLA, NO LA CONSTANTE — MEDIDO POR LA BOCA DE VERDAD.
//
// Entra por el principio y sale por el final: texto YAML → config.Config → ConfigurarFlota →
// servidor → GET /metrics por HTTP → líneas del exposition format. Es el mismo recorrido que su
// hermana de servicios, y existe por el mismo motivo: una guarda que llama a `renderFlota`
// pasándole el techo por parámetro prueba la FUNCIÓN y no el CABLE, así que cambiar http.go por
// la constante dejaba la suite entera en verde mientras la perilla no cambiaba nada en producción.
//
// El valor de prueba NO puede ser ninguno de los dos defaults, y eso se verifica en vez de
// suponerse: con 200 a los dos lados, un cable roto y uno sano se escriben igual.
//
// Sabotaje que la pone roja: en http.go, pasar la constante en vez del campo del servidor.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="s.techoServiciosPorProyecto, s.techoAprobacionesPorProyecto)"
// arnes: a="s.techoServiciosPorProyecto, aprobacionesPorProyectoDefault)"
func TestElTechoDeAprobacionesQueCortaEsElDeLaPerillaYNoLaConstante(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now().UTC()
	maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)

	const raro = 137 // ni el default del config ni la constante centinela del exportador
	if raro == aprobacionesPorProyectoDefault || raro == config.Default().Fleet.EffectiveApprovalsPerProjectExport() {
		t.Fatalf("el valor de prueba %d coincide con un default: la guarda no podría distinguir un cable roto de uno sano", raro)
	}
	if err := s.ConfigurarFlota(flotaDesdeYAML(t, "fleet:\n  approvals_per_project_export: 137\n")); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}

	salida := metricsDeVerdad(t, s)
	esperado := nombreTecho + `{kind="approvals"} ` + itoa(raro)
	if !strings.Contains(salida, esperado) {
		t.Errorf("el techo de aprobaciones que llega a /metrics no es el de la perilla.\n  esperaba: %s\n  salió   :\n%s\n"+
			"si dice %d, alguien pasó la constante donde iba el campo del servidor",
			esperado, strings.Join(lineasDe(salida, nombreTecho+"{"), "\n"), aprobacionesPorProyectoDefault)
	}
}
