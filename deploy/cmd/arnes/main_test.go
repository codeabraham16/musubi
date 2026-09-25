package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/arnes"
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
//
// Sabotaje que la hace fallar: sacarle a la rama de «cayó en otro archivo» la condición de que el
// ancla viva en un `_test.go` → un ancla de producción, cuya aserción NUNCA puede estar en su
// propio archivo, pasa a quejarse siempre.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tif strings.HasSuffix(base, \"_test.go\") && arch != base {"
// arnes: a="\tif arch != base {"
// arnes: prueba="TestElRojoSeLeeYNoSeDeduceDelExitCode"
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
			// EL ANCLA VIVE EN CÓDIGO DE PRODUCCIÓN: la aserción NO PUEDE estar en su archivo.
			//
			// Es el control de la rama de arriba. Desde que el censo mira también los `.go` que no
			// son de prueba, «cayó en otro archivo» es cierto SIEMPRE para esas anclas, y una queja
			// que se cumple sola no informa: entrena a saltearla, y con ella se saltea la de al
			// lado. La condición de la rama es `_test.go`, y este caso es el que la sostiene.
			nombre: "un ancla en codigo de produccion no se queja de caer en otro archivo",
			salida: "  ✓ ROJO, y falla en:\n      TestX\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestX · x_test.go:12: algo\n",
			archivoAncla: "internal/x/x.go",
			prueba:       "TestX",
			motivo:       "x_test.go:12: algo",
			clase:        sinQueja,
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

// EL TRAMO A CORRER, Y LA PROPIEDAD QUE DE VERDAD IMPORTA: LAS DOS MITADES PARTICIONAN.
//
// `-desde` nació de una corrida muerta: un barrido de `./internal/mcp` son ~165 sabotajes y dos
// horas, se murió en el 126 por falta de memoria, y con sólo `-limite` —que corre los PRIMEROS N—
// recuperar los últimos 39 obligaba a repetir los 126 anteriores.
//
// Por eso la guarda no enumera casos: comprueba que para CUALQUIER corte, `-limite k` y
// `-desde k+1` cubran la lista entera, cada elemento EXACTAMENTE UNA VEZ. Un off-by-one deja un
// hueco (un sabotaje que nadie corrió y que el informe da por corrido) o un solapamiento (uno
// contado dos veces), y las dos cosas son mentiras sobre la cobertura, que es lo único que este
// programa produce.
//
// Sabotaje que la hace fallar: volver `inicio = desde - 1` a `inicio = desde`.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tinicio = desde - 1\n"
// arnes: a="\t\tinicio = desde\n"
func TestLasDosMitadesDeUnBarridoCortadoCubrenTodoUnaSolaVez(t *testing.T) {
	const total = 165
	// EL CORTE ARRANCA EN 1 Y NO EN 0, Y NO ES PARA AFLOJAR LA GUARDA: `-limite 0` significa
	// «todos», que es la semántica que ese flag ya tenía antes de que `-desde` existiera. Un
	// barrido que murió ANTES del primer sabotaje no tiene dos mitades que pegar — se vuelve a
	// correr entero, sin flags. La propiedad se comprueba en todo el dominio donde reanudar
	// significa algo.
	for corte := 1; corte <= total; corte++ {
		visto := make([]int, total)

		// La primera mitad: como si hubiera muerto en `corte`.
		i1, f1 := tramoACorrer(total, 0, corte)
		for i := i1; i < f1; i++ {
			visto[i]++
		}
		// Y la segunda, reanudando en el siguiente. `corte+1` es 1-based: el número impreso.
		i2, f2 := tramoACorrer(total, corte+1, 0)
		for i := i2; i < f2; i++ {
			visto[i]++
		}

		for i, n := range visto {
			if n != 1 {
				t.Fatalf("cortando en %d, el sabotaje en la posición %d se corrió %d veces "+
					"(se esperaba exactamente 1): un hueco es un sabotaje que nadie corrió y que "+
					"el informe da por corrido; un repetido infla la cobertura", corte, i+1, n)
			}
		}
	}
}

