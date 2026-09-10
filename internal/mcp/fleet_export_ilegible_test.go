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
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA GUARDA LEÍA LA REGLA VECINA, NO LA SUYA (medido, y en verde sobre la alerta rota)
//
// Se anclaba con `strings.Index(reglas, "alert: ExportacionTruncada")` —match de PREFIJO sobre el
// YAML crudo— y después tomaba la PRIMERA línea que empezara con `expr:` a partir de ahí. Ninguna
// de las dos cosas identifica una regla:
//
//   - el prefijo matchea `alert: ExportacionTruncadaProlongada`, una hermana perfectamente
//     legítima, y si la hermana está ARRIBA la guarda se queda con la hermana;
//   - «la primera línea `expr:` que venga después» es CERCANÍA DE LÍNEAS, no pertenencia: alcanza
//     con que alguien reordene los campos de la regla, o meta cualquier cosa en el medio, para
//     que la guarda mida otra expresión.
//
// Sabotaje medido, con los dos cambios plausibles a la vez: la alerta real acotada a
// `{kind=~"projects|services"}` (o sea, `kind="unreadable"` ya no la dispara) y una hermana
// `ExportacionTruncadaProlongada` agregada arriba. La guarda leía la hermana y daba VERDE sobre
// la alerta rota — que es el caso exacto que existe para impedir.
//
// AHORA SE PARSEA EL YAML y la regla se identifica por su nombre EXACTO. Y el filtro tampoco se
// busca como texto: se parsean los selectores de la expresión y se mira si ALGUNO acota `kind`,
// venga como `=`, `!=`, `=~` o `!~`, con espacios o sin ellos, en el selector de la métrica o en
// otro. Lo que no se puede parsear es rojo con su motivo, no verde.
func TestLaAlertaDeTruncadoNoFiltraPorKindYPorEsoCubreAlIlegible(t *testing.T) {
	reglas, _ := cargarReglas(t, "musubi-alerts.yml")
	expr := exprDeLaUnicaAlerta(t, reglas, "ExportacionTruncada")

	if !mencionaMetrica(expr, nombreExportTruncado) {
		t.Fatalf("la alerta ya no mira %s: %q", nombreExportTruncado, expr)
	}

	ms, err := matchersDePromQL(expr)
	if err != nil {
		// ROJO EXPLÍCITO. Una expresión que la guarda no sabe leer es «no pude medir», y eso no
		// se parece en nada a «no filtra por kind». Callarse acá sería volver a la fuga.
		t.Fatalf("no pude parsear la expresión de ExportacionTruncada (%q): %v\n"+
			"  Mientras no se pueda parsear, NADIE está midiendo que la alerta cubra `kind=\"unreadable\"`.", expr, err)
	}
	for _, m := range ms {
		if m.etiqueta != "kind" {
			continue
		}
		t.Errorf("la alerta acota `kind` (%s%s%q) en %q: el día que se agregue un cuarto motivo de recorte va a nacer mudo, que es como nació `kind=\"unreadable\"` en el diseño original.\n"+
			"  Con un filtro de kind, la serie que dice «no pude leer parte de la flota» puede quedarse en 1 para siempre sin disparar nada.",
			m.etiqueta, m.op, m.valor, expr)
	}
}

// exprDeLaUnicaAlerta devuelve la expr de la alerta que se llama EXACTAMENTE `nombre`.
//
// Exige que haya exactamente una: cero es «esta guarda dejó de medir algo» y dos es «la guarda no
// sabe cuál de las dos midió». Las dos cosas son rojo, no verde.
func exprDeLaUnicaAlerta(t *testing.T, a archivoDeReglas, nombre string) string {
	t.Helper()
	var exprs []string
	for _, g := range a.Groups {
		for _, r := range g.Rules {
			if r.Alert == nombre {
				exprs = append(exprs, r.Expr)
			}
		}
	}
	switch len(exprs) {
	case 1:
		return exprs[0]
	case 0:
		t.Fatalf("no hay ninguna alerta que se llame exactamente %q en deploy/musubi-alerts.yml.\n"+
			"  Si se renombró, esta guarda dejó de custodiar nada: apuntala al nombre nuevo.", nombre)
	default:
		t.Fatalf("hay %d alertas que se llaman %q: no puedo saber cuál es la que tiene que cubrir `kind=\"unreadable\"`", len(exprs), nombre)
	}
	return ""
}

// matcherDeEtiqueta es un matcher de selector ya parseado: `kind=~"projects|services"` ⇒
// {etiqueta: "kind", op: "=~", valor: "projects|services"}.
type matcherDeEtiqueta struct {
	etiqueta string
	op       string
	valor    string
}

// matchersDePromQL parsea TODOS los selectores de una expresión y devuelve sus matchers.
//
// SE PARSEA, NO SE BUSCA UN TEXTO. `strings.Contains(expr, "kind=")` no ve `kind !=`, no ve
// `kind =~` con espacios y no ve un segundo selector metido con `unless`; y cada forma que se le
// agregue a esa lista le va a faltar la siguiente. Acá se recorre la expresión: los literales de
// string se saltean enteros —adentro de un literal una llave o una coma no abren ni separan
// nada— y cada bloque `{…}` se parte en matchers de verdad.
//
// Lo que NO se puede parsear devuelve error para que el que llama lo haga ROJO. Una expresión que
// la guarda no entiende es «no pude medir»; devolver «no encontré matchers» sería verde silencioso.
func matchersDePromQL(expr string) ([]matcherDeEtiqueta, error) {
	r := []rune(expr)
	var out []matcherDeEtiqueta
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '"', '\'', '`':
			fin, err := finDeLiteral(r, i)
			if err != nil {
				return nil, err
			}
			i = fin
		case '}':
			return nil, fmt.Errorf("`}` en la posición %d sin `{` que lo abra", i)
		case '{':
			cuerpo, fin, err := cuerpoDeSelector(r, i)
			if err != nil {
				return nil, err
			}
			ms, err := matchersDeCuerpo(cuerpo)
			if err != nil {
				return nil, fmt.Errorf("selector `{%s}`: %w", cuerpo, err)
			}
			out = append(out, ms...)
			i = fin
		}
	}
	return out, nil
}

