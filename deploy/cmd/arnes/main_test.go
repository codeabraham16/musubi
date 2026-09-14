package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestElRojoSeLeeYNoSeDeduceDelExitCode fija lo que `revisarElRojo` tiene que distinguir.
//
// POR QUÉ EXISTE ESTA PRUEBA. La primera versión de `correrTodos` contaba `err == nil` como «en
// ROJO» y publicó «70 corridas, 70 en rojo» con ese conteo. `sabotaje.sh` sale con 0 cuando la
// prueba cayó, y eso no dice POR QUÉ cayó: este repo tiene medido que un rojo miente por build roto
// o porque el sabotaje volteó OTRA aserción de la misma prueba. O sea que el arnés escrito para no
// creerle a un verde le estaba creyendo a un exit code, que es el mismo atajo un piso más arriba.
//
// Las setenta corridas se releyeron después a mano y aguantaron —cero motivos repetidos, cero
// fallos fuera del archivo del ancla, cero pruebas distintas de la declarada—, así que el número
// publicado no cambia. Lo que cambia es que ahora lo comprueba la herramienta y no yo con un script
// de un solo uso.
//
// Sabotaje que la hace fallar: en `revisarElRojo`, cambiar `if len(fallos) == 0 {` por `if false {`
// → un rojo sin ninguna prueba fallando pasa a devolver otra queja y el caso «mudo» cae.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="if len(fallos) == 0 {"
// arnes: a="if false {"
// arnes: prueba="TestElRojoSeLeeYNoSeDeduceDelExitCode"
//
// Y LA OTRA DIRECCIÓN, que hasta esta línea NUNCA se había ejercitado en el árbol: `arreglo_de` y
// `arreglo_a` estaban parseados, validados y despachados por `comandoMutador`, con CERO directivas
// y CERO pruebas que los usaran. Construido y nunca prendido, adentro del arnés escrito para cazar
// eso. Lo levantó otra sesión contando las apariciones en el diff, no leyendo el código.
//
// EL ARREGLO ES REDACTAR MEJOR UN MENSAJE, y no es un ejemplo inventado: es el cambio que la falla 7
// encontró apenas se prendió. La primera versión de esta prueba comparaba los textos EXACTOS de cada
// queja, así que cambiar «no imprimió» por «no nombró» —mismo significado, mejor palabra— la ponía
// ROJA. Estaba fijando la prosa y no el comportamiento.
//
// El arreglo no fue aflojar la aserción hasta que no midiera nada: fue darle a cada queja una
// IDENTIDAD que no es su prosa (`claseDeQueja`) y afirmar eso más los DATOS que el mensaje tiene que
// llevar. Medido: con el cambio de abajo, rc=1 → rc=0.
// arnes: arreglo_de="no imprimió NINGUNA prueba fallando"
// arnes: arreglo_a="no nombró NINGUNA prueba fallando"
func TestElRojoSeLeeYNoSeDeduceDelExitCode(t *testing.T) {
	// La forma EXACTA que imprime deploy/pruebas/sabotaje.sh, copiada de una corrida real.
	sano := "" +
		"  ✓ control: compila y pasa (1 subtest/s REALMENTE ejecutados)\n" +
		"  ✓ el sabotaje se aplicó y COMPILA\n" +
		"  ✓ ROJO, y falla en:\n" +
		"      TestElInventarioNoViajaEnCadaLatido\n" +
		"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
		"      TestElInventarioNoViajaEnCadaLatido · agent_test.go:101: el latido volvió a mandar el inventario\n" +
		"\n" +
		"  ✓ el sabotaje funciona\n"

	casos := []struct {
		nombre       string
		salida       string
		archivoAncla string
		prueba       string
		motivo       string       // "" = no espero motivo
		clase        claseDeQueja // la IDENTIDAD de la queja, que no depende de cómo se redacte
		debeLlevar   []string     // los DATOS que el mensaje tiene que llevar sí o sí
	}{
		{
			nombre:       "el rojo normal deja motivo y no deja queja",
			salida:       sano,
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestElInventarioNoViajaEnCadaLatido",
			motivo:       "agent_test.go:101: el latido volvió a mandar el inventario",
			clase:        sinQueja,
		},
		{
			// EL CASO QUE EL EXIT CODE NO PUEDE VER. El guion salió con 0 y no dijo qué cayó.
			nombre:       "salio con cero y no nombro ninguna prueba",
			salida:       "  ✓ control: compila y pasa (1 subtest/s REALMENTE ejecutados)\n",
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestAlgo",
			clase:        quejaSinPruebaFallando,
		},
		{
			nombre: "cayo una prueba que no es la declarada",
			salida: "  ✓ ROJO, y falla en:\n      TestOtraCosa\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestOtraCosa · agent_test.go:101: algo\n",
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestElInventarioNoViajaEnCadaLatido",
			clase:        quejaOtraPrueba,
			// LOS DOS NOMBRES SON DATOS, NO PROSA: sin ellos el mensaje no sirve para ir a mirar.
			debeLlevar: []string{"TestOtraCosa", "TestElInventarioNoViajaEnCadaLatido"},
		},
		{
			// UN SUBTEST NO ES UN DESVÍO: `TestX/caso` cuelga de `TestX`.
			nombre: "un subtest de la declarada no es un desvio",
			salida: "  ✓ ROJO, y falla en:\n      TestX/el_caso_raro\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestX/el_caso_raro · x_test.go:12: algo\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			motivo:       "x_test.go:12: algo",
			clase:        sinQueja,
		},
		{
			nombre:       "se puso en rojo sin una linea que ubique la asercion",
			salida:       "  ✓ ROJO, y falla en:\n      TestX\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			clase:        quejaSinLinea,
		},
		{
			// LA ASERCIÓN CAYÓ EN OTRO ARCHIVO. No invalida el rojo; lo manda a leer.
			nombre: "la asercion que cayo vive en otro archivo",
			salida: "  ✓ ROJO, y falla en:\n      TestX\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestX · ayudantes_test.go:12: algo\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			motivo:       "ayudantes_test.go:12: algo",
			clase:        quejaOtroArchivo,
			debeLlevar:   []string{"ayudantes_test.go", "x_test.go"},
		},
		{
			nombre: "el motivo no trae numero de linea",
			salida: "  ✓ ROJO, y falla en:\n      TestX\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestX · algo salió mal en x_test.go: sin número\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			clase:        quejaMotivoIlegible,
			// El texto que no se pudo leer TIENE que viajar: sin él no se sabe qué arreglar.
			debeLlevar: []string{"algo salió mal en x_test.go"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// La raíz es un directorio vacío A PROPÓSITO: estos casos miden cómo se LEE la salida
			// del guion, y con la raíz vacía el barrido por AST no encuentra archivo y se calla —
			// que es lo que tiene que hacer cuando no puede contestar. La otra mitad, la de mirar
			// el AST de verdad, se mide en `TestUnMotivoQueEsUnLogSeDenunciaYUnoQueEsAsercionNo`.
			motivo, clase, queja := revisarElRojo(c.salida, c.archivoAncla, c.prueba, t.TempDir(), "./internal/x/")
			if motivo != c.motivo {
				t.Errorf("motivo:\n  esperaba %q\n  vino     %q", c.motivo, motivo)
			}
			if clase != c.clase {
				t.Errorf("clase de queja: esperaba %d, vino %d (queja: %q)", c.clase, clase, queja)
			}
			// SE AFIRMA QUE HAY TEXTO Y QUE LLEVA LOS DATOS, NUNCA CÓMO ESTÁ REDACTADO. Una queja
			// vacía deja al que la lee sin nada que hacer; una queja redactada distinto no.
			if c.clase == sinQueja && queja != "" {
				t.Errorf("no esperaba queja y vino %q", queja)
			}
			if c.clase != sinQueja && strings.TrimSpace(queja) == "" {
				t.Error("la clase dice que hay queja y el texto vino vacío: el que la lee no tiene qué hacer")
			}
			for _, dato := range c.debeLlevar {
				if !strings.Contains(queja, dato) {
					t.Errorf("la queja no lleva %q, así que no se puede ir a mirar:\n  %s", dato, queja)
				}
			}
		})
	}
}

