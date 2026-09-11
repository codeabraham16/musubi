package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LAS PRUEBAS QUE ESTAS DOS FUNCIONES NUNCA TUVIERON.
//
// Interpolan texto que ARMA EL CEREBRO adentro de un programa que se EJECUTA en la máquina de
// otra persona. No tenían cobertura por un motivo mecánico y no por descuido: vivían en archivos
// con sufijo de plataforma, así que en Linux no existían y una prueba para ellas tampoco podía
// existir. Ver el encabezado de `escapes.go`.

func TestEscaparPSNoDejaCerrarLaCadena(t *testing.T) {
	// EL CASO QUE IMPORTA ES EL ATAQUE, no el texto con acentos. El resultado se interpola entre
	// comillas simples, así que lo único que puede romper la jaula es una comilla simple.
	casos := []struct {
		nombre  string
		entrada string
		quiero  string
	}{
		{"texto normal no se toca", "El disco está al 92%", "El disco está al 92%"},
		{"una comilla simple se duplica", "no' se", "no'' se"},
		{"la fuga clásica: cerrar y encadenar", `'; rm -rf /; '`, `''; rm -rf /; ''`},
		{"varias comillas, todas", `'''`, `''''''`},
		{"el salto de línea se vuelve espacio", "uno\ndos", "uno dos"},
		{"el retorno de carro también", "uno\r\ndos", "uno  dos"},
		{"la barra invertida NO se toca", `C:\ruta\x`, `C:\ruta\x`},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := escaparPS(c.entrada); got != c.quiero {
				t.Errorf("escaparPS(%q) = %q, quería %q", c.entrada, got, c.quiero)
			}
		})
	}

	// LA PROPIEDAD, dicha como propiedad y no como tabla: el resultado, metido entre comillas
	// simples, no puede tener NINGUNA comilla simple suelta. Una tabla sólo cubre los casos que
	// a alguien se le ocurrieron.
	for _, veneno := range []string{
		`'`, `''`, `'''`, `a'b`, `'+$(calc)+'`, "fin'\n'inicio", `\'`,
	} {
		jaula := "'" + escaparPS(veneno) + "'"
		if comillasSueltas(jaula) {
			t.Errorf("con %q la cadena de PowerShell queda %s, que TIENE una comilla sin par: "+
				"el texto se escapa de la jaula y lo que sigue corre como código", veneno, jaula)
		}
	}

	// LA BARRA INVERTIDA NO SE ESCAPA, y eso es correcto y hay que dejarlo escrito: adentro de una
	// cadena SIMPLE de PowerShell la barra es un carácter común. Duplicarla sería un defecto —las
	// rutas de Windows llegarían con `\\`— y esta prueba existe para que nadie lo "arregle".
	if got := escaparPS(`C:\Users\gio`); got != `C:\Users\gio` {
		t.Errorf("escaparPS tocó las barras: %q. En una cadena simple de PowerShell la barra no "+
			"escapa nada, y duplicarla rompe todas las rutas de Windows", got)
	}
}

// comillasSueltas dice si la cadena tiene alguna comilla simple que no venga de a pares adentro.
//
// Se mira el INTERIOR: el primer y el último carácter son la jaula que pone el llamador.
func comillasSueltas(jaula string) bool {
	dentro := jaula[1 : len(jaula)-1]
	for i := 0; i < len(dentro); i++ {
		if dentro[i] != '\'' {
			continue
		}
		if i+1 >= len(dentro) || dentro[i+1] != '\'' {
			return true
		}
		i++ // era un par: se salta el segundo
	}
	return false
}

func TestEscaparAppleScriptEscapaLaBarraPrimero(t *testing.T) {
	casos := []struct {
		nombre  string
		entrada string
		quiero  string
	}{
		{"texto normal no se toca", "El disco está al 92%", "El disco está al 92%"},
		{"la comilla doble se escapa", `di "hola"`, `di \"hola\"`},
		{"la barra se duplica", `C:\x`, `C:\\x`},
		// EL CASO QUE DECIDE EL ORDEN. Si la comilla se escapara ANTES que la barra, el `\` que
		// se agrega se volvería a escapar: el resultado sería `\\"`, o sea una barra literal
		// seguida de una comilla que CIERRA la cadena — el agujero completo.
		{"barra pegada a comilla: el orden importa", `a\"b`, `a\\\"b`},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := escaparAppleScript(c.entrada); got != c.quiero {
				t.Errorf("escaparAppleScript(%q) = %q, quería %q", c.entrada, got, c.quiero)
			}
		})
	}

	// LA PROPIEDAD: ninguna comilla doble del resultado puede quedar sin una cantidad IMPAR de
	// barras delante, que es la condición para que AppleScript la lea como literal y no como el
	// final de la cadena.
	for _, veneno := range []string{
		`"`, `\"`, `\\"`, `" & (do shell script "id") & "`, `fin" with title "`,
	} {
		for _, p := range posicionesDeComillaQueCierra(escaparAppleScript(veneno)) {
			t.Errorf("con %q el AppleScript queda %q y la comilla de la posición %d CIERRA la cadena: "+
				"lo que sigue se ejecuta", veneno, escaparAppleScript(veneno), p)
		}
	}
}

