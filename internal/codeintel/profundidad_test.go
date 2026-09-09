package codeintel

import (
	"strings"
	"testing"
)

// Banco de la profundidad derivada. Cada test declara UN invariante, nombrado P<n>, y hay un
// sabotaje que lo ataca en el runner de F2 — el invariante que el test dice cubrir, no una
// validación cualquiera que lo tape por defensa en profundidad.

// enCero son las señales de un cambio que no toca nada: el punto de partida contra el que se
// mide si una señal, sola, es capaz de mover la aguja.
func enCero() Senales { return Senales{} }

// --- P1: ninguna señal está cableada de adorno -----------------------------------------
//
// Es EL invariante de esta fase. Una señal que se calcula, se guarda, se serializa y nunca
// cambia el resultado es indistinguible de una que funciona — hasta que alguien confía en
// ella. Acá cada una arranca sola, con todas las demás en cero, y tiene que sumar.

func TestP1CadaSenalPuedeSubirElNivelPorSiSola(t *testing.T) {
	if base := Profundidad(enCero()); base.Puntos != 0 {
		t.Fatalf("un cambio que no toca nada tiene que puntuar 0, obtuve %d", base.Puntos)
	}
	casos := []struct {
		senal string
		s     Senales
	}{
		{"archivos", Senales{Archivos: 5}},
		{"lineas", Senales{Lineas: 151}},
		{"simbolos", Senales{Simbolos: 8}},
		{"paquetes", Senales{Paquetes: 3}},
		{"callers_en_radio", Senales{CallersEnRadio: 5}},
		{"paquetes_en_radio", Senales{PaquetesEnRadio: 2}},
		{"hay_borrados", Senales{HayBorrados: true}},
	}
	for _, c := range casos {
		v := Profundidad(c.s)
		if v.Puntos < 1 {
			t.Errorf("la señal %q no suma nada por sí sola (puntos=%d): está cableada de adorno", c.senal, v.Puntos)
		}
		// Y tiene que estar NOMBRADA en el desglose, o el número no se puede auditar.
		hallada := false
		for _, x := range v.Senales {
			if x.Nombre == c.senal {
				hallada = true
				if x.Punto < 1 {
					t.Errorf("la señal %q figura con punto %d cuando debía sumar", c.senal, x.Punto)
				}
			}
		}
		if !hallada {
			t.Errorf("la señal %q no aparece en el desglose del veredicto", c.senal)
		}
	}
}

// --- P2: los escalones calibrados son los que están EN VIGENCIA ------------------------
//
// Este test medía el proxy y no la cosa: llamaba a `escalon()` con números escritos a mano y
// comprobaba su aritmética, que nunca estuvo en duda. Lo que importa es OTRA cosa —que
// `Profundidad` aplique esos bordes— y eso no lo tocaba: mover una constante calibrada lo
// dejaba verde. Se descubrió saboteándolo, que es para lo que sirve el sabotaje.
//
// Ahora cada caso arma unas Senales reales, las pasa por Profundidad y lee el punto del
// desglose. Los bordes van escritos, no leídos de las constantes: un test que toma el mismo
// número que el código no puede detectar que el número cambió.

func TestP2LosEscalonesCalibradosEstanEnVigencia(t *testing.T) {
	casos := []struct {
		senal      string
		poner      func(*Senales, int)
		bajo, alto int
	}{
		{"archivos", func(s *Senales, v int) { s.Archivos = v }, 4, 12},
		{"lineas", func(s *Senales, v int) { s.Lineas = v }, 150, 800},
		{"simbolos", func(s *Senales, v int) { s.Simbolos = v }, 7, 40},
		{"paquetes", func(s *Senales, v int) { s.Paquetes = v }, 2, 5},
		{"callers_en_radio", func(s *Senales, v int) { s.CallersEnRadio = v }, 4, 35},
		{"paquetes_en_radio", func(s *Senales, v int) { s.PaquetesEnRadio = v }, 1, 2},
	}

	punto := func(v Veredicto, nombre string) (int, bool) {
		for _, x := range v.Senales {
			if x.Nombre == nombre {
				return x.Punto, true
			}
		}
		return 0, false
	}

	for _, c := range casos {
		bordes := []struct {
			valor, quiero int
			que           string
		}{
			{c.bajo, 0, "el borde de abajo puntúa 0"},
			{c.bajo + 1, 1, "uno más que el borde de abajo puntúa 1"},
			{c.alto, 1, "el borde de arriba todavía puntúa 1"},
			{c.alto + 1, 2, "pasado el borde de arriba puntúa 2"},
		}
		for _, b := range bordes {
			var sen Senales
			c.poner(&sen, b.valor)
			v := Profundidad(sen)
			got, hallada := punto(v, c.senal)
			if !hallada {
				t.Fatalf("%s: la señal no aparece en el desglose", c.senal)
			}
			if got != b.quiero {
				t.Errorf("%s=%d: %s — quería %d, obtuve %d", c.senal, b.valor, b.que, b.quiero, got)
			}
		}
	}
}

