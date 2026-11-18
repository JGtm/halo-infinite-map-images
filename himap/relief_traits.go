package himap

// relief_traits.go — LES TRAITS VECTORIELS DU RELIEF : RUPTURES, COURBES DE NIVEAU, LISERE.
//
// Les traits se calculent a la cuisson, une fois, dans la grille du cadre PUBLIE (celle de l'image du
// relief), a partir de la hauteur du dessus DEJA quantifiee (celle que le web relit), de la hauteur
// connue et de la zone gardee. Trois familles :
//
//   - les RUPTURES : les sauts de hauteur de plus de SeuilRupture entre deux pixels voisins, poses sur
//     les pas du treillis (relief_traits_ruptures.go) ;
//   - les COURBES DE NIVEAU : tous les EquidistanceCourbes, par la marche des carres sur les hauteurs
//     lissees, la ou la pente depasse PenteMinCourbe, a plus de RayonSansCourbePresDUneRupture de toute
//     rupture (sans quoi le lissage double chaque mur de courbes) (relief_traits_courbes.go) ;
//   - le LISERE : le bord de la zone gardee, en anneaux fermes (relief_traits_lisere.go).
//
// Chaque famille est un graphe de segments dont les noeuds ont un identifiant entier, partage par
// construction entre deux cellules voisines : le chainage (relief_traits_graphe.go) parcourt les noeuds
// par identifiant croissant, jamais une map, et la sortie est la meme a l'octet d'une execution a
// l'autre. Viennent ensuite les miettes, la simplification de Douglas-Peucker et l'encodage en dixiemes
// de pixel (relief_traits_simplifie.go).
//
// Repere des traits : x vers la droite, y vers le bas, en pixels du cadre publie, l'entier au CENTRE
// du pixel. Une rupture entre les colonnes 10 et 11 passe a x = 10,5.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
)

const (
	// SeuilRupture : ecart de hauteur, en u, entre deux pixels voisins au-dela duquel un trait de
	// rupture passe entre eux. Valeur validee a l'ecran sur la maquette ; la longueur des ruptures
	// decroit sans creux de 0,25 a 0,5 u (Recharge seul a un creux d'histogramme vers 0,25 u).
	SeuilRupture = 0.35

	// EquidistanceCourbes : ecart de hauteur, en u, entre deux courbes de niveau (un quart d'etage).
	// Les niveaux sont les multiples entiers de cette valeur.
	EquidistanceCourbes = 0.5

	// PenteMinCourbe : pente minimale (u de hauteur par u de distance, sans dimension) d'une cellule
	// pour qu'elle porte une courbe. Elle n'ecarte que le bruit d'un sol plat pose sur un niveau : de
	// 0,04 a 0,2, la longueur des courbes ne bouge que de 1 %.
	PenteMinCourbe = 0.08

	// RayonLissageCourbes : demi-cote, en pixels, du carre de lissage des hauteurs des courbes (2 : un
	// carre de 5 x 5, moyenne des seuls pixels de hauteur connue). Le web lisse l'ombrage au meme carre.
	RayonLissageCourbes = 2

	// ToleranceTraits : ecart maximal, en PIXELS, entre un trait simplifie et le trait brut
	// (Douglas-Peucker). En pixels parce qu'elle efface un defaut de trame : les noeuds tombent sur des
	// demi-pixels, et un bord oblique dessine des marches d'un demi-pixel au plus. 0,75 px vaut de
	// 0,022 u (Recharge) a 0,037 u (Behemoth).
	ToleranceTraits = 0.75

	// LongueurMinRupture : longueur totale, en u, sous laquelle une COMPOSANTE CONNEXE de ruptures
	// (toutes ses chaines, jonctions comprises) est une miette et s'ote. La composante, pas la chaine :
	// un mur fait de dix chaines courtes entre onze jonctions n'est pas une miette.
	LongueurMinRupture = 0.5

	// LongueurMinCourbe : longueur, en u, sous laquelle une CHAINE de courbe est une miette (le
	// clignotement de la pente autour de PenteMinCourbe) et s'ote.
	LongueurMinCourbe = 0.5

	// dixiemesParPixel : resolution de l'encodage des polylignes (dixiemes de pixel entiers).
	dixiemesParPixel = 10
)

