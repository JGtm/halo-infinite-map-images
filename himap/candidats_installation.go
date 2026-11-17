// Package himap — candidats_installation.go : ou Halo Infinite PEUT etre installe, et
// comment verifier qu'un dossier est bien une racine `deploy`.
//
// DeployRoot (deploy_root.go) repond a « ou est l'installation ? » avec une liste figee.
// Ce fichier repond a la question de l'appelant qui veut proposer a un utilisateur les
// installations de SON poste : il enumere les emplacements plausibles par boutique, sans
// rien valider, et ValideDeploy dit ensuite lesquels sont de vraies racines.
//
// Aucune lecture d'environnement ici. Ce qui touche le systeme (registre Windows, lecteurs
// fixes) passe par sourcesInstallation : le code propre a Windows vit dans
// candidats_installation_windows.go, un repli vide compile ailleurs, et les tests injectent
// de faux arbres.
package himap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Boutique designe la boutique ou le jeu a ete achete, donc l'organisation des dossiers.
type Boutique string

const (
	// BoutiqueSteam : bibliotheques Steam, decrites par le registre et libraryfolders.vdf.
	BoutiqueSteam Boutique = "steam"
	// BoutiqueXbox : application Xbox / Microsoft Store, `<lecteur>:\XboxGames\...`.
	BoutiqueXbox Boutique = "xbox"
)

// ErrBoutiqueInconnue : la boutique demandee n'est ni BoutiqueSteam ni BoutiqueXbox.
var ErrBoutiqueInconnue = errors.New("himap: boutique inconnue")

// ErrDeployInvalide : le dossier teste n'est pas une racine `deploy` (voir ValideDeploy).
var ErrDeployInvalide = errors.New("himap: racine deploy invalide")

// errRegistreAbsent : la cle ou la valeur n'existe pas. Steam non installe n'est pas une
// erreur a rapporter a l'appelant.
var errRegistreAbsent = errors.New("himap: valeur de registre absente")

// Ruches du registre Windows lues pour retrouver Steam.
const (
	rucheUtilisateur = "HKCU"
	rucheMachine     = "HKLM"
)

// Emplacements du dossier de Steam dans le registre : d'abord la ruche de l'utilisateur,
// puis, en repli, celle de la machine (vue 32 bits).
const (
	cleSteamUtilisateur    = `Software\Valve\Steam`
	valeurSteamUtilisateur = "SteamPath"
	cleSteamMachine        = `SOFTWARE\WOW6432Node\Valve\Steam`
	valeurSteamMachine     = "InstallPath"
)

// fichierBibliothequesSteam : liste des bibliotheques, relativement au dossier de Steam.
const fichierBibliothequesSteam = "steamapps/libraryfolders.vdf"

// sourcesInstallation regroupe ce que CandidatsInstallation demande au systeme.
type sourcesInstallation struct {
	// Registre lit une valeur chaine ; rend errRegistreAbsent si la cle ou la valeur manque.
	Registre func(ruche, chemin, valeur string) (string, error)
	// LecteursFixes rend la racine de chaque lecteur FIXE, dans l'ordre des lettres
	// (`C:\`, `D:\`...). Vide hors Windows.
	LecteursFixes func() ([]string, error)
}

// CandidatsInstallation rend les racines `deploy` candidates pour la boutique demandee,
// dans un ordre stable et sans doublon. Aucun candidat n'est valide : l'appelant les passe
// a ValideDeploy.
//
// Une erreur de lecture (registre, libraryfolders.vdf, lecteurs) n'arrete pas la recherche
// des autres emplacements : elle est rendue avec les candidats deja trouves, jointe en une
// seule erreur que l'appelant journalise. Une liste vide avec erreur nil veut dire « rien
// a proposer » (Steam absent, hors Windows...). Une boutique inconnue rend
// ErrBoutiqueInconnue.
func CandidatsInstallation(boutique Boutique) ([]string, error) {
	return candidatsInstallation(boutique, sourcesInstallation{
		Registre:      registreSysteme,
		LecteursFixes: lecteursFixesSysteme,
	})
}

func candidatsInstallation(boutique Boutique, src sourcesInstallation) ([]string, error) {
	switch boutique {
	case BoutiqueSteam:
		return candidatsSteam(src)
	case BoutiqueXbox:
		return candidatsXbox(src)
	default:
		return nil, fmt.Errorf("%w : %q", ErrBoutiqueInconnue, string(boutique))
	}
}

