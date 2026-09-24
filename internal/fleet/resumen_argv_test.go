package fleet

import (
	"strings"
	"testing"
)

// LO QUE CUSTODIA: que ResumenArgv —la línea «legible para logs y paneles»— nunca lleve la
// contraseña de una sesión de pantalla, y que tapar no se lleve puesto lo que sí hay que ver.
//
// Sale de A131 (ronda 8, 2026-09-23): el agente imprimía el argv CRUDO de cada comando que
// atendía, y el de `musubi:pantalla` lleva la contraseña en la posición 2. La guarda hermana,
// `TestElArgvDeBitacoraNuncaLlevaLaContrasena`, promete «en ninguna superficie» y mira sólo las
// del cerebro. Ésta cubre la función; la del agente (cmd/musubi) cubre su stdout real.
//
// Sabotaje: que ResumenArgv vuelva a unir el argv crudo, sin pasar por ArgvDeBitacora.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="return strings.Join(ArgvDeBitacora(argv), \" \")"
// arnes: a="return strings.Join(argv, \" \")"
func TestResumenArgvNuncaLlevaLaContrasenaDePantalla(t *testing.T) {
	const secreto = "ClaveDePruebaQueNoPuedeSalir-9f3a"
	linea := ResumenArgv([]string{OpPantalla, "ses-42", secreto, "30m"})
	if strings.Contains(linea, secreto) {
		t.Errorf("ResumenArgv dejó pasar la contraseña de la sesión de pantalla: %q\n"+
			"Esta línea es la que el agente imprime por cada comando, y en Linux eso es el journal\n"+
			"de `musubi-agente`, que es persistente. Tiene que pasar por ArgvDeBitacora.", linea)
	}
	// LOS DOS CONTROLES. Sin el primero, una línea vacía «no llevaría la contraseña» y pasaría;
	// sin el segundo, tapar TODO argv también pasaría, y el log dejaría de servir para algo.
	if !strings.Contains(linea, OpPantalla) || !strings.Contains(linea, "ses-42") {
		t.Errorf("la línea tapada perdió lo que sí hay que ver —la operación y el id de sesión, "+
			"que sirve para cruzar con la bitácora de pantalla—: %q", linea)
	}
	comun := []string{"systemctl", "restart", "nginx"}
	if got, want := ResumenArgv(comun), "systemctl restart nginx"; got != want {
		t.Errorf("un comando común salió alterado: %q, esperaba %q — tapar es SÓLO para pantalla", got, want)
	}
}