// FamilleTraits : ce que le calcul d'une famille de traits a mesure. Les longueurs sont en u.
type FamilleTraits struct {
	// Segments et LongueurBrute : les segments du graphe, avant chainage ni miettes.
	Segments      int
	LongueurBrute float64
	// Chaines : les chaines du graphe ; Miettes : celles otees (pour les ruptures, toutes les chaines
	// des composantes otees ; pour le lisere, les anneaux sous AireMinAnneau).
	Chaines, Miettes int
	// Longueur : longueur des chaines gardees, avant simplification.
	Longueur float64
	// Polylignes et Points : ce qui est ecrit, apres simplification et encodage.
	Polylignes, Points int
}

// BilanTraits est ce que CalculeTraits a mesure : de quoi journaliser une carte et tenir les tests.
type BilanTraits struct {
	Ruptures, Courbes, Liseres FamilleTraits
	// Jonctions : cellules ou trois pas ou plus sont des ruptures (avant miettes) ; JonctionsGardees :
	// celles dont la composante survit aux miettes.
	Jonctions, JonctionsGardees int
}

// grilleTraits : les entrees des traits, dans le cadre publie, indice p = j*w + i.
type grilleTraits struct {
	w, h  int
	mpp   float64
	z0    float64
	haut  []float64
	connu []bool
	zone  []bool
}

// CalculeTraits calcule les traits du relief (ruptures, courbes de niveau, lisere) et remplit
// rel.Donnees.Lines. Il se lance apres CalculeRelief, sur le relief que celui-ci a rendu ; il lit la
// hauteur quantifiee, la hauteur connue et rel.Zone, et ne change rien d'autre. Polylignes en dixiemes
// de pixel du cadre publie, premier couple absolu puis ecarts (TraitsRelief) ; une chaine fermee de
// rupture ou de courbe se termine sur son premier point, un anneau du lisere ne le repete pas.
// Deterministe : meme relief, meme sortie a l'octet.
func (rel *Relief) CalculeTraits(ctx context.Context) (BilanTraits, error) {
	g, err := rel.grilleDesTraits()
	if err != nil {
		return BilanTraits{}, err
	}
	var b BilanTraits
	ruptures, jonctions := g.ruptures()
	b.Jonctions = len(jonctions)
	rel.Donnees.Lines.Ruptures, b.JonctionsGardees = g.traitsDeRupture(ruptures, jonctions, &b.Ruptures)
	rel.Donnees.Lines.Courbes = g.traitsDeCourbe(g.courbes(), &b.Courbes)
	rel.Donnees.Lines.Liseres = g.traitsDeLisere(g.lisere(), &b.Liseres)
	slog.InfoContext(ctx, "traits du relief calcules",
		"ruptures", b.Ruptures.Polylignes, "ruptures_points", b.Ruptures.Points,
		"ruptures_longueur_brute", b.Ruptures.LongueurBrute, "ruptures_longueur", b.Ruptures.Longueur,
		"jonctions", b.Jonctions, "jonctions_gardees", b.JonctionsGardees,
		"courbes", b.Courbes.Polylignes, "courbes_points", b.Courbes.Points, "courbes_longueur", b.Courbes.Longueur,
		"lisere", b.Liseres.Polylignes, "lisere_points", b.Liseres.Points, "lisere_miettes", b.Liseres.Miettes)
	return b, nil
}

// grilleDesTraits rassemble les entrees des traits, et refuse un relief que CalculeRelief n'a pas rendu.
func (rel *Relief) grilleDesTraits() (*grilleTraits, error) {
	if rel == nil {
		return nil, errors.New("traits du relief : relief absent")
	}
	cal := rel.calage
	n := cal.Largeur * cal.Hauteur
	if n <= 0 || cal.MetresParPixel <= 0 {
		return nil, errors.New("traits du relief : relief sans cadre (CalculeRelief n'a pas tourne)")
	}
	if len(rel.hauteur) != n || len(rel.connu) != n || len(rel.Zone) != n {
		return nil, fmt.Errorf("traits du relief : hauteurs %d, connus %d, zone %d pour un cadre de %d pixels",
			len(rel.hauteur), len(rel.connu), len(rel.Zone), n)
	}
	return &grilleTraits{
		w: cal.Largeur, h: cal.Hauteur, mpp: cal.MetresParPixel, z0: rel.Donnees.Height.Z0,
		haut: rel.hauteur, connu: rel.connu, zone: rel.Zone,
	}, nil
}

