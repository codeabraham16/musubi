package mcp

// A131 · T3 — UNA POLÍTICA ES SOBRE LAS MÁQUINAS QUE NOMBRA Y SOBRE EL SERVICIO QUE NOMBRA, Y SOBRE
// NADA MÁS.
//
// El ALCANCE de una política son dos preguntas: a qué máquinas llega su `devices:`, y —en las de
// servicio— a qué servicio de cada una. Las guardas de antes lo decían bien y lo medían con UN par
// de nombres que no compartían nada (`nas` contra `pc-gio`, `sshd` contra `nombre-mal-escrito`), con
// UNA condición de host, con UN estado del inventario de servicios (existe, con otro nombre) y con
// UN caso de cada lado del conteo del inventario de máquinas (cero políticas en todo el cerebro, o
// una sobre `*`). Seis mutaciones de A131 dejaban el paquete entero en verde.
//
// Las dos tablas de este archivo recorren los ejes enteros, y entran por el barrido que corre en
// producción (aplicarPoliticas): las formas de parecerse sin ser, cada condición declarada (leída
// del AST), los cuatro lugares que leen un selector —el barrido que actúa, el inventario que lo
// cuenta, la compuerta de las concesiones y el informe del rename— y cada forma en que un servicio
// puede faltar. Los hechos contra los que comparan están ESCRITOS en cada fila: no se derivan de
// Alcanza ni de ServicioEn, que son lo que miden. Las mismas formas, contra las funciones del
// dominio solas, las recorren las tablas de internal/fleet/politica_alcance_test.go.

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// disparoDeCondicion es cómo se CUMPLE una condición sobre una máquina: el umbral de la política, el
// servicio que mira (vacío en las de host), el comando que corre, y cómo dejar la condición cumplida
// en `ahora` —una muestra fresca para las de host, un inventario fresco para las de servicio—.
type disparoDeCondicion struct {
	servicio string
	supera   float64
	hacer    []string
	preparar func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time)
}

// disparosPorCondicion dice cómo se cumple CADA condición. Es UNA tabla para todas las pruebas que
// recorren las condiciones —la del origen de A131·T5 y la del alcance de A131·T3—, y vive acá y no
// adentro de una de ellas porque dos copias de «cómo se cumple temp_c» se separan.
//
// No exige cubrir el conjunto: eso lo hace cada prueba que la recorre contra condicionesDeclaradas,
// con el porqué de su propio eje.
func disparosPorCondicion() map[fleet.Condicion]disparoDeCondicion {
	deHost := []string{"journalctl", "--vacuum-size=200M"}
	deServicio := []string{"systemctl", "restart", "nginx"}
	conMuestra := func(tocar func(*fleet.Muestra)) func(*testing.T, *McpServer, fleet.Device, time.Time) {
		return func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time) {
			m := muestraSana(50, ahora)
			tocar(&m)
			latir(t, s, d.ID, m, ahora)
		}
	}
	conServicio := func(salud fleet.SaludServicio) func(*testing.T, *McpServer, fleet.Device, time.Time) {
		return func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time) {
			salud.Tomada = ahora
			if _, _, err := s.engine.ReportarServicios(d.ID, ahora, []fleet.ReporteServicio{
				{Nombre: "nginx", Clase: "systemd", Salud: salud}}); err != nil {
				t.Fatalf("ReportarServicios: %v", err)
			}
		}
	}
	valor := func(v float64) *float64 { return &v }
	discoLleno := func(m *fleet.Muestra) { m.DiscoUsado, m.DiscoDisponible = 960<<30, 10<<30 }
	reinicios := 9
	return map[fleet.Condicion]disparoDeCondicion{
		fleet.CondDiscoPct:      {supera: 90, hacer: deHost, preparar: conMuestra(discoLleno)},
		fleet.CondDiscoLibrePct: {supera: 10, hacer: deHost, preparar: conMuestra(discoLleno)},
		fleet.CondMemPct:        {supera: 90, hacer: deHost, preparar: conMuestra(func(m *fleet.Muestra) { m.MemUsada = m.MemTotal / 100 * 95 })},
		fleet.CondCPUPct:        {supera: 90, hacer: deHost, preparar: conMuestra(func(m *fleet.Muestra) { m.CPUPct = valor(97) })},
		fleet.CondCargaPorCore:  {supera: 2, hacer: deHost, preparar: conMuestra(func(m *fleet.Muestra) { m.Load5 = valor(40) })},
		fleet.CondTempC:         {supera: 80, hacer: deHost, preparar: conMuestra(func(m *fleet.Muestra) { m.TempC = valor(95) })},
		fleet.CondServicioCaido: {servicio: "nginx", hacer: deServicio,
			preparar: conServicio(fleet.SaludServicio{Estado: fleet.EstadoFallado})},
		fleet.CondServicioReinicios: {servicio: "nginx", supera: 3, hacer: deServicio,
			preparar: conServicio(fleet.SaludServicio{Estado: fleet.EstadoCorriendo, Reinicios: &reinicios})},
	}
}

