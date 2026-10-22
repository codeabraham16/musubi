package mcp

import (
	"context"
	"time"

	"musubi/internal/config"
	"musubi/internal/logx"
	"musubi/internal/repovivo"
)

// RunRepoVivo corre la ZONA EN VIVO del checkout de este daemon (ver internal/repovivo): sube
// fotos del working tree al remoto empresarial, solas y cada pocos segundos, SIN tocar el índice,
// el HEAD ni las ramas del usuario. Corre como goroutine del daemon, con el mismo ciclo de vida
// que los schedulers de sync. Sale cuando ctx muere o cuando la zona en vivo no debe encender
// para este checkout (off, sin remoto empresarial, o el remoto apunta a github.com).
//
// projectPath es la raíz del proyecto del daemon (== MUSUBI_HOME); repovivo.Arrancar resuelve a
// partir de ahí el toplevel real del working tree, la identidad de la ref y el remoto.
func (s *McpServer) RunRepoVivo(ctx context.Context, cfg config.RepoVivoConfig) {
	// La zona en vivo NO depende del sync de MEMORIA: su encendido lo gobiernan SÓLO su propia config
	// (Modo), el team_mode del proyecto y las guardas de repovivo. En particular, acá NO se lee
	// s.syncCfg: atar el arranque a Sync.Enabled reintroduciría el bug de cableado por el que una foto
	// dejaría de subir con el sync apagado (enabled=on debe subir fotos aunque el drain esté off).
	enc, razon, err := repovivo.Arrancar(ctx, opcionesDeConfig(s.projectPath, s.memory.TeamMode, cfg))
	if err != nil {
		logx.Error("repo-vivo: no arrancó", "razon", razon, "error", err)
		return
	}
	if !enc {
		// Apagado a propósito (lo más común: un proyecto que no es empresarial). Una línea, no ruido.
		logx.Info("repo-vivo: apagado para este checkout", "razon", razon)
	}
}

// opcionesDeConfig arma las Opciones de la zona en vivo a partir de la config del proyecto (cfg), el
// projectPath del daemon y el team_mode de la memoria. Está SEPARADA de RunRepoVivo para poder fijar
// por prueba que CADA campo se cablea de verdad —un Modo "on" hardcodeado, un TeamMode siempre false o
// un SecretosExtra nil reintroducirían en silencio el bug de cableado y sobrevivirían con la suite
// verde si el mapeo viviera inline—. Es una función pura: no toca git ni red.
func opcionesDeConfig(projectPath string, teamMode bool, cfg config.RepoVivoConfig) repovivo.Opciones {
	return repovivo.Opciones{
		Dir:           projectPath,
		Remoto:        cfg.Remote,
		Modo:          cfg.Enabled,
		TeamMode:      teamMode,
		Intervalo:     time.Duration(cfg.IntervaloSegundos) * time.Second,
		SecretosExtra: cfg.SecretosExtra,
		Nodo:          cfg.Nodo,
	}
}
