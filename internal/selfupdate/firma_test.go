package selfupdate

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func parDePrueba(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

// UN RELEASE QUE NO PODEMOS PROBAR QUE ES NUESTRO NO SE INSTALA.
//
// Ola 2 del plan empresa. El `.sha256` lo publica quien publica el binario: verificarlo dice que
// el archivo llegó entero, no que lo hayamos hecho nosotros. En este sistema eso pesa más que en
// la mayoría, porque el mismo binario es el CEREBRO y el AGENTE — un release comprometido no
// entrega una máquina, entrega la flota, con exec y shell sobre todas.
//
// Sabotaje que la hace fallar: que VerificarFirma devuelva nil sin llamar a ed25519.Verify.
func TestUnManifiestoConFirmaAjenaNoVerifica(t *testing.T) {
	pub, priv := parDePrueba(t)
	otraPub, otraPriv := parDePrueba(t)

	m := Manifiesto{Version: "1.2.3", Assets: map[string]string{"musubi-linux-amd64": strings.Repeat("a", 64)}}
	datos, err := m.BytesFirmables()
	if err != nil {
		t.Fatal(err)
	}

	// Firmada por nosotros: verifica.
	if err := VerificarFirma(pub, datos, hex.EncodeToString(ed25519.Sign(priv, datos))); err != nil {
		t.Fatalf("un manifiesto firmado con la clave correcta no verificó: %v", err)
	}
	// Firmada por OTRO: no verifica. Es el ataque: quien controla el release publica binario,
	// hash y hasta una firma — pero no la NUESTRA.
	err = VerificarFirma(pub, datos, hex.EncodeToString(ed25519.Sign(otraPriv, datos)))
	if err == nil {
		t.Error("un manifiesto firmado con OTRA clave verificó: cualquiera que controle el release puede publicar lo que quiera")
	}
	// Y la clave ajena no valida nuestra firma (control: la prueba no está comparando cualquier cosa).
	if VerificarFirma(otraPub, datos, hex.EncodeToString(ed25519.Sign(priv, datos))) == nil {
		t.Error("nuestra firma verificó contra una clave ajena")
	}
	// Manifiesto ALTERADO después de firmar: no verifica.
	alterado := append([]byte(nil), datos...)
	alterado[len(alterado)-2] ^= 0xFF
	if VerificarFirma(pub, alterado, hex.EncodeToString(ed25519.Sign(priv, datos))) == nil {
		t.Error("un manifiesto alterado después de firmar verificó")
	}
}

// SIN CLAVE EMBEBIDA NO SE ACTUALIZA, y no es un detalle de implementación.
//
// La tentación es dejar pasar «porque total antes tampoco verificaba». Eso sería PEOR que antes:
// ahora existe una función de verificación que da tranquilidad y no verifica nada.
//
// Sabotaje: que VerificarFirma devuelva nil cuando la clave está vacía.
func TestSinClaveEmbebidaLaVerificacionFalla(t *testing.T) {
	m := Manifiesto{Version: "1.2.3", Assets: map[string]string{"x": strings.Repeat("b", 64)}}
	datos, _ := m.BytesFirmables()
	_, priv := parDePrueba(t)
	firma := hex.EncodeToString(ed25519.Sign(priv, datos))

	for _, caso := range []struct {
		nombre string
		pub    ed25519.PublicKey
	}{
		{"nil", nil},
		{"vacía", ed25519.PublicKey{}},
		{"de largo equivocado", ed25519.PublicKey(make([]byte, 10))},
	} {
		if err := VerificarFirma(caso.pub, datos, firma); err == nil {
			t.Errorf("con la clave %s la verificación pasó: un binario que no puede verificar no puede actualizarse", caso.nombre)
		}
	}
}

// LA FORMA CANÓNICA NO PUEDE DEPENDER DEL ORDEN DE UN MAPA.
//
// Si `BytesFirmables` produjera bytes distintos en dos corridas con el mismo manifiesto, la firma
// empezaría a fallar en algunos releases y no en otros — intermitente y con pinta de problema de
// red, que es el peor síntoma posible. `json.Marshal` de un map ordena las claves, y esta prueba
// lo clava para que nadie lo cambie por algo que no lo garantiza.
//
// Sabotaje: serializar el manifiesto recorriendo el map a mano en vez de con json.Marshal.
func TestLaFormaCanonicaEsDeterministaConMuchasClaves(t *testing.T) {
	assets := map[string]string{}
	for _, n := range []string{"z", "a", "m", "b", "y", "c", "x", "d", "w", "e"} {
		assets["musubi-"+n] = strings.Repeat(n, 64)
	}
	m := Manifiesto{Version: "1.2.3", Assets: assets}
	primero, err := m.BytesFirmables()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		otro, err := m.BytesFirmables()
		if err != nil {
			t.Fatal(err)
		}
		if string(otro) != string(primero) {
			t.Fatalf("la forma canónica cambió entre corridas: la firma fallaría de forma intermitente\n  %s\n  %s", primero, otro)
		}
	}
	// Y es JSON con las claves ordenadas, que es lo que el guion de firma produce del otro lado.
	var control map[string]any
	if err := json.Unmarshal(primero, &control); err != nil {
		t.Fatalf("la forma canónica no es JSON válido: %v", err)
	}
}

// UN ASSET QUE NO ESTÁ EN EL MANIFIESTO FIRMADO NO SE INSTALA.
//
// El manifiesto es la lista de lo que publicamos. Bajar algo que no figura ahí es bajar algo que
// no firmamos, y «seguir sin verificar» sería dejar el agujero abierto justo en el caso raro.
//
// Sabotaje: que ShaDeAsset devuelva ("", nil) para un asset ausente.
func TestUnAssetFueraDelManifiestoSeRechaza(t *testing.T) {
	m := Manifiesto{Version: "1.2.3", Assets: map[string]string{"musubi-linux-amd64": strings.Repeat("a", 64)}}
	if _, err := m.ShaDeAsset("musubi-linux-amd64"); err != nil {
		t.Fatalf("un asset que SÍ está se rechazó: %v", err)
	}
	if _, err := m.ShaDeAsset("musubi-windows-amd64.exe"); err == nil {
		t.Error("un asset que no está en el manifiesto firmado se aceptó: es bajar algo que no firmamos")
	}
	// Un sha vacío en el manifiesto también es un no: la entrada existe y no dice nada.
	m.Assets["vacio"] = "   "
	if _, err := m.ShaDeAsset("vacio"); err == nil {
		t.Error("un asset con sha vacío se aceptó")
	}
}