// TestDosSabotajesQueFallanIgualDanLaMISMAClave es la razón de ser del motivo canónico.
//
// `sabotaje.sh` imprime el motivo diciendo explícitamente para qué sirve —«para poder
// compararlos»— y NO PUEDE COMPARARLO, porque ve una corrida por vez. Dos sabotajes distintos que
// voltean la misma aserción son UN sabotaje contado dos veces: la cobertura sube y lo cubierto no.
// Por eso la clave se arma sin el nombre de la prueba: lo que identifica a una aserción es dónde
// vive y qué dice, no quién la ejercitó.
//
// Sabotaje que la hace fallar: en `revisarElRojo`, borrar el `strings.Cut(primera, " · ")` que le
// saca el nombre de la prueba de adelante → las dos claves pasan a diferir y el conteo de motivos
// repetidos da cero para siempre, que es un cero que significa «no miré».
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="if _, resto, ok := strings.Cut(primera, \" · \"); ok {"
// arnes: a="if _, resto, ok := strings.Cut(primera, \"\\x00NUNCA\\x00\"); ok {"
// arnes: prueba="TestDosSabotajesQueFallanIgualDanLaMISMAClave"
func TestDosSabotajesQueFallanIgualDanLaMISMAClave(t *testing.T) {
	uno := "  ✓ ROJO, y falla en:\n      TestUno\n" +
		"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
		"      TestUno · guarda_test.go:42: el camino sin permiso llegó al ejecutor\n"
	otro := "  ✓ ROJO, y falla en:\n      TestOtro\n" +
		"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
		"      TestOtro · guarda_test.go:42: el camino sin permiso llegó al ejecutor\n"

	// Raíz vacía: acá se mide la CLAVE, no el AST. Ver la nota en el caso de arriba.
	vacia := t.TempDir()
	a, claseA, quejaA := revisarElRojo(uno, "internal/x/guarda_test.go", "TestUno", vacia, "./internal/x/")
	b, claseB, quejaB := revisarElRojo(otro, "internal/x/guarda_test.go", "TestOtro", vacia, "./internal/x/")
	if claseA != sinQueja || claseB != sinQueja {
		t.Fatalf("no esperaba quejas: %q / %q", quejaA, quejaB)
	}
	if a == "" {
		t.Fatal("no salió motivo: sin motivo no hay nada que comparar y los repetidos dan 0 siempre")
	}
	if a != b {
		t.Errorf("dos pruebas que caen en la MISMA aserción tienen que dar la misma clave:\n  %q\n  %q", a, b)
	}
}

