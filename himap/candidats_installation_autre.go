//go:build !windows

// Package himap — candidats_installation_autre.go : repli hors Windows. Pas de registre ni de
// lecteurs : aucun candidat, pas d'erreur (le package doit compiler partout).
package himap

const cheminsInsensiblesCasse = false

func registreSysteme(string, string, string) (string, error) { return "", errRegistreAbsent }

func lecteursFixesSysteme() ([]string, error) { return nil, nil }
