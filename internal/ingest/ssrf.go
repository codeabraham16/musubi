package ingest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// ssrf.go blinda la ingesta cuando se expone en infra COMPARTIDA (el cerebro central): impide que un
// principal use el fetcher del lado del server para pegarle a destinos INTERNOS (SSRF) — loopback,
// redes privadas, link-local, el rango CGNAT del tailnet (100.64/10) y el endpoint de metadata de la
// nube. Doble capa: pre-chequeo del host (cubre yt-dlp y artículo) + un dialer que revalida la IP en
// el momento del connect (defensa contra DNS-rebinding en la ruta artículo). Model-free, sin red salvo
// el DNS lookup necesario.

// isBlockedIP indica si una IP NO es un destino público legítimo.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsPrivate() {
		return true
	}
	// CGNAT 100.64.0.0/10 — el tailnet vive acá y IsPrivate() no lo cubre.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	// Endpoint de metadata de nube (ya es link-local, pero explícito por las dudas).
	if ip.Equal(net.IPv4(169, 254, 169, 254)) {
		return true
	}
	return false
}

// assertPublicHost resuelve host y falla si el literal, o CUALQUIER IP resuelta, está bloqueada.
func assertPublicHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" {
		return fmt.Errorf("host vacío")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("destino no permitido: %s es una dirección interna", host)
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("no pude resolver %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%q no resolvió a ninguna IP", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("destino no permitido: %s resuelve a una dirección interna", host)
		}
	}
	return nil
}

// safeControl es el net.Dialer.Control que rechaza conectar a IPs bloqueadas EN EL MOMENTO del
// connect (defensa contra DNS-rebinding: aunque el host resolviera público al chequear, si al conectar
// apunta a una IP interna, se corta).
func safeControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if isBlockedIP(net.ParseIP(host)) {
		return fmt.Errorf("conexión bloqueada a dirección interna %s", host)
	}
	return nil
}

// safeHTTPClient devuelve un cliente HTTP cuyo dialer rechaza IPs internas (SSRF-safe). Además corta
// las redirecciones a destinos internos.
func safeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: safeControl}
	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return assertPublicHost(req.Context(), req.URL.Host)
		},
	}
}
