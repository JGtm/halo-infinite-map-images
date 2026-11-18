package himap

// maillage_partage.go — LE MAILLAGE D'UNE CARTE NATIVE DONT LA GEOMETRIE VIT DANS UN MODULE PARTAGE.
//
// Live Fire (`sgh_interlock`) n'a pas sa geometrie dans son module : son dossier est un talon
// d'un demi-megaoctet sans sbsp, et la cuisson prend ses instances dans
// `pc/globals/common-rtx-new.module` (levier `moduleGeometrie` de LevelUp). Son maillage de
// navigation suit la meme regle : le module `any` de la carte n'en porte aucun
// (`ChargeMaillageNatif` rend `ErrSansMaillage`), il vit dans le module `any` homonyme du module
// partage, `any/globals/common-rtx-new.module`.
//
// Un module partage porte les maillages de PLUSIEURS arenes : mesure du 2026-11-18, deux
// candidates lisibles, la ressource 20390 (786 faces, l'arene de Live Fire) et la ressource
// 20391 (287 faces, l'arene voisine, qui mord de 8 m sur le bsp retenu). Il faut donc choisir,
// et le choix se fait avec la meme matiere que celui du bsp (choix_bsp.go) : la boite du bsp
// retenu pour la geometrie, puis les ancres d'objectif.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/JGtm/halo-infinite-files-reader/himodule"
	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// fractionMinDansArene : part minimale des sommets d'un maillage qui doivent tomber dans la
// boite du bsp retenu pour que le maillage soit celui de l'arene. L'arene voisine de Live Fire
// n'en a qu'une fraction (elle deborde de la boite sur 8 m), le maillage de l'arene en a la
// totalite : le seuil n'a pas besoin d'etre serre.
const fractionMinDansArene = 0.5

// ChargeMaillageNatifGeometriePartagee rend le maillage de navigation de la carte native
// `module` quand sa geometrie est lue dans `moduleGeometrie` (chemin absolu d'un module `pc`
// partage, sous `<racineDeploy>/pc/globals/`).
//
// Le module `any` de la carte passe d'abord : une carte qui y porte un maillage le rend
// inchange, exactement comme `ChargeMaillageNatif`. Sinon, et seulement si `moduleGeometrie` est
// un module partage, la fonction cherche dans son homonyme `any/globals/<nom>` et retient le
// maillage de l'arene que `ChoisitBSP` retient pour la geometrie (voir `choisitMaillageArene`).
// Les autres candidates lisibles sont comptees dans `BilanMaillage.Secondaires`, les
// illisibles dans `BilanMaillage.Illisibles`.
//
// `ancres` est la meme liste que `OptionsCuisson.Ancres` : elle designe le bsp, puis le
// maillage. `ErrSansMaillage` (testable par errors.Is) quand aucun maillage lisible ne tombe
// dans l'arene, ou que `moduleGeometrie` n'est pas un module partage. Toute autre erreur est
// une lecture ou un decodage qui a echoue.
func ChargeMaillageNatifGeometriePartagee(ctx context.Context, racineDeploy, module, moduleGeometrie string,
	ancres [][3]float64) (*hinavmesh.Maillage, BilanMaillage, error) {
	maillage, bilan, err := ChargeMaillageNatif(ctx, racineDeploy, module)
	if !errors.Is(err, ErrSansMaillage) {
		return maillage, bilan, err
	}
	cheminAny, ok := moduleAnyPartage(racineDeploy, moduleGeometrie)
	if !ok {
		return nil, bilan, ErrSansMaillage
	}
	bsps, err := ReadModuleInstances(moduleGeometrie)
	if err != nil {
		return nil, bilan, fmt.Errorf("bsp du module de geometrie de %s : %w", module, err)
	}
	arene := ChoisitBSP(bsps, ancres).Bounds
	return chargeMaillageDuModulePartage(ctx, cheminAny, module, arene, ancres)
}

// moduleAnyPartage rend le module `any` qui porte les maillages d'un module de geometrie
// partage : `pc/globals/<nom>` -> `any/globals/<nom>`. Faux pour tout autre module (celui d'une
// carte, d'une autre variante, ou hors de la racine).
func moduleAnyPartage(racineDeploy, moduleGeometrie string) (string, bool) {
	if moduleGeometrie == "" {
		return "", false
	}
	rel, err := filepath.Rel(racineDeploy, moduleGeometrie)
	if err != nil {
		return "", false
	}
	parties := strings.Split(filepath.ToSlash(rel), "/")
	if len(parties) != 3 || parties[0] != "pc" || parties[1] != "globals" {
		return "", false
	}
	return filepath.Join(racineDeploy, "any", "globals", parties[2]), true
}