// finDeLiteral devuelve el índice de la comilla que cierra el literal que abre en `i`.
func finDeLiteral(r []rune, i int) (int, error) {
	comilla := r[i]
	for j := i + 1; j < len(r); j++ {
		// Los literales con backtick de PromQL son crudos: ahí `\` no escapa nada.
		if comilla != '`' && r[j] == '\\' {
			j++
			continue
		}
		if r[j] == comilla {
			return j, nil
		}
	}
	return 0, fmt.Errorf("literal de string sin cerrar desde la posición %d", i)
}

// cuerpoDeSelector devuelve lo que hay entre el `{` de `i` y su `}`, y el índice de ese `}`.
func cuerpoDeSelector(r []rune, i int) (string, int, error) {
	var cuerpo []rune
	for j := i + 1; j < len(r); j++ {
		switch r[j] {
		case '"', '\'', '`':
			fin, err := finDeLiteral(r, j)
			if err != nil {
				return "", 0, err
			}
			cuerpo = append(cuerpo, r[j:fin+1]...)
			j = fin
		case '{':
			return "", 0, fmt.Errorf("`{` anidado en la posición %d", j)
		case '}':
			return string(cuerpo), j, nil
		default:
			cuerpo = append(cuerpo, r[j])
		}
	}
	return "", 0, fmt.Errorf("selector sin cerrar desde la posición %d", i)
}

// matchersDeCuerpo parte el interior de un selector en sus matchers.
func matchersDeCuerpo(cuerpo string) ([]matcherDeEtiqueta, error) {
	var out []matcherDeEtiqueta
	for _, parte := range partirEnComas(cuerpo) {
		if strings.TrimSpace(parte) == "" {
			continue
		}
		m, err := parsearMatcher(parte)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// partirEnComas corta por las comas que están FUERA de un literal.
func partirEnComas(s string) []string {
	r := []rune(s)
	var partes []string
	inicio := 0
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '"', '\'', '`':
			if fin, err := finDeLiteral(r, i); err == nil {
				i = fin
			}
		case ',':
			partes = append(partes, string(r[inicio:i]))
			inicio = i + 1
		}
	}
	return append(partes, string(r[inicio:]))
}

// parsearMatcher lee `etiqueta OP "valor"`. Los cuatro operadores son TODA la gramática de un
// matcher de PromQL; cualquier otra cosa es error, o sea rojo.
func parsearMatcher(s string) (matcherDeEtiqueta, error) {
	t := strings.TrimSpace(s)
	fin := 0
	for fin < len(t) && esRuneDeIdentificador(rune(t[fin])) {
		fin++
	}
	etiqueta := t[:fin]
	if etiqueta == "" {
		return matcherDeEtiqueta{}, fmt.Errorf("matcher %q: no empieza con un nombre de etiqueta", s)
	}
	resto := strings.TrimSpace(t[fin:])
	for _, op := range []string{"=~", "!~", "!=", "="} {
		if !strings.HasPrefix(resto, op) {
			continue
		}
		valor, err := literalPelado(strings.TrimSpace(strings.TrimPrefix(resto, op)))
		if err != nil {
			return matcherDeEtiqueta{}, fmt.Errorf("matcher %q: %w", s, err)
		}
		return matcherDeEtiqueta{etiqueta: etiqueta, op: op, valor: valor}, nil
	}
	return matcherDeEtiqueta{}, fmt.Errorf("matcher %q: después de la etiqueta %q no hay ninguno de los cuatro operadores (`=`, `!=`, `=~`, `!~`)", s, etiqueta)
}

// literalPelado exige que `s` sea EXACTAMENTE un literal de string y devuelve su contenido.
func literalPelado(s string) (string, error) {
	r := []rune(s)
	if len(r) == 0 || (r[0] != '"' && r[0] != '\'' && r[0] != '`') {
		return "", fmt.Errorf("el valor %q no es un literal de string", s)
	}
	fin, err := finDeLiteral(r, 0)
	if err != nil {
		return "", err
	}
	if fin != len(r)-1 {
		return "", fmt.Errorf("el valor %q tiene texto después del literal", s)
	}
	return string(r[1:fin]), nil
}

func esRuneDeIdentificador(c rune) bool {
	return c == '_' || c == ':' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}

// mencionaMetrica dice si `expr` nombra la métrica como TOKEN y no como pedazo de otro nombre:
// `musubi_fleet_export_truncated_total` no es `musubi_fleet_export_truncated`.
func mencionaMetrica(expr, metrica string) bool {
	for i := 0; ; {
		j := strings.Index(expr[i:], metrica)
		if j < 0 {
			return false
		}
		ini := i + j
		fin := ini + len(metrica)
		antes := ini == 0 || !esRuneDeIdentificador(rune(expr[ini-1]))
		despues := fin == len(expr) || !esRuneDeIdentificador(rune(expr[fin]))
		if antes && despues {
			return true
		}
		i = fin
	}
}
