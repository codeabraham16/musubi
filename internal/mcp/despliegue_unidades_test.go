package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// LO QUE CUSTODIA: que alguien compare las unidades de systemd INSTALADAS contra las que el repo
// declara en `deploy/systemd/`.
//
// El vigía comparaba byte a byte los guiones del servidor y era ciego a los suyos. La deriva ahí
// es de las más silenciosas: una unidad editada a mano sigue corriendo, y lo único que se nota es
// que algo deja de pasar — que es indistinguible de que no haya nada que reportar.
//
// DOS DECISIONES DE DISEÑO QUE LA PRUEBA FIJA, porque las dos se equivocaron primero:
//
//  1. SE COMPARAN SÓLO LOS NOMBRES QUE EL REPO DECLARA. Un barrido por el prefijo `musubi-`
//     parece más completo y es peor: en la máquina donde nació esto hay un `musubi-mc.service`
//     que es un servidor de MINECRAFT. Una guarda que grita sobre algo ajeno se aprende a ignorar.
//  2. SE NORMALIZA LA UNIDAD INSTALADA HACIA LA PLANTILLA, no al revés. Sustituir `@REPO@` por la
//     ruta actual y comparar contra la instalada hace que la comparación dependa de DÓNDE está el
//     repo —`@REPO@` aparece también en el comentario de instalación—, y desde un worktree TODA
//     unidad daba «difiere». Se vio corriendo el guion desde un worktree, no razonándolo.

func armarUnidades(t *testing.T, raiz string) (string, string) {
	t.Helper()
	repoUnits := filepath.Join(raiz, "deploy", "systemd")
	if err := os.MkdirAll(repoUnits, 0o755); err != nil {
		t.Fatal(err)
	}
	plantilla := "[Unit]\nDescription=prueba\n\n[Service]\nType=oneshot\n" +
		"WorkingDirectory=@REPO@\nExecStart=@REPO@/deploy/algo.sh\n\n[Install]\nWantedBy=default.target\n"
	if err := os.WriteFile(filepath.Join(repoUnits, "musubi-prueba.service"), []byte(plantilla), 0o644); err != nil {
		t.Fatal(err)
	}
	instaladas := filepath.Join(raiz, "unidades")
	if err := os.MkdirAll(instaladas, 0o755); err != nil {
		t.Fatal(err)
	}
	return repoUnits, instaladas
}

// instalar escribe la unidad como la dejaría la instalación real: `@REPO@` sustituido por una ruta
// que NO es la del repo de prueba, justamente para que la normalización tenga algo que hacer.
func instalar(t *testing.T, repoUnits, instaladas, extra string) {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join(repoUnits, "musubi-prueba.service"))
	if err != nil {
		t.Fatal(err)
	}
	texto := strings.ReplaceAll(string(crudo), "@REPO@", "/otra/ruta/cualquiera") + extra
	if err := os.WriteFile(filepath.Join(instaladas, "musubi-prueba.service"), []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
}

func correrVerificadorConUnidades(t *testing.T, raiz, unidades string) string {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(raiz, "deploy", "verificar-despliegue.sh"))
	cmd.Dir = raiz
	cmd.Env = append(os.Environ(),
		"MUSUBI_SIN_FETCH=1",
		"MUSUBI_SSH=",
		"MUSUBI_UNIDADES_DIR="+unidades,
		"PROM_URL=http://127.0.0.1:1",
		"ALERT_URL=http://127.0.0.1:1",
	)
	salida, _ := cmd.CombinedOutput()
	return string(salida)
}

func seccionDeUnidades(t *testing.T, salida string) string {
	t.Helper()
	i := strings.Index(salida, "unidades de systemd")
	if i < 0 {
		t.Fatalf("el informe no trae la sección de las unidades.\nSalida:\n%s", salida)
	}
	resto := salida[i:]
	if j := strings.Index(resto, "\n\n"); j > 0 {
		resto = resto[:j]
	}
	return resto
}

