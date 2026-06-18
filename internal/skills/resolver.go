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

// ResolveSkills evalúa qué skills corresponden a los archivos modificados y tienen capabilities instaladas.
func (r *Resolver) ResolveSkills(modifiedFiles []string) ([]Skill, error) {
	allSkills, err := r.LoadSkills()
	if err != nil {
		return nil, err
	}

	var active []Skill
	for _, skill := range allSkills {
		if r.matchTriggers(skill, modifiedFiles) && r.verifyCapabilities(skill) {
			active = append(active, skill)
		}
	}

	return active, nil
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
func (r *Resolver) matchTriggers(skill Skill, files []string) bool {
	for _, file := range files {
		for _, trigger := range skill.Triggers {
			// Delega en MatchGlob para mantener una única implementación de glob.
			if MatchGlob(trigger, file) {
				return true
			}
		}
	}
	return false
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
