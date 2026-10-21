package config

import "testing"

// TestRepoVivoDefaults verifica que SIN bloque `repo_vivo:` la zona en vivo queda con sus
// defaults: enabled "auto" (opt-in por defecto), remote "origin", intervalo 5 s y sin secretos
// extra. Un config legacy (anterior a la feature) hereda exactamente esto.
func TestRepoVivoDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.RepoVivo.Enabled != "auto" {
		t.Errorf("esperaba enabled 'auto' por defecto, obtuve %q", cfg.RepoVivo.Enabled)
	}
	if cfg.RepoVivo.Remote != "origin" {
		t.Errorf("esperaba remote 'origin' por defecto, obtuve %q", cfg.RepoVivo.Remote)
	}
	if cfg.RepoVivo.IntervaloSegundos != 5 {
		t.Errorf("esperaba intervalo_segundos 5 por defecto, obtuve %d", cfg.RepoVivo.IntervaloSegundos)
	}
	if len(cfg.RepoVivo.SecretosExtra) != 0 {
		t.Errorf("esperaba secretos_extra vacío por defecto, obtuve %v", cfg.RepoVivo.SecretosExtra)
	}
}

// TestRepoVivoRespetaValoresExplicitos verifica que un bloque presente respeta todo lo que se
// escribe: el apagado explícito, un remoto distinto, un intervalo propio y la lista de secretos.
func TestRepoVivoRespetaValoresExplicitos(t *testing.T) {
	root := writeConfig(t, `version: "1.0"
mode: local
repo_vivo:
  enabled: "off"
  remote: forgejo
  intervalo_segundos: 10
  secretos_extra:
    - "*.pfx"
    - secretos/
`)
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.RepoVivo.Enabled != "off" {
		t.Errorf("esperaba enabled 'off' respetado, obtuve %q", cfg.RepoVivo.Enabled)
	}
	if cfg.RepoVivo.Remote != "forgejo" {
		t.Errorf("esperaba remote 'forgejo' respetado, obtuve %q", cfg.RepoVivo.Remote)
	}
	if cfg.RepoVivo.IntervaloSegundos != 10 {
		t.Errorf("esperaba intervalo_segundos 10 respetado, obtuve %d", cfg.RepoVivo.IntervaloSegundos)
	}
	if len(cfg.RepoVivo.SecretosExtra) != 2 || cfg.RepoVivo.SecretosExtra[0] != "*.pfx" || cfg.RepoVivo.SecretosExtra[1] != "secretos/" {
		t.Errorf("secretos_extra no respetado, obtuve %v", cfg.RepoVivo.SecretosExtra)
	}
}

// TestRepoVivoEnabledNormaliza verifica que enabled se acota a {auto,on,off}: espacios/mayúsculas
// se normalizan, las grafías booleanas (false/no/0/disabled => off; true/yes/1 => on) se reconocen
// para que `enabled: false` APAGUE, y un valor inválido o vacío degrada a "auto" (nunca por un typo).
func TestRepoVivoEnabledNormaliza(t *testing.T) {
	casos := []struct {
		yaml string
		want string
	}{
		{"repo_vivo:\n  enabled: \"  ON \"\n", "on"},
		{"repo_vivo:\n  enabled: OFF\n", "off"},
		{"repo_vivo:\n  enabled: bogus\n", "auto"},
		{"repo_vivo:\n  enabled: \"\"\n", "auto"},
		{"repo_vivo:\n  enabled: false\n", "off"},
		{"repo_vivo:\n  enabled: \"false\"\n", "off"},
		{"repo_vivo:\n  enabled: no\n", "off"},
		{"repo_vivo:\n  enabled: 0\n", "off"},
		{"repo_vivo:\n  enabled: disabled\n", "off"},
		{"repo_vivo:\n  enabled: true\n", "on"},
		{"repo_vivo:\n  enabled: yes\n", "on"},
		{"repo_vivo:\n  enabled: 1\n", "on"},
	}
	for _, c := range casos {
		root := writeConfig(t, "version: \"1.0\"\nmode: local\n"+c.yaml)
		cfg, err := Load(root)
		if err != nil {
			t.Fatalf("Load error (%q): %v", c.yaml, err)
		}
		if cfg.RepoVivo.Enabled != c.want {
			t.Errorf("enabled %q: esperaba %q, obtuve %q", c.yaml, c.want, cfg.RepoVivo.Enabled)
		}
	}
}

// TestRepoVivoRellenaAusentes verifica que un bloque presente pero incompleto rellena remote e
// intervalo con sus defaults, y que un intervalo sin sentido (0 o negativo) también cae al default
// en vez de dejar al observador en un busy-loop. El apagado NO se hace con el intervalo: es enabled.
func TestRepoVivoRellenaAusentes(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nrepo_vivo:\n  enabled: on\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.RepoVivo.Enabled != "on" {
		t.Errorf("esperaba enabled 'on' respetado, obtuve %q", cfg.RepoVivo.Enabled)
	}
	if cfg.RepoVivo.Remote != "origin" {
		t.Errorf("remote ausente debía ser 'origin', obtuve %q", cfg.RepoVivo.Remote)
	}
	if cfg.RepoVivo.IntervaloSegundos != 5 {
		t.Errorf("intervalo ausente debía ser 5, obtuve %d", cfg.RepoVivo.IntervaloSegundos)
	}

	root2 := writeConfig(t, "version: \"1.0\"\nmode: local\nrepo_vivo:\n  intervalo_segundos: -3\n")
	cfg2, err := Load(root2)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg2.RepoVivo.IntervaloSegundos != 5 {
		t.Errorf("intervalo negativo debía caer al default 5, obtuve %d", cfg2.RepoVivo.IntervaloSegundos)
	}
}