func TestElVerificadorComparaSusPropiasUnidadesInstaladas(t *testing.T) {
	guiones.Unix(t, "corre la sección de unidades de deploy/verificar-despliegue.sh contra unidades "+
		"instaladas de mentira; se mide también en macOS porque su bash 3.2 es donde aparecen "+
		"los defectos de expansión que en Linux son invisibles",
		"bash", "git", "python3")

	t.Run("la instalada es la del repo: verde aunque el repo viva en otra carpeta", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		repoUnits, instaladas := armarUnidades(t, raiz)
		instalar(t, repoUnits, instaladas, "")
		sec := seccionDeUnidades(t, correrVerificadorConUnidades(t, raiz, instaladas))
		if !strings.Contains(sec, "musubi-prueba.service coincide") {
			t.Errorf("la unidad instalada es la plantilla con su ruta sustituida: tiene que dar verde sin importar dónde esté el repo.\nSección:\n%s", sec)
		}
	})

	t.Run("una unidad editada a mano se reporta y se nombra", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		repoUnits, instaladas := armarUnidades(t, raiz)
		// El sabotaje es lo que de verdad pasa: alguien agrega una línea a la unidad instalada
		// para probar algo y se olvida de sacarla.
		instalar(t, repoUnits, instaladas, "Environment=DEBUG=1\n")
		sec := seccionDeUnidades(t, correrVerificadorConUnidades(t, raiz, instaladas))
		if !strings.Contains(sec, "difiere") {
			t.Errorf("una unidad instalada con una línea de más tiene que reportarse.\nSección:\n%s", sec)
		}
		if !strings.Contains(sec, "musubi-prueba.service") {
			t.Errorf("el aviso tiene que NOMBRAR la unidad: «algo difiere» no se puede accionar.\nSección:\n%s", sec)
		}
	})

	// EL CASO QUE FIJA LA DECISIÓN 1. Sin él, cambiar el barrido a `$UNIDADES_DIR/musubi-*` pasa
	// la prueba y rompe la máquina real: ahí hay un `musubi-mc.service` que es un servidor de
	// Minecraft, y el informe lo reportaría como unidad de Musubi con deriva.
	t.Run("una unidad ajena con el mismo prefijo no se reporta", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		repoUnits, instaladas := armarUnidades(t, raiz)
		instalar(t, repoUnits, instaladas, "")
		ajena := "[Unit]\nDescription=servidor de otra cosa\n\n[Service]\nExecStart=/bin/true\n"
		if err := os.WriteFile(filepath.Join(instaladas, "musubi-ajeno.service"), []byte(ajena), 0o644); err != nil {
			t.Fatal(err)
		}
		sec := seccionDeUnidades(t, correrVerificadorConUnidades(t, raiz, instaladas))
		if strings.Contains(sec, "musubi-ajeno") {
			t.Errorf("una unidad que el repo NO declara no es deriva de Musubi, y nombrarla enseña a ignorar el informe.\nSección:\n%s", sec)
		}
	})

	t.Run("ninguna instalada no se confunde con ninguna declarada", func(t *testing.T) {
		raiz := prepararRepoDePrueba(t)
		_, instaladas := armarUnidades(t, raiz) // el repo declara una, no se instala ninguna
		sec := seccionDeUnidades(t, correrVerificadorConUnidades(t, raiz, instaladas))
		if !strings.Contains(sec, "?") {
			t.Errorf("con unidades declaradas y ninguna instalada, el timer podría no existir y esto sólo correría cuando alguien se acuerde: no es un verde.\nSección:\n%s", sec)
		}
	})
}