// `-desde` Y `-limite` SE COMPONEN, Y EL LÍMITE CUENTA DESDE EL ARRANQUE Y NO DESDE EL PRINCIPIO.
//
// Es el error cómodo: aplicar el techo sobre la lista entera en vez de sobre el tramo. Con eso
// `-desde 127 -limite 10` correría los primeros diez y no los diez que siguen al 126, o sea que
// reanudar devolvería lo que ya se había corrido.
//
// Sabotaje que la hace fallar: contar el límite desde el principio de la lista.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tif limite > 0 && inicio+limite < fin {\n\t\tfin = inicio + limite\n\t}\n"
// arnes: a="\tif limite > 0 && limite < fin {\n\t\tfin = limite\n\t}\n"
func TestElLimiteCuentaDesdeDondeArrancaYNoDesdeElPrincipio(t *testing.T) {
	inicio, fin := tramoACorrer(165, 127, 10)
	if inicio != 126 || fin != 136 {
		t.Errorf("`-desde 127 -limite 10` dio [%d,%d), se esperaba [126,136): reanudar tiene que "+
			"traer los DIEZ QUE SIGUEN, no los diez del principio", inicio, fin)
	}
	// Y el techo no se puede pasar del final.
	if _, f := tramoACorrer(165, 160, 50); f != 165 {
		t.Errorf("el tramo terminó en %d y la lista tiene 165", f)
	}
	// Un `desde` más grande que la lista no corre nada, y no explota.
	if i, f := tramoACorrer(165, 900, 0); i != f {
		t.Errorf("un `-desde` más allá del final devolvió [%d,%d), se esperaba un tramo vacío", i, f)
	}
}

// mecanizadasDePrueba arma `total` mecanizadas de un mismo paquete, cada una con su línea igual a
// su posición: así una prueba puede comprobar que la posición que devuelve `seleccionar` es la del
// ancla que trae, y no sólo un número.
func mecanizadasDePrueba(total int, sistemas ...string) []arnes.Ancla {
	mec := make([]arnes.Ancla, total)
	for i := range mec {
		nombre := fmt.Sprintf("TestX%d", i+1)
		mec[i] = arnes.Ancla{
			Archivo: "internal/x/x_test.go", Linea: i + 1, Prueba: nombre,
			Directiva: &arnes.Directiva{Paquete: "./internal/x", Prueba: nombre, Sistemas: sistemas},
		}
	}
	return mec
}