// condicionesConDisparo devuelve, en orden, las condiciones declaradas en internal/fleet/politica.go
// (AST) con la receta de disparosPorCondicion de cada una, y exige que la tabla las cubra todas y no
// traiga ninguna de más. `sinFila` dice qué se pierde, en el eje de quien llama, con una condición
// que la tabla no recorre.
func condicionesConDisparo(t *testing.T, sinFila string) ([]fleet.Condicion, map[fleet.Condicion]disparoDeCondicion) {
	t.Helper()
	disparos := disparosPorCondicion()
	declaradas := condicionesDeclaradas(t)
	var faltan []string
	for c, nombre := range declaradas {
		if _, ok := disparos[c]; !ok {
			faltan = append(faltan, nombre+" ("+strconv.Quote(string(c))+")")
		}
	}
	sort.Strings(faltan)
	if len(faltan) > 0 {
		t.Errorf("disparosPorCondicion no sabe cumplir %s: %s", strings.Join(faltan, ", "), sinFila)
	}
	var orden []fleet.Condicion
	for c := range disparos {
		if _, ok := declaradas[c]; !ok {
			t.Errorf("disparosPorCondicion tiene una receta para %q e internal/fleet/politica.go no la declara: mide una condición que no existe", c)
			continue
		}
		orden = append(orden, c)
	}
	sort.Slice(orden, func(i, j int) bool { return orden[i] < orden[j] })
	return orden, disparos
}

// maquinasDeLaTablaDeAlcance son las máquinas de la tabla de alcance. `davantis` y `davantis-1` son
// el par REAL de la malla donde un nombre es prefijo del otro —la laptop Linux y la PC Windows, que
// Prometheus ya vio juntas—, y `nas` no comparte nada con ellas.
var maquinasDeLaTablaDeAlcance = []string{"davantis", "davantis-1", "nas"}

// mundoDeAlcance da de alta en la casa las máquinas de la tabla, con metrics y exec, y les pone
// encima UNA política con el registro dado. Devuelve las máquinas leídas del registro, por nombre.
func mundoDeAlcance(t *testing.T, pol config.PolicyConfig, reg *PrincipalRegistry) (*McpServer, map[string]fleet.Device) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	for _, n := range maquinasDeLaTablaDeAlcance {
		enrolarConExec(t, s, "casa", n)
	}
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{pol}}); err != nil {
		t.Fatalf("ConfigurarFlota(devices=%q): %v", pol.Devices, err)
	}
	s.buscarPrincipal = reg
	return s, maquinasDelMundo(t, s)
}

// maquinasDelMundo relee del registro las máquinas de la tabla, por nombre.
func maquinasDelMundo(t *testing.T, s *McpServer) map[string]fleet.Device {
	t.Helper()
	out := map[string]fleet.Device{}
	for _, n := range maquinasDeLaTablaDeAlcance {
		d, hay, err := s.engine.DevicePorNombre("casa", n)
		if err != nil || !hay {
			t.Fatalf("%q no quedó en el registro: hay=%v err=%v", n, hay, err)
		}
		out[n] = d
	}
	return out
}

// filasDelInventario lee musubi_fleet_list como una persona con exec sobre toda la casa —la que ve
// el conteo Y el detalle de cada política—, con las filas por nombre de máquina.
func filasDelInventario(t *testing.T, s *McpServer) map[string]map[string]any {
	t.Helper()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_list: %+v", e)
	}
	crudas, _ := jsonOf(t, res)["devices"].([]any)
	out := map[string]map[string]any{}
	for _, x := range crudas {
		fila, _ := x.(map[string]any)
		nombre, _ := fila["name"].(string)
		out[nombre] = fila
	}
	return out
}

// condicionCumplida dice si la política dispararía sobre esta máquina mirando SÓLO su condición: la
// muestra para las de host, y para las de servicio el servicio que nombra, buscado acá con una
// comparación propia y exacta para no depender de ServicioEn, que es lo que se mide.
func condicionCumplida(t *testing.T, s *McpServer, pol fleet.Politica, d fleet.Device, ahora time.Time) bool {
	t.Helper()
	if !pol.EsDeServicio() {
		_, dispara := pol.Dispara(d.UltimaMuestra)
		return dispara
	}
	servicios, err := s.engine.ServiciosDeDevice(d.ID)
	if err != nil {
		t.Fatalf("ServiciosDeDevice(%s): %v", d.Name, err)
	}
	for _, sv := range servicios {
		if sv.Nombre == strings.TrimSpace(pol.Servicio) {
			_, dispara := pol.DisparaSobreServicio(sv, servicioFresco(sv, ahora))
			return dispara
		}
	}
	return false
}