// TestElRecorteDelMotivoNoParteUnCaracter fija que el recorte sea POR RUNA.
//
// Este repo ya pagó el corte por byte una vez y en esta misma herramienta: la prosa se cortó a
// mitad de un carácter multibyte, la salida quedó UTF-8 inválido, `grep` contestó «binary file
// matches» y la comparación que seguía INVENTÓ cinco anclas perdidas. Casi acuso al lector de un
// agujero que estaba en el impresor.
//
// Sabotaje que la hace fallar: en `primerasRunas`, cambiar `[]rune(s)` por `[]byte(s)`.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tr := []rune(s)"
// arnes: a="\tr := []byte(s)"
// arnes: prueba="TestElRecorteDelMotivoNoParteUnCaracter"
func TestElRecorteDelMotivoNoParteUnCaracter(t *testing.T) {
	// Cada «ó» ocupa DOS bytes y UNA runa: cortar en 5 por byte parte la tercera.
	const entrada = "óóóóóóóóóó"
	got := primerasRunas(entrada, 5)
	const quiero = "óóóóó…"
	if got != quiero {
		t.Errorf("recorte por byte, no por runa:\n  esperaba %q (%d bytes)\n  vino     %q (%d bytes)",
			quiero, len(quiero), got, len(got))
	}
	for i, r := range got {
		if r == '�' {
			t.Errorf("byte %d del recorte no es UTF-8 válido: el corte partió un carácter", i)
		}
	}
	if corto := primerasRunas("abc", 10); corto != "abc" {
		t.Errorf("lo que entra entero tiene que salir entero y sin puntos suspensivos: %q", corto)
	}
}

