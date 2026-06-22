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
	fmt.Println(cCyan("============================================"))
	fmt.Println("                 " + cBold(cCyan("Musubi")))
	fmt.Println(cCyan("============================================"))
	fmt.Println()
	fmt.Println("  " + cBold("Donde queres instalar Musubi?"))
	fmt.Println()
	fmt.Println("    [" + cBold("L") + "] Solo esta carpeta (local, NO toca la PC)")
	fmt.Println("    [" + cBold("G") + "] Global en la PC (PATH del usuario, sin admin)")
	fmt.Println("    [" + cBold("Q") + "] Salir")
	fmt.Println()
	fmt.Print("  Eleccion (" + cBold("L/G/Q") + "): ")
	line, _ := reader.ReadString('\n')
	fmt.Println()

	switch menuAction(line) {
	case "local":
		if confirmLocalDir(reader) {
			setupProject()
		}
	case "global":
		installGlobalWindows()
	default:
		fmt.Println(cDim("Cancelado. No se instalo nada."))
	}

	fmt.Print("\nPresiona Enter para salir...")
	reader.ReadString('\n')
}

// confirmLocalDir protege contra la "trampa del doble clic": si se eligió instalar
// local en una carpeta que NO parece un proyecto (típico de hacer doble clic sobre el
// .exe descargado en Descargas), avisa y pide confirmación explícita. En un proyecto
// real (con go.mod/package.json/.git/...) procede sin molestar.
func confirmLocalDir(reader *bufio.Reader) bool {
	cwd, err := os.Getwd()
	if err != nil || looksLikeProject(cwd) {
		return true
	}
	fmt.Println("  " + cYellow("!") + " Esta carpeta no parece un proyecto (sin go.mod, package.json, .git, ...).")
	fmt.Println("    " + cDim(cwd))
	fmt.Println("    Si hiciste doble clic desde Descargas, Musubi se instalaria ACA.")
	fmt.Println("    Para usar Musubi en toda la PC, mejor elegi la opcion " + cBold("[G] Global") + ".")
	fmt.Print("  Instalar igual en esta carpeta? (s/N): ")
	line, _ := reader.ReadString('\n')
	if isYes(line) {
		return true
	}
	fmt.Println(cDim("  Cancelado."))
	return false
}

// isYes interpreta una respuesta afirmativa (s/si/sí/y/yes), case-insensitive.
func isYes(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "s", "si", "sí", "y", "yes":
		return true
	default:
		return false
	}
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
	printOK("Binario instalado en " + dest)

	// Agregar installDir al PATH del usuario (sin admin) y exponer MUSUBI_BIN (la ruta
	// del binario) vía PowerShell. MUSUBI_BIN hace que el .mcp.json portable resuelva el
	// binario aunque cambie la ruta o el usuario: al reinstalar se re-setea y todos los
	// proyectos vuelven a funcionar sin tocar sus .mcp.json.
	psDir := "'" + strings.ReplaceAll(installDir, "'", "''") + "'"
	psBin := "'" + strings.ReplaceAll(dest, "'", "''") + "'"
	ps := fmt.Sprintf(`$d=%s; $p=[Environment]::GetEnvironmentVariable('Path','User'); if ($p -notlike "*$d*") { [Environment]::SetEnvironmentVariable('Path', "$p;$d", 'User') }; [Environment]::SetEnvironmentVariable('MUSUBI_BIN', %s, 'User'); 'PATH y MUSUBI_BIN actualizados'`, psDir, psBin)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		printWarn(fmt.Sprintf("No se pudo actualizar PATH/MUSUBI_BIN automaticamente: %v", err))
		fmt.Println(cDim("    Agregalo manualmente al PATH: " + installDir))
	} else {
		printOK(strings.TrimSpace(string(out)))
	}

	// Setup del proyecto actual apuntando al binario instalado (ruta estable).
	setupProjectWith(dest, "")
	fmt.Println("\n" + cGreen("Global listo.") + " Abri una terminal NUEVA para usar el comando 'musubi'.")
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