// UN `ExecStart=` QUE APUNTA A UN ARCHIVO SIN BIT DE EJECUCIÓN NO ARRANCA, Y FALLA RECIÉN EL DÍA
// QUE ALGUIEN LO INSTALA.
//
// MEDIDO EL 2026-09-16, instalando el respaldo off-host de A117:
// `deploy/systemd/musubi-respaldo-local.service` declara `ExecStart=@REPO@/deploy/musubi-backup.sh`
// y ese guion estaba en **100644** en git — el único de los cuatro de `deploy/` sin el bit, contra
// `comparar-y-latir.sh`, `construir.sh` y `verificar-despliegue.sh` que sí lo tienen. Instalada tal
// como dicen sus propias instrucciones, la unidad contestó `status=203/EXEC` al primer arranque.
//
// NADIE LO HABÍA VISTO PORQUE NADIE LA HABÍA INSTALADO: una unidad que no se instala no falla, y su
// archivo se lee perfecto. Es la misma familia que las pruebas que no compilan — la ausencia se ve
// idéntica al éxito.
//
// EL MODO SE LEE DE GIT Y NO DEL DISCO, y la diferencia no es teórica: un `chmod +x` local hace que
// esta guarda pase acá y la unidad siga rota en el clone de todos los demás. El modo que viaja es el
// del índice.
// Sabotaje: apuntar el `ExecStart=` de una unidad a un archivo del repo que no tenga bit de
// ejecucion — que es el estado en el que estaba `musubi-backup.sh` hasta hoy.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="ExecStart=@REPO@/deploy/musubi-backup.sh"
// arnes: a="ExecStart=@REPO@/deploy/musubi-alerts-backup-offhost.yml"
func TestTodoExecStartDeLasUnidadesApuntaAAlgoEjecutable(t *testing.T) {
	raiz := filepath.Join("..", "..")
	salida, err := exec.Command("git", "-C", raiz, "ls-files", "-s", "deploy/").Output()
	if err != nil {
		t.Fatalf("no se pudo leer el índice de git: %v", err)
	}
	modo := map[string]string{}
	for _, l := range strings.Split(string(salida), "\n") {
		campos := strings.Fields(l)
		if len(campos) >= 4 {
			modo[campos[3]] = campos[0]
		}
	}
	if len(modo) < 5 {
		t.Fatalf("el índice devolvió %d archivos de deploy/ y hay muchos más: el parseo dejó de "+
			"funcionar y esta guarda está midiendo el vacío", len(modo))
	}

	unidades, _ := filepath.Glob(filepath.Join(raiz, "deploy", "systemd", "*.service"))
	if len(unidades) == 0 {
		t.Fatal("no encontré ninguna unidad en deploy/systemd/: el glob dejó de funcionar")
	}
	revisados := 0
	for _, u := range unidades {
		b, err := os.ReadFile(u)
		if err != nil {
			continue
		}
		for _, linea := range strings.Split(string(b), "\n") {
			linea = strings.TrimSpace(linea)
			if !strings.HasPrefix(linea, "ExecStart=") {
				continue
			}
			cmd := strings.TrimPrefix(linea, "ExecStart=")
			if i := strings.IndexByte(cmd, ' '); i > 0 {
				cmd = cmd[:i] // el binario, sin sus argumentos
			}
			// Sólo interesan los que apuntan a un archivo DEL REPO: `@REPO@/…` es la plantilla que
			// el instalador sustituye. Un ExecStart a /usr/bin/algo no es asunto de esta guarda.
			rel := ""
			if strings.HasPrefix(cmd, "@REPO@/") {
				rel = strings.TrimPrefix(cmd, "@REPO@/")
			} else if strings.HasPrefix(cmd, "deploy/") {
				rel = cmd
			}
			if rel == "" {
				continue
			}
			revisados++
			m, hay := modo[rel]
			if !hay {
				t.Errorf("%s declara `ExecStart=%s` y ese archivo NO está en el repo: la unidad no "+
					"puede arrancar donde se instale", filepath.Base(u), cmd)
				continue
			}
			if m != "100755" {
				t.Errorf("%s declara `ExecStart=%s` y ese archivo está en %s (sin bit de ejecución) "+
					"en el índice de git.\n"+
					"  Instalada como dicen sus propias instrucciones, la unidad da `status=203/EXEC`\n"+
					"  al primer arranque, y eso no se descubre leyendo el archivo: sólo instalándola.\n"+
					"  Se arregla con `git update-index --chmod=+x %s`.", filepath.Base(u), cmd, m, rel)
			}
		}
	}
	if revisados == 0 {
		t.Fatal("ninguna unidad de deploy/systemd/ declara un ExecStart al repo: o cambió la forma " +
			"de escribirlos, o el filtro dejó de reconocerlos — en los dos casos esta guarda dejó de mirar")
	}
}

