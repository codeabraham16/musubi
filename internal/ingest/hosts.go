package ingest

import (
	"net/url"
	"strings"
)

// mediaHosts mapea sufijos de host conocidos (soportados por yt-dlp) a un nombre de plataforma. NO
// pretende cubrir los ~1900 extractores de yt-dlp: cubre los comunes para CLASIFICAR sin pegar a la
// red. Para cualquier otro sitio de video, --as=video fuerza la ruta media.
var mediaHosts = map[string]string{
	"youtube.com":     "youtube",
	"youtu.be":        "youtube",
	"instagram.com":   "instagram",
	"facebook.com":    "facebook",
	"fb.watch":        "facebook",
	"tiktok.com":      "tiktok",
	"twitter.com":     "twitter",
	"x.com":           "twitter",
	"vimeo.com":       "vimeo",
	"twitch.tv":       "twitch",
	"reddit.com":      "reddit",
	"soundcloud.com":  "soundcloud",
	"dailymotion.com": "dailymotion",
}

// platformForHost devuelve el nombre de plataforma para un host conocido, o "" si no es un host de
// media reconocido. Normaliza www./m. y matchea también subdominios (p.ej. www.youtube.com).
func platformForHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 { // quita :puerto
		host = host[:i]
	}
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	if p, ok := mediaHosts[host]; ok {
		return p
	}
	for suf, p := range mediaHosts {
		if strings.HasSuffix(host, "."+suf) {
			return p
		}
	}
	return ""
}

// IsMediaHost indica si la URL apunta a una plataforma de video/redes conocida (ruta yt-dlp).
func IsMediaHost(u *url.URL) bool { return platformForHost(u.Host) != "" }

// hostOf extrae el host de una URL cruda (o "" si no parsea).
func hostOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return u.Host
}
