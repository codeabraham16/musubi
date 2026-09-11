//go:build !race

package testbudget

// BajoDetector dice si esta compilación lleva el detector de carreras; acá, no.
// Ver detector_on.go para el porqué de que sea una const y no una consulta en runtime.
const BajoDetector = false