// --- P3: el total vive en 0..13 ---------------------------------------------------------

func TestP3ElTotalNoSePasaDeTrece(t *testing.T) {
	todoAlMaximo := Senales{
		Archivos: 999, Lineas: 99999, Simbolos: 999, Paquetes: 99,
		CallersEnRadio: 999, PaquetesEnRadio: 99, HayBorrados: true,
	}
	if v := Profundidad(todoAlMaximo); v.Puntos != 13 {
		t.Errorf("el techo de la escala es 13, obtuve %d", v.Puntos)
	}
	if v := Profundidad(enCero()); v.Puntos != 0 {
		t.Errorf("el piso de la escala es 0, obtuve %d", v.Puntos)
	}
}

// --- P4: los cortes de nivel ------------------------------------------------------------

func TestP4LosCortesDeNivel(t *testing.T) {
	// Se construye el puntaje con señales reales, no seteando Puntos a mano: si el corte se
	// probara sobre un total inventado, el test no diría nada sobre el camino que corre.
	casos := []struct {
		puntosBuscados int
		quiero         NivelRevision
	}{{0, NivelMinimo}, {2, NivelMinimo}, {3, NivelEstandar}, {7, NivelEstandar}, {8, NivelProfundo}, {13, NivelProfundo}}
	for _, c := range casos {
		s := senalesQueSuman(c.puntosBuscados)
		v := Profundidad(s)
		if v.Puntos != c.puntosBuscados {
			t.Fatalf("armé mal el caso: quería %d puntos y salieron %d", c.puntosBuscados, v.Puntos)
		}
		if v.Nivel != c.quiero {
			t.Errorf("%d puntos deben dar %q, obtuve %q", c.puntosBuscados, c.quiero, v.Nivel)
		}
	}
}

// senalesQueSuman arma unas señales cuyo puntaje total es exactamente n (0..13), subiendo de a
// un punto por señal. Sirve para probar los cortes sin falsear el total.
func senalesQueSuman(n int) Senales {
	// Cada entrada sube UN punto sobre la anterior.
	escalera := []func(*Senales){
		func(s *Senales) { s.Archivos = 5 }, func(s *Senales) { s.Archivos = 13 },
		func(s *Senales) { s.Lineas = 151 }, func(s *Senales) { s.Lineas = 801 },
		func(s *Senales) { s.Simbolos = 8 }, func(s *Senales) { s.Simbolos = 41 },
		func(s *Senales) { s.Paquetes = 3 }, func(s *Senales) { s.Paquetes = 6 },
		func(s *Senales) { s.CallersEnRadio = 5 }, func(s *Senales) { s.CallersEnRadio = 36 },
		func(s *Senales) { s.PaquetesEnRadio = 2 }, func(s *Senales) { s.PaquetesEnRadio = 3 },
		func(s *Senales) { s.HayBorrados = true },
	}
	var s Senales
	for i := 0; i < n && i < len(escalera); i++ {
		escalera[i](&s)
	}
	return s
}

// --- P5: el piso de honestidad ----------------------------------------------------------

func TestP5UnRadioCiegoNoPuedeDarMinima(t *testing.T) {
	s := enCero()
	s.RadioCiego = true
	s.MotivoCiego = "3 archivo(s) sin nodo en el grafo"

	v := Profundidad(s)
	if v.Nivel == NivelMinimo {
		t.Error("con el radio sin medir, «mínima» estaría afirmando lo que no se sabe")
	}
	if v.Nivel != NivelEstandar {
		t.Errorf("el piso sube a estándar, no más: obtuve %q", v.Nivel)
	}
	// Y el motivo tiene que estar ESCRITO. Subir el nivel en silencio deja al que lee sin
	// forma de saber si el cambio era grande o si el índice estaba flojo — dos cosas muy
	// distintas que sin el motivo se ven igual.
	if !strings.Contains(strings.Join(v.Motivos, " "), "sin nodo en el grafo") {
		t.Errorf("el motivo del piso tiene que viajar en el veredicto; motivos=%v", v.Motivos)
	}
}

