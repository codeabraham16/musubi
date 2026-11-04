package crudobackup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/transcripts"
)

// localizarCarpetas devuelve los NOMBRES (relativos a dirTx) de las carpetas de proyecto a respaldar.
//
//   - AlcanceProyecto: una sola, la del workspace actual, mangleada como la nombra Claude Code.
//   - AlcanceTodos: todas las hijas directas de dirTx menos los experimentos de la temporal del
//     sistema (los mismos que salta el medidor de uso).
func localizarCarpetas(alc Alcance, dirTx, workspace string) ([]string, error) {
	switch alc {
	case AlcanceProyecto:
		if strings.TrimSpace(workspace) == "" {
			return nil, fmt.Errorf("respaldo del proyecto actual sin workspace")
		}
		// Claude Code mangle la ruta ABSOLUTA de trabajo. Si musubi corre desde donde se abrió
		// Claude (el caso normal), coincide; si no, la carpeta no existe y damos un error que dice
		// qué se buscó, para que el usuario use --todos o --dir.
		abs, err := filepath.Abs(workspace)
		if err != nil {
			return nil, fmt.Errorf("no pude resolver el workspace %s: %w", workspace, err)
		}
		nombre := transcripts.CarpetaDeProyecto(abs)
		carpeta := filepath.Join(dirTx, nombre)
		st, err := os.Stat(carpeta)
		if err != nil || !st.IsDir() {
			return nil, fmt.Errorf("no hay carpeta de transcripts para %s (buscada: %s); si Claude Code se abrió en otra ruta, usá --todos", abs, carpeta)
		}
		return []string{nombre}, nil
	case AlcanceTodos:
		ents, err := os.ReadDir(dirTx)
		if err != nil {
			return nil, fmt.Errorf("no pude listar %s: %w", dirTx, err)
		}
		excl := transcripts.ExclusionesPorDefecto()
		var out []string
		for _, e := range ents {
			if !e.IsDir() || transcripts.Excluida(e.Name(), excl) {
				continue
			}
			out = append(out, e.Name())
		}
		return out, nil
	default:
		return nil, fmt.Errorf("alcance desconocido: %d", alc)
	}
}

// armarPaquete copia las carpetas a un stage QUIETO y tar-ea esa copia estable. Devuelve el número
// de archivos regulares empaquetados. El stage se borra al salir, pase lo que pase.
func armarPaquete(destino, dirTx string, carpetas []string, logf func(string)) (int, error) {
	stage, err := os.MkdirTemp("", "musubi-crudo-stage-*")
	if err != nil {
		return 0, fmt.Errorf("no pude crear el stage: %w", err)
	}
	defer os.RemoveAll(stage)

	var entradas int
	for _, c := range carpetas {
		n, err := copiarArbol(filepath.Join(dirTx, c), filepath.Join(stage, c))
		if err != nil {
			return 0, fmt.Errorf("copiando %s al stage: %w", c, err)
		}
		entradas += n
		logf(fmt.Sprintf("al stage: %d archivo(s) de %s", n, c))
	}
	if err := tarGz(destino, stage); err != nil {
		return 0, err
	}
	return entradas, nil
}

// copiarArbol copia recursivamente src→dst preservando la estructura. Copia TODO lo que es archivo
// regular (incluidas las carpetas subagents/ y el journal de workflow): esto es un respaldo del
// crudo literal, NO una medición, así que no filtra nada —a diferencia de transcripts.Recorrer—.
// Devuelve cuántos archivos regulares copió.
func copiarArbol(src, dst string) (int, error) {
	var n int
	err := filepath.WalkDir(src, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, ruta)
		if err != nil {
			return err
		}
		objetivo := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(objetivo, 0o755)
		}
		if !d.Type().IsRegular() {
			// Enlaces y archivos especiales no tienen sentido en un backup de transcripts; se
			// saltean sin ruido.
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(objetivo), 0o755); err != nil {
			return err
		}
		if err := copiarArchivo(ruta, objetivo); err != nil {
			return err
		}
		n++
		return nil
	})
	return n, err
}

func copiarArchivo(src, dst string) error {
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

// tarGz empaqueta el árbol `raiz` (ya quieto) en destino, como tar.gz puro-Go. Los paths dentro del
// tar son relativos a raiz, así que al extraer se reconstruye projects/<carpeta>/...
func tarGz(destino, raiz string) (err error) {
	f, err := os.Create(destino)
	if err != nil {
		return fmt.Errorf("no pude crear %s: %w", destino, err)
	}
	// Si algo falla a mitad, el .tgz parcial no sirve de nada: se borra para no dejar un respaldo
	// mentiroso en disco.
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(destino)
		}
	}()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	walkErr := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if ruta == raiz {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(raiz, ruta)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(ruta)
		if err != nil {
			return err
		}
		// CopyN EXACTO al tamaño del header: aunque el archivo cambiara (no debería, el stage está
		// quieto), el tar nunca sale descoordinado entre tamaño declarado y bytes escritos —que fue
		// justo la falla del tar manual—.
		_, err = io.CopyN(tw, in, info.Size())
		in.Close()
		if err != nil {
			return fmt.Errorf("copiando %s al tar: %w", rel, err)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// verificarPaquete reabre el .tgz, lo recorre ENTERO (forzando la descompresión de cada entrada) y
// calcula el sha256 del archivo completo. Devuelve (sha, nEntradasRegulares). Un gzip/tar truncado
// falla acá, en origen, antes de enviar nada.
func verificarPaquete(ruta string) (string, int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	tee := io.TeeReader(f, h) // cada byte leído del archivo alimenta el hash

	gz, err := gzip.NewReader(tee)
	if err != nil {
		return "", 0, fmt.Errorf("gzip no abre: %w", err)
	}
	tr := tar.NewReader(gz)
	var n int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", 0, fmt.Errorf("tar no recorre: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return "", 0, fmt.Errorf("entrada %s no se descomprime: %w", hdr.Name, err)
			}
			n++
		}
	}
	if err := gz.Close(); err != nil {
		return "", 0, fmt.Errorf("cierre gzip: %w", err)
	}
	// El tar termina en sus bloques cero; el archivo puede tener todavía el trailer del gzip. Hay
	// que drenar el resto POR EL TEE para que el hash sea del archivo COMPLETO, no sólo de lo que el
	// tar consumió.
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return "", 0, fmt.Errorf("drenando el resto del archivo: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// purgarViejos borra los musubi-crudo-*.tgz de dir con mtime más viejo que retentionDays. <=0 ⇒ no
// purga (0 no es «borrar todo»: es «conservar todo», el lado seguro). Devuelve cuántos borró.
func purgarViejos(dir string, retentionDays int, ahora time.Time) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	corte := ahora.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	var n int
	for _, e := range ents {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "musubi-crudo-") || !strings.HasSuffix(e.Name(), ".tgz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(corte) {
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				n++
			}
		}
	}
	return n, nil
}