// posicionesDeComillaQueCierra devuelve las comillas dobles precedidas por una cantidad PAR de
// barras invertidas, que son las que AppleScript lee como fin de cadena.
func posicionesDeComillaQueCierra(s string) []int {
	var fuera []int
	for i := 0; i < len(s); i++ {
		if s[i] != '"' {
			continue
		}
		barras := 0
		for k := i - 1; k >= 0 && s[k] == '\\'; k-- {
			barras++
		}
		if barras%2 == 0 {
			fuera = append(fuera, i)
		}
	}
	return fuera
}

// NADIE VUELVE A ESCRIBIR LA REGLA A MANO.
//
// Eran TRES copias: `avisador_windows.go`, `avisador_darwin.go` e `install.go`, que tenía la suya
// inline para armar el comando que toca el PATH del usuario. Es el defecto dominante del repo —la
// cautela escrita dos veces y divergiendo— sobre la superficie de más consecuencia que hay.
func TestNadieReimplementaElEscapeDePowerShellAMano(t *testing.T) {
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude listar el paquete: %v", err)
	}
	// La regla de PowerShell escrita a mano: duplicar la comilla simple.
	//
	// SE ARMA EN DOS PEDAZOS porque si no esta guarda SE DETECTA A SÍ MISMA — la línea que
	// declara la aguja contiene la aguja. Lo cómodo habría sido exceptuar este archivo, y eso la
	// dejaría ciega justo sobre el archivo donde vive, que es la forma exacta del defecto que
	// persigue.
	aguja := `"'", ` +
		`"''"`
	revisados := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || e.Name() == "escapes.go" {
			continue
		}
		crudo, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			t.Fatalf("no pude leer %s: %v", e.Name(), err)
		}
		revisados++
		for n, linea := range strings.Split(string(crudo), "\n") {
			desnuda := strings.TrimSpace(linea)
			if strings.HasPrefix(desnuda, "//") || !strings.Contains(linea, aguja) {
				continue
			}
			t.Errorf("%s:%d reimplementa el escape de PowerShell a mano:\n    %s\n"+
				"  Usá `escaparPS`. Tres copias de esta regla es como llegó a no tener ninguna prueba: "+
				"cada una se arregla por su cuenta y la que se queda vieja es la del camino que menos "+
				"se mira.", e.Name(), n+1, desnuda)
		}
	}
	if revisados < 20 {
		t.Fatalf("sólo se revisaron %d archivos .go del paquete y hay bastantes más: cambió dónde "+
			"viven y esta guarda está en verde sin mirar nada", revisados)
	}
}

// Y LOS ESCAPADORES SIGUEN EN UN ARCHIVO SIN SUFIJO DE PLATAFORMA.
//
// Es lo que hace que estas pruebas existan. Devolverlos a `avisador_windows.go` las borraría en
// silencio: en Linux el archivo no compila, las funciones no existen, y `go test` contesta `ok`.
func TestLosEscapadoresViajanEnUnArchivoSinSufijoDePlataforma(t *testing.T) {
	crudo, err := os.ReadFile("escapes.go")
	if err != nil {
		t.Fatalf("`escapes.go` ya no existe: si los escapadores volvieron a un archivo con sufijo de "+
			"plataforma, estas pruebas dejan de compilar en Linux y desaparecen sin dejar rastro: %v", err)
	}
	texto := string(crudo)
	for _, fn := range []string{"func escaparPS(", "func escaparAppleScript("} {
		if !strings.Contains(texto, fn) {
			t.Errorf("`%s` ya no vive en escapes.go: mudarlo a un archivo `_windows.go` o `_darwin.go` "+
				"lo saca del build de Linux y borra su prueba sin que nada se ponga rojo", fn)
		}
	}
	// Un `//go:build` adentro de escapes.go lograría lo mismo que el sufijo.
	if strings.Contains(texto, "//go:build") || strings.Contains(texto, "// +build") {
		t.Error("escapes.go tiene una restricción de build: el archivo existe pero no compila en todos " +
			"lados, que es la misma desaparición con otra forma")
	}
}