func TestP5ElPisoNoBajaUnNivelYaAlto(t *testing.T) {
	s := senalesQueSuman(13)
	s.RadioCiego = true
	if v := Profundidad(s); v.Nivel != NivelProfundo {
		t.Errorf("el piso sólo SUBE: un cambio profundo con el radio ciego sigue profundo, obtuve %q", v.Nivel)
	}
}

// --- P6: el panel respeta el piso y el techo --------------------------------------------

func TestP6NingunPanelSeSaleDelPisoNiDelTecho(t *testing.T) {
	for n := 0; n <= 13; n++ {
		v := Profundidad(senalesQueSuman(n))
		p := v.Panel
		if p.Jueces < 1 || p.Rondas < 1 || p.Quorum < 1 {
			t.Errorf("%d puntos: ningún panel baja de 1 juez / 1 ronda / 1 voto, obtuve %+v", n, p)
		}
		if p.Jueces > 5 {
			t.Errorf("%d puntos: el techo es 5 jueces, obtuve %d", n, p.Jueces)
		}
		// DOS rondas es lo que el motor de debate soporta; con más, AdvanceDebate se vuelve un
		// no-op mudo y el panel cree que debatió cuando no debatió.
		if p.Rondas > 2 {
			t.Errorf("%d puntos: el techo es 2 rondas (lo que el motor soporta), obtuve %d", n, p.Rondas)
		}
		if p.Quorum > p.Jueces {
			t.Errorf("%d puntos: un quórum de %d con %d jueces es inalcanzable", n, p.Quorum, p.Jueces)
		}
	}
}

// --- P7: un borrado puro no se lee como «no pasó nada» ----------------------------------

func TestP7UnHunkQueSoloBorraCuenta(t *testing.T) {
	// Hunk de borrado puro: el lado nuevo tiene 0 líneas, así que NO deja rango nuevo (y está
	// bien que no lo deje: los rangos son coordenadas del estado nuevo). Si el tamaño del
	// cambio se midiera sólo por ahí, borrar tres líneas mediría igual que no tocar nada.
	diff := strings.Join([]string{
		"diff --git a/internal/motor.go b/internal/motor.go",
		"--- a/internal/motor.go",
		"+++ b/internal/motor.go",
		"@@ -10,4 +9,0 @@",
		"-func Viejo() {}",
		"-func TambienViejo() {}",
		"-var yaNoSeUsa = 1",
	}, "\n")

	files := ParseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("esperaba 1 archivo, obtuve %d", len(files))
	}
	if got := files[0].Borradas; got != 3 {
		t.Errorf("el hunk borra 3 líneas, conté %d", got)
	}
	if len(files[0].NewRanges) != 0 {
		t.Errorf("un borrado puro no aporta rango nuevo, obtuve %v", files[0].NewRanges)
	}

	s := SenalesDelDiff(files, 0)
	if s.Lineas != 3 {
		t.Errorf("las líneas borradas cuentan como cambio: quería 3, obtuve %d", s.Lineas)
	}
	if !s.HayBorrados {
		t.Error("la séptima señal existe justamente para esto: el borrado tiene que marcarse")
	}
	if Profundidad(s).Puntos < 1 {
		t.Error("borrar tres funciones no puede puntuar igual que no hacer nada")
	}
}

func TestP7UnArchivoBorradoEnteroTambienCuenta(t *testing.T) {
	files := []FileDiff{{Path: "internal/viejo.go", ChangeType: ChangeDeleted}}
	if !SenalesDelDiff(files, 0).HayBorrados {
		t.Error("un archivo borrado entero es un borrado")
	}
}

// --- P8: la derivación del diff cuenta lo que dice contar --------------------------------

func TestP8SenalesDelDiffCuentaArchivosPaquetesYLineas(t *testing.T) {
	files := []FileDiff{
		{Path: "internal/a/uno.go", ChangeType: ChangeModified, Agregadas: 10, Borradas: 2},
		{Path: "internal/a/dos.go", ChangeType: ChangeModified, Agregadas: 5},
		{Path: "cmd/tres.go", ChangeType: ChangeAdded, Agregadas: 40},
		{Path: "assets/logo.png", ChangeType: ChangeModified, Binary: true, Agregadas: 999},
	}
	s := SenalesDelDiff(files, 7)

	if s.Archivos != 3 {
		t.Errorf("los binarios no cuentan como archivo: quería 3, obtuve %d", s.Archivos)
	}
	if s.Lineas != 57 {
		t.Errorf("líneas = agregadas + borradas de los no binarios (10+2+5+40): quería 57, obtuve %d", s.Lineas)
	}
	if s.Paquetes != 2 {
		t.Errorf("dos directorios distintos (internal/a y cmd): quería 2, obtuve %d", s.Paquetes)
	}
	if s.Simbolos != 7 {
		t.Errorf("los símbolos entran por parámetro: quería 7, obtuve %d", s.Simbolos)
	}
	// Y NO enciende la séptima señal: los tres archivos agregan más de lo que sacan. Antes de
	// calibrar esto daba true —bastaba una línea borrada— y por eso la señal valía lo mismo que
	// una constante. Ver TestP10.
	if s.HayBorrados {
		t.Error("agregar 10 y borrar 2 es MODIFICAR, no sacar: la séptima señal no debe encenderse")
	}
}

