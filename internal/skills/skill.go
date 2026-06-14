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
}

type Resolver struct {
	skillsDir string
}

func NewResolver(projectPath string) *Resolver {
	return &Resolver{
		skillsDir: projectPath,
	}
}
