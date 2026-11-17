//go:build windows

// Package himap — candidats_installation_windows.go : les deux lectures du systeme que
// CandidatsInstallation ne peut pas faire sans Windows (registre, lecteurs fixes).
package himap

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// chemins Windows insensibles a la casse : `c:/steam` et `C:\Steam` sont le meme dossier.
const cheminsInsensiblesCasse = true

// registreSysteme lit une valeur chaine du registre. Cle ou valeur absente : errRegistreAbsent.
func registreSysteme(ruche, chemin, valeur string) (string, error) {
	var racine registry.Key
	switch ruche {
	case rucheUtilisateur:
		racine = registry.CURRENT_USER
	case rucheMachine:
		racine = registry.LOCAL_MACHINE
	default:
		return "", fmt.Errorf("himap: ruche inconnue %q", ruche)
	}
	k, err := registry.OpenKey(racine, chemin, registry.QUERY_VALUE)
	if err != nil {
		return "", erreurRegistre(err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue(valeur)
	if err != nil {
		return "", erreurRegistre(err)
	}
	return v, nil
}

func erreurRegistre(err error) error {
	if errors.Is(err, registry.ErrNotExist) {
		return errRegistreAbsent
	}
	return err
}

// lecteursFixesSysteme rend la racine de chaque lecteur fixe (`C:\`, `D:\`...), par lettre.
func lecteursFixesSysteme() ([]string, error) {
	masque, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	var racines []string
	for i := 0; i < 26; i++ {
		if masque&(1<<uint(i)) == 0 {
			continue
		}
		racine := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(racine)
		if err != nil {
			continue
		}
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			racines = append(racines, racine)
		}
	}
	return racines, nil
}
