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
	var a analyseurVdf
	for _, j := range jetons {
		if err := a.consomme(j); err != nil {
			return a.chemins, err
		}
	}
	return a.chemins, a.termine(errLex)
}

// analyseurVdf suit la structure des jetons : cle en attente de valeur, profondeur des blocs.
type analyseurVdf struct {
	chemins    []string
	cle        string
	aCle       bool
	profondeur int
	blocs      int
}

// consomme avance d'un jeton ; une erreur de structure arrete l'analyse.
func (a *analyseurVdf) consomme(j jetonVdf) error {
	switch j.genre {
	case jetonChaine:
		a.chaine(j.texte)
	case jetonOuvre:
		return a.ouvre()
	case jetonFerme:
		return a.ferme()
	}
	return nil
}

// chaine : premiere chaine = cle ; seconde = valeur de cette cle, retenue si c'est un `path`
// non vide dans un bloc.
func (a *analyseurVdf) chaine(texte string) {
	if !a.aCle {
		a.cle, a.aCle = texte, true
		return
	}
	if a.profondeur > 0 && strings.EqualFold(a.cle, "path") && texte != "" {
		a.chemins = append(a.chemins, texte)
	}
	a.aCle = false
}

func (a *analyseurVdf) ouvre() error {
	if !a.aCle {
		return errors.New("accolade ouvrante sans nom de bloc")
	}
	a.aCle = false
	a.profondeur++
	a.blocs++
	return nil
}

func (a *analyseurVdf) ferme() error {
	if a.aCle {
		return fmt.Errorf("cle %q sans valeur", a.cle)
	}
	a.profondeur--
	if a.profondeur < 0 {
		return errors.New("accolade fermante sans ouvrante")
	}
	return nil
}

// termine rend l'erreur de fin de lecture : celle du decoupage en jetons d'abord, puis l'etat
// structurel laisse par le dernier jeton.
func (a *analyseurVdf) termine(errLex error) error {
	switch {
	case errLex != nil:
		return errLex
	case a.aCle:
		return fmt.Errorf("cle %q sans valeur", a.cle)
	case a.profondeur != 0:
		return errors.New("accolade ouvrante jamais fermee")
	case a.blocs == 0:
		return errors.New("aucun bloc VDF")
	}
	return nil
}

// jetonsVdf decoupe le texte en chaines (entre guillemets, avec `\\`, `\"`, `\n`, `\t` ;
// tout autre `\x` est garde tel quel), accolades et commentaires `//` (ignores). Un jeton
// non cite ou une chaine non fermee arrete la lecture : jetons deja lus + erreur.
func jetonsVdf(s string) ([]jetonVdf, error) {
	l := lecteurVdf{s: s}
	var jetons []jetonVdf
	for {
		l.saute()
		if l.i >= len(l.s) {
			return jetons, nil
		}
		switch l.s[l.i] {
		case '{':
			jetons = append(jetons, jetonVdf{genre: jetonOuvre})
			l.i++
		case '}':
			jetons = append(jetons, jetonVdf{genre: jetonFerme})
			l.i++
		case '"':
			texte, ferme := l.chaine()
			if !ferme {
				return jetons, errors.New("chaine non fermee")
			}
			jetons = append(jetons, jetonVdf{genre: jetonChaine, texte: texte})
		default:
			return jetons, fmt.Errorf("jeton non cite a l'octet %d", l.i)
		}
	}
}

// lecteurVdf est le curseur du decoupage en jetons : `i` est l'octet courant de `s`.
type lecteurVdf struct {
	s string
	i int
}

// saute avance au prochain octet utile : blancs et commentaires `//` (jusqu'a la fin de ligne).
func (l *lecteurVdf) saute() {
	for l.i < len(l.s) {
		switch {
		case estBlancVdf(l.s[l.i]):
			l.i++
		case strings.HasPrefix(l.s[l.i:], "//"):
			for l.i < len(l.s) && l.s[l.i] != '\n' {
				l.i++
			}
		default:
			return
		}
	}
}

func estBlancVdf(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

// chaine lit la chaine entre guillemets qui commence a l'octet courant (le guillemet ouvrant)
// et laisse le curseur apres le guillemet fermant. `ferme` est faux si la fin du texte arrive
// avant ; une antislash final sans caractere a echapper est alors un caractere ordinaire.
func (l *lecteurVdf) chaine() (texte string, ferme bool) {
	var b strings.Builder
	l.i++
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '"':
			l.i++
			return b.String(), true
		case c == '\\' && l.i+1 < len(l.s):
			l.i++
			b.WriteString(echappementVdf(l.s[l.i]))
		default:
			b.WriteByte(c)
		}
		l.i++
	}
	return b.String(), false
}

// echappementVdf rend ce que vaut `\c` dans une chaine : `\\`, `\"`, `\n`, `\t` ; tout autre
// `\c` est garde tel quel (antislash compris).
func echappementVdf(c byte) string {
	switch c {
	case '\\', '"':
		return string(c)
	case 'n':
		return "\n"
	case 't':
		return "\t"
	}
	return "\\" + string(c)
}
