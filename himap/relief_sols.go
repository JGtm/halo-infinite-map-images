package himap

// relief_sols.go — LES SOLS DU MAILLAGE DANS LES DONNEES DU RELIEF, ET LES ETAGES QUI S'EN DEDUISENT.
//
// CalculeSols remplit DonneesRelief.Floors (conception §1.2) : les faces gardees du maillage qui
// touchent le cadre publie, en sommets indexes au centieme d'u (repere monde), et les etages detectes
// sur la pile de sols par pixel (relief_sols_pile.go, relief_sols_etages.go). Le web en tire la pile
// des sols (coupe par etage), le sol porteur du joueur ouvert et les murs de la coupe.
//
// FACES. Le parcours des faces gardees est celui de A.1 (trianglesDesFacesGardees) : chaque face est
// un eventail de triangles depuis son premier sommet, et chaque triangle est une entree de
// Floors.Faces. La hauteur d'un point d'une face est donc celle de l'eventail, ce que la pile des
// sols de la cuisson et celle du web calculent aussi. Les sommets sont dedoublonnes par leur valeur
// au centieme d'u ; la pile est calculee sur ces memes sommets, pour que la cuisson et le web
// voient les memes sols.

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

const (
	// FusionSols : ecart, en u, sous lequel deux sols empiles sur un pixel n'en font qu'un (le plus
	// haut est gardee). Ecrit dans Floors.Fusion : le web reconstruit la pile avec la meme valeur.
	FusionSols = 0.4

	// CouchesPile : nombre de sols empiles qu'un pixel garde. Au-dela, DegradationPileTronquee.
	CouchesPile = 4

	// PasHistogrammeEtages : largeur, en u, d'une case de l'histogramme des hauteurs de sol.
	PasHistogrammeEtages = 0.25

	// PartMinEtage : part des echantillons de sol, en compte lisse, qu'une case doit depasser pour
	// etre un etage.
	PartMinEtage = 0.03

	// EcartMinEtages : ecart minimal, en u, entre deux etages : deux etages plus proches que deux
	// fois la bande de la coupe (0,7 u) montrent la meme chose.
	EcartMinEtages = 1.4

	// MaxEtages : nombre maximal d'etages detectes.
	MaxEtages = 5

	// DemiFenetreEtage : demi-largeur, en u, de la fenetre autour d'un pic dont la mediane des
	// echantillons est la hauteur de l'etage.
	DemiFenetreEtage = 0.25

	// echelleSommets : les sommets de Floors sont des entiers au centieme d'u.
	echelleSommets = 100.0
)

// DegradationPileTronquee : un pixel a plus de CouchesPile sols distincts ; les plus bas, apres les
// premiers poses, sont ignores de la pile des etages.
const DegradationPileTronquee = "pile-tronquee"

// CalculeSols remplit rel.Donnees.Floors avec les sols du maillage `m` et les etages detectes.
//
// `m` et `garde` sont ceux de CalculeRelief (nil et nil : carte sans maillage). Sont rendus dans
// Floors :
//
//   - Sommets et Faces : les faces gardees dont la boite touche le cadre publie, au format de la
//     conception §1.2 (x, y, z a plat au centieme d'u, repere monde ; une face = trois indices) ;
//   - Fusion : FusionSols ;
//   - Niveaux : la hauteur de chaque etage, du BAS vers le HAUT, en u. Ce sont des hauteurs, pas des
//     noms : le consommateur en tire des rangs (le plus bas est le rez, puis un etage 1, un etage 2,
//     et ainsi de suite), la brique n'ecrit aucun libelle ;
//   - Min et Max : le 1er et le 99e centile des hauteurs de sol (bornes du curseur).
//
// Sans face gardee dans le cadre (ou sans pixel qu'un sol couvre), Floors reste vide, Min et Max sont
// nil, et DegradationSansMaillage est posee si CalculeRelief ne l'a pas posee. Un pixel qui empile
// plus de CouchesPile sols pose DegradationPileTronquee. Le resultat est deterministe, et un second
// appel refait le meme calcul.
func (rel *Relief) CalculeSols(ctx context.Context, m *hinavmesh.Maillage, garde []bool) error {
	cal := rel.calage
	if rel.Image == nil || cal.Largeur <= 0 || cal.Hauteur <= 0 {
		return errors.New("relief : les sols se calculent apres CalculeRelief")
	}
	if m != nil {
		if err := verifieIndicesDeSommets(m); err != nil {
			return err
		}
	}
	rel.Donnees.Floors = floorsVide(rel.Donnees.Floors)
	sommets, faces := sommetsEtFacesDuCadre(trianglesDesFacesGardees(m, garde), cal)
	pile := empileLesSols(sommets, faces, cal)
	niveaux, bas, haut, ok := etagesDeLaPile(pile)
	if !ok {
		rel.ajouteDegradation(DegradationSansMaillage)
		slog.WarnContext(ctx, "relief : aucun sol du maillage dans le cadre, ni etage ni sol")
		return nil
	}
	f := &rel.Donnees.Floors
	f.Niveaux, f.Min, f.Max, f.Fusion = niveaux, &bas, &haut, FusionSols
	f.Sommets, f.Faces = sommets, faces
	if pile.debordes > 0 {
		rel.ajouteDegradation(DegradationPileTronquee)
	}
	slog.InfoContext(ctx, "relief : sols calcules", "sommets", len(sommets)/3, "faces", len(faces),
		"etages", len(niveaux), "min", bas, "max", haut, "echantillons_ecartes", pile.debordes)
	return nil
}

