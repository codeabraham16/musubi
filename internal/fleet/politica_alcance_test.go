package fleet

// A131 · T3 — A QUÉ ALCANZA UNA POLÍTICA: A QUÉ MÁQUINAS, Y A QUÉ SERVICIO ADENTRO DE CADA UNA.
//
// Las dos preguntas se contestan con NOMBRES —el de la máquina contra un selector, el del servicio
// contra el que la política declara— y las dos tenían la misma forma de agujero: una comparación
// escrita a mano, custodiada por una prueba cuyo vecino no compartía nada con el nombre buscado.
// Así, aflojar la comparación a un prefijo no ponía nada en rojo. Estas tablas recorren las formas
// en que un nombre se le parece a otro sin ser él, contra las funciones del dominio que ahora son
// las únicas que comparan: SelectorAlcanza/SelectorNombra (y EntradaDeAllowlist, que lee con ellas la
// clave de una allowlist de comandos) y Politica.ServicioEn.
//
// Lo que estas tablas NO pueden ver es que un consumidor deje de usarlas y vuelva a comparar por
// su cuenta. Eso lo miden, del lado de internal/mcp, TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra
// (el barrido, el inventario, la compuerta de las concesiones, la de los comandos y los informes del
// rename, contra hechos escritos) y TestUnaPoliticaDeServicioSoloMiraElServicioQueNombra (el barrido,
// contra cada forma de ausencia y de parecido del servicio), y TestLaAllowlistYElComodinSeLeenSoloConLaGramatica
// mira el código: nadie lee `fleet_exec_allow` por clave ni el comodín fuera de la gramática. Las
// cuatro tablas —dos de máquinas, dos de servicios— exigen en su PISO las MISMAS formas de parecido,
// las de internal/fleet/fleettest. Hasta la revisión de T3 la de los consumidores tenía su lista de
// formas escrita a mano y le faltaba el glob; hasta la revisión 2, las dos de servicios tenían la
// suya, sin glob, y una regla glob o la normalización de `.service` en ServicioEn dejaban verdes a
// los dos paquetes.

import (
	"strings"
	"testing"

	"musubi/internal/fleet/fleettest"
)

