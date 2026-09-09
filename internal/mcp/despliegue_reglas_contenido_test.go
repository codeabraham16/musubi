package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TODO ARCHIVO DE REGLAS DESPLEGABLE SE COMPARA POR CONTENIDO, NO SÓLO POR NOMBRE.
//
// POR QUÉ EXISTE, MEDIDO TRES VECES.
//
// La sección 2 de `verificar-despliegue.sh` compara los NOMBRES de las reglas cargadas contra los
// que declara cada `.yml`. Eso deja pasar el caso que más duele: **mismo juego de nombres,
// distinto número adentro**.
//
//   - 2026-09-05: el `musubi-alerts.yml` desplegado decía `!= 24` y el repo `!= 27`, **con los dos
//     archivos pesando los mismos 23912 bytes**. `ReglasDeFlotaSinDesplegar` estaba en VERDE
//     mientras faltaban tres reglas.
//   - 2026-09-08: se desplegó a mano `musubi-alerts-flota.yml` (27→28) y NO su hermano
//     `musubi-alerts.yml`, que lleva el umbral cruzado. El informe decía `✔ musubi-alerts.yml —
//     sus 23 reglas están cargadas` —cierto— con el archivo difiriendo en el renglón que decide.
//   - 2026-09-09: con la comparación por sha256 ya puesta, la primera corrida real lo cazó:
//     `✘ musubi-alerts.yml DIFIERE del repo`. El diff era **una línea**: `!= 27` contra `!= 28`.
//
// LA PREMISA QUE SOSTENÍA TODO ESTO NO LA CUSTODIABA NADA. `deploy/musubi-alerts.yml:374-377`
// explica que las dos guardas cruzadas funcionan porque «el despliegue copia los dos archivos
// JUNTOS, así que envejecen juntos». Es cierto para `preparar.sh` —los instala en el mismo
// bloque— y es una CONVENCIÓN DEL PROCEDIMIENTO, no una guarda: un despliegue a mano de un solo
// archivo la rompe y nada se entera. El sha256 no impide romperla; hace que romperla se VEA.
//
// QUÉ CUSTODIA ESTA PRUEBA, Y QUÉ NO. No puede correr el `ssh` ni comprobar el sha (eso lo hace el
// verificador contra un servidor real). Custodia lo único que se puede romper EN EL REPO y quedar
// invisible: **que un archivo de reglas nuevo quede fuera de la comparación sin que nadie lo
// note**. El bucle usa globs; un `.yml` que no matchee ningún glob no se compara y no se dice.
//
// Sabotajes que la ponen en rojo (verificados):
//   - agregar un `deploy/musubi-slo.yml` con `# despliegue:` (no lo matchea ningún glob de hoy);
//   - sacar `musubi-recording.yml` de la lista de globs del verificador;
//   - borrar el bucle de comparación entero.
func TestTodoArchivoDeReglasDesplegableSeComparaPorContenido(t *testing.T) {
	guion := filepath.Join("..", "..", "deploy", "verificar-despliegue.sh")
	crudo, err := os.ReadFile(guion)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", guion, err)
	}

	// LOS GLOBS SE LEEN DEL GUION, NO SE COPIAN ACÁ. Una lista repetida a mano en la prueba es
	// otra fuente de verdad sobre lo mismo: el día que cambie una sola, las dos se dan la razón.
	// Es la misma trampa que el propio archivo de alertas documenta en :374-377.
	linea := regexp.MustCompile(`(?m)^\s*for f_r in ([^\n;]+); do`)
	m := linea.FindSubmatch(crudo)
	if m == nil {
		t.Fatal("no encontré el bucle `for f_r in ...` que compara los archivos de reglas por " +
			"contenido en verificar-despliegue.sh. O se renombró, o se borró: sin él, un umbral " +
			"cambiado con los mismos nombres de regla vuelve a ser invisible (pasó el 2026-09-05 y " +
			"el 2026-09-08)")
	}

	repoDir := filepath.Join("..", "..")
	comparados := map[string]bool{}
	for _, patron := range strings.Fields(string(m[1])) {
		// El guion los escribe como "$REPO"/deploy/xxx.yml
		patron = strings.ReplaceAll(patron, `"$REPO"/`, "")
		patron = strings.ReplaceAll(patron, `$REPO/`, "")
		patron = strings.Trim(patron, `"'`)
		if patron == "" {
			continue
		}
		rutas, err := filepath.Glob(filepath.Join(repoDir, patron))
		if err != nil {
			t.Fatalf("glob inválido %q tomado del guion: %v", patron, err)
		}
		for _, r := range rutas {
			comparados[filepath.Base(r)] = true
		}
	}

	// EL UNIVERSO: todo `.yml` de deploy/ que DECLARE cuándo se despliega. Esa línea es lo que
	// convierte a un archivo en «desplegable», y su presencia ya la custodia
	// TestCadaArchivoDeReglasDeclaraCuandoSeDespliega.
	yamls, err := filepath.Glob(filepath.Join(repoDir, "deploy", "*.yml"))
	if err != nil {
		t.Fatalf("no se pudo listar deploy/*.yml: %v", err)
	}
	var desplegables, sinComparar []string
	for _, y := range yamls {
		b, err := os.ReadFile(y)
		if err != nil {
			continue
		}
		if !regexp.MustCompile(`(?m)^#\s*despliegue:`).Match(b) {
			continue
		}
		nombre := filepath.Base(y)
		desplegables = append(desplegables, nombre)
		if !comparados[nombre] {
			sinComparar = append(sinComparar, nombre)
		}
	}

	// SIN ESTO LA GUARDA SE APAGA SOLA: si `deploy/` se mueve o el glob deja de encontrar nada,
	// `sinComparar` queda vacío y esto pasa en VERDE sin haber mirado un archivo.
	if len(desplegables) < 4 {
		t.Fatalf("sólo encontré %d archivo(s) de reglas con `# despliegue:` en deploy/ (%v); se "+
			"esperaban al menos 4. El barrido dejó de mirar y un verde acá no significaría nada",
			len(desplegables), desplegables)
	}

	if len(sinComparar) > 0 {
		sort.Strings(sinComparar)
		t.Errorf("hay %d archivo(s) de reglas desplegables que el verificador NO compara por "+
			"contenido: %s\n\n"+
			"Los globs de `for f_r in ...` (verificar-despliegue.sh) no los matchean, así que su "+
			"contenido nunca se cruza contra el servidor. La sección 2 los daría por buenos igual, "+
			"porque compara NOMBRES de reglas: un umbral cambiado con los mismos nombres es "+
			"invisible. Pasó el 2026-09-05 (`!= 24` contra `!= 27`, mismos 23912 bytes) y el "+
			"2026-09-08 (`!= 27` contra `!= 28`).\n"+
			"Agregá el archivo al glob del bucle, o sacale la línea `# despliegue:` si no se "+
			"despliega.",
			len(sinComparar), strings.Join(sinComparar, ", "))
	}
}
