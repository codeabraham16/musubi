package config

import "testing"

// TestCrudoDefaults verifica que SIN bloque `crudo:` el respaldo del crudo queda inerte: sin destino,
// sin staging propio, sin secretos — un config legacy (anterior a la feature) hereda exactamente esto,
// y el comando falla-cerrado hasta que se declare un remote. Los defaults EFECTIVOS se resuelven por
// método: scp y 14 días.
func TestCrudoDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Crudo.Remote != "" {
		t.Errorf("esperaba remote vacío por defecto, obtuve %q", cfg.Crudo.Remote)
	}
	if cfg.Crudo.LocalDir != "" {
		t.Errorf("esperaba local_dir vacío por defecto, obtuve %q", cfg.Crudo.LocalDir)
	}
	if cfg.Crudo.AllowLocalOnly {
		t.Errorf("esperaba allow_local_only false por defecto, obtuve true")
	}
	if cfg.Crudo.WaitNetworkSeconds != 0 {
		t.Errorf("esperaba wait_network_seconds 0 por defecto, obtuve %d", cfg.Crudo.WaitNetworkSeconds)
	}
	if cfg.Crudo.EffectiveMethod() != "scp" {
		t.Errorf("esperaba método efectivo 'scp' por defecto, obtuve %q", cfg.Crudo.EffectiveMethod())
	}
	if cfg.Crudo.EffectiveRetentionDays() != 14 {
		t.Errorf("esperaba retención efectiva 14 por defecto, obtuve %d", cfg.Crudo.EffectiveRetentionDays())
	}
}

// TestCrudoRespetaValoresExplicitos verifica que un bloque presente respeta todo lo que se escribe: el
// destino, el método, el staging propio, la retención, el escape-hatch local-only y la espera de red.
func TestCrudoRespetaValoresExplicitos(t *testing.T) {
	root := writeConfig(t, `version: "1.0"
mode: local
crudo:
  remote: musubi@100.79.126.62:/srv/respaldos/crudo
  method: scp
  local_dir: /var/tmp/crudo
  retention_days: 30
  allow_local_only: true
  wait_network_seconds: 60
`)
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Crudo.Remote != "musubi@100.79.126.62:/srv/respaldos/crudo" {
		t.Errorf("remote no respetado, obtuve %q", cfg.Crudo.Remote)
	}
	if cfg.Crudo.Method != "scp" || cfg.Crudo.EffectiveMethod() != "scp" {
		t.Errorf("method no respetado, obtuve %q (efectivo %q)", cfg.Crudo.Method, cfg.Crudo.EffectiveMethod())
	}
	if cfg.Crudo.LocalDir != "/var/tmp/crudo" {
		t.Errorf("local_dir no respetado, obtuve %q", cfg.Crudo.LocalDir)
	}
	if cfg.Crudo.RetentionDays != 30 || cfg.Crudo.EffectiveRetentionDays() != 30 {
		t.Errorf("retention_days no respetado, obtuve %d (efectivo %d)", cfg.Crudo.RetentionDays, cfg.Crudo.EffectiveRetentionDays())
	}
	if !cfg.Crudo.AllowLocalOnly {
		t.Errorf("allow_local_only no respetado, obtuve false")
	}
	if cfg.Crudo.WaitNetworkSeconds != 60 {
		t.Errorf("wait_network_seconds no respetado, obtuve %d", cfg.Crudo.WaitNetworkSeconds)
	}
}

// TestCrudoEffectiveMethod verifica que el método se normaliza (trim + minúsculas) y que el vacío cae a
// "scp". Un método distinto se devuelve tal cual: la guarda de "sólo scp" vive en el comando, no acá.
func TestCrudoEffectiveMethod(t *testing.T) {
	casos := []struct {
		in   string
		want string
	}{
		{"", "scp"},
		{"  SCP  ", "scp"},
		{"Scp", "scp"},
		{"rsync", "rsync"},
	}
	for _, c := range casos {
		got := CrudoConfig{Method: c.in}.EffectiveMethod()
		if got != c.want {
			t.Errorf("EffectiveMethod(%q): esperaba %q, obtuve %q", c.in, c.want, got)
		}
	}
}

// TestCrudoEffectiveRetentionDays verifica el default (0⇒14), el respeto de un valor positivo y que un
// negativo se conserva (purgarViejos lo interpreta como "no purgar nunca"): 0 NO debe significar
// "borrar todo", que sería el lado peligroso.
func TestCrudoEffectiveRetentionDays(t *testing.T) {
	casos := []struct {
		in   int
		want int
	}{
		{0, 14},
		{7, 7},
		{30, 30},
		{-1, -1},
	}
	for _, c := range casos {
		got := CrudoConfig{RetentionDays: c.in}.EffectiveRetentionDays()
		if got != c.want {
			t.Errorf("EffectiveRetentionDays(%d): esperaba %d, obtuve %d", c.in, c.want, got)
		}
	}
}
