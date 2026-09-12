package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNadieEscribeBinarioRustdeskAMano — que la costura del doble siga pasando por el candado.
//
// EL DEFECTO QUE CIERRA, Y ERA UNA CARRERA DE VERDAD. `binarioRustdesk` lo LEE una goroutine que
// sobrevive a quien la creó: `aplicarSesionPantalla` larga un `time.AfterFunc(ttl, …)` para cerrar
// la sesión al vencer, y ese cierre llega hasta `rutaRustdesk()`. Las pruebas, mientras tanto, lo
// ESCRIBÍAN a mano para apuntarlo al doble. Con el TTL de 50 ms de
// `TestAlVencerSeReemplazaLaContrasenaNoSeBorra` las dos cosas se cruzan.
//
// Lo cazó el detector de carreras EN CI, una sola vez. Localmente no se reprodujo: 12 corridas del
// test solo y 3 del paquete entero con `-race`, todas verdes. Una carrera intermitente en la que
// el lector es código de producción no se arregla con «acordate de no escribirla».
//
// LO QUE ESTA GUARDA ES Y LO QUE NO ES. Es SINTÁCTICA: busca asignaciones a `binarioRustdesk` en
// los fuentes del paquete, fuera del archivo que la declara. Eso caza la regresión obvia —alguien
// escribe lo natural— y NO caza a un adversario: una asignación por puntero, por reflect o
// disfrazada se le escapa. Se pone igual porque el caso real acá no es un adversario, es la
// próxima persona que necesite un doble y escriba lo que parece.
//
// La red que SÍ es de primer orden es el candado: aunque alguien escriba la variable, el lector
// pasa por `binarioForzado()` y el `-race` del CI lo vuelve a cazar. Esto es para que se vea antes
// y con un mensaje que explique por qué.
func TestNadieEscribeBinarioRustdeskAMano(t *testing.T) {
	const duenio = "rustdesk_ruta.go"
	asigna := regexp.MustCompile(`(?m)^\s*binarioRustdesk\s*=[^=]`)

	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	revisados := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || e.Name() == duenio {
			continue
		}
		crudo, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			continue
		}
		revisados++
		if m := asigna.FindString(string(crudo)); m != "" {
			t.Errorf(`%s ESCRIBE binarioRustdesk A MANO:
  %s

Esa variable la lee una goroutine de produccion que sobrevive a quien la creo (el
time.AfterFunc del vencimiento de sesion, pantalla.go, llega hasta rutaRustdesk()), asi que
escribirla sin candado es una CARRERA — cazada por el detector en CI el 2026-09-12.

Arreglo: usa el setter.  t.Cleanup(ForzarBinarioRustdesk("/la/ruta"))`, e.Name(), strings.TrimSpace(m))
		}
	}

	// EL CONTROL: que el barrido haya visto archivos. Si el filtro se rompiera, esta guarda pasaria
	// en verde sin haber leido ninguno, que es la falla que persigue.
	if revisados < 5 {
		t.Fatalf("se revisaron %d archivos .go de este paquete y son decenas: el recorrido dejo de "+
			"mirar y esta guarda esta en verde sin haber comprobado nada", revisados)
	}
}