// LOS FRAGMENTOS PARTICIONAN: CADA MECANIZADA CAE EN EXACTAMENTE UNO.
//
// Es la misma propiedad que las dos mitades de un barrido cortado, un piso más arriba: un hueco es
// un sabotaje que NINGUNA noche corre y que el informe da por corrido, y un repetido infla la
// cuenta. Se prueba sobre `seleccionar` y no sobre `enFragmento` suelta, porque `seleccionar` es
// lo que el corredor recorre: una aritmética perfecta que nadie llama también deja pasar las dos.
//
// Los totales incluyen los bordes del reparto por turno —menos mecanizadas que fragmentos, justo
// n, n+1— y el tamaño real del árbol el día que entró el nocturno.
//
// Sabotaje que la hace fallar: que `seleccionar` deje de preguntarle a `enFragmento` → cada
// fragmento corre la lista entera.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tif !enFragmento(p.pos, k, n) {"
// arnes: a="\t\tif !enFragmento(p.pos, k, n) && false {"
//
// Sabotaje que la hace fallar: correr el turno un lugar → el resto 0 no le toca a ningún
// fragmento y esas posiciones no corren nunca. El arreglo es la misma cuenta escrita al revés, que
// tiene que seguir en verde: la guarda fija la partición, no la forma de la aritmética.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\treturn (pos-1)%n == k-1"
// arnes: a="\treturn (pos-1)%n == k"
// arnes: arreglo_de="\treturn (pos-1)%n == k-1"
// arnes: arreglo_a="\treturn (pos-1)%n+1 == k"
func TestLosFragmentosCubrenLaListaEnteraUnaSolaVez(t *testing.T) {
	for _, total := range []int{1, 7, 8, 9, 949} {
		mec := mecanizadasDePrueba(total)
		for n := 1; n <= 12; n++ {
			visto := make([]int, total+1)
			for k := 1; k <= n; k++ {
				aCorrer, noAplican := seleccionar(mec, "", 0, 0, k, n, "linux")
				if len(noAplican) != 0 {
					t.Fatalf("sin `sistema=` en ninguna directiva, el fragmento %d/%d apartó %d como «no aplica»",
						k, n, len(noAplican))
				}
				for _, p := range aCorrer {
					if p.pos < 1 || p.pos > total || p.ancla.Linea != p.pos {
						t.Fatalf("el fragmento %d/%d devolvió la posición %d con el ancla de la línea %d: "+
							"el número impreso tiene que ser la posición del ancla que se corre", k, n, p.pos, p.ancla.Linea)
					}
					visto[p.pos]++
				}
			}
			for pos := 1; pos <= total; pos++ {
				if visto[pos] != 1 {
					t.Fatalf("con %d mecanizadas en %d fragmentos, la posición %d se corrió %d veces (se "+
						"esperaba exactamente 1): un hueco es un sabotaje que ninguna noche corre, un "+
						"repetido infla la cuenta", total, n, pos, visto[pos])
				}
			}
		}
	}

	// CONTROL: sin `-fragmento` corre todo, que es lo que `-correr` hacía antes de que existiera.
	if aCorrer, _ := seleccionar(mecanizadasDePrueba(949), "", 0, 0, 0, 0, "linux"); len(aCorrer) != 949 {
		t.Errorf("sin fragmento se seleccionaron %d de 949", len(aCorrer))
	}
}

