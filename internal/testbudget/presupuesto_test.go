package testbudget

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// salidaCon arma una salida de `go test` con los paquetes y segundos que se le pasen, en el
// formato exacto que imprime Go. Se genera en vez de tipearse para que la prueba no dependa de
// que yo copie bien un log.
func salidaCon(pares ...any) string {
	var b strings.Builder
	b.WriteString("=== RUN   TestAlgo\n--- PASS: TestAlgo (0.01s)\nPASS\n")
	for i := 0; i+1 < len(pares); i += 2 {
		b.WriteString("ok  \t" + pares[i].(string) + "\t" + pares[i+1].(string) + "s\n")
	}
	return b.String()
}

func politica(techo string, margen float64) Politica {
	d, err := time.ParseDuration(techo)
	if err != nil {
		panic(err)
	}
	return Politica{Timeout: d, MargenMinimo: margen}
}

// EL GUARD SE PONE ROJO CUANDO EL MARGEN SE COME EL UMBRAL.
//
// Esta es la prueba central del cabo: lo que decide es Rojo(), calculado como TECHO/MÁS_LENTO
// contra el mínimo de la política. NO mira el texto del informe ni un comentario: mira el
// booleano que hace salir 1 al comando.
//
// Los casos van pegados al umbral en los DOS lados con el MISMO techo, así que un guard que
// devolviera siempre true o siempre false falla en uno de los dos.
func TestMargenComidoDaRojo(t *testing.T) {
	casos := []struct {
		nombre       string
		techo        string
		margenMinimo float64
		masLentoSeg  string
		quieroRojo   bool
		quieroMargen float64
	}{
		{
			// El caso que motivó el cabo: 567,7 s contra el techo viejo de 20 min es 2,11× —
			// por encima de 2,0 por un pelo. Un runner 2,2× más lento ya no termina.
			nombre: "20m contra 567,7s roza el umbral y todavia pasa",
			techo:  "20m", margenMinimo: 2.0, masLentoSeg: "567.7",
			quieroRojo: false, quieroMargen: 1200.0 / 567.7,
		},
		{
			// Un 6 % más caro y el mismo techo ya no sostiene la política.
			nombre: "20m contra 601s se come el umbral",
			techo:  "20m", margenMinimo: 2.0, masLentoSeg: "601.0",
			quieroRojo: true, quieroMargen: 1200.0 / 601.0,
		},
		{
			nombre: "justo en el umbral no es rojo",
			techo:  "20m", margenMinimo: 2.0, masLentoSeg: "600.0",
			quieroRojo: false, quieroMargen: 2.0,
		},
		{
			// Con el techo nuevo el mismo paquete tiene aire de sobra.
			nombre: "30m contra 567,7s tiene margen",
			techo:  "30m", margenMinimo: 2.0, masLentoSeg: "567.7",
			quieroRojo: false, quieroMargen: 1800.0 / 567.7,
		},
		{
			// Y el techo nuevo también se puede comer: el guard sigue vigilando.
			nombre: "30m contra 901s se come el umbral igual",
			techo:  "30m", margenMinimo: 2.0, masLentoSeg: "901.0",
			quieroRojo: true, quieroMargen: 1800.0 / 901.0,
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// El más lento va en el MEDIO de la lista: un guard que se quedara con el primero
			// o con el último elegiría el paquete equivocado y este caso lo delata.
			salida := salidaCon(
				"musubi/internal/logx", "1.2",
				"musubi/internal/mcp", c.masLentoSeg,
				"musubi/internal/memory", "460.0",
			)
			v, err := Analizar(salida, politica(c.techo, c.margenMinimo))
			if err != nil {
				t.Fatalf("Analizar devolvió error inesperado: %v", err)
			}
			if v.MasLento.Nombre != "musubi/internal/mcp" {
				t.Fatalf("el más lento tendría que ser internal/mcp, fue %q (%.1fs)",
					v.MasLento.Nombre, v.MasLento.Duracion.Seconds())
			}
			if dif := v.Margen - c.quieroMargen; dif > 0.001 || dif < -0.001 {
				t.Errorf("margen = %.4f, quería %.4f", v.Margen, c.quieroMargen)
			}
			if v.Rojo() != c.quieroRojo {
				t.Errorf("Rojo() = %v, quería %v (margen %.3f× vs mínimo %.2f×)",
					v.Rojo(), c.quieroRojo, v.Margen, c.margenMinimo)
			}
		})
	}
}

