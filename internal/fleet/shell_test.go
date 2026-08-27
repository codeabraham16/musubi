package fleet

// Pruebas del dominio de la sesión de shell (S5b).

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// LO QUE ESTA ESTRUCTURA NO TIENE ES SU DISEÑO: no hay dónde guardar el CONTENIDO de la sesión.
//
// Se mira la ESTRUCTURA por reflexión y no una instancia, por el mismo motivo que la prueba
// gemela de SesionPantalla: una prueba sobre valores pasaría con el campo presente y vacío, que
// es exactamente el estado en el que alguien lo agrega «por ahora, para poder depurar».
//
// Grabar lo que alguien teclea en una terminal ajena —contraseñas de sudo incluidas— es una
// decisión legal antes que técnica, y no se toma de rebote agregando un campo.
//
// Sabotaje que la hace fallar: agregarle a SesionShell un campo `Salida`, `Buffer` o similar.
func TestSesionShellNoTienePorDondeGuardarLoQuePasoPorLaTerminal(t *testing.T) {
	tipo := reflect.TypeOf(SesionShell{})
	prohibidos := []string{"contenido", "salida", "grabacion", "transcripcion", "buffer", "stdout", "stdin", "entrada", "historial", "bytes"}
	for i := 0; i < tipo.NumField(); i++ {
		nombre := strings.ToLower(tipo.Field(i).Name)
		for _, p := range prohibidos {
			if strings.Contains(nombre, p) {
				t.Errorf("SesionShell tiene el campo %q: el contenido de una sesión no se guarda, y no hay dónde ponerlo a propósito", tipo.Field(i).Name)
			}
		}
	}
	// Control positivo: la estructura tiene que tener los campos de AUDITORÍA, o la prueba de
	// arriba pasaría también sobre una estructura vacía.
	for _, hace_falta := range []string{"Principal", "DeviceID", "Creada", "Cerrada"} {
		if _, ok := tipo.FieldByName(hace_falta); !ok {
			t.Errorf("falta el campo de auditoría %q: sin él la bitácora no sirve para nada", hace_falta)
		}
	}
}

// ValidarAperturaShell dice el problema REAL, no uno tres capas más abajo.
//
// El fallo más común al abrir una shell en un Tier B es que la máquina no tenga dirección. Sin
// esta guarda, el error llega como un "could not resolve hostname" de ssh y manda a alguien a
// mirar el DNS de una máquina que nunca tuvo host configurado.
func TestAbrirUnaShellSinDireccionDiceElProblemaDeVerdad(t *testing.T) {
	sinDir := Device{Name: "router", Tier: TierProtocolo, Caps: []Cap{CapShell}}
	err := ValidarAperturaShell(sinDir)
	if err == nil {
		t.Fatal("un Tier B sin dirección no tiene a dónde conectarse")
	}
	if !strings.Contains(err.Error(), "dirección") {
		t.Errorf("el error no menciona la dirección: %q", err)
	}

	// Y una máquina a la que no se le concedió `shell` se rechaza antes de tocar la red.
	sinCap := Device{Name: "nas", Tier: TierProtocolo, Address: "gio@nas", Caps: []Cap{CapExec}}
	if err := ValidarAperturaShell(sinCap); err == nil {
		t.Fatal("se validó la apertura en una máquina sin la capacidad `shell`")
	}

	buena := Device{Name: "nas", Tier: TierProtocolo, Address: "gio@nas", Caps: []Cap{CapShell}}
	if err := ValidarAperturaShell(buena); err != nil {
		t.Errorf("una máquina bien configurada no debería fallar: %v", err)
	}
}

// Viva() consulta el DOMINIO y no la columna de estado. Una sesión cuya fila dice «activa» y
// venció hace una hora está muerta: preguntarle a `estado` daría por viva a cualquiera que nadie
// fue a marcar — el mismo criterio que el «en línea» de un dispositivo.
func TestUnaSesionVivaSeDerivaYNoSeLeeDeLaColumna(t *testing.T) {
	base := time.Now()
	s := SesionShell{
		Estado: ShellActiva, Creada: base,
		Vence: base.Add(ShellVidaMax), UltimoTrafico: base,
	}
	if !s.Viva(base.Add(time.Minute)) {
		t.Error("una sesión con tráfico reciente tendría que estar viva")
	}
	// La columna sigue diciendo «activa» y ya no lo está.
	if s.Viva(base.Add(ShellVidaMax + time.Minute)) {
		t.Error("una sesión pasada de su vida máxima figuró viva porque su columna decía «activa»")
	}
	// Y un estado terminal manda aunque no haya vencido nada.
	cerrada := s
	cerrada.Estado = ShellCerrada
	if cerrada.Viva(base.Add(time.Minute)) {
		t.Error("una sesión cerrada figuró viva")
	}
}
