package config

import "testing"

// TestPathConstantes verifica que las constantes de rutas del workspace de Musubi
// tengan los valores esperados por la convención del proyecto.
func TestPathConstantes(t *testing.T) {
	casos := []struct {
		nombre   string
		valor    string
		esperado string
	}{
		{"DirName", DirName, ".musubi"},
		{"SkillsDir", SkillsDir, "skills"},
		{"DBFile", DBFile, "memory.db"},
		{"ConfigFile", ConfigFile, "config.yaml"},
		{"SentinelFile", SentinelFile, ".skills-generated"},
		{"ClaudeDir", ClaudeDir, ".claude"},
		{"ClaudeSettingsFile", ClaudeSettingsFile, "settings.json"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if c.valor != c.esperado {
				t.Errorf("constante %s: esperaba %q, obtuve %q", c.nombre, c.esperado, c.valor)
			}
		})
	}
}
