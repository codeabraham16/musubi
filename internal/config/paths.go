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
)