// EL ORDEN DE `seleccionar` ES PAQUETE → TRAMO → FRAGMENTO → SISTEMA, Y LA POSICIÓN ES ABSOLUTA.
//
// La prueba de arriba llama a `seleccionar` sin `-paquete`, sin `-desde` y sin `-limite`, y así
// dejaba sin custodia tres de los cuatro pasos. Lo midió la revisión del PR: apagar el filtro de
// paquete, ignorar el tramo o numerar relativo al tramo dejaban el paquete entero en verde. Y las
// tres cosas se usan: el nocturno imprime `══ 517` y quien reanuda escribe `-desde 517 -fragmento
// 7/8`. Eso tiene que correr el RESTO DE ESE fragmento, con los mismos números, y nada de otro
// paquete.
//
// El censo tiene dos paquetes intercalados y dos directivas de Windows en el pedido —una antes del
// tramo y otra adentro—, y se pide `-paquete ./internal/x -desde 5 -limite 3` en cada fragmento de
// 1 a 4. Se exige, en este orden: que no se cuele nada de `./internal/y`; que cada posición traiga
// el ancla de esa posición; que no se salga del tramo 5..7; que cada posición caiga en el mismo
// fragmento que en la corrida sin tramo; que Linux y Windows repartan igual; y que la unión de los
// fragmentos sea el tramo, una vez cada posición. El orden importa: cada sabotaje de abajo cae en
// SU aserción, y el arnés no los cuenta como un motivo repetido.
//
// Lo que NO fija: QUÉ fragmento le toca a cada posición (eso es de la prueba de arriba, que a
// propósito tampoco lo fija), ni el orden de salida dentro de un fragmento.
//
// Sabotaje que la hace fallar: apagar el filtro de paquete → se cuelan las de `./internal/y`, y
// `-correr -paquete ./internal/arnes` correría el árbol entero.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tif paquete != \"\" && a.Directiva.Paquete != paquete {"
// arnes: a="\t\tif false && paquete != \"\" && a.Directiva.Paquete != paquete {"
//
// Sabotaje que la hace fallar: ignorar `-desde` y `-limite` → reanudar vuelve a empezar desde la 1
// y el límite deja de limitar.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tinicio, fin := tramoACorrer(len(delPaquete), desde, limite)"
// arnes: a="\tinicio, fin := tramoACorrer(len(delPaquete), 0, 0)"
//
// Sabotaje que la hace fallar: numerar relativo al tramo → `-desde 517 -fragmento 7/8` corre otro
// fragmento e imprime números que no sirven para volver a reanudar.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tp := puesto{pos: i + 1, ancla: delPaquete[i]}"
// arnes: a="\t\tp := puesto{pos: i + 1 - inicio, ancla: delPaquete[i]}"
//
// Sabotaje que la hace fallar: filtrar por sistema ANTES de numerar → en Linux se corre la
// numeración, el fragmento 3 deja de ser el mismo que en Windows y lo apartado no se nombra.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tdelPaquete = append(delPaquete, a)\n"
// arnes: a="\t\tif aplicaEn(a.Directiva, goos) {\n\t\t\tdelPaquete = append(delPaquete, a)\n\t\t}\n"
func TestPaqueteTramoFragmentoYSistemaEnEseOrden(t *testing.T) {
	const pedido = "./internal/x"
	// Diez de `./internal/x` con la línea igual a su posición EN EL PAQUETE, y cinco de
	// `./internal/y` intercaladas con líneas que no se confunden. La 2 y la 6 de x son de Windows.
	var mec []arnes.Ancla
	var x, y int
	for _, c := range "yxxyxxyxxxyxxxy" {
		if c == 'x' {
			x++
			nombre := fmt.Sprintf("TestX%d", x)
			d := &arnes.Directiva{Paquete: pedido, Prueba: nombre}
			if x == 2 || x == 6 {
				d.Sistemas = []string{"windows"}
			}
			mec = append(mec, arnes.Ancla{Archivo: "internal/x/x_test.go", Linea: x, Prueba: nombre, Directiva: d})
			continue
		}
		y++
		nombre := fmt.Sprintf("TestY%d", y)
		mec = append(mec, arnes.Ancla{Archivo: "internal/y/y_test.go", Linea: 100 + y, Prueba: nombre,
			Directiva: &arnes.Directiva{Paquete: "./internal/y", Prueba: nombre}})
	}

	// elegidas junta lo corrido y lo apartado: el reparto es de los dos, el sistema sólo decide
	// cuál de las dos listas. El mapa es «posición → línea del ancla».
	elegidas := func(goos string, desde, limite, k, n int) (map[int]int, []puesto) {
		aCorrer, noAplican := seleccionar(mec, pedido, desde, limite, k, n, goos)
		todo := append(append([]puesto{}, aCorrer...), noAplican...)
		m := map[int]int{}
		for _, p := range todo {
			m[p.pos] = p.ancla.Linea
		}
		return m, todo
	}

	for n := 1; n <= 4; n++ {
		veces := map[int]int{}
		for k := 1; k <= n; k++ {
			win, todo := elegidas("windows", 5, 3, k, n)
			for _, p := range todo {
				if p.ancla.Directiva.Paquete != pedido {
					t.Fatalf("`-paquete %s -fragmento %d/%d` trajo %s, que es de %s: el filtro de paquete no "+
						"filtra y `-correr -paquete` correría el árbol entero", pedido, k, n,
						p.ancla.Prueba, p.ancla.Directiva.Paquete)
				}
			}
			for _, p := range todo {
				if p.ancla.Linea != p.pos {
					t.Fatalf("`-desde 5 -limite 3 -fragmento %d/%d` imprime la posición %d para el ancla que es "+
						"la %d del paquete: ese número no sirve para reanudar con `-desde`", k, n, p.pos, p.ancla.Linea)
				}
			}
			for _, p := range todo {
				if p.pos < 5 || p.pos > 7 {
					t.Fatalf("`-desde 5 -limite 3 -fragmento %d/%d` trajo la posición %d y se pidieron la 5, la 6 "+
						"y la 7: reanudar volvería a empezar, o el límite no limita", k, n, p.pos)
				}
			}
			completo, _ := elegidas("windows", 0, 0, k, n)
			for pos := range win {
				if _, ok := completo[pos]; !ok {
					t.Fatalf("con `-desde 5 -limite 3` la posición %d cayó en el fragmento %d/%d, y en la corrida "+
						"sin tramo no es de ése: reanudar un fragmento correría sabotajes de otro", pos, k, n)
				}
			}
			if lin, _ := elegidas("linux", 5, 3, k, n); fmt.Sprint(lin) != fmt.Sprint(win) {
				t.Fatalf("el fragmento %d/%d reparte distinto según la máquina (posición → línea): linux %v, "+
					"windows %v. El sistema tiene que decidirse DESPUÉS de numerar", k, n, lin, win)
			}
			for pos := range win {
				veces[pos]++
			}
		}
		for pos := 5; pos <= 7; pos++ {
			if veces[pos] != 1 {
				t.Errorf("con %d fragmentos, la posición %d del tramo salió %d veces (se esperaba 1)", n, pos, veces[pos])
			}
		}
	}

	// Y en Linux la de Windows que cae en el tramo (la 6) sale apartada con su número, no corrida.
	aCorrer, noAplican := seleccionar(mec, pedido, 5, 3, 0, 0, "linux")
	if len(aCorrer) != 2 || len(noAplican) != 1 || noAplican[0].pos != 6 || noAplican[0].ancla.Linea != 6 {
		t.Errorf("en linux, `-paquete %s -desde 5 -limite 3` dejó %d a correr y %d apartadas (%v): esperaba "+
			"la 5 y la 7 a correr y la 6 apartada", pedido, len(aCorrer), len(noAplican), noAplican)
	}
}

