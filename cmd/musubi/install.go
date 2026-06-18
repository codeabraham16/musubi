package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// noArgs maneja la invocación sin comando. Si se ejecuta en una consola
// interactiva en Windows (típico del doble clic), muestra el menú de instalación;
// si no (lanzado por Claude/otro proceso, o por pipe), imprime la ayuda y sale.
func noArgs() {
	if runtime.GOOS == "windows" && isInteractive() {
		runInteractiveMenu()
		return
	}
	printUsage()
	os.Exit(1)
}

// isInteractive devuelve true si stdin es una consola (no un pipe/redirección).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// menuAction parsea la elección del menú interactivo (case-insensitive).
func menuAction(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "L", "LOCAL":
		return "local"
	case "G", "GLOBAL":
		return "global"
	default:
		return "quit"
	}
}

// runInteractiveMenu muestra el menú de instalación por doble clic y ejecuta la
// opción elegida. Mantiene la ventana abierta hasta que el usuario presiona Enter.
func runInteractiveMenu() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("============================================")
	fmt.Println("                 Musubi")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Println("  Donde queres instalar Musubi?")
	fmt.Println()
	fmt.Println("    [L] Solo esta carpeta (local, NO toca la PC)")
	fmt.Println("    [G] Global en la PC (PATH del usuario, sin admin)")
	fmt.Println("    [Q] Salir")
	fmt.Println()
	fmt.Print("  Eleccion (L/G/Q): ")
	line, _ := reader.ReadString('\n')
	fmt.Println()

	switch menuAction(line) {
	case "local":
		setupProject()
	case "global":
		installGlobalWindows()
	default:
		fmt.Println("Cancelado. No se instalo nada.")
	}

	fmt.Print("\nPresiona Enter para salir...")
	reader.ReadString('\n')
}

// installGlobalWindows copia el binario en ejecución a una carpeta del usuario,
// la agrega al PATH (sin admin) y corre el setup del proyecto actual apuntando a
// esa ubicación estable.
func installGlobalWindows() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("Error al ubicar el ejecutable: %v\n", err)
		return
	}
	installDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "musubi")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Printf("Error al crear %s: %v\n", installDir, err)
		return
	}
	dest := filepath.Join(installDir, "musubi.exe")
	if !sameFile(exe, dest) {
		if err := copyFile(exe, dest); err != nil {
			fmt.Printf("Error al copiar el binario: %v\n", err)
			return
		}
	}
	fmt.Printf("  ✓ Binario instalado en %s\n", dest)

	// Agregar installDir al PATH del usuario (sin admin) vía PowerShell.
	psDir := "'" + strings.ReplaceAll(installDir, "'", "''") + "'"
	ps := fmt.Sprintf(`$d=%s; $p=[Environment]::GetEnvironmentVariable('Path','User'); if ($p -notlike "*$d*") { [Environment]::SetEnvironmentVariable('Path', "$p;$d", 'User'); 'PATH actualizado' } else { 'PATH ya incluia el directorio' }`, psDir)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		fmt.Printf("  ! No se pudo actualizar el PATH automaticamente: %v\n", err)
		fmt.Printf("    Agregalo manualmente al PATH: %s\n", installDir)
	} else {
		fmt.Printf("  ✓ %s", string(out))
	}

	// Setup del proyecto actual apuntando al binario instalado (ruta estable).
	setupProjectWith(dest)
	fmt.Println("\nGlobal listo. Abri una terminal NUEVA para usar el comando 'musubi'.")
}

// sameFile compara dos rutas resueltas (case-insensitive, para Windows).
func sameFile(a, b string) bool {
	ap, e1 := filepath.Abs(a)
	bp, e2 := filepath.Abs(b)
	return e1 == nil && e2 == nil && strings.EqualFold(ap, bp)
}

// copyFile copia src a dst (binario).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
