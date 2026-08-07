// Package skills carga y resuelve las skills (reglas de comportamiento en YAML)
// que Musubi inyecta según el contexto del proyecto.
package skills

// Skill representa una regla de comportamiento cargada desde un archivo YAML.
type Skill struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Triggers     []string `yaml:"triggers"`
	Capabilities []string `yaml:"capabilities"`
	Rules        string   `yaml:"rules"`
	// GeneratedBy indica el origen de la skill; "auto-discovery" para skills generadas automáticamente.
	// omitempty mantiene el YAML limpio para skills escritas a mano.
	GeneratedBy string `yaml:"generated_by,omitempty"`
	// GeneratedAt es la marca de tiempo RFC3339 en que se generó la skill; vacío para skills a mano.
	GeneratedAt string `yaml:"generated_at,omitempty"`
	// Source identifica el catálogo o repositorio de origen de la skill (p.ej. "musubi-catalog-v1").
	// omitempty mantiene el YAML limpio para skills escritas a mano sin procedencia de catálogo.
	Source string `yaml:"source,omitempty"`
	// SourceURL es la URL al archivo de reglas completo en el catálogo de origen.
	// omitempty mantiene el YAML limpio cuando el campo está vacío.
	SourceURL string `yaml:"source_url,omitempty"`
	// AlwaysBecause declara POR QUÉ esta skill se activa siempre (trigger "*").
	//
	// Existe porque el vocabulario de triggers sólo sabe de globs de ARCHIVO, y hay skills que no
	// se activan por archivo sino por TIPO DE TAREA: orquestar, planificar, auditar. Ésas no tienen
	// cómo expresarse salvo con "*". Medido sobre las 12 skills de este repo, 7 usan "*" y ninguna
	// por descuido. Sin este campo, un "*" deliberado y uno por pereza son indistinguibles —y en un
	// arsenal compartido esa diferencia decide si el contexto de otro se llena de reglas que no
	// aplican. omitempty: una skill con triggers acotados no lo necesita.
	AlwaysBecause string `yaml:"always_because,omitempty"`
	// ManagedChecksum es el sha256 (hex) del contenido canónico de una skill COGNITIVA
	// manejada por Musubi (writeCognitiveSkills), con este mismo campo vacío. Es la prueba de
	// si el archivo sigue tal como Musubi lo escribió: si el checksum del archivo coincide, la
	// skill está intacta y una actualización del binario la refresca; si no coincide (o no
	// existe), fue editada a mano y se preserva. Vacío/omitempty para skills a mano o de
	// auto-discovery (no las gestiona Musubi).
	ManagedChecksum string `yaml:"managed_checksum,omitempty"`
}

type Resolver struct {
	skillsDir string
}

func NewResolver(projectPath string) *Resolver {
	return &Resolver{
		skillsDir: projectPath,
	}
}
