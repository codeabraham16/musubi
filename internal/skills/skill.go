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
}

type Resolver struct {
	skillsDir string
}

func NewResolver(projectPath string) *Resolver {
	return &Resolver{
		skillsDir: projectPath,
	}
}