// exigirLosOtrosLectoresDelSelector mide, para un `devices:` y los dos hechos de su fila, a los
// lectores del selector que no dependen de la condición: la compuerta de una concesión con el MISMO
// selector en cada capacidad que la máquina admite —armada con el parser de principals.yaml, así que
// llega recortada como llegaría de verdad— y los dos informes del rename, el de las políticas y el de
// las concesiones.
//
// No es un t.Helper A PROPÓSITO: cada aserción tiene que reportar su propia línea, que es con lo que
// el arnés distingue un sabotaje de otro.
func exigirLosOtrosLectoresDelSelector(t *testing.T, sobre []string, alcanza, nombra map[string]bool) {
	grants, err := parsearFleet("op-selector", map[string][]string{"exec": sobre, "metrics": sobre})
	if err != nil {
		t.Fatalf("parsearFleet(exec y metrics: %q): %v", sobre, err)
	}
	concesion := Principal{Name: "op-selector", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa", Fleet: grants}
	registro := registroDePrueba(concesion)
	pol := politicaDeMemoria()
	pol.Devices = sobre
	s, maquinas := mundoDeAlcance(t, pol, registro)
	for _, n := range maquinasDeLaTablaDeAlcance {
		// Las capacidades salen de la máquina y no de una lista escrita acá: cada una que admite es un
		// lugar más donde la concesión lee el selector.
		if len(maquinas[n].Caps) < 2 {
			t.Fatalf("PISO: %s admite %v, y la concesión se mide en al menos dos capacidades", n, maquinas[n].Caps)
		}
		for _, c := range maquinas[n].Caps {
			if got := PuedeSobreDevice(&concesion, maquinas[n], c); got != alcanza[n] {
				t.Errorf("una concesión `%s: %q` alcanza a %s = %v, y una política con el mismo `devices:` la alcanza = %v: "+
					"las dos gramáticas de «qué máquinas» se separaron, y la concesión le da a una persona una máquina que "+
					"no nombró (o le quita una que sí)", c, sobre, n, got, alcanza[n])
			}
		}
		if got := slices.Contains(s.politicasQueNombran(n), pol.Name); got != nombra[n] {
			t.Errorf("el informe del rename de %s lista a la política con `devices: %q` = %v, y la fila dice que la "+
				"nombra = %v: el informe avisa que se rompe algo que no se rompe, o calla lo que sí", n, sobre, got, nombra[n])
		}
		if got := slices.Contains(registro.impactoDeNombre(n).Concesiones, concesion.Name); got != nombra[n] {
			t.Errorf("el informe del rename de %s lista a la concesión `exec: %q` = %v, y la fila dice que la nombra = %v",
				n, sobre, got, nombra[n])
		}
	}
}

// A131 · T3 — UNA POLÍTICA ACTÚA Y FIGURA EXACTAMENTE SOBRE LAS MÁQUINAS QUE SU `devices:` ALCANZA,
// EN CADA CONDICIÓN, Y EL SELECTOR SE LEE IGUAL EN LOS CUATRO LUGARES QUE LO LEEN.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABAN LAS GUARDAS DE ANTES
//
//   - EL SELECTOR. TestUnaPoliticaSoloTocaLasMaquinasQueNombra pone la política sobre `nas` y mira
//     `pc-gio`: ningún parecido. Comparar por prefijo en Alcanza (P2-m4) pasaba. Y
//     TestElInventarioDiceQueActuaSoloSobreCadaMaquina usa sólo `["*"]`, que alcanza a TODA máquina:
//     sacarle al inventario el filtro de alcance (P3-m6) pasaba, y una política que sólo nombra
//     `otra-pc` aparecía, con el detalle entero, en la fila de pc-gio.
//   - LA CLASE. La primera guarda mira una política de `mem_pct`. Saltear el alcance sólo para las
//     de servicio (P2-m5: `&& !pol.EsDeServicio()`) pasaba, y una política de servicio sobre `nas`
//     reiniciaba el servicio en pc-gio.
//   - EL CONTEO. TestSinPoliticasElInventarioNoMencionaNada mira un cerebro SIN políticas. Publicar
//     `politicas_activas: 0` cuando hay políticas pero ninguna alcanza a la máquina (P4-m1: `total >
//     0 || len(s.politicas) > 0`) pasaba: el ruido que esa guarda dice evitar, en cada fila.
//
// Y UN DEFECTO VIVO DEL MISMO EJE, encontrado al mover el alcance: el barrido contaba la ventana de
// mantenimiento ANTES de mirar el alcance (vivía adentro de evaluarPolitica), así que una ventana en
// una máquina que la política ni nombra le sumaba `mantenimiento` a esa política. El contador que
// contesta «¿no actuó porque estaba en mantenimiento?» decía que sí donde nunca iba a actuar.
//
// Exposición medida (auditoría A131): el estado que vuelve vivo al selector y al conteo EXISTE hoy.
// La única política, `vaciar-journal`, está acotada a `["musubi-server"]`, y su principal
// `auto-heal` tiene `exec: ["*"]`: el `devices:` es lo único que la mantiene ahí. De las 4 máquinas
// del cerebro, 3 no están en su alcance, así que con P4-m1 3 de 4 filas de musubi_fleet_list traerían
// `politicas_activas: 0`, y con P3-m6 esas mismas 3 mostrarían a vaciar-journal con su detalle. La
// clase y la ventana, 0: no hay políticas de servicio, y el contador `mantenimiento` de vaciar-journal
// dio 0 como máximo en 30 días. La compuerta de las concesiones, 0: los 4 principals con `exec` lo
// tienen como `["*"]`.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una fila por selector, con dos HECHOS escritos: a qué máquinas ALCANZA y a cuáles NOMBRA (el
// comodín alcanza sin nombrar). Las máquinas son `davantis`, `davantis-1` y `nas`, y los selectores
// recorren el nombre exacto de cada lado del par prefijo, un prefijo de las dos, un sufijo, las
// mayúsculas, una lista y los bordes con espacios. Por fila:
//
//  1. LOS OTROS DOS LECTORES DEL SELECTOR, que no dependen de la condición: la compuerta de una
//     concesión con el MISMO selector, parseada como la de principals.yaml, alcanza exactamente donde
//     la política alcanza, en cada capacidad que la máquina admite; y los dos informes del rename
//     (políticas y concesiones) la listan como «se rompe» exactamente donde la nombra.
//  2. En CADA condición declarada (AST), con la condición cumplida en las tres máquinas: el barrido
//     actúa exactamente en las que alcanza y cuenta `ok` esas veces y nada más; el inventario publica
//     `politicas_activas` —nunca en cero— exactamente en ésas, con el detalle de la política; y con
//     las tres en una ventana de mantenimiento, el segundo barrido cuenta `mantenimiento` una vez por
//     máquina alcanzada, no por máquina del proyecto.
//
// PISO: en cada fila y condición, la condición se cumple en las tres máquinas (si no, «no actuó sobre
// ésta» no distinguiría alcance de condición); la ventana no está abierta en el primer barrido y sí
// en el segundo; y la tabla trae, para cada par de máquinas donde un nombre es prefijo del otro, una
// fila que alcanza a cada una sin la otra.
//
// Sabotaje: el barrido saltea el alcance de las políticas de servicio (P2-m5, portada: el alcance
// ya no vive en evaluarPolitica sino en el bucle de aplicarPoliticas).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\t\tif !pol.Alcanza(d.Name) {"
// arnes: a="\t\t\tif !pol.Alcanza(d.Name) && !pol.EsDeServicio() {"
//
// Sabotaje: el inventario cuenta y detalla las políticas sin mirar el alcance (P3-m6).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\tif !pol.Alcanza(d.Name) {\n\t\t\tcontinue\n\t\t}\n\t\ttotal++\n"
// arnes: a="\t\ttotal++\n"
//
// Sabotaje: el inventario publica el conteo aunque sea cero (P4-m1).
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="; total > 0 {"
// arnes: a="; total >= 0 {"
//
// Sabotaje: el barrido vuelve a contar la ventana ANTES de mirar el alcance (el defecto vivo). La
// ventana se pregunta con otras palabras (`pausada :=`) para no duplicar el `de` de
// TestElSchedulerNoAplicaPoliticasSobreUnaMaquinaEnVentana; el ancla es la misma que la del P4-m5
// de TestUnaPoliticaDeServicioDecidePorSuInventarioYNoPorLaMuestraDelHost, y las dos insertan sin
// romper la de la otra.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n"
// arnes: a="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n\t\t\tif pausada := enMantenimiento[d.ID]; pausada {\n\t\t\t\ts.metrics.contarPolitica(pol.Nombre, \"mantenimiento\")\n\t\t\t\tcontinue\n\t\t\t}\n"
//
// Sabotaje: la compuerta de las concesiones suma una regla propia que acepta prefijos: vuelve a
// haber dos gramáticas, y la de las concesiones es la floja.
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="func tieneGrant(p *Principal, c fleet.Cap, nombreDevice string) bool {\n"
// arnes: a="func tieneGrant(p *Principal, c fleet.Cap, nombreDevice string) bool {\n\tfor _, sel := range p.Fleet[c] {\n\t\tif sel != \"\" && len(sel) < len(nombreDevice) && nombreDevice[:len(sel)] == sel {\n\t\t\treturn true\n\t\t}\n\t}\n"
//
// Sabotaje: el informe del rename lee el selector con Alcanza y lista como «se rompe» una política
// sobre todas, que sobrevive a cualquier rename.
// arnes: archivo="internal/mcp/methods_renombrar.go"
// arnes: de="if fleet.SelectorNombra(sel, nombre) {"
// arnes: a="if fleet.SelectorAlcanza(sel, nombre) {"
func TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra(t *testing.T) {
	filas := []struct {
		caso    string
		sobre   []string
		alcanza []string // HECHO: donde actúa, donde figura y donde la concesión deja ejecutar
		nombra  []string // HECHO: donde el rename la lista como «se rompe»
	}{
		{"comodín", []string{"*"}, maquinasDeLaTablaDeAlcance, nil},
		{"el nombre exacto que es prefijo de otra máquina", []string{"davantis"}, []string{"davantis"}, []string{"davantis"}},
		{"el nombre exacto que extiende a otra máquina", []string{"davantis-1"}, []string{"davantis-1"}, []string{"davantis-1"}},
		{"un prefijo de las dos", []string{"davan"}, nil, nil},
		{"un sufijo", []string{"antis"}, nil, nil},
		{"el nombre en mayúsculas", []string{"DAVANTIS"}, nil, nil},
		{"una lista cuyo primer selector no nombra a nadie", []string{"otra-pc", "nas"}, []string{"nas"}, []string{"nas"}},
		{"el nombre con espacios en los bordes", []string{" davantis "}, []string{"davantis"}, []string{"davantis"}},
	}

	// PISO: por cada par de máquinas donde un nombre es prefijo del otro, una fila que alcanza a cada
	// una sin la otra. Sin ellas, la tabla no distingue la igualdad de un prefijo.
	pares := 0
	for _, a := range maquinasDeLaTablaDeAlcance {
		for _, b := range maquinasDeLaTablaDeAlcance {
			if a == b || !strings.HasPrefix(b, a) {
				continue
			}
			pares++
			for _, par := range [][2]string{{a, b}, {b, a}} {
				hay := false
				for _, f := range filas {
					if slices.Contains(f.alcanza, par[0]) && !slices.Contains(f.alcanza, par[1]) {
						hay = true
					}
				}
				if !hay {
					t.Errorf("PISO: ninguna fila alcanza a %q sin %q: la tabla no distingue la igualdad de un prefijo", par[0], par[1])
				}
			}
		}
	}
	if pares == 0 {
		t.Fatalf("PISO: entre %q no hay ningún par donde un nombre sea prefijo del otro, y es el eje que esta tabla vino a recorrer", maquinasDeLaTablaDeAlcance)
	}

	condiciones, disparos := condicionesConDisparo(t, "una condición sin receta es justo la clase por la que el "+
		"alcance puede saltearse sin que nada se ponga rojo, como pasó con las de servicio")
	// Un curador que puede correr los dos comandos de la tabla en toda la casa: lo que se mide es el
	// ALCANCE, y ninguna fila puede quedarse sin actuar por la allowlist o por la compuerta.
	curador := Principal{
		Name: "curador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"*": {"journalctl", "systemctl"}},
	}

	for _, f := range filas {
		t.Run(f.caso, func(t *testing.T) {
			alcanza := map[string]bool{}
			for _, n := range f.alcanza {
				alcanza[n] = true
			}
			nombra := map[string]bool{}
			for _, n := range f.nombra {
				nombra[n] = true
			}

			// ── 1 · LOS OTROS LECTORES DEL MISMO SELECTOR ──────────────────────────────────────
			exigirLosOtrosLectoresDelSelector(t, f.sobre, alcanza, nombra)

			// ── 2 · EL BARRIDO Y EL INVENTARIO, EN CADA CONDICIÓN ─────────────────────────────────
			for _, cond := range condiciones {
				disp := disparos[cond]
				t.Run(string(cond), func(t *testing.T) {
					pol := config.PolicyConfig{
						Name: "alcance-" + string(cond), Principal: curador.Name, When: string(cond), Threshold: disp.supera,
						Devices: f.sobre, Service: disp.servicio, Run: disp.hacer, CooldownMinutes: 60,
					}
					s, maquinas := mundoDeAlcance(t, pol, registroDePrueba(curador))
					// Al segundo: `last_seen` y las ventanas se guardan en RFC3339.
					ahora := time.Now().UTC().Truncate(time.Second)
					despues := ahora.Add(90 * time.Minute)
					for _, n := range maquinasDeLaTablaDeAlcance {
						d := maquinas[n]
						// Las ventanas se declaran ya, hacia adelante: no tocan el primer barrido y cubren
						// el segundo.
						if _, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
							DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
							Desde: ahora.Add(time.Hour), Hasta: ahora.Add(3 * time.Hour), Motivo: "la segunda mitad de la fila",
						}); err != nil {
							t.Fatalf("AbrirMantenimiento(%s): %v", n, err)
						}
						disp.preparar(t, s, d, ahora)
					}
					maquinas = maquinasDelMundo(t, s)
					dominio := s.politicas[0]
					for _, n := range maquinasDeLaTablaDeAlcance {
						if !condicionCumplida(t, s, dominio, maquinas[n], ahora) {
							t.Fatalf("PISO: la condición %s no se cumple en %s, y sin eso «no actuó» no distingue el alcance de la condición", cond, n)
						}
					}
					if en := s.ventanasParaPoliticas(ahora); len(en) != 0 {
						t.Fatalf("PISO: el primer barrido ya ve %d máquina(s) en ventana, y no mediría el alcance sino la pausa", len(en))
					}

					// EL PRIMER BARRIDO: actúa exactamente donde alcanza.
					s.aplicarPoliticas("casa", ahora)
					for _, n := range maquinasDeLaTablaDeAlcance {
						quiero := 0
						if alcanza[n] {
							quiero = 1
						}
						if got := accionesDePolitica(t, s, maquinas[n], disp.hacer); got != quiero {
							t.Errorf("%s · `devices: %q`: la política actuó %d vez/veces sobre %s y la fila dice %d. Una política "+
								"actúa sobre una máquina que no nombra —un comando que nadie escribió para ella—, o deja de actuar "+
								"sobre la que sí", cond, f.sobre, got, n, quiero)
						}
					}
					for _, r := range resultadosDePolitica {
						var quiero int64
						if r == "ok" {
							quiero = int64(len(f.alcanza))
						}
						if got := contarPoliticaOK(s, pol.Name, r); got != quiero {
							t.Errorf("%s · `devices: %q`: después del primer barrido el resultado %q contó %d y la fila dice %d: "+
								"una máquina fuera del alcance no puede dejar ninguna huella en el contador de la política",
								cond, f.sobre, r, got, quiero)
						}
					}

					// EL INVENTARIO: figura exactamente donde alcanza, y nunca con un cero.
					inventario := filasDelInventario(t, s)
					for _, n := range maquinasDeLaTablaDeAlcance {
						fila, ok := inventario[n]
						if !ok {
							t.Fatalf("%s no figura en el inventario de una credencial con exec sobre la casa", n)
						}
						v, figura := fila["politicas_activas"]
						if cuenta, esNumero := v.(float64); figura && (!esNumero || cuenta < 1) {
							t.Errorf("%s · `devices: %q`: la fila de %s trae `politicas_activas: %v`. Un conteo en cero en cada "+
								"fila es el ruido que entrena a ignorar la columna: el campo va sólo donde algo actúa", cond, f.sobre, n, v)
						}
						if figura != alcanza[n] {
							t.Errorf("%s · `devices: %q`: la fila de %s trae `politicas_activas` = %v, y la política la alcanza = %v. "+
								"El inventario le atribuye a la máquina un automatismo que no actúa sobre ella, o esconde uno que sí "+
								"(detalle: %v)", cond, f.sobre, n, figura, alcanza[n], fila["politicas"])
							continue
						}
						det, _ := fila["politicas"].([]any)
						if !figura {
							if _, hay := fila["politicas"]; hay {
								t.Errorf("%s · `devices: %q`: la fila de %s trae el detalle de políticas sin conteo: %v", cond, f.sobre, n, fila["politicas"])
							}
							continue
						}
						if v != float64(1) || len(det) != 1 {
							t.Errorf("%s · `devices: %q`: la fila de %s cuenta %v política(s) con %d en el detalle, y hay UNA sobre ella",
								cond, f.sobre, n, v, len(det))
							continue
						}
						if p, _ := det[0].(map[string]any); p["nombre"] != pol.Name {
							t.Errorf("%s · `devices: %q`: el detalle en %s nombra a %v y la política es %q", cond, f.sobre, n, p["nombre"], pol.Name)
						}
					}

					// EL SEGUNDO BARRIDO, CON LAS TRES EN VENTANA: la pausa se cuenta donde la política
					// habría actuado, y en ningún otro lado.
					if en := s.ventanasParaPoliticas(despues); len(en) != len(maquinasDeLaTablaDeAlcance) {
						t.Fatalf("PISO: el segundo barrido ve %d máquina(s) en ventana y tienen que ser las %d", len(en), len(maquinasDeLaTablaDeAlcance))
					}
					s.aplicarPoliticas("casa", despues)
					for _, r := range resultadosDePolitica {
						var quiero int64
						if r == "ok" || r == "mantenimiento" {
							quiero = int64(len(f.alcanza))
						}
						if got := contarPoliticaOK(s, pol.Name, r); got != quiero {
							t.Errorf("%s · `devices: %q`: con las %d máquinas en ventana, el resultado %q contó %d y la fila dice %d. "+
								"`mantenimiento` contesta «¿no actuó porque estaba en mantenimiento?», y sobre una máquina que la "+
								"política no nombra la respuesta es que nunca iba a actuar", cond, f.sobre,
								len(maquinasDeLaTablaDeAlcance), r, got, quiero)
						}
					}
				})
			}
		})
	}
}

