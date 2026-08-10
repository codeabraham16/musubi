package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"musubi/internal/config"
	"musubi/internal/logx"

	"gopkg.in/yaml.v3"
)

// LoadSkills busca y parsea todos los archivos de skills del directorio .musubi/skills/
func (r *Resolver) LoadSkills() ([]Skill, error) {
	skillsDir := filepath.Join(r.skillsDir, config.DirName, config.SkillsDir)

	// Si no existe el directorio, retornar slice vacío (no nil) sin error para resiliencia
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return []Skill{}, nil
	}

	files, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("error al leer directorio de skills: %w", err)
	}

	var loaded []Skill
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(skillsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			logx.Warn("skill omitida: no se pudo leer el archivo", "path", path, "error", err)
			continue
		}

		var skill Skill
		if err := yaml.Unmarshal(data, &skill); err != nil {
			logx.Warn("skill omitida: YAML inválido", "path", path, "error", err)
			continue
		}
		if skill.Name == "" {
			logx.Warn("skill omitida: campo 'name' vacío", "path", path)
			continue
		}

		loaded = append(loaded, skill)
	}

	return loaded, nil
}

// ResolveSkills evalúa qué skills corresponden a la situación declarada y tienen capabilities
// instaladas.
//
// DOS CAMINOS INDEPENDIENTES, Y ES ADITIVO: una skill entra si matchea por ARCHIVO (sus globs) o
// por ALCANCE DECLARADO (su AppliesTo contra la fase/tarea del request). Cualquiera de los dos
// alcanza.
//
// Que sea un OR y no un AND es deliberado y es lo que evita una regresión: las skills que hoy
// declaran `triggers: ['*']` y reciben un AppliesTo seguirían activándose por archivo mientras
// ningún llamador declare todavía su fase. Primero existe el canal, después se aprieta.
func (r *Resolver) ResolveSkills(req ResolveRequest) ([]Skill, error) {
	resueltas, err := r.ResolveConDetalle(req)
	if err != nil {
		return nil, err
	}
	var active []Skill
	for _, s := range resueltas {
		active = append(active, s.Skill)
	}
	return active, nil
}

// ResolveConDetalle resuelve lo mismo que ResolveSkills pero diciendo POR QUÉ entró cada skill.
//
// Existe porque el ahorro de tokens necesita esa razón: el cuerpo de una skill viaja sólo cuando hay
// evidencia de que aplica (ver niveles.go). El CONJUNTO de skills activas es exactamente el mismo —
// acá no se filtra nada nuevo—; lo único que se agrega es la clasificación.
func (r *Resolver) ResolveConDetalle(req ResolveRequest) ([]SkillResuelta, error) {
	allSkills, err := r.LoadSkills()
	if err != nil {
		return nil, err
	}

	var resueltas []SkillResuelta
	for _, skill := range allSkills {
		if !r.verifyCapabilities(skill) {
			continue
		}
		if como, ok := clasificarMatcheo(skill, req); ok {
			resueltas = append(resueltas, SkillResuelta{Skill: skill, Matcheo: como})
		}
	}

	return resueltas, nil
}

// matchAlcance compara el AppliesTo de la skill contra lo que el llamador DECLARÓ.
//
// Igualdad exacta de strings: sin prefijos, sin heurística, sin distancia de edición. Es lo que
// mantiene el matcher determinista y sin costo — y lo que hace que el vocabulario cerrado sirva de
// algo, porque un typo no puede colarse pareciéndose a un valor bueno.
func matchAlcance(skill Skill, req ResolveRequest) bool {
	if len(skill.AppliesTo) == 0 {
		return false // una skill que no declara alcance nunca matchea por declaración
	}
	for _, a := range skill.AppliesTo {
		if req.Phase != "" && a == req.Phase {
			return true
		}
		if req.Task != "" && a == req.Task {
			return true
		}
	}
	return false
}

// MatchGlob indica si file coincide con glob por nombre base o ruta completa.
// Soporta los patrones de path.Match (*, ?, rangos de caracteres). Normaliza los
// separadores ('\' -> '/') de forma determinista para que un trigger estilo ruta
// con '/' matchee en Windows (donde WalkDir entrega paths con '\').
// Un patrón inválido devuelve false sin hacer panic.
func MatchGlob(glob, file string) bool {
	glob = strings.ReplaceAll(glob, "\\", "/")
	file = strings.ReplaceAll(file, "\\", "/")
	base := path.Base(file)
	mb, _ := path.Match(glob, base)
	mp, _ := path.Match(glob, file)
	return mb || mp
}

// matchTriggers comprueba si alguno de los archivos coincide con los globs declarados en la skill.
//
// Delega en clasificarMatcheo —que distingue glob real de comodín— y descarta la distinción: acá la
// pregunta es sólo «¿matcheó por archivo?». Sin `Phase` ni `Task`, la rama de alcance no puede
// dispararse, así que el resultado es exactamente el de antes. Se escribe así para que exista UNA
// sola implementación del recorrido de triggers: dos que hagan lo mismo se desincronizan.
func (r *Resolver) matchTriggers(skill Skill, files []string) bool {
	_, ok := clasificarMatcheo(skill, ResolveRequest{ModifiedFiles: files})
	return ok
}

// verifyCapabilities valida que las herramientas necesarias (como compiladores o linters) existan en el PATH.
func (r *Resolver) verifyCapabilities(skill Skill) bool {
	for _, cap := range skill.Capabilities {
		if _, err := exec.LookPath(cap); err != nil {
			// Si falla en encontrar el comando en PATH, la skill no se activa
			return false
		}
	}
	return true
}
