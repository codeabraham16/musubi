package crudobackup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/transcripts"
)

// relojFijo hace deterministas el nombre del paquete y el sello de las marcas.
func relojFijo() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func escribir(t *testing.T, ruta, contenido string) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(ruta), 0o755))
	must(t, os.WriteFile(ruta, []byte(contenido), 0o644))
}

// transporteFake simula el destino. En modo fiel calcula el sha256 REAL del paquete que recibe (un
// destino que recibió el archivo íntegro); shaForzado/errForzado simulan corrupción o caída.
type transporteFake struct {
	enviados   []string
	shaForzado string
	errForzado error
}

func (f *transporteFake) Enviar(paquete, remote string) (string, error) {
	f.enviados = append(f.enviados, paquete)
	if f.errForzado != nil {
		return "", f.errForzado
	}
	if f.shaForzado != "" {
		return f.shaForzado, nil
	}
	fp, err := os.Open(paquete)
	if err != nil {
		return "", err
	}
	defer fp.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fp); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// montarProyecto arma un dirTx con la carpeta de transcripts del workspace (nombrada como lo haría
// Claude Code), con 3 archivos regulares incluyendo uno bajo subagents/.
func montarProyecto(t *testing.T) (dirTx, workspace, out string) {
	t.Helper()
	base := t.TempDir()
	dirTx = filepath.Join(base, "projects")
	workspace = filepath.Join(base, "work", "miproyecto")
	out = filepath.Join(base, "out")

	abs, err := filepath.Abs(workspace)
	must(t, err)
	carpeta := filepath.Join(dirTx, transcripts.CarpetaDeProyecto(abs))
	escribir(t, filepath.Join(carpeta, "sesion1.jsonl"), "{\"a\":1}\n")
	escribir(t, filepath.Join(carpeta, "sesion2.jsonl"), "{\"b\":2}\n")
	escribir(t, filepath.Join(carpeta, "subagents", "hijo.jsonl"), "{\"c\":3}\n")
	must(t, os.MkdirAll(workspace, 0o755))
	return dirTx, workspace, out
}

func TestArmarYShipearFlujoFeliz(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	fake := &transporteFake{}
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		Remote:         "user@host:/backups",
		Transporte:     fake,
		Ahora:          relojFijo,
	})
	must(t, err)
	if !res.Enviado {
		t.Error("Enviado=false, esperaba true")
	}
	if res.Entradas != 3 {
		t.Errorf("Entradas=%d, esperaba 3 (incluye subagents/)", res.Entradas)
	}
	if len(res.Carpetas) != 1 {
		t.Errorf("Carpetas=%v, esperaba 1", res.Carpetas)
	}
	if st, serr := os.Stat(res.Paquete); serr != nil || st.Size() == 0 {
		t.Errorf("el paquete no existe o está vacío: %v", serr)
	}
	// Marca buena presente, marca de error ausente.
	if _, e := os.Stat(filepath.Join(out, MarcaOffhost)); e != nil {
		t.Errorf("falta la marca offhost: %v", e)
	}
	if _, e := os.Stat(filepath.Join(out, MarcaOffhostError)); !os.IsNotExist(e) {
		t.Error("no debería haber marca de error tras un envío exitoso")
	}
	// El paquete reabre con 3 entradas y el sha coincide con el informado.
	sha, n, verr := verificarPaquete(res.Paquete)
	must(t, verr)
	if n != 3 {
		t.Errorf("verificación: %d entradas, esperaba 3", n)
	}
	if sha != res.SHA256 {
		t.Errorf("sha de verificación %s ≠ sha informado %s", sha, res.SHA256)
	}
	if len(fake.enviados) != 1 {
		t.Errorf("se envió %d veces, esperaba 1", len(fake.enviados))
	}
}