// A131 · T3 — UNA POLÍTICA DE SERVICIO ACTÚA SOBRE EL SERVICIO QUE NOMBRA Y SOBRE NINGÚN OTRO: SI
// NO ESTÁ, NO ACTÚA, TENGA EL INVENTARIO LA FORMA QUE TENGA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABA LA GUARDA DE ANTES
//
// TestUnServicioAusenteDelInventarioNoDisparaLaPolitica arma UN inventario: existe, trae `sshd`
// corriendo y la política busca `nombre-mal-escrito`. Dos ejes quedaban quietos:
//
//   - EL VECINO no se parece al buscado ni cumple la condición. Comparar por prefijo (P4-m7) pasaba:
//     con `nginx` ausente y `nginx-exporter` caído, la política reiniciaba nginx por el estado de otro.
//   - EL INVENTARIO nunca está vacío, que es la forma MÁS común de la ausencia: una máquina que no
//     enumera (un Tier B sin enumerador, un agente que todavía no mandó el bloque). Tratar el vacío
//     como caída (P4-m8) pasaba.
//
// Y UN DEFECTO VIVO DE LA MISMA BÚSQUEDA: comparaba el `service:` crudo, y Validar y ClaveDeCooldown
// lo recortan. `service: " nginx "` validaba, figuraba sobre la máquina, y no actuaba NUNCA.
//
// Exposición medida (auditoría A131): 0 para los tres, porque no hay ninguna política de servicio.
// Los estados sí existen: altura-db (Tier B) tiene el inventario vacío —0 servicios vivos, declarados
// ni históricos—, y en el inventario real hay tres pares donde un nombre es prefijo de otro
// (NetworkManager/NetworkManager-wait-online, agora-searx/agora-searxng, forgejo/forgejo-runner).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una subprueba por cada condición de servicio declarada (AST) y por cada forma del inventario. El
// conjunto de formas de la AUSENCIA lo cierra la consulta que lee el barrido, ServiciosDeDevice
// (`WHERE device_id = ? AND revoked = 0`): una máquina sin filas, con filas de otros nombres, o con
// la de este nombre dada de baja. A eso se suman el servicio presente pero sano entre parecidos que
// cumplen, su espejo (presente y cumpliendo entre parecidos sanos), el control (presente y
// cumpliendo, solo) y el nombre de la política con bordes. En las formas de la ausencia los parecidos
// CUMPLEN la condición: si la política los mirara, actuaría. Todo entra por aplicarPoliticas, y se
// mira la bitácora de la máquina, la marca de cooldown (prueba que la decisión llegó al tramo de
// acción) y el contador de la política.
//
// PISO: cada forma se relee del registro y tiene que ser la que dice (vacío, sin el nombre, con el
// nombre revocado), con lo último que reportó la máquina entero en el inventario activo; y los
// parecidos se parecen de verdad —una comparación laxa los confunde con el buscado— y están en el
// estado que la fila dice.
//
// Sabotaje: el barrido trata un inventario vacío como el servicio caído (P4-m8, portada: la
// búsqueda ya no es un bucle y el umbral ya no es una variable local).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tsv, esta := pol.ServicioEn(servicios)\n"
// arnes: a="\tif len(servicios) == 0 {\n\t\tv := 1.0\n\t\treturn s.actuarSiCorresponde(pol, d, &v, ahora)\n\t}\n\tsv, esta := pol.ServicioEn(servicios)\n"
//
// Sabotaje: el barrido, cuando la búsqueda exacta no encuentra, se conforma con un parecido: la
// función del dominio sigue exacta y el que la llama la afloja.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif !esta {\n"
// arnes: a="\tfor _, x := range servicios {\n\t\tif !esta && strings.HasPrefix(x.Nombre, strings.TrimSpace(pol.Servicio)) {\n\t\t\tsv, esta = x, true\n\t\t}\n\t}\n\tif !esta {\n"
func TestUnaPoliticaDeServicioSoloMiraElServicioQueNombra(t *testing.T) {
	var condiciones []fleet.Condicion
	for c := range condicionesDeclaradas(t) {
		if (fleet.Politica{Cuando: c}).EsDeServicio() {
			condiciones = append(condiciones, c)
		}
	}
	sort.Slice(condiciones, func(i, j int) bool { return condiciones[i] < condiciones[j] })
	// PISO: hoy son dos (servicio_caido y servicio_reinicios).
	if len(condiciones) == 0 {
		t.Fatal("PISO: condicionesDeclaradas no trae ninguna condición de servicio: la tabla no mediría nada")
	}
	// Por condición, una salud que la CUMPLE y una SANA. Un conjunto cerrado: una condición de servicio
	// nueva sin sus dos saludes pone esto rojo.
	nueve, cero := 9, 0
	saludes := map[fleet.Condicion]struct {
		supera       float64
		cumple, sana fleet.SaludServicio
	}{
		fleet.CondServicioCaido: {cumple: fleet.SaludServicio{Estado: fleet.EstadoFallado},
			sana: fleet.SaludServicio{Estado: fleet.EstadoCorriendo}},
		fleet.CondServicioReinicios: {supera: 3, cumple: fleet.SaludServicio{Estado: fleet.EstadoCorriendo, Reinicios: &nueve},
			sana: fleet.SaludServicio{Estado: fleet.EstadoCorriendo, Reinicios: &cero}},
	}
	parecidos := []string{"nginx-exporter", "openresty-nginx", "ngin", "NGINX"}

	type reporte struct {
		nombre string
		cumple bool
	}
	// Cada forma dice también cómo tiene que quedar el registro antes del barrido —sin filas activas,
	// con el servicio buscado activo, con él dado de baja—, y el PISO lo relee: una fila que no deja
	// el registro como dice mide otra cosa.
	type registroEsperado struct{ vacio, activo, revocado bool }
	formas := []struct {
		caso     string
		servicio string      // el `service:` de la política
		lotes    [][]reporte // lo que la máquina reporta, lote por lote; después de cada uno se poda lo ausente
		queda    registroEsperado
		actua    bool
		porque   string
	}{
		{"el servicio está y cumple la condición", "nginx", [][]reporte{{{"nginx", true}}},
			registroEsperado{activo: true}, true,
			"es el control: sin él, cada «no actuó» pasaría contra un barrido que no hace nada"},
		{"inventario vacío: la máquina nunca enumeró", "nginx", nil,
			registroEsperado{vacio: true}, false,
			"la forma más común de la ausencia; no saber no es una razón para tocar una máquina"},
		{"sólo parecidos, y todos cumplen la condición", "nginx", [][]reporte{
			{{parecidos[0], true}, {parecidos[1], true}, {parecidos[2], true}, {parecidos[3], true}}},
			registroEsperado{}, false,
			"cada uno es OTRO servicio: actuar por su estado es reiniciar nginx porque se cayó su exportador"},
		{"el servicio está sano y los parecidos cumplen", "nginx", [][]reporte{
			{{"nginx", false}, {parecidos[0], true}, {parecidos[1], true}, {parecidos[2], true}, {parecidos[3], true}}},
			registroEsperado{activo: true}, false,
			"la política juzga SU servicio, no el primero que se le parezca"},
		{"el servicio cumple y los parecidos están sanos", "nginx", [][]reporte{
			{{"nginx", true}, {parecidos[0], false}, {parecidos[1], false}, {parecidos[2], false}, {parecidos[3], false}}},
			registroEsperado{activo: true}, true,
			"el espejo de la anterior: una búsqueda que se quede con un parecido sano deja caído al servicio que sí nombra"},
		{"el servicio está dado de baja con la última salud cumpliendo", "nginx", [][]reporte{
			{{"nginx", true}, {"sshd", false}}, {{"sshd", false}}},
			registroEsperado{revocado: true}, false,
			"lo que salió del inventario no se reinicia por el último estado que se le conoció"},
		{"la política escribe el nombre con espacios en los bordes", " nginx ", [][]reporte{{{"nginx", true}}},
			registroEsperado{activo: true}, true,
			"Validar y ClaveDeCooldown recortan el nombre, y la búsqueda no puede ser la única que no"},
	}

	for _, cond := range condiciones {
		salud, ok := saludes[cond]
		if !ok {
			t.Errorf("la tabla no sabe cumplir ni dejar sana la condición de servicio %q: queda sin medir", cond)
			continue
		}
		for _, f := range formas {
			t.Run(string(cond)+"/"+f.caso, func(t *testing.T) {
				s := newTestServer(t, embedding.NoopProvider{})
				enrolarConExec(t, s, "casa", "davantis")
				pol := config.PolicyConfig{
					Name: "revivir-nginx", Principal: "curador", When: string(cond), Threshold: salud.supera,
					Devices: []string{"*"}, Service: f.servicio, Run: []string{"systemctl", "restart", "nginx"}, CooldownMinutes: 10,
				}
				if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{pol}}); err != nil {
					t.Fatalf("ConfigurarFlota(service=%q): %v", f.servicio, err)
				}
				s.buscarPrincipal = registroQuePermiteSystemctl()
				d, _, _ := s.engine.DevicePorNombre("casa", "davantis")
				ahora := time.Now().UTC().Truncate(time.Second)
				for i, lote := range f.lotes {
					var reportes []fleet.ReporteServicio
					var vivos []string
					for _, r := range lote {
						sal := salud.sana
						if r.cumple {
							sal = salud.cumple
						}
						sal.Tomada = ahora
						reportes = append(reportes, fleet.ReporteServicio{Nombre: r.nombre, Clase: "systemd", Salud: sal})
						vivos = append(vivos, r.nombre)
					}
					if _, _, err := s.engine.ReportarServicios(d.ID, ahora, reportes); err != nil {
						t.Fatalf("ReportarServicios (lote %d): %v", i, err)
					}
					if _, err := s.engine.PodarServiciosAusentes(d.ID, vivos, false); err != nil {
						t.Fatalf("PodarServiciosAusentes (lote %d): %v", i, err)
					}
				}

				// PISO: la forma es la que dice, leída del registro.
				activos, err := s.engine.ServiciosDeDevice(d.ID)
				if err != nil {
					t.Fatal(err)
				}
				todos, err := s.engine.ListarServicios("casa", d.ID, true)
				if err != nil {
					t.Fatal(err)
				}
				buscado := strings.TrimSpace(f.servicio)
				estaActivo := slices.ContainsFunc(activos, func(sv fleet.Servicio) bool { return sv.Nombre == buscado })
				estaRevocado := slices.ContainsFunc(todos, func(sv fleet.Servicio) bool { return sv.Nombre == buscado && sv.Revocado })
				if queda := (registroEsperado{vacio: len(activos) == 0, activo: estaActivo, revocado: estaRevocado}); queda != f.queda {
					t.Fatalf("PISO: la fila dice que el registro queda %+v y quedó %+v (%d servicio(s) activos): mide otra forma del inventario",
						f.queda, queda, len(activos))
				}
				// Y lo que la máquina reporta AHORA —el último lote— está entero en el inventario activo: un
				// parecido que el registro no guardó es una trampa menos, y la fila lo contaría igual.
				if n := len(f.lotes); n > 0 {
					for _, r := range f.lotes[n-1] {
						if !slices.ContainsFunc(activos, func(sv fleet.Servicio) bool { return sv.Nombre == r.nombre }) {
							t.Fatalf("PISO: la máquina reportó %q y el inventario activo no lo tiene: la fila no mide la forma que dice", r.nombre)
						}
					}
				}
				dominio := s.politicas[0]
				// Los parecidos se parecen DE VERDAD —una comparación laxa los confunde con el buscado— y
				// están en el estado que la fila dice: si cumplen, la política actuaría mirándolos; si
				// están sanos, mirarlos la dejaría quieta.
				reportado := map[string]bool{}
				if n := len(f.lotes); n > 0 {
					for _, r := range f.lotes[n-1] {
						reportado[r.nombre] = r.cumple
					}
				}
				for _, sv := range activos {
					if sv.Nombre == buscado || !slices.Contains(parecidos, sv.Nombre) {
						continue
					}
					parece := strings.HasPrefix(sv.Nombre, buscado) || strings.HasPrefix(buscado, sv.Nombre) ||
						strings.HasSuffix(sv.Nombre, buscado) || strings.EqualFold(sv.Nombre, buscado)
					otra := dominio
					otra.Servicio = sv.Nombre
					if _, dispara := otra.DisparaSobreServicio(sv, servicioFresco(sv, ahora)); !parece || dispara != reportado[sv.Nombre] {
						t.Fatalf("PISO: el parecido %q se parece=%v y cumple=%v (la fila lo reportó cumpliendo=%v): no es la trampa que la fila dice",
							sv.Nombre, parece, dispara, reportado[sv.Nombre])
					}
				}

				s.aplicarPoliticas("casa", ahora)
				quiero := 0
				if f.actua {
					quiero = 1
				}
				if got := accionesDePolitica(t, s, d, dominio.Hacer); got != quiero {
					t.Errorf("%s · %s: la política actuó %d vez/veces sobre davantis y la fila dice %d (%s)", cond, f.caso, got, quiero, f.porque)
				}
				if _, marcado := s.ultimoDisparo.Load(dominio.ClaveDeCooldown(d.ID)); marcado != f.actua {
					t.Errorf("%s · %s: la marca de cooldown está=%v y la fila dice actúa=%v: la decisión llegó al tramo de acción "+
						"donde no correspondía, o no llegó donde sí (%s)", cond, f.caso, marcado, f.actua, f.porque)
				}
				for _, r := range resultadosDePolitica {
					var quiero int64
					if r == "ok" && f.actua {
						quiero = 1
					}
					if got := contarPoliticaOK(s, pol.Name, r); got != quiero {
						t.Errorf("%s · %s: el resultado %q contó %d y la fila dice %d", cond, f.caso, r, got, quiero)
					}
				}
			})
		}
	}
}
