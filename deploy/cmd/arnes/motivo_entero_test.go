package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// TestElMotivoSeComparaEnteroYNoPorSusPrimeros150 fija que el motivo de un rojo salga ENTERO.
//
// `arnes` cuenta «motivos repetidos» usando la línea que imprime motivo.awk como CLAVE. Mientras esa
// línea salía recortada a 150 caracteres, dos rojos distintos que compartían el principio del
// mensaje se contaban como uno — y los mensajes de este repo ponen el dato que distingue al final.
// Medido el 2026-09-14 sobre tres sabotajes reales de ciclos_de_fondo_test.go: el corte dejaba
// afuera «*ast.GoStmt», «*ast.DeferStmt» e «*ast.IfStmt», que era lo único distinto. El peligro no
// es el número: es que alguien, creyéndole, borre dos de tres directivas legítimas.
//
// POR QUÉ CORRE EL PROGRAMA Y NO BUSCA `substr` EN EL TEXTO: un `grep` de `substr(` no ve un
// `cut -c`, un `head -c` ni un `printf "%.150s"`. Lo que importa es la PROPIEDAD —dos mensajes que
// difieren después del caracter 150 dan dos motivos distintos—, y eso se pregunta ejecutando.
//
// LA ENTRADA NO TIENE SUBTESTS A PROPÓSITO. motivo.awk usa `\s`, que mawk no entiende y busybox sí:
// con subtests los dos imprimen distinta cantidad de líneas. Sin subtests la salida es idéntica en
// los dos —medido—, así que esta prueba no depende del sabor de awk de la máquina.
//
// POR QUÉ POR BASH Y CON `Portable`. La primera versión ejecutaba `awk` directo bajo
// `guiones.Exigir`, y la guarda de alcance de la compuerta la rechazó en la CI de los tres sistemas:
// «llama a guiones.Exigir y NO ejecuta ninguna shell» — un salteo de propósito general disfrazado.
// Tenía razón: los tres modos son para pruebas que corren guiones de shell. La salida no fue
// esquivar la guarda sino mirar cómo corre esto en producción: a motivo.awk no lo ejecuta nadie
// directo, lo ejecuta sabotaje.sh, que es bash. Así lo corre la prueba. Y `Portable` no saltea en
// ningún lado: además de mawk mide el gawk de Git Bash y el awk de macOS, que es justo lo que la
// entrada sin subtests hace posible.
//
// Sabotaje que la pone roja: volver a recortar el motivo a 150 caracteres.
// arnes: archivo="deploy/pruebas/motivo.awk"
// arnes: de="\" · \" linea; got=1"
// arnes: a="\" · \" substr(linea,1,150); got=1"
// arnes: arreglo_de="\" · \" linea; got=1"
// arnes: arreglo_a="\" · \" substr(linea,1); got=1"
func TestElMotivoSeComparaEnteroYNoPorSusPrimeros150(t *testing.T) {
	guiones.Portable(t, "corre deploy/pruebas/motivo.awk con bash, igual que sabotaje.sh, y no saltea en ningún sistema", "bash")

	dir := t.TempDir()
	escribir := func(nombre, cuerpo string) string {
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, []byte(cuerpo), 0o644); err != nil {
			t.Fatal(err)
		}
		return ruta
	}
	const logDeControl = "control: el corpus trae 7 casos"
	base := escribir("base.txt", "=== RUN   TestX\n    x_test.go:5: "+logDeControl+"\n--- PASS: TestX (0.00s)\n")

	// Un principio compartido MÁS LARGO que el viejo tope, y la diferencia recién al final.
	prefijo := strings.Repeat("la guarda describe con detalle qué esperaba encontrar ", 4)
	if len(prefijo) <= 150 {
		t.Fatalf("el prefijo mide %d bytes: tiene que pasar de 150 o esta prueba no pregunta nada", len(prefijo))
	}
	programa := filepath.ToSlash(filepath.Join("..", "..", "pruebas", "motivo.awk"))
	motivoDe := func(cola string) string {
		con := escribir("con-"+strings.Trim(cola, "*.")+".txt",
			"--- FAIL: TestX (0.00s)\n    x_test.go:5: "+logDeControl+"\n    x_test.go:9: "+prefijo+cola+"\nFAIL\n")
		out, err := exec.Command("bash", "-c", `awk -f "$0" "$1" "$2"`,
			programa, filepath.ToSlash(base), filepath.ToSlash(con)).CombinedOutput()
		if err != nil {
			t.Fatalf("motivo.awk no corrió: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	uno, otro := motivoDe("*ast.GoStmt"), motivoDe("*ast.DeferStmt")

	// CONTROL de «midió algo»: sin esto, dos salidas vacías también serían «iguales» y el rojo de
	// abajo acusaría al recorte de algo que en realidad es un programa que no imprimió nada.
	if !strings.Contains(uno, "x_test.go:9:") {
		t.Fatalf("motivo.awk no eligió la aserción: salió %q", uno)
	}
	// CONTROL de que sacarlo a su archivo no rompió la resta: el log que también sale en el verde
	// no puede ser el motivo.
	if strings.Contains(uno, logDeControl) {
		t.Errorf("el motivo es el log de control, no la aserción: %q", uno)
	}

	if uno == otro {
		t.Errorf("dos rojos que difieren DESPUÉS del caracter 150 dieron el MISMO motivo:\n  %q\n"+
			"  `arnes` los contaría como un sabotaje repetido. El motivo tiene que salir entero.", uno)
	}
	if !strings.HasSuffix(uno, "*ast.GoStmt") {
		t.Errorf("el motivo perdió el final del mensaje, que es donde está el dato que lo distingue:\n  %q", uno)
	}
}
