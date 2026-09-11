package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// UNA VARIABLE PEGADA A UN CARÁCTER NO-ASCII MATA EL GUION EN macOS, Y EN LINUX NO SE VE.
//
// LO QUE PASÓ, MEDIDO EN CI EL 2026-09-10. `verificar-despliegue.sh` llevaba esto:
//
//	dudoso "el árbol NO es origin/main: está en «$REF_RAMA» (…)"
//
// En bash 5.2 con locale UTF-8 —lo que corre en la máquina de desarrollo— anda perfecto. En el
// runner de macOS, que trae **bash 3.2** y arranca sin locale UTF-8, el primer byte de `»` (0xC2)
// pasa el chequeo de «carácter válido para un identificador», así que bash busca la variable
// `REF_RAMA\xc2`, que no existe. Con `set -u` eso no es un aviso: el guion MUERE ahí.
//
//	verificar-despliegue.sh: line 343: REF_RAMA<byte>: unbound variable
//
// El informe quedaba cortado justo después del título de la sección, y el resto —la versión del
// cerebro, el esquema, los guiones derivados— no se miraba. Con la salida 2 mezclada entre
// «no pude preguntar» y «me morí», que es la confusión que este guion entero existe para evitar.
//
// POR QUÉ UNA GUARDA Y NO UN ARREGLO SUELTO: cuando lo busqué había CUATRO casos y sólo uno era
// mío. Los otros tres —`$am_raiz»` dos veces y `$PRESTART»`— estaban desde antes y nunca se
// vieron, porque viven DESPUÉS del corte por Prometheus y CI no llega hasta ahí. O sea que no es
// un descuido de una vez: es una forma que este repo escribe naturalmente —las comillas angulares
// son el idioma de sus mensajes— y que sólo se castiga en una plataforma que no es la de todos
// los días. Exactamente la clase de defecto que necesita una guarda y no un arreglo.
//
// EL ARREGLO ES `${VAR}`: con llaves, el nombre termina donde dice la llave y el byte de al lado
// deja de ser candidato.
func TestNingunaVariableDeShellQuedaPegadaAUnCaracterNoAscii(t *testing.T) {
	dir := filepath.Join("..", "..", "deploy")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", dir, err)
	}

	// `\$NOMBRE` seguido inmediatamente de un byte no-ASCII. La forma con llaves no matchea, que
	// es justo lo que se quiere: es el arreglo.
	pegada := regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*[^\x00-\x7f]`)

	revisados := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		crudo, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("no pude leer %s: %v", e.Name(), err)
		}
		revisados++
		for n, linea := range strings.Split(string(crudo), "\n") {
			// Los comentarios se saltan: un `$VAR»` adentro de un comentario no lo evalúa nadie, y
			// marcarlo sería un falso positivo sobre los bloques de prosa que este repo escribe —
			// una guarda que grita sobre algo inofensivo se aprende a ignorar.
			if strings.HasPrefix(strings.TrimSpace(linea), "#") {
				continue
			}
			if m := pegada.FindString(linea); m != "" {
				r, _ := utf8.DecodeLastRuneInString(m)
				nombre := strings.TrimSuffix(m, string(r))
				t.Errorf("%s:%d — %s está pegada a %q, y en bash 3.2 (macOS) sin locale UTF-8 el "+
					"primer byte de ese carácter cuenta como parte del nombre: bash busca una "+
					"variable que no existe y, con `set -u`, el guion MUERE ahí. En Linux con bash 5 "+
					"no se ve.\nEscribilo con llaves: %s}%s",
					e.Name(), n+1, nombre, string(r), strings.Replace(nombre, "$", "${", 1), string(r))
			}
		}
	}

	// EL CONTROL. Sin esto, el día que estos guiones se muden de carpeta la prueba pasaría en verde
	// sin haber abierto un solo archivo, y ese verde diría «no hay problema» en vez de «no miré».
	if revisados < 5 {
		t.Fatalf("sólo se revisaron %d guiones de deploy/, y hay bastantes más: probablemente "+
			"cambió dónde viven y esta guarda está en verde sin mirar nada", revisados)
	}
}
