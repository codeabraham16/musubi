// Package config centraliza las rutas y la configuración del workspace de Musubi.
package config

const (
	// DirName es el nombre de la carpeta de estado de Musubi dentro de un proyecto.
	DirName = ".musubi"

	// SkillsDir es el subdirectorio (dentro de DirName) donde viven las skills YAML.
	SkillsDir = "skills"

	// DBFile es el nombre del archivo SQLite de memoria persistente.
	DBFile = "memory.db"

	// ConfigFile es el nombre del archivo de configuración del workspace.
	ConfigFile = "config.yaml"

	// SentinelFile marca que las skills ya fueron auto-generadas para este proyecto.
	// Vive en DirName/SkillsDir. Borrarlo manualmente permite re-generar.
	SentinelFile = ".skills-generated"

	// ClaudeDir es la carpeta de configuración de Claude Code en el proyecto.
	ClaudeDir = ".claude"

	// ClaudeSettingsFile es el archivo de settings (hooks) de Claude Code.
	ClaudeSettingsFile = "settings.json"
)