// UN CERO TIENE QUE SIGNIFICAR «MEDÍ Y ESTÁ BIEN», NUNCA «NO PUDE MEDIR».
//
// Cada caso de acá es una forma de NO tener medición. Si Analizar devolviera un Veredicto en
// vez de un error, el margen saldría infinito o absurdo y el guard daría verde justo cuando
// dejó de mirar — el modo de falla que este repo ya pagó con un emisor que contestaba 0
// incondicional.
func TestSinMedicionEsError(t *testing.T) {
	casos := []struct {
		nombre string
		salida string
	}{
		{"salida vacía", ""},
		{"corrida que murió antes de reportar ningún paquete",
			"=== RUN   TestAlgo\npanic: test timed out after 10m0s\n"},
		{"sólo paquetes sin tests",
			"?   \tmusubi/internal/logx\t[no test files]\n?   \tmusubi/cmd/musubi\t[no test files]\n"},
		{"todo servido del caché: no trae segundos",
			"ok  \tmusubi/internal/mcp\t(cached)\nok  \tmusubi/internal/memory\t(cached)\n"},
		{"un paquete cacheado puede esconder al más lento",
			"ok  \tmusubi/internal/logx\t1.2s\nok  \tmusubi/internal/mcp\t(cached)\n"},
		{"el más lento reporta 0 s: la corrida no midió nada",
			"ok  \tmusubi/internal/logx\t0.0s\nok  \tmusubi/internal/mcp\t0.0s\n"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v, err := Analizar(c.salida, politica("30m", 2.0))
			if err == nil {
				t.Fatalf("Analizar dio VERDE sin medición: margen %.2f× sobre %q (%.1fs). "+
					"«no pude medir» tiene que ser rojo, no un margen inventado",
					v.Margen, v.MasLento.Nombre, v.MasLento.Duracion.Seconds())
			}
		})
	}
}

// Una corrida con medición REAL sí tiene que pasar: si el guard fuera «error siempre», el test
// de arriba estaría verde por la razón equivocada.
func TestConMedicionRealNoEsError(t *testing.T) {
	salida := "ok  \tmusubi/internal/logx\t1.2s\nok  \tmusubi/internal/mcp\t567.7s\n"
	v, err := Analizar(salida, politica("30m", 2.0))
	if err != nil {
		t.Fatalf("una corrida medida no puede dar error: %v", err)
	}
	if len(v.Medidos) != 2 {
		t.Fatalf("midió %d paquetes, quería 2", len(v.Medidos))
	}
	if errors.Is(err, ErrSinMedicion) {
		t.Fatal("no debería ser ErrSinMedicion")
	}
}

// Un paquete que FALLÓ también reporta sus segundos, y su tiempo cuenta para el presupuesto:
// el techo lo choca igual esté verde o rojo el paquete.
func TestFAILTambienCuenta(t *testing.T) {
	salida := "ok  \tmusubi/internal/logx\t1.2s\nFAIL\tmusubi/internal/mcp\t900.0s\n"
	v, err := Analizar(salida, politica("30m", 2.0))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if v.MasLento.Nombre != "musubi/internal/mcp" {
		t.Fatalf("un paquete FAIL con 900 s tiene que ser el más lento, fue %q", v.MasLento.Nombre)
	}
}

// El ancla va al CAMPO, no a un substring: una línea de salida de un test que arranque con
// «ok » no es un paquete. Sin esto, un test que imprime «ok  algo  5s» inventaría un paquete.
func TestNoConfundeSalidaDeUnTestConUnPaquete(t *testing.T) {
	salida := "    ok  musubi/falso  9999.0s (esto lo imprimió un test, no `go test`)\n" +
		"ok  \tmusubi/internal/mcp\t100.0s\n"
	v, err := Analizar(salida, politica("30m", 2.0))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(v.Medidos) != 1 || v.MasLento.Nombre != "musubi/internal/mcp" {
		t.Fatalf("contó %d paquetes, más lento %q: una línea indentada de un test no es un paquete",
			len(v.Medidos), v.MasLento.Nombre)
	}
}