// traitsDeRupture chaine les ruptures, ote les composantes de moins de LongueurMinRupture, simplifie et
// encode. Rend aussi le nombre de jonctions dont la composante est gardee.
func (g *grilleTraits) traitsDeRupture(t trace, jonctions []int64, f *FamilleTraits) ([][]int, int) {
	gr := construitGraphe(t)
	chaines := gr.chaines()
	g.mesureBrute(gr, len(chaines), f)
	composante, longueurs := gr.composantes(g.mpp)
	gardees := make([]chaine, 0, len(chaines))
	for _, c := range chaines {
		if longueurs[composante[c.noeuds[0]]] < LongueurMinRupture {
			f.Miettes++
			continue
		}
		gardees = append(gardees, c)
	}
	jonctionsGardees := 0
	for _, id := range jonctions {
		if k, ok := gr.indice(id); ok && longueurs[composante[k]] >= LongueurMinRupture {
			jonctionsGardees++
		}
	}
	return g.ecrit(gr, gardees, true, f), jonctionsGardees
}

// traitsDeCourbe chaine les courbes, ote les chaines de moins de LongueurMinCourbe, simplifie et encode.
func (g *grilleTraits) traitsDeCourbe(t trace, f *FamilleTraits) [][]int {
	gr := construitGraphe(t)
	chaines := gr.chaines()
	g.mesureBrute(gr, len(chaines), f)
	gardees := make([]chaine, 0, len(chaines))
	for _, c := range chaines {
		if gr.longueur(c)*g.mpp < LongueurMinCourbe {
			f.Miettes++
			continue
		}
		gardees = append(gardees, c)
	}
	return g.ecrit(gr, gardees, true, f)
}

// traitsDeLisere chaine le bord de la zone, ote les anneaux de moins de AireMinAnneau, simplifie et
// encode. L'aire d'un anneau se juge comme celle d'une miette de zone (eteintLesMiettes) : en pixels,
// l'anneau qui passe au milieu des aretes du treillis entoure un demi-pixel de moins que les pixels de
// sa composante (un quart de pixel par coin, quatre coins de plus que de coins rentrants). Une
// composante gardee par la zone garde ainsi toujours son anneau, et un trou de la zone de moins de
// AireMinAnneau reste un trou, sans anneau.
func (g *grilleTraits) traitsDeLisere(t trace, f *FamilleTraits) [][]int {
	gr := construitGraphe(t)
	chaines := gr.chaines()
	g.mesureBrute(gr, len(chaines), f)
	pixelsMin := pixelsMinDUnAnneau(g.mpp)
	gardees := make([]chaine, 0, len(chaines))
	for _, c := range chaines {
		if gr.aire(c)+0.5 < pixelsMin {
			f.Miettes++
			continue
		}
		gardees = append(gardees, c)
	}
	return g.ecrit(gr, gardees, false, f)
}

// pixelsMinDUnAnneau : nombre de pixels a partir duquel une composante de la zone garde son anneau,
// la regle de eteintLesMiettes (n * mpp2 >= AireMinAnneau).
func pixelsMinDUnAnneau(mpp float64) float64 {
	return math.Ceil(AireMinAnneau/(mpp*mpp) - 1e-9)
}

// mesureBrute pose les segments, la longueur brute et le nombre de chaines d'une famille.
func (g *grilleTraits) mesureBrute(gr *graphe, chaines int, f *FamilleTraits) {
	f.Segments = len(gr.bouts)
	f.LongueurBrute = gr.longueurTotale() * g.mpp
	f.Chaines = chaines
}

// ecrit simplifie et encode les chaines gardees, puis trie les polylignes par (y0, x0). `repete` : une
// chaine fermee se termine sur son premier point (ruptures et courbes, que le web trace en polylignes
// ouvertes) ; sans, un anneau ne le repete pas (lisere).
func (g *grilleTraits) ecrit(gr *graphe, gardees []chaine, repete bool, f *FamilleTraits) [][]int {
	out := make([][]int, 0, len(gardees))
	for _, c := range gardees {
		f.Longueur += gr.longueur(c) * g.mpp
		pts := simplifieChaine(gr, c, ToleranceTraits, repete)
		if p := encodePolyligne(pts); p != nil {
			out = append(out, p)
			f.Points += len(p) / 2
		}
	}
	slices.SortFunc(out, comparePolylignes)
	f.Polylignes = len(out)
	return out
}

// comparePolylignes ordonne deux polylignes encodees par (y0, x0), puis par leurs ecarts : un ordre
// total, donc une sortie qui ne depend pas de l'ordre de calcul.
func comparePolylignes(a, b []int) int {
	if c := cmp.Compare(a[1], b[1]); c != 0 {
		return c
	}
	if c := cmp.Compare(a[0], b[0]); c != 0 {
		return c
	}
	return slices.Compare(a[2:], b[2:])
}