func TestArmarYShipearShaRemotoMaloFalla(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	fake := &transporteFake{shaForzado: "0000000000000000000000000000000000000000000000000000000000000000"}
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		Remote:         "user@host:/backups",
		Transporte:     fake,
		Ahora:          relojFijo,
	})
	if err == nil {
		t.Fatal("esperaba error: el sha remoto no coincide con el de origen")
	}
	if res.Enviado {
		t.Error("Enviado=true pese al sha remoto distinto")
	}
	// Dead-man: marca de error presente, marca buena ausente.
	if _, e := os.Stat(filepath.Join(out, MarcaOffhostError)); e != nil {
		t.Errorf("un sha remoto malo debería dejar marca de error: %v", e)
	}
	if _, e := os.Stat(filepath.Join(out, MarcaOffhost)); !os.IsNotExist(e) {
		t.Error("no debería haber marca buena cuando la llegada no se confirmó")
	}
}

func TestArmarYShipearEnvioCaidoFalla(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	fake := &transporteFake{errForzado: fmt.Errorf("conexión rechazada")}
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		Remote:         "user@host:/backups",
		Transporte:     fake,
		Ahora:          relojFijo,
	})
	if err == nil {
		t.Fatal("esperaba error por envío caído")
	}
	// El paquete local igual quedó armado y verificado (es un respaldo local válido).
	if res.Paquete == "" {
		t.Error("el paquete local debería existir aunque el envío falle")
	}
	if _, e := os.Stat(filepath.Join(out, MarcaOffhostError)); e != nil {
		t.Errorf("un envío caído debería dejar marca de error: %v", e)
	}
}

func TestArmarYShipearSinDestinoFallaCerrado(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	_, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		Ahora:          relojFijo,
	})
	if err == nil {
		t.Fatal("esperaba falla-cerrado: sin Remote y sin AllowLocalOnly")
	}
	if _, e := os.Stat(filepath.Join(out, MarcaOffhostError)); e != nil {
		t.Errorf("el falla-cerrado debería dejar marca de error: %v", e)
	}
}

func TestArmarYShipearLocalOnly(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		AllowLocalOnly: true,
		Ahora:          relojFijo,
	})
	must(t, err)
	if res.Enviado {
		t.Error("Enviado=true en modo local-only")
	}
	if _, e := os.Stat(res.Paquete); e != nil {
		t.Errorf("falta el paquete local: %v", e)
	}
	// Local-only no deja marca offhost: el doctor dirá «sin medir», que es honesto.
	if _, e := os.Stat(filepath.Join(out, MarcaOffhost)); !os.IsNotExist(e) {
		t.Error("local-only no debería dejar marca offhost")
	}
}

func TestArmarYShipearTodosExcluyeExperimentos(t *testing.T) {
	base := t.TempDir()
	dirTx := filepath.Join(base, "projects")
	out := filepath.Join(base, "out")
	// Una carpeta de proyecto "real".
	escribir(t, filepath.Join(dirTx, "-home-x-proj", "s.jsonl"), "{}\n")
	// Una que simula un experimento de la temporal del sistema: nombre derivado igual que lo hace
	// ExclusionesPorDefecto, para que la exclusión la alcance en cualquier OS.
	expMangle := transcripts.CarpetaDeProyecto(filepath.Clean(os.TempDir()))
	escribir(t, filepath.Join(dirTx, expMangle, "exp.jsonl"), "{}\n")

	fake := &transporteFake{}
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceTodos,
		Workspace:      base,
		DirTranscripts: dirTx,
		LocalDir:       out,
		Remote:         "user@host:/backups",
		Transporte:     fake,
		Ahora:          relojFijo,
	})
	must(t, err)
	if len(res.Carpetas) != 1 || res.Carpetas[0] != "-home-x-proj" {
		t.Errorf("Carpetas=%v, esperaba sólo [-home-x-proj] (el experimento de la temporal se excluye)", res.Carpetas)
	}
	if res.Entradas != 1 {
		t.Errorf("Entradas=%d, esperaba 1", res.Entradas)
	}
}