// TestElOverlaySoloAlcanzaLoQueLeeElComandoGo fija el tercer canasto del modo `-overlay`.
//
// POR QUÉ EXISTE. La primera corrida completa del detector marcó 6 «candidatos a rojo falso» y
// CINCO eran esto: sabotajes contra `.yml`, `.sh` y `.conf`. `go test -overlay` reemplaza archivos
// para el BUILD DE GO; un YAML que la prueba lee en runtime queda intacto, el sabotaje no se aplica
// y la prueba pasa. O sea que su verde estaba garantizado de antemano y no decía nada sobre la
// guarda. Es el instrumento contestando bien OTRA pregunta — escrito, encima, para cazar eso mismo.
//
// «No pude medir» y «medí y está sano» tienen que salir por puertas distintas, así que ahora salen
// por un contador propio.
//
// Sabotaje que la hace fallar: en `elOverlayPuedeTocar`, aceptar cualquier archivo → los cinco
// `.yml`/`.sh`/`.conf` de la tabla vuelven a contarse como medibles.
// arnes: archivo="deploy/cmd/arnes/main.go"
// El sabotaje va en la PRIMERA rama y no en el `return` final, y no es una preferencia: con
// `return true` al final, `base` queda `declared and not used` y el sabotaje NO COMPILA. Es la
// clase que este repo ya tiene medida —cuando la guarda es la única lectora de una variable, el
// sabotaje literal es imposible— y el arnés la contó como SIN VEREDICTO, que es lo correcto.
// arnes: de="\tif strings.HasSuffix(archivo, \".go\") {"
// arnes: a="\tif true {"
// arnes: prueba="TestElOverlaySoloAlcanzaLoQueLeeElComandoGo"
func TestElOverlaySoloAlcanzaLoQueLeeElComandoGo(t *testing.T) {
	// Los cinco de la corrida real, y un `.go` de control: sin el control, «devolver false siempre»
	// también pasaría, y eso apagaría el modo entero en silencio.
	casos := []struct {
		archivo string
		puede   bool
	}{
		{"internal/arnes/arnes.go", true},
		{"internal/mcp/sonda_permiso_test.go", true},
		{"deploy/musubi-alerts-flota.yml", false},
		{"deploy/rustdesk/compose.yml", false},
		{"deploy/redesplegar-cerebro.sh", false},
		{"deploy/systemd/musubi-agente-contenedores.conf", false},
		{"deploy/docker/compose.yml", false},
		// LAS DOS FILAS QUE DESARMAN LAS DOS REGLAS EQUIVOCADAS, y por eso son las que importan.
		//
		// `go.mod` NO es fuente y NO lo lee el compilador, y el overlay lo cambia igual: medido con
		// `go test -overlay` sobre un go.mod que pide `go 1.99` → rc=1, «requires go >= 1.99». Con
		// la regla «el compilador lo lee como fuente» esta fila decía `false` y estaba MAL.
		{"go.mod", true},
		{"deploy/go.sum", true},
		// Y no alcanza con el sufijo: `HasSuffix(…, "go.mod")` aceptaría esto, que no es un go.mod.
		{"internal/cargo.mod", false},
		{"internal/algo.golden", false},
		// EL CASO QUE DESARMA LA RAZÓN FÁCIL. `assets/dashboard.html` ENTRA AL BUILD: viaja adentro
		// del binario por `//go:embed` en cmd/musubi/dashboard.go:24. Y el overlay igual no lo toca,
		// medido con control en el mismo overlay.json. Si la regla fuera «entra al build» esta fila
		// diría `true` y estaría mal; la regla es «el compilador lo lee como FUENTE».
		// El árbol declara un sabotaje sobre este archivo en cmd/musubi/flota_test.go:244.
		{"cmd/musubi/assets/dashboard.html", false},
	}
	for _, c := range casos {
		if got := elOverlayPuedeTocar(c.archivo); got != c.puede {
			t.Errorf("%s: esperaba puede=%v, vino %v — un `no medí` contado como veredicto es "+
				"exactamente el defecto que este canasto existe para evitar", c.archivo, c.puede, got)
		}
	}
}