// floorsVide rend des Floors sans sols ni etages : seuls Tolerance et PartMaillage, que CalculeRelief
// a remplis, sont gardes. Les listes sont vides et non nulles (`[]` dans le JSON).
func floorsVide(f EtagesRelief) EtagesRelief {
	return EtagesRelief{
		Niveaux: []float64{}, Sommets: []int{}, Faces: [][]int{},
		Tolerance: f.Tolerance, PartMaillage: f.PartMaillage,
	}
}

// ajouteDegradation ajoute un code aux degradations s'il n'y est pas deja.
func (rel *Relief) ajouteDegradation(code string) {
	if !slices.Contains(rel.Donnees.Degradations, code) {
		rel.Donnees.Degradations = append(rel.Donnees.Degradations, code)
	}
}

// sommetsEtFacesDuCadre indexe les triangles qui touchent le cadre publie : sommets au centieme d'u
// (x, y, z a plat, dans l'ordre de premiere apparition) et, par triangle, ses trois indices. Un
// triangle dont deux sommets tombent sur le meme centieme est ecarte, sans creer de sommet.
func sommetsEtFacesDuCadre(tris [][3]hinavmesh.Point, cal CalageFond) ([]int, [][]int) {
	sommets, faces := []int{}, [][]int{}
	index := map[[3]int]int{}
	indice := func(c [3]int) int {
		i, ok := index[c]
		if !ok {
			i = len(index)
			index[c] = i
			sommets = append(sommets, c[0], c[1], c[2])
		}
		return i
	}
	for _, t := range tris {
		if !toucheLeCadre(t, cal) {
			continue
		}
		a, b, c := auCentieme(t[0]), auCentieme(t[1]), auCentieme(t[2])
		if a == b || b == c || a == c {
			continue
		}
		faces = append(faces, []int{indice(a), indice(b), indice(c)})
	}
	return sommets, faces
}

// auCentieme arrondit un point, en u, au centieme d'u.
func auCentieme(p hinavmesh.Point) [3]int {
	return [3]int{centieme(p.X), centieme(p.Y), centieme(p.Z)}
}

// centieme arrondit une coordonnee en u au centieme d'u.
func centieme(v float64) int { return int(math.Round(v * echelleSommets)) }

// toucheLeCadre dit si la boite du triangle touche le rectangle du cadre publie (bords inclus).
func toucheLeCadre(t [3]hinavmesh.Point, cal CalageFond) bool {
	x1 := cal.OrigineX + float64(cal.Largeur)*cal.MetresParPixel
	y0 := cal.OrigineY - float64(cal.Hauteur)*cal.MetresParPixel
	return max(t[0].X, t[1].X, t[2].X) >= cal.OrigineX && min(t[0].X, t[1].X, t[2].X) <= x1 &&
		max(t[0].Y, t[1].Y, t[2].Y) >= y0 && min(t[0].Y, t[1].Y, t[2].Y) <= cal.OrigineY
}
