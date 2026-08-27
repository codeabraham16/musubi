package fleet

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// G1 — LA GARANTÍA CENTRAL DEL SLICE: la sesión NO tiene dónde guardar la contraseña.
//
// No es una prueba de comportamiento, es una prueba de FORMA — y esa es la única manera de
// custodiar una ausencia. Si alguien le agrega el campo «para poder reconectar», la base de
// Musubi se convierte en un llavero de acceso a la flota entera y esta prueba lo dice.
func TestLaSesionNoTieneDondeGuardarLaContrasena(t *testing.T) {
	tipo := reflect.TypeOf(SesionPantalla{})
	prohibidos := []string{"pass", "password", "contrasena", "contraseña", "secret", "clave", "credencial", "hash", "token"}
	for i := 0; i < tipo.NumField(); i++ {
		nombre := strings.ToLower(tipo.Field(i).Name)
		for _, p := range prohibidos {
			if strings.Contains(nombre, p) {
				t.Errorf("SesionPantalla tiene el campo %q: la contraseña NO puede persistirse, "+
					"ni en claro ni hasheada. Se acuña, viaja dos veces y se descarta.", tipo.Field(i).Name)
			}
		}
	}
}

// La contraseña tiene entropía real y es DICTABLE: se lee en voz alta más seguido de lo que uno
// cree, y un 0/O ambiguo se paga con una llamada más larga.
func TestLaContrasenaEsFuerteYDictable(t *testing.T) {
	vistas := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := NuevaPassPantalla()
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != largoPassPantalla {
			t.Fatalf("largo %d, esperaba %d", len(p), largoPassPantalla)
		}
		if vistas[p] {
			t.Fatalf("se repitió una contraseña en 200 tiradas: %q", p)
		}
		vistas[p] = true
		for _, c := range p {
			if !strings.ContainsRune(alfabetoPantalla, c) {
				t.Fatalf("carácter fuera del alfabeto: %q en %q", c, p)
			}
		}
		// Los ambiguos al dictar no pueden aparecer.
		for _, amb := range []string{"0", "O", "1", "l", "I", "5", "S", "8", "B"} {
			if strings.Contains(p, amb) {
				t.Errorf("la contraseña %q trae %q, que se confunde al dictarla", p, amb)
			}
		}
	}
}

// G2 — el vencimiento se DERIVA, no es una columna que alguien tiene que ir a actualizar.
func TestElVencimientoSeDerivaDelReloj(t *testing.T) {
	ahora := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	s := SesionPantalla{Creada: ahora, Vence: ahora.Add(30 * time.Minute)}
	if s.Vencida(ahora.Add(10 * time.Minute)) {
		t.Error("una sesión dentro de su ventana figuró vencida")
	}
	if !s.Vencida(ahora.Add(31 * time.Minute)) {
		t.Error("una sesión pasada su ventana no figuró vencida")
	}
	// Sin ventana declarada, no vence: es una sesión a medio construir, no una eterna.
	if (SesionPantalla{}).Vencida(ahora) {
		t.Error("una sesión sin `Vence` no debería reportarse vencida")
	}
}

func TestLaDuracionSeAcota(t *testing.T) {
	casos := []struct{ pide, quiero time.Duration }{
		{0, SesionDuracionDefault},
		{-time.Hour, SesionDuracionDefault},
		{5 * time.Minute, 5 * time.Minute},
		{100 * time.Hour, SesionDuracionMax},
	}
	for _, c := range casos {
		if got := NormalizarDuracion(c.pide); got != c.quiero {
			t.Errorf("NormalizarDuracion(%s) = %s, esperaba %s", c.pide, got, c.quiero)
		}
	}
	// Y el máximo no puede aflojarse sin que alguien lo note: una sesión que dura un día es
	// indistinguible de un acceso permanente.
	if SesionDuracionMax > 8*time.Hour {
		t.Errorf("SesionDuracionMax es %s: una sesión tan larga es acceso permanente con otro nombre", SesionDuracionMax)
	}
}