// LA POLÍTICA DEL REPO TIENE QUE SER LEÍBLE Y COHERENTE.
//
// Esto es lo que ata el archivo presupuesto-de-pruebas.env al guard: si alguien lo borra, lo
// renombra, o le rompe una clave, esto se pone rojo acá y no dentro de seis semanas en un CI
// que nadie mira.
func TestPoliticaDelRepoSeLee(t *testing.T) {
	p, err := CargarPoliticaDelRepo(".")
	if err != nil {
		t.Fatalf("no se pudo cargar %s del repo: %v", NombreArchivoPolitica, err)
	}
	// El techo tiene que ser mayor que el default de Go: si fuera <= 10m, pasarlo explícitamente
	// no arreglaría nada y el `-timeout` de CI sería decorativo.
	if p.Timeout < TimeoutPiso || p.Timeout > TimeoutTecho {
		t.Errorf("RACE_TIMEOUT = %v, fuera de [%v, %v]", p.Timeout, TimeoutPiso, TimeoutTecho)
	}
	if p.MargenMinimo < MargenMinimoPiso || p.MargenMinimo > MargenMinimoTecho {
		t.Errorf("MARGEN_MINIMO = %v, fuera de [%v, %v]", p.MargenMinimo, MargenMinimoPiso, MargenMinimoTecho)
	}
	if p.UmbralLineasTest < UmbralLineasPiso || p.UmbralLineasTest > UmbralLineasTecho {
		t.Errorf("UMBRAL_GUARDA_LINEAS_TEST = %d, fuera de [%d, %d]",
			p.UmbralLineasTest, UmbralLineasPiso, UmbralLineasTecho)
	}
}

// UNA POLÍTICA DEGRADADA HASTA SER INOFENSIVA ES UN ERROR, NO UNA POLÍTICA.
//
// Éste era el agujero: el guard exigía «margen > 1» y «techo > 10m», así que un MARGEN_MINIMO de
// 1,01 o un RACE_TIMEOUT de 500h lo apagaban PARA SIEMPRE y pasaban verde. El aparato entero
// seguía corriendo en cada PR sin poder decir que no — el peor de los dos mundos: el costo de la
// guarda sin la guarda.
func TestUnaPoliticaDegradadaNoPasa(t *testing.T) {
	dir := t.TempDir()
	casos := map[string]string{
		"margen apenas mayor que 1":     "RACE_TIMEOUT=40m\nMARGEN_MINIMO=1.01\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"margen de 1,4":                 "RACE_TIMEOUT=40m\nMARGEN_MINIMO=1.4\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"margen inalcanzable":           "RACE_TIMEOUT=40m\nMARGEN_MINIMO=50\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"techo de 500h":                 "RACE_TIMEOUT=500h\nMARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"techo por debajo del de Go":    "RACE_TIMEOUT=5m\nMARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"umbral que no alcanza a nadie": "RACE_TIMEOUT=40m\nMARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=1000000\n",
		"sin umbral":                    "RACE_TIMEOUT=40m\nMARGEN_MINIMO=2.0\n",
	}
	for nombre, contenido := range casos {
		t.Run(nombre, func(t *testing.T) {
			ruta := dir + "/" + strings.ReplaceAll(nombre, " ", "_") + ".env"
			escribir(t, ruta, contenido)
			p, err := CargarPolitica(ruta)
			if err == nil {
				t.Fatalf("CargarPolitica aceptó una política que apaga el guard: %+v", p)
			}
			t.Logf("rojo (correcto): %v", err)
		})
	}
}

// Una clave que falta es un error, no un cero silencioso.
func TestPoliticaIncompletaEsError(t *testing.T) {
	dir := t.TempDir()
	casos := map[string]string{
		"sin RACE_TIMEOUT":         "MARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"sin MARGEN_MINIMO":        "RACE_TIMEOUT=30m\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"timeout no parseable":     "RACE_TIMEOUT=veinte\nMARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"margen de 1 no es margen": "RACE_TIMEOUT=30m\nMARGEN_MINIMO=1.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n",
		"todo comentado":           "# RACE_TIMEOUT=30m\n# MARGEN_MINIMO=2.0\n",
	}
	for nombre, contenido := range casos {
		t.Run(nombre, func(t *testing.T) {
			ruta := dir + "/" + strings.ReplaceAll(nombre, " ", "_") + ".env"
			escribir(t, ruta, contenido)
			if p, err := CargarPolitica(ruta); err == nil {
				t.Fatalf("CargarPolitica aceptó una política rota: %+v", p)
			}
		})
	}
}

func escribir(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}