// UN FRAGMENTO MAL ESCRITO ES UN ERROR, NO UN VALOR POR DEFECTO.
//
// El `-fragmento` lo arma el workflow con la aritmética de la matriz. Si un error ahí se leyera
// como «sin fragmento», un job correría los 949 y se cortaría a las seis horas; si se leyera como
// un fragmento imposible (`9/8`), no correría ninguno. Las dos tienen que ser un exit 2 con la
// causa, antes de censar nada.
//
// Sabotaje que la hace fallar: no exigir 1 ≤ k ≤ n → `9/8` y `0/8` pasan.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tif k < 1 || k > n {"
// arnes: a="\tif false {"
func TestUnFragmentoMalEscritoSeRechazaYNoCorreNadaPorAccidente(t *testing.T) {
	for _, s := range []string{"0/8", "9/8", "8", "a/8", "3/0", "3/-1", "3/", "/8", "3/8/1"} {
		if k, n, err := parsearFragmento(s); err == nil {
			t.Errorf("parsearFragmento(%q) = %d/%d sin error: ese fragmento correría todo o nada, "+
				"y ninguna de las dos es lo que se pidió", s, k, n)
		}
	}
	for _, c := range []struct {
		s    string
		k, n int
	}{{"3/8", 3, 8}, {"1/1", 1, 1}, {"8/8", 8, 8}, {"", 0, 0}} {
		k, n, err := parsearFragmento(c.s)
		if err != nil || k != c.k || n != c.n {
			t.Errorf("parsearFragmento(%q) = %d, %d, %v; esperaba %d, %d, nil", c.s, k, n, err, c.k, c.n)
		}
	}
}

