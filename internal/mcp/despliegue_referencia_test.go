package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// LA GUARDA ES DE COMPORTAMIENTO Y NO DE TEXTO, A PROPÓSITO.
//
// La forma barata de custodiar esto sería un grep sobre `verificar-despliegue.sh` buscando
// «origin/main». Sería una guarda hueca de manual: en ese archivo el texto `origin/main` aparece
// además en comentarios, y —peor— seguiría verde si alguien deja la sección escrita pero rompe la
// condición que DECIDE. Este repo ya pagó siete veces esa lección (A107, A120): la guarda tiene
// que mirar donde se decide, no donde se nombra.
//
// Así que acá se CORRE el guion, en un repo de prueba armado a mano, y se mira lo que dice de la
// referencia. Si alguien saca el `dudoso`, el caso B se pone rojo aunque el comentario siga.
//
// EL DEFECTO QUE CUSTODIA (medido el 2026-09-10): todo el verificador compara contra `$REPO/...`,
// o sea contra el ÁRBOL DE TRABAJO, y nunca decía cuál. Con el checkout parado en una rama, la
// corrida de las 08:12 dijo «cerebro en 0.139.7 — mismo release que el repo (0.139.7)» y salió 0
// mientras `origin/main` estaba en 0.140.1 hacía tres horas.

// prepararRepoDePrueba arma un repo git mínimo con lo que el verificador exige para arrancar
// (`VERSION` y `deploy/`), con el guion real adentro, y con un `origin/main` local apuntando al
// único commit. Devuelve la ruta.
func prepararRepoDePrueba(t *testing.T) string {
	t.Helper()
	guion, err := os.ReadFile("../../deploy/verificar-despliegue.sh")
	if err != nil {
		t.Fatalf("no se pudo leer el guion real: %v", err)
	}
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "deploy", "verificar-despliegue.sh"), guion, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "VERSION"), []byte("9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = raiz
		// Un `user.name` global ausente en CI haría fallar el commit por un motivo que no es el
		// que se está probando, así que la identidad va acá y no se hereda.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=prueba", "GIT_AUTHOR_EMAIL=prueba@local",
			"GIT_COMMITTER_NAME=prueba", "GIT_COMMITTER_EMAIL=prueba@local",
		)
		if salida, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s falló: %v\n%s", strings.Join(args, " "), err, salida)
		}
	}
	git("init", "--quiet", "-b", "main")
	git("add", "-A")
	git("commit", "--quiet", "-m", "base")
	// El ref de seguimiento se escribe a mano: el repo de prueba no tiene remoto, y lo que se
	// custodia es la COMPARACIÓN contra origin/main, no el transporte que la trae.
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	return raiz
}

// correrVerificador corre el guion y devuelve su salida combinada. No se mira el código de salida:
// sin Prometheus del otro lado el guion sale 2 en los dos casos, así que el código no distingue
// nada y creerle sería la clase de aserción que pasa por el motivo equivocado.
func correrVerificador(t *testing.T, raiz string) string {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(raiz, "deploy", "verificar-despliegue.sh"))
	cmd.Dir = raiz
	cmd.Env = append(os.Environ(),
		// Sin fetch: el repo de prueba no tiene remoto. Ejercita además esa rama del guion.
		"MUSUBI_SIN_FETCH=1",
		// Puerto 1: el rechazo es inmediato, así que la prueba no espera timeouts de red.
		"PROM_URL=http://127.0.0.1:1",
		"ALERT_URL=http://127.0.0.1:1",
		"MUSUBI_SSH=",
	)
	salida, _ := cmd.CombinedOutput()
	return string(salida)
}

// seccionDeLaReferencia recorta la parte del informe que habla de la referencia, para que una
// aserción no pueda satisfacerse con una línea de otra sección que casualmente diga lo mismo.
func seccionDeLaReferencia(t *testing.T, salida string) string {
	t.Helper()
	i := strings.Index(salida, "la referencia")
	if i < 0 {
		t.Fatalf("el informe no trae la sección de la referencia.\nSalida:\n%s", salida)
	}
	resto := salida[i:]
	if j := strings.Index(resto, "cadena de alertas"); j > 0 {
		resto = resto[:j]
	}
	return resto
}

