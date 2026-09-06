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
		{"archivos", Senales{Archivos: 3}},
		{"lineas", Senales{Lineas: 31}},
		{"simbolos", Senales{Simbolos: 3}},
		{"paquetes", Senales{Paquetes: 2}},
		{"callers_en_radio", Senales{CallersEnRadio: 1}},
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

// --- P2: los escalones son EXACTAMENTE los de la tabla ---------------------------------

func TestP2LosEscalonesCaenDondeDiceLaTabla(t *testing.T) {
	casos := []struct {
		nombre           string
		bajo, alto       int
		enBajo, sobreDos int
	}{
		{"archivos/simbolos", 2, 9, 2, 10},
		{"lineas", 30, 200, 30, 201},
		{"paquetes", 1, 3, 1, 4},
		{"callers", 0, 9, 0, 10},
	}
	for _, c := range casos {
		if got := escalon(c.enBajo, c.bajo, c.alto); got != 0 {
			t.Errorf("%s: el borde de abajo (%d) debe puntuar 0, obtuve %d", c.nombre, c.enBajo, got)
		}
		if got := escalon(c.enBajo+1, c.bajo, c.alto); got != 1 {
			t.Errorf("%s: uno más que el borde (%d) debe puntuar 1, obtuve %d", c.nombre, c.enBajo+1, got)
		}
		if got := escalon(c.alto, c.bajo, c.alto); got != 1 {
			t.Errorf("%s: el borde de arriba (%d) debe puntuar 1, obtuve %d", c.nombre, c.alto, got)
		}
		if got := escalon(c.sobreDos, c.bajo, c.alto); got != 2 {
			t.Errorf("%s: pasado el borde (%d) debe puntuar 2, obtuve %d", c.nombre, c.sobreDos, got)
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
	}{{0, NivelMinimo}, {2, NivelMinimo}, {3, NivelEstandar}, {6, NivelEstandar}, {7, NivelProfundo}, {13, NivelProfundo}}
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
		func(s *Senales) { s.Archivos = 3 }, func(s *Senales) { s.Archivos = 10 },
		func(s *Senales) { s.Lineas = 31 }, func(s *Senales) { s.Lineas = 201 },
		func(s *Senales) { s.Simbolos = 3 }, func(s *Senales) { s.Simbolos = 10 },
		func(s *Senales) { s.Paquetes = 2 }, func(s *Senales) { s.Paquetes = 4 },
		func(s *Senales) { s.CallersEnRadio = 1 }, func(s *Senales) { s.CallersEnRadio = 10 },
		func(s *Senales) { s.PaquetesEnRadio = 2 }, func(s *Senales) { s.PaquetesEnRadio = 4 },
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
	if s.HayBorrados != true {
		t.Error("uno de los archivos borra 2 líneas")
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