// candidatsSteam : le dossier de Steam, puis chaque bibliotheque de libraryfolders.vdf.
func candidatsSteam(src sourcesInstallation) ([]string, error) {
	var errs []error
	racines, err := dossierSteam(src)
	if err != nil {
		errs = append(errs, err)
	}
	if len(racines) > 0 {
		vdf := filepath.Join(racines[0], filepath.FromSlash(fichierBibliothequesSteam))
		biblios, err := lisBibliothequesSteam(vdf)
		if err != nil {
			errs = append(errs, err)
		}
		racines = append(racines, biblios...)
	}
	var liste dedoublonneur
	for _, r := range racines {
		liste.ajoute(filepath.Join(r, filepath.FromSlash(sousCheminSteam)))
	}
	return liste.chemins, errors.Join(errs...)
}

// dossierSteam rend le dossier de Steam (zero ou un element) : la ruche de l'utilisateur
// d'abord, celle de la machine seulement si la premiere n'a rien donne.
func dossierSteam(src sourcesInstallation) ([]string, error) {
	var errs []error
	sources := []struct{ ruche, cle, valeur string }{
		{rucheUtilisateur, cleSteamUtilisateur, valeurSteamUtilisateur},
		{rucheMachine, cleSteamMachine, valeurSteamMachine},
	}
	for _, s := range sources {
		v, err := src.Registre(s.ruche, s.cle, s.valeur)
		if err != nil {
			if !errors.Is(err, errRegistreAbsent) {
				errs = append(errs, fmt.Errorf("himap: registre %s\\%s : %w", s.ruche, s.cle, err))
			}
			continue
		}
		if v = strings.TrimSpace(v); v != "" {
			return []string{filepath.Clean(v)}, errors.Join(errs...)
		}
	}
	return nil, errors.Join(errs...)
}

// lisBibliothequesSteam lit les chemins `"path"` de libraryfolders.vdf. Un fichier mal forme
// rend l'erreur ET les chemins lus avant la rupture.
func lisBibliothequesSteam(chemin string) ([]string, error) {
	octets, err := os.ReadFile(chemin)
	if err != nil {
		return nil, fmt.Errorf("himap: bibliotheques Steam illisibles : %w", err)
	}
	biblios, err := analyseBibliothequesVdf(string(octets))
	if err != nil {
		return biblios, fmt.Errorf("himap: %s : %w", chemin, err)
	}
	return biblios, nil
}

// candidatsXbox : `<lecteur>\XboxGames\...` sur chaque lecteur fixe.
func candidatsXbox(src sourcesInstallation) ([]string, error) {
	lecteurs, err := src.LecteursFixes()
	if err != nil {
		err = fmt.Errorf("himap: lecteurs fixes illisibles : %w", err)
	}
	var liste dedoublonneur
	for _, l := range lecteurs {
		liste.ajoute(filepath.Join(l, filepath.FromSlash(sousCheminXbox)))
	}
	return liste.chemins, err
}

// dedoublonneur garde l'ordre d'arrivee et ecarte les chemins deja vus.
type dedoublonneur struct {
	vus     map[string]bool
	chemins []string
}

func (d *dedoublonneur) ajoute(chemin string) {
	chemin = filepath.Clean(chemin)
	cle := cleDeChemin(chemin)
	if d.vus[cle] {
		return
	}
	if d.vus == nil {
		d.vus = map[string]bool{}
	}
	d.vus[cle] = true
	d.chemins = append(d.chemins, chemin)
}

// ValideDeploy verifie que `chemin` est une racine `deploy` : `<chemin>/<variante>/levels/multi`
// doit exister pour au moins une des variantes connues (pc, ds, any). Rend nil si c'est le
// cas, sinon une erreur qui enveloppe ErrDeployInvalide et dit ce qui manque : le dossier
// lui-meme, ou chacun des dossiers de cartes attendus.
func ValideDeploy(chemin string) error {
	info, err := os.Stat(chemin)
	if err != nil {
		return fmt.Errorf("%w : %q illisible : %w", ErrDeployInvalide, chemin, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w : %q n'est pas un dossier", ErrDeployInvalide, chemin)
	}
	manquants := make([]string, 0, len(variantesDeploy))
	for _, v := range variantesDeploy {
		dir := dossierCartes(chemin, v)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return nil
		}
		manquants = append(manquants, dir)
	}
	return fmt.Errorf("%w : %q ne contient aucun dossier de cartes (manque %s)",
		ErrDeployInvalide, chemin, strings.Join(manquants, ", "))
}

// cleDeChemin : clef de comparaison de deux chemins (casse ignoree sous Windows).
func cleDeChemin(chemin string) string {
	if cheminsInsensiblesCasse {
		return strings.ToLower(chemin)
	}
	return chemin
}
