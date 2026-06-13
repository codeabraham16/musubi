package telemetry

import (
	"bytes"
	"fmt"
	"musubi/internal/memory"
	"os/exec"
)

type Runner struct {
	engine *memory.DbEngine
}

func NewRunner(engine *memory.DbEngine) *Runner {
	return &Runner{engine: engine}
}

// RunValidation ejecuta un comando de validación (linter/compilador) y registra fallas en la BD de telemetría.
func (r *Runner) RunValidation(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outputStr := stdout.String() + "\n" + stderr.String()

	if err != nil {
		// Comando falló, registramos en telemetría
		// En un entorno de producción, intentaríamos parsear el archivo específico que falló
		// Por ahora guardamos el log general en telemetría
		saveErr := r.engine.SaveTelemetryLog("cmd:"+name, outputStr, "")
		if saveErr != nil {
			return outputStr, fmt.Errorf("el comando falló (%v) y no se pudo guardar en telemetría: %w", err, saveErr)
		}
		return outputStr, fmt.Errorf("el comando falló: %w", err)
	}

	return outputStr, nil
}
