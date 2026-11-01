package memory

import "musubi/internal/codeintel"

// codepath.go: la memoria de código (tools MCP) y el hook PreToolUse (precheck) referencian estas
// dos funciones como memory.NormalizeCodePath / memory.FileFingerprint, de modo que lo que guarda
// una tool sea encontrable por el hook. La IMPLEMENTACIÓN se mudó a internal/codeintel —la comparte
// internal/specgraph, que es FS-nativo y no puede depender del motor de memoria—; acá quedan
// reenvíos finos para no tocar los más de veinte call-sites ni el borde exportado del paquete.

// NormalizeCodePath: ver codeintel.NormalizeCodePath.
func NormalizeCodePath(root, path string) string { return codeintel.NormalizeCodePath(root, path) }

// FileFingerprint: ver codeintel.FileFingerprint.
func FileFingerprint(root, path string) (string, error) { return codeintel.FileFingerprint(root, path) }