// A131 · T3 — UN SELECTOR DE MÁQUINA ALCANZA AL COMODÍN O AL NOMBRE EXACTO, Y A NADA MÁS.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJE CLAVABA LA GUARDA DE ANTES
//
// En este paquete no había NINGUNA prueba de Politica.Alcanza. La única que la ejercitaba,
// TestUnaPoliticaSoloTocaLasMaquinasQueNombra (internal/mcp), pone una política sobre `nas` y mira
// que no actúe sobre `pc-gio`: dos nombres que no comparten ni una letra. Cualquier comparación
// laxa —prefijo, sufijo, subcadena, mayúsculas indistintas— da el mismo «no» sobre ese par, así
// que comparar por prefijo (P2-m4) dejaba verdes los dos paquetes. Y la copia de la compuerta de
// las concesiones (tieneGrant) tenía el mismo punto ciego: TestLaConcesionEsPorMaquina prueba
// `servidor-critico` contra `pc-gio` y `nas`, que no son prefijo, sufijo ni variante uno del otro.
// Medido en T3 con una regla de prefijo agregada a tieneGrant: el paquete internal/mcp entero, sin la
// tabla nueva, quedaba en verde.
//
// Exposición medida (auditoría A131): hoy nada afectado, y el estado que lo vuelve vivo existe. En
// el cerebro hay 4 máquinas (altura-db, davantis-1, gio, musubi-server) y ningún par donde un nombre
// sea prefijo de otro; pero Prometheus ya vio el par `davantis` (la laptop Linux) y `davantis-1`
// (la PC Windows). La única política real, `vaciar-journal`, está acotada a `musubi-server` y su
// principal `auto-heal` tiene `exec: ["*"]`: su `devices:` —leído por Alcanza— es lo ÚNICO que la
// mantiene en esa máquina. Con el prefijo, la primera máquina que se llame `musubi-server-2`
// recibiría el comando sin un solo error de permisos.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Cada fila es un selector y un nombre de máquina con dos HECHOS escritos: si el selector la
// ALCANZA y si la NOMBRA (el comodín alcanza sin nombrar). Se miden las dos funciones de la
// gramática y Politica.Alcanza, con el selector solo y detrás de otro que no nombra a nadie (una
// lista es un «o»); y EntradaDeAllowlist con el selector como única clave de una allowlist de
// comandos, que manda sobre la máquina donde el selector la alcanza y la nombra donde la nombra
// (revisión 2 de T3: la clave de `fleet_exec_allow` también es un selector). Las filas recorren las
// formas en que un selector se le parece a un nombre sin serlo: prefijo, sufijo, subcadena, el
// nombre como prefijo del selector, mayúsculas, un glob; y las dos normalizaciones que sí valen: el
// comodín, y los espacios de los bordes del selector.
//
// PISO: la tabla tiene que traer al menos una fila de cada forma de parecido —se clasifican
// mirando el par, no el nombre del caso, con fleettest.DeParecido—, todas con «no alcanza». Una
// tabla a la que se le caen esas filas sigue en verde contra cualquier comparación laxa, que es
// exactamente la guarda de antes.
//
// Sabotaje: Politica.Alcanza compara por prefijo (P2-m4, portada: la comparación ya no está escrita
// en Alcanza sino en SelectorAlcanza, así que el sabotaje le devuelve a Alcanza el cuerpo que tenía
// la mutación original, byte por byte: el bucle sobre LimpiarSelectores y el prefijo).
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tfor _, s := range p.Sobre {\n\t\tif SelectorAlcanza(s, nombreDevice) {\n"
// arnes: a="\tfor _, s := range LimpiarSelectores(p.Sobre) {\n\t\tif s == \"*\" || strings.HasPrefix(nombreDevice, s) {\n"
//
// Sabotaje: la gramática misma deja de distinguir mayúsculas, para todos los que la leen.
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="return s != \"\" && !EsComodin(s) && s == nombreDevice"
// arnes: a="return s != \"\" && !EsComodin(s) && strings.EqualFold(s, nombreDevice)"
//
// Sabotaje: la búsqueda de la allowlist lee la clave CRUDA, sin recortar y con `==` —lo que hacían
// argvPermitido y comandosPermitidos con `p.ExecAllow[d.Name]` hasta la revisión 2—, aunque sigue
// excluyendo al comodín.
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\t\tif SelectorNombra(clave, nombreDevice) {\n"
// arnes: a="\t\tif clave == nombreDevice && !EsComodin(clave) {\n"
func TestUnSelectorAlcanzaSoloAlComodinOAlNombreExacto(t *testing.T) {
	type fila struct {
		selector, maquina string
		alcanza, nombra   bool
		porque            string
	}
	filas := []fila{
		// ── Lo que SÍ alcanza ──
		{"davantis", "davantis", true, true, "el nombre exacto"},
		{"*", "davantis", true, false, "el comodín alcanza a todas y no nombra a ninguna: sobrevive a cualquier rename"},
		{" davantis ", "davantis", true, true, "los bordes del selector se recortan, como en LimpiarSelectores y parsearFleet"},
		{" * ", "davantis", true, false, "el comodín con bordes sigue siendo el comodín"},
		{"*", "*", true, false, "una máquina que se llama `*` la alcanza el comodín, pero el comodín no la NOMBRA"},

		// ── Lo que se le PARECE y no alcanza: el eje que la guarda de antes dejaba quieto ──
		{"davantis", "davantis-1", false, false, "PREFIJO: el par real de la malla, la laptop Linux y la PC Windows"},
		{"davan", "davantis", false, false, "PREFIJO corto"},
		{"davantis-1", "davantis", false, false, "el NOMBRE es prefijo del selector: la comparación al revés"},
		{"antis", "davantis", false, false, "SUFIJO"},
		{"1", "davantis-1", false, false, "SUFIJO de un carácter"},
		{"vant", "davantis", false, false, "SUBCADENA"},
		{"DAVANTIS", "davantis", false, false, "MAYÚSCULAS: el índice único de devices distingue, así que son dos máquinas posibles"},
		{"Davantis", "davantis", false, false, "MAYÚSCULAS en una sola letra"},
		{"davantis*", "davantis-1", false, false, "GLOB: el asterisco sólo es comodín solo"},

		// ── Lo vacío no alcanza nada, ni siquiera a lo vacío ──
		{"", "davantis", false, false, "un selector vacío no es «todas»"},
		{"  ", "davantis", false, false, "sólo espacios es vacío"},
		{"", "", false, false, "vacío contra vacío tampoco: ninguna máquina guardada tiene nombre vacío, y el día que una lo tuviera no la alcanzaría una línea en blanco"},
	}

	// PISO: la tabla trae cada forma de parecido, clasificada por el PAR y no por el texto del caso.
	// La clasificación es la de fleettest, la MISMA que exige la tabla de los consumidores en
	// internal/mcp: en la revisión de T3 esa tabla tenía su lista escrita a mano y le faltaba el glob.
	porForma := map[fleettest.Forma]int{}
	for _, f := range filas {
		if f.alcanza {
			continue
		}
		if forma := fleettest.DeParecido(f.selector, f.maquina); forma != "" {
			porForma[forma]++
		}
	}
	for _, forma := range fleettest.Formas() {
		if porForma[forma] == 0 {
			t.Errorf("PISO: la tabla no trae ninguna fila de %s que no alcance. Sin ella, una comparación que "+
				"acepte esa forma de parecido deja esta guarda en verde, como dejaba a la de antes", forma)
		}
	}

	for _, f := range filas {
		// LA FILA CONTRA SÍ MISMA: nombrar implica alcanzar, y alcanzar sin nombrar es sólo del comodín.
		if f.nombra && !f.alcanza {
			t.Errorf("LA FILA ESTÁ MAL ESCRITA (%q sobre %q): dice que nombra y que no alcanza", f.selector, f.maquina)
		}
		if f.alcanza && !f.nombra && strings.TrimSpace(f.selector) != ComodinMaquinas {
			t.Errorf("LA FILA ESTÁ MAL ESCRITA (%q sobre %q): alcanza sin nombrar y no es el comodín", f.selector, f.maquina)
		}

		if got := SelectorAlcanza(f.selector, f.maquina); got != f.alcanza {
			t.Errorf("SelectorAlcanza(%q, %q) = %v y la fila dice %v (%s): lo que se escribió para una máquina "+
				"alcanza a otra, o deja de alcanzar a la que nombra", f.selector, f.maquina, got, f.alcanza, f.porque)
		}
		if got := SelectorNombra(f.selector, f.maquina); got != f.nombra {
			t.Errorf("SelectorNombra(%q, %q) = %v y la fila dice %v (%s): el informe del rename lista lo que no se "+
				"rompe, o calla lo que sí", f.selector, f.maquina, got, f.nombra, f.porque)
		}
		sola := Politica{Sobre: []string{f.selector}}
		if got := sola.Alcanza(f.maquina); got != f.alcanza {
			t.Errorf("una política con `devices: [%q]` alcanza a %q = %v y la fila dice %v (%s): la política actúa "+
				"sobre una máquina que no nombra, o deja de actuar sobre la que sí", f.selector, f.maquina, got, f.alcanza, f.porque)
		}
		enLista := Politica{Sobre: []string{"otra-pc", f.selector}}
		if got := enLista.Alcanza(f.maquina); got != f.alcanza {
			t.Errorf("una política con `devices: [\"otra-pc\", %q]` alcanza a %q = %v y la fila dice %v: una lista es "+
				"un «o», y un selector que no nombra a nadie no puede tapar ni ampliar al siguiente", f.selector, f.maquina, got, f.alcanza)
		}
		// La allowlist de comandos con el selector como única clave: su entrada manda sobre la máquina
		// exactamente donde el selector la alcanza, y la NOMBRA donde el selector la nombra.
		if _, nombrada, hay := EntradaDeAllowlist(map[string][]string{f.selector: {"uptime"}}, f.maquina); hay != f.alcanza || nombrada != f.nombra {
			t.Errorf("EntradaDeAllowlist con la clave %q sobre %q da manda=%v y nombra=%v, y la fila dice alcanza=%v y "+
				"nombra=%v (%s): la allowlist de comandos lee su clave con otra gramática que la de las máquinas, y "+
				"acota (o deja de acotar) a una máquina que no nombró", f.selector, f.maquina, hay, nombrada, f.alcanza, f.nombra, f.porque)
		}
	}

	// Y una política sin selectores no alcanza a nadie: la ausencia nunca significa «todas».
	for _, sobre := range [][]string{nil, {}, {"", "  "}} {
		if (Politica{Sobre: sobre}).Alcanza("davantis") {
			t.Errorf("una política con `devices: %q` alcanza a davantis: la ausencia se leyó como el comodín", sobre)
		}
	}
}

// A131 · T3 — UNA POLÍTICA DE SERVICIO ENCUENTRA SU SERVICIO POR EL NOMBRE EXACTO, O NO LO ENCUENTRA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJE CLAVABA LA GUARDA DE ANTES
//
// La búsqueda vivía en internal/mcp como un bucle con `if sv.Nombre != pol.Servicio { continue }`, y
// su guarda, TestUnServicioAusenteDelInventarioNoDisparaLaPolitica, ponía en el inventario un solo
// vecino, `sshd`, junto a una política sobre `nombre-mal-escrito`: ninguna comparación laxa confunde
// esos dos. Cambiar la comparación por un prefijo (P4-m7) la dejaba en verde, y con `nginx` ausente
// y `nginx-exporter` caído, la política reiniciaba nginx por el estado de otro servicio.
//
// Exposición medida (auditoría A131): 0 hoy. No hay ninguna política de servicio configurada. Pero
// el inventario real tiene 123 servicios activos en 3 máquinas y tres pares donde un nombre es
// prefijo de otro (NetworkManager/NetworkManager-wait-online, agora-searx/agora-searxng,
// forgejo/forgejo-runner): la primera política sobre el más corto, en una máquina que sólo corre el
// largo, actuaba por el estado del otro.
//
// Y UN DEFECTO VIVO DE LA MISMA COMPARACIÓN, encontrado al cerrar éste: comparaba el `service:` CRUDO
// de la política, mientras Validar y ClaveDeCooldown lo recortan. `service: " nginx"` validaba,
// figuraba en el inventario como una política sobre la máquina con `puede_actuar: true`, y no
// encontraba NUNCA su servicio, porque todo nombre guardado está recortado. Exposición: 0, por lo
// mismo (ninguna política de servicio).
//
// Y SU LISTA DE FORMAS ERA OTRA COPIA ESCRITA A MANO (revisión 2 de T3). El PISO de esta tabla era un
// `switch` propio con cinco formas —el buscado como prefijo y como sufijo del reportado, el reportado
// como prefijo y como subcadena del buscado, mayúsculas—, sin glob: el defecto que la revisión 1 marcó
// BLOQUEANTE en la tabla de máquinas de internal/mcp, y que el doc de fleettest describe («dos copias
// de "qué es un parecido" se separan»). Medido en la revisión 2 sobre 5b19840, contra los dos
// paquetes enteros: una regla glob propia en ServicioEn (`service: "ngin*"` alcanza a lo que empieza
// con `ngin`, NS3) → internal/fleet ok 3,888 s e internal/mcp ok 88,222 s; normalizar el sufijo
// `.service` (NS2, el vecino que el manifiesto de P4-m7 pedía) → ok 3,907 s y ok 92,258 s.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Politica.ServicioEn sobre cada forma del inventario: vacío, sólo parecidos, el exacto entre
// parecidos (y tiene que devolver EL exacto, no el primer parecido), el nombre de la política con
// bordes, una política cuyo `service:` es un PATRÓN con lo que el patrón calza en el inventario; y
// una política de host, que no mira ningún servicio.
//
// PISO: entre el servicio de cada política de la tabla y los nombres de su inventario, cada forma de
// fleettest.Formas() —la MISMA clasificación que exigen las tablas de máquinas— en los DOS sentidos:
// el buscado como selector del reportado (`nginx` frente a `nginx-exporter`) y al revés (`gin` frente
// a `nginx`). Clasificado mirando el par y no el nombre del caso, así que una forma nueva de fleettest
// se vuelve obligatoria acá sin tocar esta prueba. Y el nombre con el sufijo `.service`, que no es una
// forma sino un HECHO de systemd —`systemctl` toma `nginx` y `nginx.service` como la misma unidad—,
// así que es la normalización que alguien va a querer agregar.
//
// Sabotaje: la búsqueda vuelve a ser un recorrido con prefijo (P4-m7, portada a ServicioEn).
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tsv, esta := porNombre[buscado]\n\treturn sv, esta\n"
// arnes: a="\tfor _, sv := range inventario {\n\t\tif strings.HasPrefix(sv.Nombre, buscado) {\n\t\t\treturn sv, true\n\t\t}\n\t}\n\treturn Servicio{}, false\n"
//
// Sabotaje: el nombre de la política se compara crudo (el defecto vivo).
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tbuscado := strings.TrimSpace(p.Servicio)\n"
// arnes: a="\tbuscado := p.Servicio\n"
//
// Sabotaje: la búsqueda normaliza el sufijo de unidad de systemd y toma `nginx.service` por `nginx`
// (NS2 de la revisión 2, tal cual lo midió).
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\t\tporNombre[sv.Nombre] = sv\n"
// arnes: a="\t\tporNombre[strings.TrimSuffix(sv.Nombre, \".service\")] = sv\n"
//
// Sabotaje: la búsqueda suma una regla GLOB propia —un `service:` que termina en `*` alcanza a lo que
// empieza igual— (NS3 de la revisión 2: el mismo bloque, insertado antes de armar el mapa para no
// pisar el `de` de P4-m7).
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tporNombre := make(map[string]Servicio, len(inventario))\n"
// arnes: a="\tif n := len(buscado) - 1; n > 0 && buscado[n] == '*' {\n\t\tfor _, sv := range inventario {\n\t\t\tif strings.HasPrefix(sv.Nombre, buscado[:n]) {\n\t\t\t\treturn sv, true\n\t\t\t}\n\t\t}\n\t}\n\tporNombre := make(map[string]Servicio, len(inventario))\n"
func TestUnaPoliticaDeServicioEncuentraSuServicioPorElNombreExacto(t *testing.T) {
	// Los parecidos de `nginx`. Qué forma tiene cada uno no lo dice esta lista: lo clasifica fleettest
	// en el PISO de abajo, mirando el par.
	parecidos := []string{"nginx-exporter", "openresty-nginx", "lua-nginx-module", "nginx.service",
		"ngin", "gin", "ginx", "NGINX", "Nginx", "*ginx"}

	inventario := func(nombres ...string) []Servicio {
		out := make([]Servicio, 0, len(nombres))
		for _, n := range nombres {
			out = append(out, Servicio{Nombre: n, DeviceID: "d1", ProjectID: "casa"})
		}
		return out
	}
	deServicio := func(nombre string) Politica {
		return Politica{Nombre: "revivir", Principal: "curador", Cuando: CondServicioCaido, Sobre: []string{"*"},
			Servicio: nombre, Hacer: []string{"systemctl", "restart", "nginx"}}
	}
	casos := []struct {
		caso   string
		pol    Politica
		inv    []Servicio
		esta   bool
		porque string
	}{
		{"inventario nil", deServicio("nginx"), nil, false, "la máquina nunca enumeró: no saber no es «caído»"},
		{"inventario vacío", deServicio("nginx"), inventario(), false, "la máquina dijo que no corre nada"},
		{"sólo parecidos", deServicio("nginx"), inventario(parecidos...), false,
			"cada uno es OTRO servicio: actuar por su estado es reiniciar nginx porque se cayó su exportador"},
		{"el exacto entre parecidos", deServicio("nginx"), inventario(append(append([]string{}, parecidos...), "nginx")...), true,
			"está, y tiene que ser ÉL y no el primer parecido del recorrido"},
		{"el nombre de la política con bordes", deServicio(" nginx "), inventario("nginx-exporter", "nginx"), true,
			"Validar y ClaveDeCooldown lo recortan: la búsqueda no puede ser la única que no"},
		{"el `service:` es un patrón", deServicio("ngin*"), inventario("nginx", "nginx-exporter"), false,
			"el nombre de un servicio es un NOMBRE: `ngin*` no es nginx ni su exportador, y leerlo como patrón es reiniciar lo que nadie nombró"},
		{"política de host", Politica{Nombre: "vaciar", Principal: "curador", Cuando: CondMemPct, Sobre: []string{"*"},
			Hacer: []string{"journalctl"}}, inventario("nginx", ""), false,
			"una política de host no mira ningún servicio, ni siquiera uno sin nombre"},
		{"política de host con un servicio escrito", Politica{Nombre: "vaciar", Principal: "curador", Cuando: CondMemPct,
			Sobre: []string{"*"}, Servicio: "nginx", Hacer: []string{"journalctl"}}, inventario("nginx"), false,
			"Validar la rechaza; si llegara igual, no se lee como una de servicio"},
	}

	// PISO: entre el servicio de cada política de servicio de la tabla y los nombres de su inventario,
	// cada forma de fleettest en los DOS sentidos, clasificada por el par; y el sufijo de unidad de
	// systemd. Es la MISMA clasificación que exigen las tablas de máquinas: hasta la revisión 2 de T3
	// esta tabla tenía la suya, escrita a mano y sin glob.
	for _, p := range parecidos {
		if p == "nginx" {
			t.Fatalf("la lista de parecidos trae el nombre exacto: no mide nada")
		}
	}
	directas, inversas := map[fleettest.Forma]int{}, map[fleettest.Forma]int{}
	conSufijoDeUnidad := 0
	for _, c := range casos {
		buscado := strings.TrimSpace(c.pol.Servicio)
		if !c.pol.EsDeServicio() || buscado == "" {
			continue
		}
		for _, sv := range c.inv {
			if sv.Nombre == buscado {
				continue
			}
			directas[fleettest.DeParecido(buscado, sv.Nombre)]++
			inversas[fleettest.DeParecido(sv.Nombre, buscado)]++
			if sv.Nombre == buscado+".service" {
				conSufijoDeUnidad++
			}
		}
	}
	for _, forma := range fleettest.Formas() {
		if directas[forma] == 0 {
			t.Errorf("PISO: ninguna fila pone el servicio de su política frente a un nombre del inventario con la forma "+
				"%q (el buscado como selector del reportado); una búsqueda que la acepte queda en verde", forma)
		}
		if inversas[forma] == 0 {
			t.Errorf("PISO: ninguna fila pone un nombre del inventario frente al servicio de su política con la forma "+
				"%q (el reportado como selector del buscado); una búsqueda que compare al revés queda en verde", forma)
		}
	}
	if conSufijoDeUnidad == 0 {
		t.Errorf("PISO: ninguna fila trae el servicio buscado con el sufijo `.service` en el inventario: `systemctl` los " +
			"toma como la misma unidad, y una búsqueda que los normalice decidiría por el estado de otro nombre")
	}

	for _, c := range casos {
		sv, esta := c.pol.ServicioEn(c.inv)
		if esta != c.esta {
			// Con el nombre que devolvió: dos búsquedas flojas distintas (un prefijo, el sufijo `.service`)
			// caen en la misma fila, y lo que las distingue es a QUIÉN confundieron con el buscado.
			t.Errorf("%s: ServicioEn dice está=%v (devolvió %q) y la fila dice %v (%s)", c.caso, esta, sv.Nombre, c.esta, c.porque)
			continue
		}
		if esta && sv.Nombre != strings.TrimSpace(c.pol.Servicio) {
			t.Errorf("%s: ServicioEn devolvió %q buscando %q: la política decidiría por el estado de OTRO servicio",
				c.caso, sv.Nombre, c.pol.Servicio)
		}
		if !esta && sv != (Servicio{}) {
			t.Errorf("%s: ServicioEn dice que no está y devuelve %q: un llamador que se olvide del booleano actúa sobre él",
				c.caso, sv.Nombre)
		}
	}
}
