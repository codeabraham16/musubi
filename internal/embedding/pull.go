package embedding

// pull.go implementa la descarga TURNKEY de una tabla estática de embeddings (model2vec/
// POTION) con checksum pinneado. La tabla es PURO DATO (embeddings destilados offline): se
// baja UNA vez en el setup y en runtime no corre ninguna red ni modelo — sigue siendo
// model-free at inference. El checksum SHA-256 va pinneado, así que la descarga es
// verificable e inmutable aunque la fuente sea de un tercero o un mirror propio.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ModelFile es un archivo de una tabla, con su URL y checksum/tamaño pinneados.
type ModelFile struct {
	Name   string
	URL    string
	SHA256 string
	Size   int64
}

// ModelSpec describe una tabla de embeddings descargable.
type ModelSpec struct {
	Name  string
	Files []ModelFile
}

// KnownModels es el registro de tablas descargables con checksum pinneado. La tabla
// multilingüe (ES+EN) es la recomendada para memoria bilingüe; usa tokenizer Unigram
// (ver spm.go). Los checksums se verificaron contra el oid LFS publicado por la fuente.
var KnownModels = map[string]ModelSpec{
	"potion-multilingual-128M": {
		Name: "potion-multilingual-128M",
		Files: []ModelFile{
			{
				Name:   "model.safetensors",
				URL:    "https://huggingface.co/minishlab/potion-multilingual-128M/resolve/main/model.safetensors",
				SHA256: "14b5eb39cb4ce5666da8ad1f3dc6be4346e9b2d601c073302fa0a31bf7943397",
				Size:   512361560,
			},
			{
				Name:   "tokenizer.json",
				URL:    "https://huggingface.co/minishlab/potion-multilingual-128M/resolve/main/tokenizer.json",
				SHA256: "19f1909063da3cfe3bd83a782381f040dccea475f4816de11116444a73e1b6a1",
				Size:   18616131,
			},
		},
	},
}

// PullModel descarga cada archivo de spec a destDir verificando tamaño y SHA-256, de forma
// ATÓMICA (baja a <name>.part y renombra sólo tras verificar) e IDEMPOTENTE (si el archivo
// ya existe con el checksum correcto, no lo re-descarga). client nil ⇒ uno con timeout
// amplio (las tablas pesan cientos de MB). progress, si no es nil, reporta el avance por
// archivo. Ante checksum incorrecto falla fail-closed y deja el .part para diagnóstico.
func PullModel(destDir string, spec ModelSpec, client *http.Client, progress func(file string, done, total int64)) error {
	if len(spec.Files) == 0 {
		return fmt.Errorf("tabla %q sin archivos", spec.Name)
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Minute}
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("crear %s: %w", destDir, err)
	}
	for _, f := range spec.Files {
		final := filepath.Join(destDir, f.Name)
		if ok, _ := fileHasChecksum(final, f.SHA256); ok {
			continue // ya está y coincide: idempotente
		}
		if err := downloadVerified(client, f, final, progress); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

// downloadVerified baja f.URL a un archivo temporal calculando el SHA-256 al vuelo, valida
// tamaño y hash contra lo pinneado, y sólo entonces renombra al destino final (atómico).
func downloadVerified(client *http.Client, f ModelFile, final string, progress func(string, int64, int64)) error {
	resp, err := client.Get(f.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s devolvió %d", f.URL, resp.StatusCode)
	}

	tmp := final + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 1<<20) // 1 MiB
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(f.Name, done, f.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}

	if f.Size > 0 && done != f.Size {
		return fmt.Errorf("tamaño inesperado: bajé %d bytes, esperaba %d", done, f.Size)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != f.SHA256 {
		return fmt.Errorf("checksum incorrecto: obtuve %s, esperaba %s (%s queda para diagnóstico)", got, f.SHA256, tmp)
	}
	return os.Rename(tmp, final)
}

// fileHasChecksum indica si el archivo en path ya existe y su SHA-256 coincide con want.
func fileHasChecksum(path, want string) (bool, error) {
	fh, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}
