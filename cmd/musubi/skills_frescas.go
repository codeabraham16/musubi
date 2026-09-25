package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/fleet"
	"musubi/internal/selfupdate"
	"musubi/internal/skills"
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
// La lógica para refrescar SIN PISAR lo editado a mano ya existía y era buena: escribirCognitivas sólo
// reescribe lo que Musubi escribió y nadie tocó (ManagedChecksum), y exportarSkillsAlAgente preserva
// los SKILL.md editados. Lo que faltaba era el disparador; esto lo cuelga del arranque de cada sesión.
//
// Lo que una revisión adversarial le corrigió a la primera versión, antes de publicarla:
//   - La huella vivía en un archivo `.musubi/skills-huella` que git no ignoraba: cada build del repo
//     salía `-sucio`. Ahora vive en la meta de la base, que está ignorada en todo repo.
//   - La huella miraba QUIÉN escribiría (versión + commit del binario), no QUÉ: dos builds sucios del
//     mismo commit, con otro texto de manual, daban la misma huella y el manual no se refrescaba —el
//     síntoma original, en la máquina donde más se compila—. Ahora mide el contenido que se
//     escribiría con el stack de hoy, que además cubre un cambio de stack.
//   - Un manual borrado a mano resucitaba en el arranque siguiente; un binario viejo degradaba lo que
//     había escrito uno nuevo; y la huella se tomaba después del export, así que una skill que llegaba
//     en esa ventana quedaba marcada como procesada sin exportarse.

// Claves de meta del refresco automático.
const (
	// metaHuellaSkills: con qué huella se refrescaron por última vez los manuales.
	metaHuellaSkills = "skills_huella"
	// metaSkillsEscritasPor: la versión del binario que los refrescó por última vez.
	metaSkillsEscritasPor = "skills_escritas_por"
	// metaSkillsCognitivasConocidas: qué manuales cognitivos escribió Musubi alguna vez en este proyecto.
	metaSkillsCognitivasConocidas = "skills_cognitivas_conocidas"
)

// almacenDeManuales es lo que el refresco necesita de la base: recordar su huella y su historia.
type almacenDeManuales interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
}

