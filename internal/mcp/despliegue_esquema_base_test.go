package mcp

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// LO QUE CUSTODIA: que alguien compare el esquema al que apunta el binario del cerebro contra el
// que tiene la base.
//
// NADIE LO COMPARABA, y es la razón por la que «falta el redespliegue del cerebro» pudo quedar
// escrito en TRES filas del registro sin que nada lo contradijera: la única forma de saberlo era
// entrar a mano. Medido el 2026-09-10, los dos están en 53 — pero eso se supo entrando a mano, que
// es justo el paso que esta sección elimina.
//
// ES UNA PREGUNTA DISTINTA DE LA VERSIÓN. El modo de falla que importa no es «binario viejo» sino
// «binario nuevo y migración que no corrió»: ahí la versión coincide y la base se quedó atrás. Es
// lo que A111 dice que el guion de redespliegue dejó de verificar.
//
// Y EL NÚMERO DE LA BASE SE LEE DEL ENCABEZADO, no con `sqlite3`: en ese cerebro `sqlite3` NO ESTÁ
// INSTALADO, que es probablemente por qué la verificación de A111 estaba muerta. Una guarda que
// asumiera `sqlite3` volvería a morir por el mismo motivo.

// baseFalsa escribe un archivo con la forma mínima de una base SQLite para lo único que se lee de
// ella: `user_version`, cuatro bytes big-endian en el offset 60 del encabezado.
func baseFalsa(t *testing.T, dir string, userVersion uint32) string {
	t.Helper()
	cab := make([]byte, 100)
	copy(cab, "SQLite format 3\x00")
	binary.BigEndian.PutUint32(cab[60:64], userVersion)
	ruta := filepath.Join(dir, "memoria-falsa.db")
	if err := os.WriteFile(ruta, cab, 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// musubiFalso deja en el PATH un `musubi` que contesta lo que se le diga a `version --esquema`.
// Con `esquema` vacío no contesta nada, que es el caso del binario viejo que no conoce la bandera.
func musubiFalso(t *testing.T, dir, esquema string) {
	t.Helper()
	cuerpo := "#!/usr/bin/env bash\n"
	if esquema != "" {
		cuerpo += "if [ \"${1:-}\" = version ] && [ \"${2:-}\" = --esquema ]; then echo " + esquema + "; exit 0; fi\n"
	}
	cuerpo += "exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "musubi"), []byte(cuerpo), 0o755); err != nil {
		t.Fatal(err)
	}
}

func seccionDelEsquema(t *testing.T, salida string) string {
	t.Helper()
	i := strings.Index(salida, "esquema de la base")
	if i < 0 {
		t.Fatalf("el informe no trae la sección del esquema de la base.\nSalida:\n%s", salida)
	}
	resto := salida[i+len("esquema de la base"):]
	// La sección termina en el próximo título en negrita; alcanza con cortar en el primer salto
	// doble, porque `titulo` imprime una línea en blanco antes de cada uno.
	if j := strings.Index(resto, "\n\n"); j > 0 {
		resto = resto[:j]
	}
	return resto
}

func correrVerificadorConEsquema(t *testing.T, raiz, bin, db string) string {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(raiz, "deploy", "verificar-despliegue.sh"))
	cmd.Dir = raiz
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MUSUBI_SIN_FETCH=1",
		"MUSUBI_SSH=", // sin host: `corre_alla` evalúa acá, contra el `musubi` falso del PATH
		"MUSUBI_BRAIN_DB="+db,
		"PROM_URL=http://127.0.0.1:1",
		"ALERT_URL=http://127.0.0.1:1",
	)
	salida, _ := cmd.CombinedOutput()
	return string(salida)
}

func TestElVerificadorComparaElEsquemaDeLaBaseContraElBinario(t *testing.T) {
	guiones.Unix(t, "corre la sección del esquema de deploy/verificar-despliegue.sh, que compara el "+
		"`user_version` de la base contra el binario; se mide también en macOS porque su bash "+
		"3.2 es donde aparecen los defectos de expansión que en Linux son invisibles",
		"bash", "git", "python3", "od")

	preparar := func(t *testing.T, esquemaBin string, esquemaBase uint32) (string, string, string) {
		t.Helper()
		raiz := prepararRepoDePrueba(t)
		bin := filepath.Join(raiz, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		musubiFalso(t, bin, esquemaBin)
		return raiz, bin, baseFalsa(t, raiz, esquemaBase)
	}

	t.Run("los dos números coinciden: verde", func(t *testing.T) {
		raiz, bin, db := preparar(t, "53", 53)
		sec := seccionDelEsquema(t, correrVerificadorConEsquema(t, raiz, bin, db))
		if !strings.Contains(sec, "esquema 53") || strings.Contains(sec, "apunta al esquema") {
			t.Errorf("con los dos números en 53 la sección tiene que declararlo al día.\nSección:%s", sec)
		}
	})

	// EL CASO QUE JUSTIFICA LA SECCIÓN: binario nuevo, base vieja. La VERSIÓN coincidiría —es el
	// mismo binario— y sin esto nadie lo vería.
	t.Run("la migración no llegó a la base: lo dice, y dice los dos números", func(t *testing.T) {
		raiz, bin, db := preparar(t, "53", 47)
		sec := seccionDelEsquema(t, correrVerificadorConEsquema(t, raiz, bin, db))
		if !strings.Contains(sec, "53") || !strings.Contains(sec, "47") {
			t.Errorf("tienen que salir LOS DOS números: sin ellos no se puede decidir si falta migrar o falta un checkpoint.\nSección:%s", sec)
		}
		if !strings.Contains(sec, "?") {
			t.Errorf("una base atrasada no puede pasar como verde.\nSección:%s", sec)
		}
	})

	// Un binario que no sabe contestar NO es una base atrasada: son dos cosas distintas y el
	// informe tiene que poder decir cuál. Si las confundiera, un cerebro viejo se leería como una
	// migración pendiente y mandaría a correr una migración que no hace falta.
	t.Run("un binario que no sabe decir su esquema no se lee como base atrasada", func(t *testing.T) {
		raiz, bin, db := preparar(t, "", 47)
		sec := seccionDelEsquema(t, correrVerificadorConEsquema(t, raiz, bin, db))
		if !strings.Contains(sec, "no supo decir") {
			t.Errorf("tiene que decir que el BINARIO no contestó, no que la base esté atrasada.\nSección:%s", sec)
		}
		if strings.Contains(sec, "47") {
			t.Errorf("sin el número del binario no hay comparación que reportar, así que el de la base no debería aparecer como si la hubiera.\nSección:%s", sec)
		}
	})
}