// CERO CORRIDAS NO ES UNA NOCHE LIMPIA.
//
// Medido antes de este cambio: `arnes -correr -paquete ./internal/noexiste` imprimía «corridas : 0
// … ✓ el árbol quedó como estaba» y salía 0. En el nocturno eso es un fragmento vacío en verde.
//
// Sabotaje que la hace fallar: volver a la condición vieja, que no mira las corridas.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tif corridas == 0 || verdes > 0 || errores > 0 {"
// arnes: a="\tif verdes > 0 || errores > 0 {"
func TestCeroCorridasNoEsUnaNocheLimpia(t *testing.T) {
	for _, c := range []struct {
		corridas, verdes, errores, quiero int
		por                               string
	}{
		{0, 0, 0, 1, "cero corridas es «no medí», no «todo bien»"},
		{5, 0, 0, 0, "CONTROL: cinco rojos sanos y nada más es la noche limpia"},
		{5, 1, 0, 1, "una guarda hueca es el hallazgo"},
		{5, 0, 1, 1, "sin veredicto no es un verde"},
	} {
		if got := codigoDeSalida(c.corridas, c.verdes, c.errores); got != c.quiero {
			t.Errorf("codigoDeSalida(%d, %d, %d) = %d, esperaba %d: %s",
				c.corridas, c.verdes, c.errores, got, c.quiero, c.por)
		}
	}
}

// UNA DIRECTIVA DE OTRO SISTEMA SE APARTA COMO «NO APLICA», Y NO CUENTA COMO CORRIDA.
//
// `TestEscribirArchivoAtomicoConElDestinoAbiertoEnWindows` hace `t.Skip` fuera de Windows. En el
// runner Linux, sin `sistema=`, `sabotaje.sh` la daba por «NO SE MIDIÓ NADA» y su fragmento salía
// rojo todas las noches. Y la otra mitad importa igual: si lo apartado contara como corrida, un
// fragmento hecho SÓLO de directivas ajenas diría «corridas : N» sin haber medido nada.
//
// Sabotaje que la hace fallar: que `aplicaEn` conteste siempre que sí. Conserva el uso de `slices`
// a propósito: un `return true` dejaría el import huérfano, no compilaría, y el arnés contestaría
// «sin veredicto» en vez de rojo.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\treturn len(d.Sistemas) == 0 || slices.Contains(d.Sistemas, goos)"
// arnes: a="\treturn len(d.Sistemas) >= 0 || slices.Contains(d.Sistemas, goos)"
//
// Sabotaje que la hace fallar: que `seleccionar` deje de preguntarle a `aplicaEn` → la de Windows
// vuelve a correr en Linux.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\t\tif !aplicaEn(p.ancla.Directiva, goos) {"
// arnes: a="\t\tif !aplicaEn(p.ancla.Directiva, goos) && false {"
func TestUnaDirectivaDeOtroSistemaNoApareceComoSinVeredicto(t *testing.T) {
	sin := &arnes.Directiva{}
	win := &arnes.Directiva{Sistemas: []string{"windows"}}
	for _, c := range []struct {
		d      *arnes.Directiva
		goos   string
		quiero bool
	}{
		{sin, "linux", true}, {sin, "windows", true}, {sin, "darwin", true},
		{win, "linux", false}, {win, "darwin", false}, {win, "windows", true},
	} {
		if got := aplicaEn(c.d, c.goos); got != c.quiero {
			t.Errorf("aplicaEn(sistema=%v, %s) = %v, esperaba %v", c.d.Sistemas, c.goos, got, c.quiero)
		}
	}

	// Mezcladas: la 2 y la 4 son de Windows.
	mec := mecanizadasDePrueba(5)
	for _, i := range []int{1, 3} {
		mec[i].Directiva.Sistemas = []string{"windows"}
	}
	aCorrer, noAplican := seleccionar(mec, "", 0, 0, 0, 0, "linux")
	if len(aCorrer) != 3 || len(noAplican) != 2 || noAplican[0].pos != 2 || noAplican[1].pos != 4 {
		t.Errorf("en linux, con la 2 y la 4 de Windows, quedaron %d a correr y %d apartadas (%v): "+
			"esperaba 3 a correr y apartadas la 2 y la 4", len(aCorrer), len(noAplican), noAplican)
	}
	if aCorrer, noAplican := seleccionar(mec, "", 0, 0, 0, 0, "windows"); len(aCorrer) != 5 || len(noAplican) != 0 {
		t.Errorf("en windows se apartaron %d de 5: la de Windows tiene que correr donde existe", len(noAplican))
	}

	// EL FRAGMENTO HECHO SÓLO DE DIRECTIVAS AJENAS SALE ROJO: nada corrió, así que nada se midió.
	ajenas := mecanizadasDePrueba(3, "windows")
	aCorrer, noAplican = seleccionar(ajenas, "", 0, 0, 0, 0, "linux")
	if len(aCorrer) != 0 || len(noAplican) != 3 {
		t.Fatalf("tres directivas de Windows en linux: %d a correr, %d apartadas; esperaba 0 y 3",
			len(aCorrer), len(noAplican))
	}
	if rc := codigoDeSalida(len(aCorrer), 0, 0); rc != 1 {
		t.Errorf("un fragmento que sólo apartó directivas de otro sistema salió %d: no midió nada", rc)
	}
}