// huellaSkills resume lo que, si cambia, obliga a volver a escribir o exportar los manuales:
//
//  1. el CONTENIDO que el binario escribiría para cada manual cognitivo con el stack de hoy (su
//     checksum), y no la identidad del binario: así un build distinto con los mismos manuales no
//     reescribe nada, y uno con otro texto sí, aunque tenga la misma versión y el mismo commit;
//  2. la plantilla del SKILL.md que se exporta al agente, que puede cambiar sin que cambie un manual;
//  3. lo que hay hoy en .musubi/skills/ (nombre, tamaño, fecha, siguiendo symlinks), para exportar
//     también las skills que llegan por otro camino —el arsenal que baja del central—.
func huellaSkills(root string, stack []detector.StackResult) string {
	h := sha256.New()
	for _, sk := range cognitiveSkills(stack) {
		sum, err := skillContentChecksum(sk)
		fmt.Fprintf(h, "cognitiva:%s|%s|%t\n", sk.Name, sum, err != nil)
	}
	fmt.Fprintf(h, "plantilla:%s\n", huellaDeLaPlantilla())
	dir := filepath.Join(root, config.DirName, config.SkillsDir)
	entradas, _ := os.ReadDir(dir) // ordenado por nombre
	for _, e := range entradas {
		info, err := os.Stat(filepath.Join(dir, e.Name())) // Stat y no e.Info(): sigue un symlink
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s|%d|%d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// huellaDeLaPlantilla resume cómo el binario escribe un SKILL.md, renderizando una skill fija.
func huellaDeLaPlantilla() string {
	sum, err := skills.ChecksumSkillMD(skills.Skill{Name: "plantilla", Description: "d", Triggers: []string{"*"}, Rules: "r\n"})
	if err != nil {
		return "sin-plantilla"
	}
	return sum
}

// escritaPorUnoMasNuevo dice si la versión registrada es de un release POSTERIOR al de este binario.
// Si alguna de las dos no tiene núcleo MAJOR.MINOR.PATCH (`dev`, un build sin ldflags), no se compara:
// inventar un orden sería peor que no tenerlo.
func escritaPorUnoMasNuevo(registrada, actual string) bool {
	nr, ok1 := fleet.NucleoDeVersion(registrada)
	na, ok2 := fleet.NucleoDeVersion(actual)
	return ok1 && ok2 && selfupdate.NeedsUpdate(na, nr)
}

// refrescarSkillsSiHaceFalta vuelve a escribir los manuales del proyecto si la huella cambió desde la
// última vez. Devuelve cuáles se escribieron (vacío si no hizo falta).
//
// Las decisiones que importan:
//   - Sólo en proyectos que YA tienen .musubi/skills/: el hook de arranque puede correr en un repo que
//     nunca eligió tener manuales de Musubi, y crearlos ahí sería imponerlos sin permiso. Y sólo con
//     base: sin ella no hay dónde recordar la huella, y refrescar a ciegas en cada arranque no.
//   - Un binario MÁS VIEJO que el que los escribió no los toca: sin esto, con dos versiones
//     conviviendo sobre el mismo proyecto, cada una reescribía lo de la otra en cada arranque.
//   - NO RECREA lo que el usuario borró: un manual cognitivo que Musubi ya escribió alguna vez y ya
//     no está, fue sacado a propósito. Uno que el binario trae por primera vez, sí se crea.
//   - La huella se toma ENTRE la escritura y el export: ya incluye lo que escribió el refresco, y no
//     lo que pueda llegar durante el export —eso cambia la huella y se exporta en la sesión siguiente—.
//     Se guarda sólo si todo salió bien, para que un fallo a mitad se reintente.
//   - Los errores se devuelven, no se imprimen: quien llama es el hook de arranque y decide él.
func refrescarSkillsSiHaceFalta(root string, store almacenDeManuales, stack []detector.StackResult) ([]string, error) {
	dirSkills := filepath.Join(root, config.DirName, config.SkillsDir)
	if st, err := os.Stat(dirSkills); err != nil || !st.IsDir() || store == nil {
		return nil, nil
	}
	if registrada, ok, _ := store.GetMeta(metaSkillsEscritasPor); ok && escritaPorUnoMasNuevo(registrada, version) {
		return nil, nil
	}
	if previa, ok, _ := store.GetMeta(metaHuellaSkills); ok && previa == huellaSkills(root, stack) {
		return nil, nil
	}

	conocidas := leerCognitivasConocidas(store)
	escritas, err := escribirCognitivas(root, stack, func(nombre string) bool { return !conocidas[nombre] })
	if err != nil {
		return escritas, fmt.Errorf("refrescar las skills de .musubi/skills: %w", err)
	}
	for _, sk := range cognitiveSkills(stack) {
		if _, err := os.Stat(filepath.Join(dirSkills, sk.Name+".yaml")); err == nil {
			conocidas[sk.Name] = true
		}
	}
	huella := huellaSkills(root, stack)

	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		return escritas, fmt.Errorf("exportar las skills al agente: %w", err)
	}
	escritas = append(escritas, rep.Escritas...)

	guardarCognitivasConocidas(store, conocidas)
	_ = store.SetMeta(metaSkillsEscritasPor, version)
	if err := store.SetMeta(metaHuellaSkills, huella); err != nil {
		return escritas, fmt.Errorf("guardar la huella de las skills: %w", err)
	}
	return escritas, nil
}

func leerCognitivasConocidas(store almacenDeManuales) map[string]bool {
	out := map[string]bool{}
	if raw, ok, _ := store.GetMeta(metaSkillsCognitivasConocidas); ok && raw != "" {
		var nombres []string
		if json.Unmarshal([]byte(raw), &nombres) == nil {
			for _, n := range nombres {
				out[n] = true
			}
		}
	}
	return out
}

func guardarCognitivasConocidas(store almacenDeManuales, conocidas map[string]bool) {
	nombres := make([]string, 0, len(conocidas))
	for n := range conocidas {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	if b, err := json.Marshal(nombres); err == nil {
		_ = store.SetMeta(metaSkillsCognitivasConocidas, string(b))
	}
}