// chargeMaillageDuModulePartage decode les candidates lisibles du module `any` partage et
// retient celle de l'arene.
func chargeMaillageDuModulePartage(ctx context.Context, cheminAny, module string, arene Bounds,
	ancres [][3]float64) (*hinavmesh.Maillage, BilanMaillage, error) {
	var bilan BilanMaillage
	m, err := himodule.Open(cheminAny)
	if err != nil {
		return nil, bilan, fmt.Errorf("module any partage de %s : %w", module, err)
	}
	defer func() { _ = m.Close() }()
	ressources, illisibles, err := m.RessourcesNavmesh(ctx)
	bilan.Illisibles = illisibles
	if err != nil {
		return nil, bilan, fmt.Errorf("ressources de maillage du module partage de %s : %w", module, err)
	}
	var maillages []*hinavmesh.Maillage
	var octets, indices []int
	for _, r := range ressources {
		brut, err := m.Extract(r)
		if err == nil {
			var mm *hinavmesh.Maillage
			if mm, err = hinavmesh.DecodeNatif(brut); err == nil {
				maillages, octets, indices = append(maillages, mm), append(octets, len(brut)), append(indices, r.Index)
				continue
			}
		}
		bilan.Illisibles++
		slog.WarnContext(ctx, "maillage du module partage non decodable", "module", module, "index", r.Index, "err", err)
	}
	if illisibles > 0 {
		slog.WarnContext(ctx, fmt.Sprintf("maillage de l'arene possiblement illisible : %d candidates non extraites", illisibles),
			"module", module)
	}
	choix := choisitMaillageArene(maillages, arene, ancres)
	if choix < 0 {
		slog.InfoContext(ctx, "module partage sans maillage dans l'arene de la carte", "module", module,
			"candidates", len(maillages), "illisibles", bilan.Illisibles)
		return nil, bilan, ErrSansMaillage
	}
	bilan.Ressource, bilan.Octets, bilan.Secondaires = indices[choix], octets[choix], len(maillages)-1
	slog.InfoContext(ctx, "maillage de navigation du module partage charge", "module", module,
		"ressource", bilan.Ressource, "sommets", len(maillages[choix].Sommets), "faces", len(maillages[choix].Faces),
		"secondaires", bilan.Secondaires, "illisibles", bilan.Illisibles)
	return maillages[choix], bilan, nil
}

// choisitMaillageArene rend l'index du maillage de l'arene `arene` (la boite du bsp retenu par
// ChoisitBSP), ou -1 si aucun n'en est.
//
// Un maillage est ELIGIBLE quand au moins `fractionMinDansArene` de ses sommets tombent dans la
// boite de l'arene (vue de dessus : la hauteur d'un maillage n'est pas celle de la boite du
// bsp). Parmi les eligibles, la meme regle que ChoisitBSP : le plus d'ANCRES dans la boite du
// maillage ; a egalite, la part de sommets dans l'arene la plus haute ; a egalite encore,
// l'emprise au sol la plus petite. Les ancres de Live Fire tiennent toutes dans l'emprise de son
// maillage et aucune dans celle de l'arene voisine.
func choisitMaillageArene(maillages []*hinavmesh.Maillage, arene Bounds, ancres [][3]float64) int {
	meilleur := -1
	var mAncres int
	var mPart, mEmprise float64
	for i, m := range maillages {
		part := partDansArene(m, arene)
		if part < fractionMinDansArene {
			continue
		}
		boite := Bounds{Min: [3]float64{m.Min.X, m.Min.Y, 0}, Max: [3]float64{m.Max.X, m.Max.Y, 0}}
		n := ancresDansBoiteXY(boite, ancres)
		emprise := EmpriseAuSol(boite)
		if meilleur < 0 || n > mAncres || (n == mAncres && (part > mPart || (part == mPart && emprise < mEmprise))) {
			meilleur, mAncres, mPart, mEmprise = i, n, part, emprise
		}
	}
	return meilleur
}

// partDansArene rend la part des sommets du maillage dont (x, y) tombe dans la boite de l'arene.
func partDansArene(m *hinavmesh.Maillage, arene Bounds) float64 {
	if len(m.Sommets) == 0 {
		return 0
	}
	dedans := 0
	for _, s := range m.Sommets {
		if s.X >= arene.Min[0] && s.X <= arene.Max[0] && s.Y >= arene.Min[1] && s.Y <= arene.Max[1] {
			dedans++
		}
	}
	return float64(dedans) / float64(len(m.Sommets))
}

// ancresDansBoiteXY compte les ancres dont (x, y) tombe dans la boite, hauteur ignoree.
func ancresDansBoiteXY(b Bounds, ancres [][3]float64) int {
	n := 0
	for _, a := range ancres {
		if a[0] >= b.Min[0] && a[0] <= b.Max[0] && a[1] >= b.Min[1] && a[1] <= b.Max[1] {
			n++
		}
	}
	return n
}
