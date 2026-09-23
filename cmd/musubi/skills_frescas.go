package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"musubi/internal/config"
)

// skills_frescas.go mantiene al día, sin que nadie lo pida, los manuales que Musubi le deja al agente.
//
// EL DEFECTO QUE ESTO ARREGLA (medido 2026-09-23). Los manuales de .musubi/skills/ y su copia en
// .claude/skills/ los escribía SÓLO `musubi setup`. Once de los doce de este repo estaban fechados el
// 2026-08-10, 44 días atrás, y nada los refrescaba: ni un hook, ni un timer, ni la instalación. Un
// binario nuevo con manuales corregidos dejaba los viejos en disco, y como .claude/ está en el
// .gitignore tampoco llegaban por un pull. El dueño lo anticipó con una pregunta —«¿no crees que en
// algún momento se quede atrás?»— y ya había pasado.
//
// La lógica para refrescar SIN PISAR lo editado a mano ya existía y era buena: writeCognitiveSkills
// sólo reescribe lo que Musubi escribió y nadie tocó (ManagedChecksum), y exportarSkillsAlAgente
// preserva los SKILL.md editados. Lo único que faltaba era el disparador; esto lo cuelga del arranque
// de cada sesión.

// archivoHuellaSkills guarda con qué huella se refrescaron por última vez los manuales del proyecto.
// Vive FUERA de .musubi/skills/ a propósito: adentro formaría parte de lo que la huella mide.
const archivoHuellaSkills = "skills-huella"

// huellaSkills resume lo que, si cambia, obliga a volver a escribir los manuales: QUÉ BINARIO los
// escribiría y QUÉ HAY hoy en .musubi/skills/.
//
// El binario se identifica por versión + commit + árbol sucio, y no por la versión sola: en un build
// local `version` vale "dev" siempre, así que una huella hecha sólo con ella no cambiaría nunca entre
// dos compilaciones y el refresco no se dispararía jamás en la máquina donde más se compila.
//
// El listado de .musubi/skills/ (nombre, tamaño, fecha) entra para que también se exporten las skills
// que llegan por otro camino —el arsenal que baja del central— y no sólo las que trae el binario.
//
// Es barata a propósito: corre en CADA arranque de sesión. Medido en este repo, lo caro —detectar el
// stack para saber qué manuales escribir— cuesta ~95 ms; esto no lo paga salvo que la huella cambie.
func huellaSkills(root string) string {
	h := sha256.New()
	fmt.Fprintf(h, "binario:%s|%s\n", version, identidadDelBuild())
	entradas, _ := os.ReadDir(filepath.Join(root, config.DirName, config.SkillsDir)) // ordenado por nombre
	for _, e := range entradas {
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s|%d|%d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// identidadDelBuild devuelve el commit y si el árbol estaba sucio al compilar, tal como los graba el
// toolchain. Vacío si el binario no los tiene (`go run`, o compilado fuera de un repo git): en ese
// caso la huella sólo depende de `version` y del listado, que es lo que había antes.
//
// Es una variable y no una función para que una prueba pueda simular «otro commit»: el commit es la
// razón de no usar la versión sola, y sin poder moverlo no habría forma de probar que la huella lo mira.
var identidadDelBuild = identidadDelBuildReal

func identidadDelBuildReal() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, sucio string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			sucio = s.Value
		}
	}
	return rev + "|" + sucio
}

// refrescarSkillsSiHaceFalta vuelve a escribir los manuales del proyecto si la huella cambió desde la
// última vez. Devuelve cuáles se reescribieron (vacío si no hizo falta).
//
// Tres decisiones que importan:
//   - Sólo en proyectos que YA tienen .musubi/skills/. El hook de arranque puede correr en un repo que
//     nunca eligió tener manuales de Musubi, y crearlos ahí sería imponerlos sin permiso.
//   - La huella se guarda DESPUÉS de escribir y SÓLO si todo salió bien. Después, porque escribir
//     cambia las fechas que la huella mide; si se guardara antes, el arranque siguiente vería una huella
//     distinta y volvería a escribir todo. Sólo si salió bien, para que un fallo a mitad se reintente
//     en la sesión siguiente en vez de quedar marcado como hecho.
//   - Los errores se devuelven, no se imprimen: quien llama es el hook de arranque y decide él.
func refrescarSkillsSiHaceFalta(root string) ([]string, error) {
	dirSkills := filepath.Join(root, config.DirName, config.SkillsDir)
	if st, err := os.Stat(dirSkills); err != nil || !st.IsDir() {
		return nil, nil
	}
	rutaHuella := filepath.Join(root, config.DirName, archivoHuellaSkills)
	if previa, err := os.ReadFile(rutaHuella); err == nil && string(bytes.TrimSpace(previa)) == huellaSkills(root) {
		return nil, nil
	}

	refrescadas, err := writeCognitiveSkills(root)
	if err != nil {
		return refrescadas, fmt.Errorf("refrescar las skills de .musubi/skills: %w", err)
	}
	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		return refrescadas, fmt.Errorf("exportar las skills al agente: %w", err)
	}
	refrescadas = append(refrescadas, rep.Escritas...)

	if err := escribirArchivoAtomico(rutaHuella, []byte(huellaSkills(root)+"\n"), 0o644); err != nil {
		return refrescadas, fmt.Errorf("guardar la huella de las skills: %w", err)
	}
	return refrescadas, nil
}