func TestElVerificadorDiceContraQueArbolCompara(t *testing.T) {
	saltarSiWindows(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("hace falta git")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("el guion se corta sin python3 antes de llegar a la sección que se prueba")
	}

	t.Run("un árbol que ES origin/main y está limpio se declara como tal", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		sec := seccionDeLaReferencia(t, correrVerificador(t, raiz))
		if !strings.Contains(sec, "el árbol es origin/main exacto") {
			t.Errorf("un checkout limpio de origin/main tiene que declararse como referencia buena.\nSección:\n%s", sec)
		}
		if strings.Contains(sec, "el árbol NO es origin/main") {
			t.Errorf("un checkout limpio de origin/main no puede reportarse como divergente.\nSección:\n%s", sec)
		}
	})

	// LAS DOS DIRECCIONES DEL SABOTAJE. Sin este caso, la guarda la satisface un guion que diga
	// siempre «es origin/main»; sin el de arriba, uno que diga siempre «NO es». Una sola dirección
	// deja pasar la mitad de las formas de romperlo.
	t.Run("un árbol adelantado de origin/main NO puede pasar por referencia", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		if err := os.WriteFile(filepath.Join(raiz, "VERSION"), []byte("9.9.10\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "commit", "--quiet", "-am", "bump")
		cmd.Dir = raiz
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=prueba", "GIT_AUTHOR_EMAIL=prueba@local",
			"GIT_COMMITTER_NAME=prueba", "GIT_COMMITTER_EMAIL=prueba@local",
		)
		if salida, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("no se pudo adelantar el árbol: %v\n%s", err, salida)
		}
		sec := seccionDeLaReferencia(t, correrVerificador(t, raiz))
		if !strings.Contains(sec, "el árbol NO es origin/main") {
			t.Errorf("un árbol con un commit que origin/main no tiene NO puede declararse referencia buena: es el caso que dejó ciega la corrida del 2026-09-10.\nSección:\n%s", sec)
		}
		if !strings.Contains(sec, "commits que origin/main no") {
			t.Errorf("el aviso tiene que decir EN QUÉ DIRECCIÓN difiere: «no coincide» sin la dirección no dice si falta desplegar o falta mergear.\nSección:\n%s", sec)
		}
	})

	t.Run("un árbol sucio NO puede pasar por referencia", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		// Un archivo SIN TRACKEAR, no uno modificado: es la mitad que `construir.sh` no veía
		// cuando declaró limpio un binario que se llevaba código de más.
		if err := os.WriteFile(filepath.Join(raiz, "deploy", "regla-nueva.yml"), []byte("- alert: X\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		sec := seccionDeLaReferencia(t, correrVerificador(t, raiz))
		if !strings.Contains(sec, "el árbol NO es origin/main") {
			t.Errorf("un archivo sin commitear cambia lo que el verificador compara, así que el árbol no es la referencia.\nSección:\n%s", sec)
		}
		if !strings.Contains(sec, "sin commitear") {
			t.Errorf("el aviso tiene que nombrar la causa —lo que falta commitear— y no sólo que difiere.\nSección:\n%s", sec)
		}
	})
}

// saltarSiWindows corta las pruebas que EJECUTAN los guiones de `deploy/`.
//
// NO ES UNA EXENCIÓN POR COMODIDAD: esos guiones son bash y corren en la máquina del operador
// —Linux o macOS—, nunca en Windows. El banco además necesita un shim de `ssh` ejecutable por
// shebang y rutas POSIX, que en Windows no existen. Probarlos ahí no mide nada del sistema real;
// mide el emulador.
//
// SE HACE CON UN `t.Skip` Y NO CON UN `//go:build !windows`, a propósito. Este repo ya pagó esa
// diferencia: cuatro pruebas en un `*_windows_test.go` NO COMPILABAN fuera de Windows, `go test`
// contestaba `ok`, `go vet` pasaba, y las pruebas simplemente NO EXISTÍAN. Un `t.Skip` sale
// impreso en `go test -v` con su motivo; una restricción de build no deja rastro.
//
// macOS NO se saltea, y eso importa: es donde apareció el defecto de `${VAR}` pegada a un
// carácter no-ASCII que en Linux era invisible.
func saltarSiWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("los guiones de deploy/ son bash y corren en la máquina del operador (Linux/macOS); " +
			"el banco necesita rutas POSIX y un `ssh` ejecutable por shebang")
	}
}
