package mcp

import (
	"fmt"
	"strings"
)

// principals_metricas.go publica en /metrics el VENCIMIENTO de las credenciales del cerebro.
//
// El vencimiento existe desde el 2026-09-03 (`expires:` en principals.yaml) y hasta acá no
// avisaba: una credencial con fecha se caía el día que tocaba, y lo primero que se veía era el
// sync de una máquina muerto. Estas series son el aviso ANTES, y las leen tres alertas de
// deploy/musubi-alerts.yml (CredencialPorVencer, CredencialRecienVencida y
// RegistroDePrincipalsSinPoderRecargarse).
//
// NINGUNA LLEVA EL NOMBRE DE UN PRINCIPAL. Quién tiene acceso es un dato sólo para admin
// (toolTokenList), y una etiqueta `principal=` se lo daría a cualquiera que scrapee. La alerta
// dice CUÁNTAS y CUÁNDO; cuál es, lo dice `musubi token list` en el servidor.
//
// Y EL BLOQUE ENTERO SÓLO LO VE QUIEN VE TODO (read=all), o la confianza local. Sin nombres
// igual cuenta cosas —que hay un bearer admin que no vence, cuántas credenciales son eternas—, y
// eso no es para un tercero con read=own. El scrape de Prometheus usa una identidad read=all,
// así que las alertas lo reciben igual.

const (
	nombreProximoVencimiento   = "musubi_principal_next_expiry_seconds"
	nombreCredencialesVencidas = "musubi_principals_expired"
	nombreCredencialesEternas  = "musubi_principals_without_expiry"
	nombreLegacyHabilitado     = "musubi_legacy_token_enabled"
	nombreLegacyAciertos       = "musubi_legacy_token_auth_total"
	nombreRegistroSinRecargar  = "musubi_principals_reload_failing"
	nombreRegistroRecargasMal  = "musubi_principals_reload_failures_total"
)

// veElRegistro decide quién recibe el bloque: quien ve todos los proyectos, o la confianza local
// (sin principal), que es la misma regla que usa el resto del código para «ve todo».
func veElRegistro(quien *Principal) bool {
	if quien == nil {
		return true
	}
	read, _ := quien.caps()
	return read == ReadAll
}

// renderVencimientoDeCredenciales emite el resumen del registro. No emite NADA sin registro
// (confianza local sin credenciales: no hay nada que vencer) ni para quien no ve todo.
//
// El reloj es ahoraParaVencimiento y no time.Now: es el MISMO que decide si una credencial
// autentica, así que la serie no puede decir «le quedan 3 s» de una que resolve() ya niega.
func (s *McpServer) renderVencimientoDeCredenciales(b *strings.Builder, reg principalResolver, quien *Principal) {
	if reg == nil || !veElRegistro(quien) {
		return
	}
	r := reg.resumenDeVencimientos(ahoraParaVencimiento())

	fmt.Fprintf(b, "# HELP %s Segundos que le quedan a la PRÓXIMA credencial en vencer, entre las que todavía autentican. AUSENTE si ninguna tiene fecha futura: un 0 acá se leería como «vence en este instante». Sin nombre a propósito: cuál es, lo dice `musubi token list`.\n# TYPE %s gauge\n",
		nombreProximoVencimiento, nombreProximoVencimiento)
	if r.HayProximo {
		fmt.Fprintf(b, "%s %d\n", nombreProximoVencimiento, int64(r.Proximo.Seconds()))
	}

	fmt.Fprintf(b, "# HELP %s Credenciales de principals.yaml cuya fecha ya pasó: no autentican ni actúan. Un salto hacia arriba es un servicio que se acaba de quedar afuera.\n# TYPE %s gauge\n",
		nombreCredencialesVencidas, nombreCredencialesVencidas)
	fmt.Fprintf(b, "%s %d\n", nombreCredencialesVencidas, r.Vencidas)

	fmt.Fprintf(b, "# HELP %s Credenciales de principals.yaml sin `expires:`: valen para siempre.\n# TYPE %s gauge\n",
		nombreCredencialesEternas, nombreCredencialesEternas)
	fmt.Fprintf(b, "%s %d\n", nombreCredencialesEternas, r.SinVencimiento)

	fmt.Fprintf(b, "# HELP %s 1 si el cerebro admite el bearer legacy (service.auth_token_env): admin federado, fuera de `musubi token list` y sin vencimiento posible.\n# TYPE %s gauge\n",
		nombreLegacyHabilitado, nombreLegacyHabilitado)
	fmt.Fprintf(b, "%s %d\n", nombreLegacyHabilitado, unoSi(r.Legacy))

	fmt.Fprintf(b, "# HELP %s Veces que autenticó el bearer legacy desde que arrancó el proceso, por CUALQUIER puerta (/mcp, /metrics, /api/*). Es lo que dice si se puede retirar: el ledger de tools no ve las puertas que no son tools.\n# TYPE %s counter\n",
		nombreLegacyAciertos, nombreLegacyAciertos)
	fmt.Fprintf(b, "%s %d\n", nombreLegacyAciertos, r.LegacyAciertos)

	fmt.Fprintf(b, "# HELP %s 1 si principals.yaml cambió y la relectura en caliente se rechazó: el registro que autentica NO es el del disco (una revocación nueva no se aplicó) y el próximo reinicio no va a arrancar con ese archivo.\n# TYPE %s gauge\n",
		nombreRegistroSinRecargar, nombreRegistroSinRecargar)
	fmt.Fprintf(b, "%s %d\n", nombreRegistroSinRecargar, unoSi(r.RecargaFallando))

	fmt.Fprintf(b, "# HELP %s Relecturas en caliente de principals.yaml rechazadas desde que arrancó el proceso. Se reintenta cada 10 s mientras el archivo siga roto, así que crece solo.\n# TYPE %s counter\n",
		nombreRegistroRecargasMal, nombreRegistroRecargasMal)
	fmt.Fprintf(b, "%s %d\n", nombreRegistroRecargasMal, r.RecargasFallidas)
}

// unoSi lleva un bool al 0/1 de un gauge.
func unoSi(v bool) int {
	if v {
		return 1
	}
	return 0
}
