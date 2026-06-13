package skills

type Skill struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Triggers     []string `yaml:"triggers"`
	Capabilities []string `yaml:"capabilities"`
	Rules        string   `yaml:"rules"`
}

type Resolver struct {
	skillsDir string
}

func NewResolver(projectPath string) *Resolver {
	return &Resolver{
		skillsDir: projectPath,
	}
}
