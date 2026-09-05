//go:build !linux && !windows && !darwin

package fleet

// colector_otros.go es el colector de los sistemas operativos que TODAVÍA no tienen uno.
//
// Linux, Windows y macOS ya tienen el suyo (colector_{linux,windows,darwin}.go); esto cubre al
// resto —FreeBSD, un Android compilado como linux/arm64 que igual entra por el de Linux, lo que
// venga—. El build tag se va acotando a medida que llega cada plataforma, y ése es el punto: el
// seam no cambia, sólo se le suman implementaciones.
//
// DEVUELVE UN ERROR, NO UNA MUESTRA DE CEROS, y esa diferencia es el invariante D4.
//
// La alternativa —devolver una Muestra vacía— haría que cada Windows de la flota apareciera en
// el panel con 0 % de CPU, 0 de RAM y 0 de disco. Eso no es «sin datos»: es un dato FALSO, con
// la misma forma que un dato bueno, y por lo tanto invisible. Un panel que dice «esta máquina no
// reporta métricas todavía» se arregla; uno que dice 0 % se cree.
//
// Cuando llegue el colector de Windows (su propio slice), reemplaza a este archivo por build tag
// y nada más del sistema cambia: el seam ya está.

type colectorNoImplementado struct{}

// NuevoColector devuelve el colector de este sistema operativo.
func NuevoColector() Colector { return colectorNoImplementado{} }

func (colectorNoImplementado) Tomar() (Muestra, error) { return Muestra{}, ErrSinColector }
