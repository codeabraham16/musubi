package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"musubi/internal/config"
)

// arsenal_bootstrap.go — B2 de la spec «arsenal-arranque»: que un proyecto recién unido al
// cerebro arranque CON el arsenal, no vacío.
//
// El principio del track es la asimetría: **curaduría al subir, adopción en bloque al bajar**.
// Promover es de a una y explícito justamente para que el arsenal se mantenga limpio; si esa
// curaduría es real, todo lo que está adentro ya fue juzgado digno de ser conocimiento de
// empresa, y bajarlo entero no contradice el «nada automático» — lo cobra. Y deja el riesgo
// donde corresponde: subir ensucia el espacio de todos, bajar sólo afecta tu proyecto y se
// deshace borrando un archivo.

// ArsenalReport es el resultado de bajar el arsenal a un proyecto.
type ArsenalReport struct {
	Instaladas []string // escritas en .musubi/skills/
	Salteadas  []string // ya existían en el proyecto; NO se pisan (G10)
	Fallidas   []string // rechazadas (nombre inválido, gate de calidad); se reportan, no se tragan
}

// installArsenal instala en el proyecto del server las skills del arsenal que falten.
//
// Escribe reusando toolInstallSkill —la MISMA puerta que musubi_install_skill— y no una copia.
// El contenido del arsenal es dato remoto, y este camino nuevo es exactamente donde se cuela una
// segunda puerta de escritura con la excusa de que «total ya se valida al instalar» (G12).
//
// Best-effort por skill, igual que el resto de provision: una que falla no aborta a las demás,
// pero queda en Fallidas. Tragarse el rechazo sería peor que fallar entero.
func (s *McpServer) installArsenal(dryRun bool) (ArsenalReport, error) {
	var rep ArsenalReport
	if s.syncClient == nil {
		return rep, fmt.Errorf("no hay cerebro central configurado: definí sync.central_url y sync.auth_token_env")
	}
	remotas, err := s.syncClient.ListArsenal("")
	if err != nil {
		return rep, fmt.Errorf("no se pudo leer el arsenal del central: %w", err)
	}
	locales, err := s.resolver.LoadSkills()
	if err != nil {
		return rep, fmt.Errorf("no se pudieron leer las skills del proyecto: %w", err)
	}
	tengo := make(map[string]bool, len(locales))
	for _, sk := range locales {
		tengo[sk.Name] = true
	}

	for _, sk := range remotas {
		// G10: provision es idempotente y se corre varias veces sobre el mismo proyecto. Pisar
		// una skill que editaste a mano sería una pérdida silenciosa de trabajo, así que acá no
		// hay overwrite: lo que ya está, se saltea.
		if tengo[sk.Name] {
			rep.Salteadas = append(rep.Salteadas, sk.Name)
			continue
		}
		if dryRun {
			rep.Instaladas = append(rep.Instaladas, sk.Name)
			continue
		}
		args, merr := json.Marshal(map[string]any{"name": sk.Name})
		if merr != nil {
			rep.Fallidas = append(rep.Fallidas, sk.Name)
			continue
		}
		if _, rerr := s.toolInstallSkill(args); rerr != nil {
			rep.Fallidas = append(rep.Fallidas, sk.Name)
			continue
		}
		rep.Instaladas = append(rep.Instaladas, sk.Name)
	}
	return rep, nil
}

// InstallArsenalInto baja el arsenal del cerebro central al proyecto projectDir.
//
// Es el punto de entrada de `musubi provision --skills`: arma el cliente de sync desde el config
// del proyecto —el que `ensureSyncConfig` acaba de escribir— y delega en la puerta de siempre.
func InstallArsenalInto(projectDir string, dryRun bool) (ArsenalReport, error) {
	cfg, err := config.Load(projectDir)
	if err != nil {
		return ArsenalReport{}, fmt.Errorf("no se pudo leer la config del proyecto: %w", err)
	}
	if !cfg.Sync.Enabled || strings.TrimSpace(cfg.Sync.CentralURL) == "" {
		return ArsenalReport{}, fmt.Errorf("el sync al cerebro central no está configurado en %s", projectDir)
	}
	client, err := NewSyncClient(cfg.Sync)
	if err != nil {
		return ArsenalReport{}, fmt.Errorf("no se pudo armar el cliente del central: %w", err)
	}
	// engine nil a propósito: instalar una skill escribe un YAML en disco, no toca la base.
	// writeSkillFile ya contempla el caso (sólo actualiza la huella del stack si hay engine).
	s := NewMcpServer(nil, projectDir, nil)
	s.SetSyncClient(client, cfg.Sync)
	return s.installArsenal(dryRun)
}