// --- P9: la cuenta que muestra es la cuenta que hizo -------------------------------------

func TestP9ElDesgloseSumaElTotal(t *testing.T) {
	for n := 0; n <= 13; n++ {
		v := Profundidad(senalesQueSuman(n))
		suma := 0
		for _, x := range v.Senales {
			suma += x.Punto
		}
		if suma != v.Puntos {
			// Un desglose que no suma el total es peor que no tener desglose: invita a
			// verificar y devuelve una verificación falsa.
			t.Errorf("%d puntos: el desglose suma %d y el veredicto dice %d", n, suma, v.Puntos)
		}
		if len(v.Senales) != 7 {
			t.Errorf("son siete señales, el desglose trae %d", len(v.Senales))
		}
	}
}

// --- P10: modificar no es borrar --------------------------------------------------------
//
// EL INVARIANTE QUE FALTABA, y el que hizo que la séptima señal valiera lo mismo que una
// constante durante toda su primera versión.
//
// En un diff unificado, cambiar una línea se escribe como un borrado seguido de un agregado.
// Entonces «hay al menos una línea borrada» —que era el predicado— es cierto en casi todo
// cambio: medido sobre los PRs de este repo, el 83%. Una señal así no ordena; le suma un punto
// a todo el mundo, y el único efecto es que la escala entera se corre para arriba.
//
// Nada de eso se veía en un test: los casos que había usaban un borrado PURO (agregadas 0),
// donde los dos predicados —el flojo y el estricto— dan lo mismo. El invariante hay que
// atacarlo por el lado donde se diferencian, que es la modificación normal.
func TestP10ModificarNoEsBorrar(t *testing.T) {
	casos := []struct {
		nombre   string
		fd       FileDiff
		enciende bool
		porque   string
	}{
		{
			"una modificación normal", // agrega más de lo que saca
			FileDiff{Path: "internal/a.go", ChangeType: ChangeModified, Agregadas: 40, Borradas: 12},
			false, "reescribir doce líneas y sumar cuarenta no saca nada del sistema",
		},
		{
			"una línea cambiada", // el caso mínimo, y el más frecuente de todos
			FileDiff{Path: "internal/a.go", ChangeType: ChangeModified, Agregadas: 1, Borradas: 1},
			false, "un renglón editado es la forma más común de cambio que existe",
		},
		{
			"un archivo nuevo",
			FileDiff{Path: "internal/nuevo.go", ChangeType: ChangeAdded, Agregadas: 200},
			false, "agregar no es sacar",
		},
		{
			"el archivo se vació",
			FileDiff{Path: "internal/a.go", ChangeType: ChangeModified, Agregadas: 2, Borradas: 180},
			true, "sacar 180 y poner 2 es desmantelar el archivo",
		},
		{
			"un archivo borrado entero",
			FileDiff{Path: "internal/viejo.go", ChangeType: ChangeDeleted, Borradas: 90},
			true, "el archivo dejó de existir",
		},
	}
	for _, c := range casos {
		got := SenalesDelDiff([]FileDiff{c.fd}, 0).HayBorrados
		if got != c.enciende {
			t.Errorf("%s: la séptima señal dio %v y debía dar %v — %s", c.nombre, got, c.enciende, c.porque)
		}
	}
}

// P10b: el predicado se evalúa POR ARCHIVO, no sobre el total.
//
// Sin esto, un PR que borra un módulo entero mientras agrega otro más grande sumaría 300
// agregadas contra 200 borradas y saldría limpio, tapando el único acto que la señal existe
// para ver.
func TestP10ElBorradoDeUnArchivoNoLoTapaOtroQueCrece(t *testing.T) {
	files := []FileDiff{
		{Path: "internal/nuevo.go", ChangeType: ChangeAdded, Agregadas: 300},
		{Path: "internal/viejo.go", ChangeType: ChangeDeleted, Borradas: 200},
	}
	if !SenalesDelDiff(files, 0).HayBorrados {
		t.Error("un archivo borrado sigue siendo un borrado aunque el PR sume líneas en total")
	}
}