func TestVerificarPaqueteTruncadoFalla(t *testing.T) {
	dirTx, workspace, out := montarProyecto(t)
	res, err := ArmarYShipear(Opciones{
		Alcance:        AlcanceProyecto,
		Workspace:      workspace,
		DirTranscripts: dirTx,
		LocalDir:       out,
		AllowLocalOnly: true,
		Ahora:          relojFijo,
	})
	must(t, err)
	data, rerr := os.ReadFile(res.Paquete)
	must(t, rerr)
	// Truncar a la mitad simula el gotcha del .tgz nacido incompleto.
	must(t, os.WriteFile(res.Paquete, data[:len(data)/2], 0o644))
	if _, _, e := verificarPaquete(res.Paquete); e == nil {
		t.Error("verificarPaquete no detectó el truncamiento: el verde sería falso")
	}
}

func TestNombresDeMarcaSonElContrato(t *testing.T) {
	// El doctor-check crudo_offhost (internal/memory) DUPLICA estos literales porque no puede
	// importar este paquete (ciclo crudobackup→transcripts→memory). Este assert clava el lado
	// PRODUCTOR al literal; el consumidor tiene el suyo. Si cambian acá, cambialos allá.
	if MarcaOffhost != ".last_crudo_offhost" {
		t.Errorf("MarcaOffhost=%q; el doctor espera .last_crudo_offhost", MarcaOffhost)
	}
	if MarcaOffhostError != ".last_crudo_offhost_error" {
		t.Errorf("MarcaOffhostError=%q; el doctor espera .last_crudo_offhost_error", MarcaOffhostError)
	}
}

func TestPurgarViejos(t *testing.T) {
	dir := t.TempDir()
	viejo := filepath.Join(dir, "musubi-crudo-proyecto-20260101-0000.tgz")
	nuevo := filepath.Join(dir, "musubi-crudo-proyecto-20261103-1200.tgz")
	ajeno := filepath.Join(dir, "musubi-backup-20261103.tgz") // NO es del crudo: no se toca
	escribir(t, viejo, "x")
	escribir(t, nuevo, "y")
	escribir(t, ajeno, "z")
	// Envejecer el viejo 100 días.
	hace100 := relojFijo().Add(-100 * 24 * time.Hour)
	must(t, os.Chtimes(viejo, hace100, hace100))

	n, err := purgarViejos(dir, 14, relojFijo())
	must(t, err)
	if n != 1 {
		t.Errorf("purgados=%d, esperaba 1", n)
	}
	if _, e := os.Stat(viejo); !os.IsNotExist(e) {
		t.Error("el paquete viejo debería haberse borrado")
	}
	if _, e := os.Stat(nuevo); e != nil {
		t.Error("el paquete nuevo no debería borrarse")
	}
	if _, e := os.Stat(ajeno); e != nil {
		t.Error("un .tgz que no es del crudo no debería tocarse")
	}
}

// TestPartirRemote cubre el parseo de user@host:/ruta y el blindaje: un userHost que empieza con '-'
// se rechaza, porque scp/ssh lo tomarían como una opción (argument injection) en vez de un destino.
func TestPartirRemote(t *testing.T) {
	casos := []struct {
		in       string
		userHost string
		ruta     string
		wantErr  bool
	}{
		{"musubi@100.79.126.62:/srv/crudo", "musubi@100.79.126.62", "/srv/crudo", false},
		{"  host:/r  ", "host", "/r", false},                                // se recorta
		{"host:/ruta/con:dos:puntos", "host", "/ruta/con:dos:puntos", false}, // parte en el PRIMER ':'
		{"", "", "", true},
		{":/ruta", "", "", true}, // sin host
		{"host:", "", "", true},  // sin ruta
		{"-oProxyCommand=x:/r", "", "", true}, // BLINDAJE: host con '-' se rechaza
	}
	for _, c := range casos {
		uh, ruta, err := partirRemote(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("partirRemote(%q): esperaba error, obtuve userHost=%q ruta=%q", c.in, uh, ruta)
			}
			continue
		}
		if err != nil {
			t.Errorf("partirRemote(%q): error inesperado %v", c.in, err)
			continue
		}
		if uh != c.userHost || ruta != c.ruta {
			t.Errorf("partirRemote(%q): obtuve (%q,%q), esperaba (%q,%q)", c.in, uh, ruta, c.userHost, c.ruta)
		}
	}
}
