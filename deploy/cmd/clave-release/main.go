// clave-release genera el par ed25519 con el que se firman los manifiestos de release.
//
// SE CORRE UNA SOLA VEZ, Y LA PRIVADA NO SE COPIA A NINGÚN SERVIDOR NI AL CI. Si la privada
// estuviera en el pipeline, quien comprometa el pipeline firma lo que quiera y la firma no compra
// nada — sería un sha256 más caro. Guardala fuera de línea y en modo 600.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no se pudo generar el par: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("# PRIVADA — guardala fuera de línea, modo 600, y NO la subas a ningún lado")
	fmt.Println(hex.EncodeToString(priv.Seed()))
	fmt.Println()
	fmt.Println("# PÚBLICA — va embebida en el binario:")
	fmt.Printf("#   go build -ldflags \"-X main.clavePublicaDeReleaseHex=%s\"\n", hex.EncodeToString(pub))
	fmt.Println(hex.EncodeToString(pub))
}