// LAS FUNCIONES DE ARRIBA SON PURAS, Y ESO NO ALCANZA: HAY QUE VER QUE EL CORREDOR LAS USE.
//
// Es la forma que este repo ya midió como «guarda definida y desconectada»: pruebas en verde sobre
// una función que nadie llama. `correrTodos` y `main` no tienen prueba —corren `sabotaje.sh` y
// `os.Exit`—, así que si alguien le saca el fragmento a la llamada, o el corredor vuelve a recorrer
// la lista entera, o el cierre vuelve a `if verdes > 0 || errores > 0`, las cuatro de arriba siguen
// en verde y cada fragmento correría los 949, o cero corridas volvería a salir 0.
//
// Así que se lee `main.go` y se exige el cableado: `main` le pasa a `correrTodos` y a
// `contraOverlay` lo que devolvió `parsearFragmento`; las dos le pasan a `seleccionar` sus propios
// k y n y recorren lo que dejó para correr; y `correrTodos` termina en
// `return codigoDeSalida(corridas, verdes, errores)`.
//
// Sabotaje que la hace fallar: `main` le pasa 0, 0 a `correrTodos` → cada fragmento corre todo.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="correrTodos(*raiz, censo, *paquete, *desde, *limite, fragK, fragN)"
// arnes: a="correrTodos(*raiz, censo, *paquete, *desde, *limite, 0, 0)"
//
// Sabotaje que la hace fallar: `correrTodos` le pasa 0, 0 a `seleccionar`.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="aCorrer, noAplican := seleccionar(c.Mecanizadas(), soloPaquete, desde, limite, k, n, runtime.GOOS)"
// arnes: a="aCorrer, noAplican := seleccionar(c.Mecanizadas(), soloPaquete, desde, limite, 0, 0, runtime.GOOS)"
//
// Sabotaje que la hace fallar: `correrTodos` recorre también lo apartado → la de Windows vuelve a
// correr en Linux.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\tfor _, p := range aCorrer {"
// arnes: a="\tfor _, p := range append(aCorrer, noAplican...) {"
//
// Sabotaje que la hace fallar: el cierre vuelve a la condición vieja, sin `codigoDeSalida`.
// arnes: archivo="deploy/cmd/arnes/main.go"
// arnes: de="\treturn codigoDeSalida(corridas, verdes, errores)\n"
// arnes: a="\tif verdes > 0 || errores > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n"
func TestElCorredorUsaLaSeleccionYCeroCorridasSaleRojo(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("no pude leer main.go: %v", err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
			funcs[fd.Name.Name] = fd
		}
	}
	for _, nombre := range []string{"main", "correrTodos", "contraOverlay"} {
		if funcs[nombre] == nil {
			t.Fatalf("main.go ya no tiene `%s`: esta guarda no puede mirar lo que dice mirar", nombre)
		}
	}
	ident := func(e ast.Expr) string {
		if id, ok := e.(*ast.Ident); ok {
			return id.Name
		}
		return ""
	}
	llamadas := func(n ast.Node, a string) []*ast.CallExpr {
		var out []*ast.CallExpr
		ast.Inspect(n, func(x ast.Node) bool {
			if c, ok := x.(*ast.CallExpr); ok && ident(c.Fun) == a {
				out = append(out, c)
			}
			return true
		})
		return out
	}

	// 1 · `main` pasa lo que parseó.
	var fragK, fragN string
	ast.Inspect(funcs["main"], func(x ast.Node) bool {
		if as, ok := x.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && len(as.Lhs) == 3 {
			if c, ok := as.Rhs[0].(*ast.CallExpr); ok && ident(c.Fun) == "parsearFragmento" {
				fragK, fragN = ident(as.Lhs[0]), ident(as.Lhs[1])
			}
		}
		return true
	})
	if fragK == "" || fragN == "" {
		t.Fatal("`main` no asigna el resultado de `parsearFragmento` a dos variables: el flag no llega a ningún lado")
	}
	for _, corredor := range []string{"correrTodos", "contraOverlay"} {
		cs := llamadas(funcs["main"], corredor)
		if len(cs) != 1 {
			t.Errorf("`main` llama %d veces a `%s`, esperaba 1", len(cs), corredor)
			continue
		}
		args := cs[0].Args
		if len(args) < 2 || ident(args[len(args)-2]) != fragK || ident(args[len(args)-1]) != fragN {
			t.Errorf("`main` no le pasa a `%s` el fragmento parseado (%s, %s) en sus dos últimos "+
				"argumentos: cada fragmento correría la lista entera", corredor, fragK, fragN)
		}
	}

	// 2 · Los dos corredores le pasan a `seleccionar` sus propios k y n, y recorren lo que devuelve.
	for _, corredor := range []string{"correrTodos", "contraOverlay"} {
		fd := funcs[corredor]
		var params []string
		for _, campo := range fd.Type.Params.List {
			for _, nm := range campo.Names {
				params = append(params, nm.Name)
			}
		}
		if len(params) < 2 {
			t.Errorf("`%s` no recibe un fragmento", corredor)
			continue
		}
		k, n := params[len(params)-2], params[len(params)-1]
		cs := llamadas(fd.Body, "seleccionar")
		if len(cs) != 1 || len(cs[0].Args) != 7 || ident(cs[0].Args[4]) != k || ident(cs[0].Args[5]) != n {
			t.Errorf("`%s` no le pasa a `seleccionar` sus propios `%s, %s`: el fragmento que recibe no "+
				"decide lo que corre", corredor, k, n)
			continue
		}
		var lista string
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if as, ok := x.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && as.Rhs[0] == cs[0] && len(as.Lhs) == 2 {
				lista = ident(as.Lhs[0])
			}
			return true
		})
		recorre := false
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if r, ok := x.(*ast.RangeStmt); ok && lista != "" && ident(r.X) == lista {
				recorre = true
			}
			return true
		})
		if !recorre {
			t.Errorf("`%s` no recorre la lista que `seleccionar` dejó para correr (%q): lo apartado "+
				"por fragmento o por sistema volvería a correr", corredor, lista)
		}
	}

	// 3 · `correrTodos` termina en `codigoDeSalida`, y con los tres contadores.
	cuerpo := funcs["correrTodos"].Body.List
	ret, ok := cuerpo[len(cuerpo)-1].(*ast.ReturnStmt)
	var c *ast.CallExpr
	if ok && len(ret.Results) == 1 {
		c, _ = ret.Results[0].(*ast.CallExpr)
	}
	if c == nil || ident(c.Fun) != "codigoDeSalida" || len(c.Args) != 3 ||
		ident(c.Args[0]) != "corridas" || ident(c.Args[1]) != "verdes" || ident(c.Args[2]) != "errores" {
		t.Error("`correrTodos` ya no termina en `return codigoDeSalida(corridas, verdes, errores)`: " +
			"cero corridas puede volver a salir 0 y un fragmento vacío dejaría la noche en verde")
	}
}