// EL VIGÍA TIENE QUE PODER RECUPERAR LA RAÍZ DEL REPO DE TODA UNIDAD QUE USE `@REPO@`.
//
// `verificar-despliegue.sh` compara cada unidad INSTALADA contra su plantilla, y para eso
// normaliza la instalada «hacia» la plantilla: toma la ruta concreta que la unidad declara y la
// devuelve a `@REPO@`. Si no puede recuperar esa ruta, la comparación queda entre una plantilla
// con `@REPO@` y un archivo con una ruta absoluta, o sea que **dice «difiere» siempre**.
//
// MEDIDO EL 2026-09-16, Y ERA 1 DE 7: `musubi-respaldo-local.service` usaba `@REPO@` y no declaraba
// `WorkingDirectory`, que era la única pista que el verificador miraba. El informe la reportaba
// divergente ESTANDO BIEN INSTALADA. Un aviso que no puede apagarse nunca es peor que no avisar:
// enseña a leer el informe salteando los rojos conocidos, y el próximo rojo de verdad se pierde ahí.
//
// LA GUARDA LLEVA DOS CONTROLES, Y EL SEGUNDO ES EL QUE IMPORTA. El arreglo tenía dos formas: darle
// un `WorkingDirectory` a esa unidad, o enseñarle al verificador a recuperarse por `ExecStart`. Se
// eligió la segunda porque arregla la CLASE y no el caso — pero entonces el camino nuevo necesita
// que alguien lo pise, y la primera forma lo dejaría sin nadie. Por eso se exige que siga habiendo
// al menos una unidad con `@REPO@` y SIN `WorkingDirectory`: sin eso, el respaldo del verificador
// sería código muerto y su rotura no la notaría ninguna corrida.
//
// Sabotaje que la hace fallar: darle un `WorkingDirectory` a la única unidad que ejercita el
// respaldo, que es exactamente el arreglo cómodo que taparía el defecto.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="Environment=MUSUBI_HOME=@REPO@"
// arnes: a="WorkingDirectory=@REPO@\nEnvironment=MUSUBI_HOME=@REPO@"
func TestElVigiaPuedeRecuperarLaRaizDeTodaUnidadQueUseElMarcador(t *testing.T) {
	raiz := filepath.Join("..", "..")
	patron := filepath.Join(raiz, "deploy", "systemd", "*")
	rutas, err := filepath.Glob(patron)
	if err != nil {
		t.Fatalf("glob de deploy/systemd: %v", err)
	}
	var conMarcador, sinWorkingDirectory int
	for _, ruta := range rutas {
		ext := filepath.Ext(ruta)
		if ext != ".service" && ext != ".timer" {
			continue
		}
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatalf("leer %s: %v", ruta, err)
		}
		texto := string(crudo)
		// El marcador del comentario de instalación no cuenta: lo que hay que poder normalizar son
		// las DIRECTIVAS, y una unidad que sólo lo nombra en una línea `#` no tiene nada que recuperar.
		var directivas []string
		for _, l := range strings.Split(texto, "\n") {
			if l != "" && !strings.HasPrefix(l, "#") {
				directivas = append(directivas, l)
			}
		}
		cuerpo := strings.Join(directivas, "\n")
		if !strings.Contains(cuerpo, "@REPO@") {
			continue
		}
		conMarcador++
		tieneWD := strings.Contains(cuerpo, "\nWorkingDirectory=") || strings.HasPrefix(cuerpo, "WorkingDirectory=")
		// El respaldo del verificador: `ExecStart=<raíz>/deploy/...`, que se recorta en `/deploy/`.
		recuperablePorExec := false
		for _, l := range directivas {
			if strings.HasPrefix(l, "ExecStart=") && strings.Contains(strings.TrimPrefix(l, "ExecStart="), "/deploy/") {
				recuperablePorExec = true
			}
		}
		if !tieneWD {
			sinWorkingDirectory++
		}
		if !tieneWD && !recuperablePorExec {
			t.Errorf("%s usa `@REPO@` en una directiva y el vigía NO puede recuperar la raíz del repo:\n"+
				"  no declara `WorkingDirectory=` y su `ExecStart=` no pasa por `/deploy/`.\n"+
				"  Con eso, `verificar-despliegue.sh` compara una plantilla con `@REPO@` contra una ruta\n"+
				"  absoluta y esta unidad va a decir «difiere» SIEMPRE, aun instalada como corresponde.",
				filepath.Base(ruta))
		}
	}
	if conMarcador < 2 {
		t.Fatalf("sólo %d unidad(es) de deploy/systemd/ usan `@REPO@` en una directiva: esta guarda está midiendo el vacío", conMarcador)
	}
	if sinWorkingDirectory == 0 {
		t.Errorf("las %d unidades con `@REPO@` declaran todas un `WorkingDirectory=`, así que NINGUNA ejercita el\n"+
			"  respaldo por `ExecStart` del verificador. Ese camino existe porque hubo una unidad sin\n"+
			"  `WorkingDirectory` y el informe la daba por divergente para siempre; si ahora no queda ninguna,\n"+
			"  el respaldo es código muerto y el día que se rompa no lo va a notar ninguna corrida.\n"+
			"  Si de verdad ya no hace falta, lo que hay que borrar es el respaldo del verificador, no este control.",
			conMarcador)
	}
}
