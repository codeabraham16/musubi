package mcp

// despliegue_reglas_viejas_test.go — que una regla condicional de preparar.sh no quede cargada con
// su versión vieja mientras el guion dice que no está.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestUnaReglaCondicionalNoQuedaViejaEnRules — la rama «NO instaladas» tiene que decir la verdad.
//
// preparar.sh instala varios archivos de reglas SÓLO si se cumple algo (hay destino off-host, hay
// scrape de altura, el cerebro expone flota, el tablero empujó alguna foto). Cuando la condición
// deja de cumplirse, la rama `else` imprime «NO instaladas»; y si no BORRA el archivo de `rules/`,
// ese archivo sigue cargado en Prometheus con la versión del despliegue anterior: el guion dice una
// cosa y el server hace otra, y cada re-despliegue deja las reglas más viejas.
//
// HAY DOS SALIDAS HONESTAS, y el repo usa las dos. Borrar en la rama `else` (backup, altura, flota):
// se desinstala y lo dice. O un TRINQUETE en la condición, `[ -f "$DEST/rules/<archivo>" ] || …`
// (el tablero): una vez instalado se reinstala siempre, y la rama `else` no llega a correr con el
// archivo puesto. El tablero NO puede borrar: su condición es que hayan llegado fotos, y su alerta es
// la que avisa que dejaron de llegar.
//
// El trinquete del tablero se escribió con un porqué equivocado —«sin él, re-correr el guion
// DESINSTALARÍA la alerta»—, cuando su rama `else` no borra nada. Lo encontró la revisión del punto
// 32, y también que sacarlo dejaba todas las guardas en verde. Ésta es la que lo custodia.
//
// Sabotaje que la hace fallar: el tablero sin su trinquete.
// arnes: archivo="deploy/docker/preparar.sh"
// arnes: de="[ -f \"$DEST/rules/musubi-alerts-tablero.yml\" ] ||"
// arnes: a="[ -f \"$DEST/rules/musubi-alerts-tablero.yml.viejo\" ] ||"
// Sabotaje que la hace fallar: altura deja de borrar en su rama «NO instaladas».
// arnes: archivo="deploy/docker/preparar.sh"
// arnes: de="\trm -f \"$DEST/rules/musubi-alerts-altura.yml\"\n"
// arnes: a="\t: rm -f \"$DEST/rules/musubi-alerts-altura.yml\"\n"
func TestUnaReglaCondicionalNoQuedaViejaEnRules(t *testing.T) {
	ruta := filepath.Join("..", "..", "deploy", "docker", "preparar.sh")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v", ruta, err)
	}
	lineas := strings.Split(strings.ReplaceAll(string(crudo), "\r\n", "\n"), "\n")

	// LOS BLOQUES DE PRIMER NIVEL: `if` en la columna 0, su condición hasta el `then`, y `else`/`fi`
	// también en la columna 0. Los anidados van con tabulador, así que no cortan nada acá.
	type bloque struct {
		linea                int
		cond, entonces, sino []string
	}
	var bloques []bloque
	for i := 0; i < len(lineas); i++ {
		if !strings.HasPrefix(lineas[i], "if ") {
			continue
		}
		b := bloque{linea: i + 1}
		for ; i < len(lineas); i++ {
			b.cond = append(b.cond, lineas[i])
			if strings.HasSuffix(strings.TrimSpace(lineas[i]), "then") {
				break
			}
		}
		destino := &b.entonces
		for i++; i < len(lineas) && lineas[i] != "fi"; i++ {
			if lineas[i] == "else" || strings.HasPrefix(lineas[i], "elif ") {
				destino = &b.sino
				continue
			}
			*destino = append(*destino, lineas[i])
		}
		bloques = append(bloques, b)
	}

	reInstala := regexp.MustCompile(`^\s*install\b.*"\$DEST/rules/([^"]+)"\s*$`)
	vistas := map[string]bool{}
	for _, b := range bloques {
		for _, l := range b.entonces {
			m := reInstala.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			archivo := m[1]
			vistas[archivo] = true
			enRules := `"$DEST/rules/` + archivo + `"`
			trinquete := false
			for _, c := range b.cond {
				if strings.Contains(c, "-f "+enRules) {
					trinquete = true
				}
			}
			reBorra := regexp.MustCompile(`^\s*rm\s+-f\s+` + regexp.QuoteMeta(enRules) + `\s*$`)
			borra := false
			for _, s := range b.sino {
				if reBorra.MatchString(s) {
					borra = true
				}
			}
			if !trinquete && !borra {
				t.Errorf("%s:%d instala rules/%s sólo si se cumple su condición, y cuando deja de cumplirse "+
					"no lo borra ni lo reinstala: el archivo queda cargado con la versión VIEJA mientras el "+
					"guion dice «NO instaladas».\n  O su rama `else` hace `rm -f %s`, o su condición "+
					"arranca por `[ -f %s ] ||` para que un re-despliegue lo actualice.",
					ruta, b.linea, archivo, enRules, enRules)
			}
		}
	}
	// EL CONTROL: esta guarda existe por el tablero, y si no lo ve es que dejó de mirar.
	if !vistas["musubi-alerts-tablero.yml"] || len(vistas) < 2 {
		t.Fatalf("sólo vi %d instalación/es condicional/es de rules/ en %s (%v) y el tablero tiene que "+
			"estar entre ellas: el lector de bloques dejó de entender el guion y un verde acá no diría nada",
			len(vistas), ruta, vistas)
	}
}