// TestUnMotivoQueEsUnLogSeDenunciaYUnoQueEsAsercionNo cierra la mitad que el guion NO puede mirar.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ SE MIDIÓ ANTES DE ESCRIBIR ESTO
//
// El motivo con que se distinguen dos sabotajes era «la primera línea `_test.go:N:` bajo el
// `--- FAIL:`», y `t.Logf` imprime con EL MISMO formato que `t.Errorf`. Medido en un módulo
// aparte, con el comando real del guion (`corrida()`, SIN `-v`):
//
//	--- FAIL: TestUnaConLogDeControlAntes
//	    m_test.go:6: control: el corpus trae 7 casos      ← ganaba ésta
//	    m_test.go:7: LA ASERCION QUE REALMENTE CAYO
//	--- FAIL: TestOtraConELMISMOLogDeControl
//	    m_test.go:11: control: el corpus trae 7 casos     ← y ésta
//	    m_test.go:12: UNA ASERCION COMPLETAMENTE DISTINTA
//
// Dos sabotajes que caen por motivos opuestos daban un motivo con el MISMO texto: el contador no
// medía qué ASERCIÓN cayó sino qué PRUEBA cayó, y dos guardas distintas de una misma prueba se
// veían como una sola contada dos veces.
//
// `sabotaje.sh` saca los logs que salen IGUAL en el control. Lo que sobrevive a esa resta es un
// log cuyo texto CAMBIA con el sabotaje, y ése no se puede distinguir desde el texto: ni con
// `-json`, que además implica `-v` e invierte el orden del que depende el extractor. Desde el AST
// sí, y acá el AST ya está a mano.
//
// LO QUE NO HACE, Y ES A PROPÓSITO: no invalida el rojo ni descarta el motivo. Lo DENUNCIA, que es
// la diferencia entre «medí y esto es raro» y «no se midió».
//
// Sabotaje que la hace fallar: en `laLineaEsUnLog`, que el `case` de los logs no case nunca → un
// motivo que es un `t.Logf` deja de denunciarse y entra callado a la comparación.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tcase \"Log\", \"Logf\":"
// arnes: a="\t\tcase \"NUNCA_NINGUN_LOG\":"
// arnes: prueba="TestUnMotivoQueEsUnLogSeDenunciaYUnoQueEsAsercionNo"
func TestUnMotivoQueEsUnLogSeDenunciaYUnoQueEsAsercionNo(t *testing.T) {
	// El fuente se escribe acá y los números de línea se DERIVAN de él. Clavarlos a mano sería una
	// copia: alguien agrega una línea arriba y la prueba pasa a preguntar por otra cosa sin que
	// nada se ponga rojo.
	lineas := []string{
		"package x",
		"",
		"import \"testing\"",
		"",
		"func TestAlgo(t *testing.T) {",
		"\tt.Logf(\"control: el corpus trae %d casos\", 7)",
		"\tt.Errorf(\"la aserción que de verdad cayó\")",
		"\tayudante(t)",
		"}",
		"",
		"func ayudante(t *testing.T) { t.Helper() }",
		"",
	}
	numeroDe := func(aguja string) int {
		n := 0
		for i, l := range lineas {
			if strings.Contains(l, aguja) {
				if n != 0 {
					t.Fatalf("%q aparece más de una vez en el fuente de prueba: el número de línea "+
						"que derive de ahí sería ambiguo", aguja)
				}
				n = i + 1
			}
		}
		if n == 0 {
			t.Fatalf("no encontré %q en el fuente de prueba: esta prueba no estaría midiendo nada", aguja)
		}
		return n
	}

	raiz := t.TempDir()
	dir := filepath.Join(raiz, "internal", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guarda_test.go"),
		[]byte(strings.Join(lineas, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	salidaCon := func(linea int, mensaje string) string {
		return "  ✓ ROJO, y falla en:\n      TestAlgo\n" +
			"  ── motivo (la primera línea de cada fallo que NO esté en el control) ──\n" +
			fmt.Sprintf("      TestAlgo · guarda_test.go:%d: %s\n", linea, mensaje)
	}
	revisar := func(linea int, mensaje string) (string, claseDeQueja, string) {
		return revisarElRojo(salidaCon(linea, mensaje), "internal/x/guarda_test.go", "TestAlgo",
			raiz, "./internal/x/")
	}

	t.Run("un motivo que cae en un t.Logf se denuncia", func(t *testing.T) {
		motivo, clase, queja := revisar(numeroDe("t.Logf("), "control: el corpus trae 9 casos")
		if clase != quejaMotivoEsUnLog {
			t.Fatalf("clase = %d, esperaba quejaMotivoEsUnLog (%d) — queja: %q\n"+
				"Si vino sinQueja, o el AST no encontró el archivo o el `case` de los logs no casa: "+
				"en los dos casos esta guarda no estaría mirando nada.",
				clase, quejaMotivoEsUnLog, queja)
		}
		if strings.TrimSpace(queja) == "" {
			t.Error("la clase dice que hay queja y el texto vino vacío")
		}
		// EL MOTIVO NO SE TIRA: un rojo denunciado sigue siendo un rojo, y sin clave no se podría
		// comparar contra los otros sabotajes. Denunciar no es descartar.
		if motivo == "" {
			t.Error("se perdió el motivo al denunciarlo: la denuncia tiene que agregar información, " +
				"no sacarla")
		}
	})

	t.Run("un motivo que cae en la aserción NO se denuncia", func(t *testing.T) {
		// LA MITAD QUE IMPIDE QUE ESTO SEA UN «SIEMPRE DENUNCIA». Sin esta rama, cambiar el `case`
		// de los logs por `default` pasaría la prueba de arriba y rompería la herramienta entera.
		motivo, clase, queja := revisar(numeroDe("t.Errorf("), "la aserción que de verdad cayó")
		if clase != sinQueja {
			t.Errorf("una aserción se denunció como log: clase = %d, queja = %q", clase, queja)
		}
		if motivo == "" {
			t.Error("no salió motivo para una aserción legítima")
		}
	})

	t.Run("si en esa línea no hay ninguna llamada a t.X, no se opina", func(t *testing.T) {
		// UNA GUARDA QUE ESCRIBE SU ASERCIÓN EN UN AYUDANTE CON `t.Helper()` hace que `go test`
		// reporte la línea del LLAMADOR, que no es una llamada a `t.X` y no se puede clasificar.
		// Contestar «no es un log» ahí sería afirmar algo que no se midió.
		_, clase, queja := revisar(numeroDe("ayudante(t)"), "lo que sea que haya dicho el ayudante")
		if clase != sinQueja {
			t.Errorf("opinó sobre una línea que no puede clasificar: clase = %d, queja = %q", clase, queja)
		}
	})

	t.Run("CONTROL: sin archivo que parsear no se opina, y el resto sigue funcionando", func(t *testing.T) {
		// Si esto se pusiera rojo, el barrido estaría inventando una clasificación desde un archivo
		// que no existe — y entonces el verde de las otras ramas no significaría nada.
		_, clase, queja := revisarElRojo(salidaCon(numeroDe("t.Logf("), "da igual"),
			"internal/x/guarda_test.go", "TestAlgo", t.TempDir(), "./internal/x/")
		if clase != sinQueja {
			t.Errorf("opinó sin archivo: clase = %d, queja = %q", clase, queja)
		}
	})
}

// TestLaMarcaDeSinMotivoPropioSigueEnElGuion evita que dos programas se desincronicen callados.
//
// `marcaSinMotivoPropio` es una COPIA de un literal que vive en `deploy/pruebas/sabotaje.sh`. Este
// repo tiene medido qué pasa con las copias: no fallan, dejan de coincidir. Si alguien reescribe
// esa frase en el guion, acá no se reconocería y el desenlace «no me quedó ninguna línea propia»
// volvería a salir por la puerta de `quejaMotivoIlegible` —que manda a mirar la redacción— sin que
// nada se ponga rojo.
//
// Sabotaje que la hace fallar: cambiarle el valor a `marcaSinMotivoPropio` → la constante deja de
// coincidir con la frase que el guion imprime, que es exactamente la desincronización que se teme.
//
// VA SOBRE LA CONSTANTE Y NO SOBRE EL GUION, Y NO ES UNA COMODIDAD: el guion sería, en esa corrida,
// el intérprete que se está ejecutando. `bash` lee el archivo a medida que avanza, así que mutarlo
// en vuelo no mide esta guarda, mide otra cosa. Los dos lados de una desincronización dan el mismo
// rojo, así que se elige el lado seguro.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="const marcaSinMotivoPropio = \"SIN MOTIVO PROPIO:\""
// arnes: a="const marcaSinMotivoPropio = \"UNA FRASE QUE EL GUION NO DICE:\""
// arnes: prueba="TestLaMarcaDeSinMotivoPropioSigueEnElGuion"
func TestLaMarcaDeSinMotivoPropioSigueEnElGuion(t *testing.T) {
	// LA PREGUNTA TAUTOLÓGICA, ATAJADA: `strings.Contains(x, "")` es true siempre. Sin esto, vaciar
	// la constante dejaría la guarda en verde para siempre — que es la forma en que una guarda deja
	// de medir sin avisar.
	if marcaSinMotivoPropio == "" {
		t.Fatal("`marcaSinMotivoPropio` está vacía: `Contains` contra \"\" es true siempre y esta " +
			"guarda no podría fallar nunca")
	}
	const guion = "../../pruebas/sabotaje.sh"
	b, err := os.ReadFile(guion)
	if err != nil {
		t.Fatalf("no pude leer %s: %v — no medí nada", guion, err)
	}
	if !strings.Contains(string(b), marcaSinMotivoPropio) {
		t.Errorf("`%s` no dice %q en ninguna parte.\n"+
			"  Esa frase es la que el guion imprime cuando, después de restar el control, no le\n"+
			"  queda ninguna línea con que identificar el rojo — y `revisarElRojo` la reconoce por\n"+
			"  texto para darle su propia clase. Si la reescribiste en el guion, actualizá también\n"+
			"  `marcaSinMotivoPropio`; si no, ese desenlace vuelve a salir por la puerta equivocada.",
			guion, marcaSinMotivoPropio)
	}
}

// TestLosTagsSeAGREGANaGOFLAGSyNoLoPISAN fija que el entorno heredado sobreviva.
//
// LO ESCRIBÍ MAL Y LO ENCONTRÓ OTRA SESIÓN LEYENDO EL PR. La línea era
// `cmd.Env = append(os.Environ(), "GOFLAGS=-tags="+d.Tags)`, que PARECE que agregara —lo dice el
// `append`— y reemplaza: Go se queda con la última de las claves repetidas. Un
// `GOFLAGS=-mod=readonly` del entorno se evaporaba sin una línea de aviso.
//
// LO QUE HACE FALTA DECIR ES POR QUÉ SOBREVIVIÓ A SU PROPIA MEDICIÓN. Medí las dos direcciones de
// `tags=` —sin la clave «no tests to run», con la clave `--- PASS`— y las dos daban bien, porque
// en este repo NADIE setea GOFLAGS: el defecto sólo se ve con un entorno que el árbol no tiene
// hoy. Es la forma de la guarda apagada por una variable que nadie setea, mirada desde el otro
// lado: no es que la guarda esté apagada, es que el caso que la rompe no existe todavía. Un verde
// sobre el único mundo que se puede montar no dice nada del mundo de al lado, y el día que alguien
// agregue `GOFLAGS: -mod=readonly` a un job el arnés se lo comería lejos de acá y sin relación
// aparente.
//
// Por eso la prueba MONTA el entorno que el repo no tiene, en vez de medir el que tiene.
//
// Sabotaje que la pone roja: volver a la forma que reemplaza, `return "GOFLAGS=-tags=" + tags`
// para cualquier entorno.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tif heredado == \"\" {\n\t\treturn \"GOFLAGS=-tags=\" + tags\n\t}\n\treturn \"GOFLAGS=\" + heredado + \" -tags=\" + tags"
// arnes: a="\treturn \"GOFLAGS=-tags=\" + tags"
// arnes: arreglo_de="\treturn \"GOFLAGS=\" + heredado + \" -tags=\" + tags"
// arnes: arreglo_a="\treturn \"GOFLAGS=\" + heredado + \" \" + \"-tags=\" + tags"
func TestLosTagsSeAGREGANaGOFLAGSyNoLoPISAN(t *testing.T) {
	t.Run("sin GOFLAGS heredado, sale solo lo nuestro", func(t *testing.T) {
		t.Setenv("GOFLAGS", "")
		if got := entornoConTags("treesitter"); got != "GOFLAGS=-tags=treesitter" {
			t.Errorf("entornoConTags = %q, esperaba %q", got, "GOFLAGS=-tags=treesitter")
		}
	})

	t.Run("con GOFLAGS heredado, lo heredado SOBREVIVE", func(t *testing.T) {
		// LA MITAD QUE IMPORTA. Con la forma vieja esto daba `GOFLAGS=-tags=treesitter` y el
		// `-mod=readonly` desaparecía: la corrida pasaba a resolver módulos de otra manera que la
		// que el job pidió, sin una línea que lo dijera.
		t.Setenv("GOFLAGS", "-mod=readonly")
		got := entornoConTags("treesitter")
		if !strings.Contains(got, "-mod=readonly") {
			t.Errorf("entornoConTags = %q: se comió el `-mod=readonly` del entorno", got)
		}
		if !strings.Contains(got, "-tags=treesitter") {
			t.Errorf("entornoConTags = %q: perdió los tags de la directiva", got)
		}
		// Y el orden importa: los nuestros ÚLTIMOS, porque el último `-tags` gana y la directiva
		// sabe qué necesita esta prueba mejor que una variable de ambiente.
		if strings.Index(got, "-tags=treesitter") < strings.Index(got, "-mod=readonly") {
			t.Errorf("entornoConTags = %q: los tags de la directiva tienen que ir ÚLTIMOS", got)
		}
	})

	t.Run("CONTROL: es una entrada de entorno con la forma que `exec` espera", func(t *testing.T) {
		// Sin esto la guarda pasaría con cualquier string que contenga los dos textos —incluido uno
		// sin el `GOFLAGS=` de adelante, que `exec` ignoraría en silencio y dejaría al arnés
		// corriendo sin tags otra vez.
		t.Setenv("GOFLAGS", "-mod=readonly")
		clave, valor, ok := strings.Cut(entornoConTags("x"), "=")
		if !ok || clave != "GOFLAGS" || valor == "" {
			t.Errorf("entornoConTags no devolvió una entrada `GOFLAGS=<algo>`: %q", entornoConTags("x"))
		}
	})
}

// TestElAvisoDeTagsHeredadosMiraElFlagYNoUnaSubcadena fija que `traeTagsPropios` lea GOFLAGS como Go.
//
// El aviso de `entornoConTags` salía con `GOFLAGS=-ldflags=-X=main.origen=-tags`: el `-tags` era
// parte del valor de OTRO flag y la subcadena no lo distinguía. No es la lógica —los tags de la
// directiva van últimos igual— pero un aviso que sale cuando no corresponde entrena a ignorar los
// que sí.
//
// Los casos salen del toolchain, no de una suposición: `-tags=x` y `--tags=x` Go los acepta, y
// `-tags x` separado lo rechaza, así que cada entrada separada por blancos es un flag entero.
//
// Sabotaje que la pone roja: volver a aceptar la subcadena, sumándola a la comparación por nombre.
// El `nombre == "tags"` se deja a propósito: sacarlo dejaría `nombre` sin usar, el paquete no
// compilaría y el arnés contestaría «sin veredicto» en vez de ROJO.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tif nombre == \"tags\" {"
// arnes: a="\t\tif nombre == \"tags\" || strings.Contains(f, \"-tags\") {"
// arnes: arreglo_de="\t\tif nombre == \"tags\" {"
// arnes: arreglo_a="\t\tif \"tags\" == nombre {"
func TestElAvisoDeTagsHeredadosMiraElFlagYNoUnaSubcadena(t *testing.T) {
	casos := []struct {
		goflags string
		quiero  bool
	}{
		{"", false},
		{"-mod=readonly", false},
		{"-tags=treesitter", true},
		{"--tags=treesitter", true},
		{"-mod=readonly -tags=a,b", true},
		// EL CASO POR EL QUE EXISTE: el `-tags` vive adentro del valor de otro flag.
		{"-ldflags=-X=main.origen=-tags", false},
		{"-mod=readonly -ldflags=-X=main.v=-tags", false},
	}
	for _, c := range casos {
		if got := traeTagsPropios(c.goflags); got != c.quiero {
			t.Errorf("traeTagsPropios(%q) = %v, esperaba %v", c.goflags, got, c.quiero)
		}
	}
}
