package main

import "testing"

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
		motivo       string // "" = no espero motivo
		queja        string // "" = no espero queja
	}{
		{
			nombre:       "el rojo normal deja motivo y no deja queja",
			salida:       sano,
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestElInventarioNoViajaEnCadaLatido",
			motivo:       "agent_test.go:101: el latido volvió a mandar el inventario",
		},
		{
			// EL CASO QUE EL EXIT CODE NO PUEDE VER. El guion salió con 0 y no dijo qué cayó.
			nombre:       "salio con cero y no nombro ninguna prueba",
			salida:       "  ✓ control: compila y pasa (1 subtest/s REALMENTE ejecutados)\n",
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestAlgo",
			queja:        "salió con 0 y no imprimió NINGUNA prueba fallando: no sé qué se puso en rojo",
		},
		{
			nombre: "cayo una prueba que no es la declarada",
			salida: "  ✓ ROJO, y falla en:\n      TestOtraCosa\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestOtraCosa · agent_test.go:101: algo\n",
			archivoAncla: "cmd/musubi/agent_test.go",
			prueba:       "TestElInventarioNoViajaEnCadaLatido",
			queja:        "cayó `TestOtraCosa` y el ancla declara `TestElInventarioNoViajaEnCadaLatido`",
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
		},
		{
			nombre:       "se puso en rojo sin una linea que ubique la asercion",
			salida:       "  ✓ ROJO, y falla en:\n      TestX\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			queja:        "se puso en rojo sin una línea de `_test.go` que ubique la aserción",
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
			queja:        "la aserción que cayó vive en ayudantes_test.go y el ancla está en x_test.go",
		},
		{
			nombre: "el motivo no trae numero de linea",
			salida: "  ✓ ROJO, y falla en:\n      TestX\n" +
				"  ── motivo (la primera línea de cada fallo, para poder compararlos) ──\n" +
				"      TestX · algo salió mal en x_test.go: sin número\n",
			archivoAncla: "internal/x/x_test.go",
			prueba:       "TestX",
			queja:        "el motivo no nombra un `_test.go:línea`: algo salió mal en x_test.go: sin número",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			motivo, queja := revisarElRojo(c.salida, c.archivoAncla, c.prueba)
			if motivo != c.motivo {
				t.Errorf("motivo:\n  esperaba %q\n  vino     %q", c.motivo, motivo)
			}
			if queja != c.queja {
				t.Errorf("queja:\n  esperaba %q\n  vino     %q", c.queja, queja)
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

	a, quejaA := revisarElRojo(uno, "internal/x/guarda_test.go", "TestUno")
	b, quejaB := revisarElRojo(otro, "internal/x/guarda_test.go", "TestOtro")
	if quejaA != "" || quejaB != "" {
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
