// Package himap — candidats_vdf.go : lecteur minimal du format texte VDF de Valve, juste ce
// qu'il faut pour sortir les chemins `"path"` de steamapps/libraryfolders.vdf.
package himap

import (
	"errors"
	"fmt"
	"strings"
)

type genreJetonVdf int

const (
	jetonChaine genreJetonVdf = iota
	jetonOuvre
	jetonFerme
)

type jetonVdf struct {
	genre genreJetonVdf
	texte string
}

// analyseBibliothequesVdf rend, dans l'ordre du fichier, la valeur de chaque cle `"path"`.
// Les antislashs doubles du fichier (D:, deux antislashs, SteamLibrary) deviennent simples.
//
// Un fichier mal forme (chaine non fermee, accolades desequilibrees, jeton non cite, cle sans
// valeur, aucun bloc) rend une erreur ET les chemins lus avant la rupture.
func analyseBibliothequesVdf(contenu string) ([]string, error) {
	jetons, errLex := jetonsVdf(contenu)
	var chemins []string
	var cle string
	aCle, profondeur, blocs := false, 0, 0
	for _, j := range jetons {
		switch j.genre {
		case jetonChaine:
			if !aCle {
				cle, aCle = j.texte, true
				continue
			}
			if profondeur > 0 && strings.EqualFold(cle, "path") && j.texte != "" {
				chemins = append(chemins, j.texte)
			}
			aCle = false
		case jetonOuvre:
			if !aCle {
				return chemins, errors.New("accolade ouvrante sans nom de bloc")
			}
			aCle = false
			profondeur++
			blocs++
		case jetonFerme:
			if aCle {
				return chemins, fmt.Errorf("cle %q sans valeur", cle)
			}
			profondeur--
			if profondeur < 0 {
				return chemins, errors.New("accolade fermante sans ouvrante")
			}
		}
	}
	switch {
	case errLex != nil:
		return chemins, errLex
	case aCle:
		return chemins, fmt.Errorf("cle %q sans valeur", cle)
	case profondeur != 0:
		return chemins, errors.New("accolade ouvrante jamais fermee")
	case blocs == 0:
		return chemins, errors.New("aucun bloc VDF")
	}
	return chemins, nil
}

// jetonsVdf decoupe le texte en chaines (entre guillemets, avec `\\`, `\"`, `\n`, `\t` ;
// tout autre `\x` est garde tel quel), accolades et commentaires `//` (ignores). Un jeton
// non cite ou une chaine non fermee arrete la lecture : jetons deja lus + erreur.
func jetonsVdf(s string) ([]jetonVdf, error) {
	var jetons []jetonVdf
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '{':
			jetons = append(jetons, jetonVdf{genre: jetonOuvre})
			i++
		case c == '}':
			jetons = append(jetons, jetonVdf{genre: jetonFerme})
			i++
		case c == '"':
			var b strings.Builder
			i++
			ferme := false
			for i < len(s) && !ferme {
				switch {
				case s[i] == '"':
					ferme = true
				case s[i] == '\\' && i+1 < len(s):
					i++
					switch s[i] {
					case '\\', '"':
						b.WriteByte(s[i])
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					default:
						b.WriteByte('\\')
						b.WriteByte(s[i])
					}
				default:
					b.WriteByte(s[i])
				}
				i++
			}
			if !ferme {
				return jetons, errors.New("chaine non fermee")
			}
			jetons = append(jetons, jetonVdf{genre: jetonChaine, texte: b.String()})
		default:
			return jetons, fmt.Errorf("jeton non cite a l'octet %d", i)
		}
	}
	return jetons, nil
}
