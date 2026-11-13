package himap

// maillage_natif.go — LE MAILLAGE DE NAVIGATION D'UNE CARTE NATIVE.
//
// Une carte Forge publie un `navmesh.blob` ; une carte native le cuit dans une ressource sans
// groupe de son module `any` (le module `pc`, lui, porte la geometrie de rendu). Ce fichier fait
// le pont : ouvrir le module `any`, choisir la ressource, la decoder. Le maillage obtenu se
// passe a `OptionsCuisson.Navmesh`.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/JGtm/halo-infinite-files-reader/himodule"
	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// ErrSansMaillage : la carte n'a pas de maillage de navigation dans son module `any`
// (`sgh_interlock` et `academy_tutorial`). Ce n'est pas un echec de cuisson : l'appelant cuit
// sans maillage et consigne la degradation.
var ErrSansMaillage = errors.New("himap: aucun maillage de navigation dans le module de la carte")

// BilanMaillage chiffre le choix de la ressource de maillage d'une carte native.
type BilanMaillage struct {
	// Ressource : index (dans le module) de la ressource decodee, la plus grosse.
	Ressource int
	// Octets : taille decompressee de cette ressource.
	Octets int
	// Secondaires : ressources de maillage en plus de la principale (chasm, va_launchsite,
	// ridgeline en portent). Elles ne sont pas decodees.
	Secondaires int
	// Illisibles : candidates que le lecteur n'a pas pu extraire.
	Illisibles int
}

// ChargeMaillageNatif rend le maillage de navigation de la carte native `module` (nom de son
// dossier, ex. `ctf_bazaar`), lu dans `<racineDeploy>/any/levels/multi/<module>/<module>-rtx-new.module`.
//
// `ErrSansMaillage` (testable par errors.Is) quand la carte n'en a pas. Toute autre erreur est
// une lecture ou un decodage qui a echoue : le module n'a pas pu etre ouvert, aucune
// ressource candidate n'a ete lisible, ou la principale n'est pas un maillage decodable.
func ChargeMaillageNatif(ctx context.Context, racineDeploy, module string) (*hinavmesh.Maillage, BilanMaillage, error) {
	var bilan BilanMaillage
	chemin := filepath.Join(racineDeploy, "any", "levels", "multi", module, module+"-rtx-new.module")
	m, err := himodule.Open(chemin)
	if err != nil {
		return nil, bilan, fmt.Errorf("module any de %s : %w", module, err)
	}
	defer func() { _ = m.Close() }()
	ressources, illisibles, err := m.RessourcesNavmesh(ctx)
	bilan.Illisibles = illisibles
	if err != nil {
		return nil, bilan, fmt.Errorf("ressources de maillage de %s : %w", module, err)
	}
	if len(ressources) == 0 {
		slog.InfoContext(ctx, "carte native sans maillage de navigation", "module", module, "illisibles", illisibles)
		return nil, bilan, ErrSansMaillage
	}
	principale := ressources[0]
	octets, err := m.Extract(principale)
	if err != nil {
		return nil, bilan, fmt.Errorf("extraction du maillage de %s : %w", module, err)
	}
	maillage, err := hinavmesh.DecodeNatif(octets)
	if err != nil {
		return nil, bilan, fmt.Errorf("decodage du maillage de %s : %w", module, err)
	}
	bilan.Ressource, bilan.Octets, bilan.Secondaires = principale.Index, len(octets), len(ressources)-1
	slog.InfoContext(ctx, "maillage de navigation natif charge", "module", module,
		"sommets", len(maillage.Sommets), "faces", len(maillage.Faces),
		"secondaires", bilan.Secondaires, "illisibles", illisibles)
	return maillage, bilan, nil
}
