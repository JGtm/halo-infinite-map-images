package himap

// relief.go — LE RELIEF D'UNE CARTE : HAUTEUR DU DESSUS PAR PIXEL, DANS LE CALAGE DU FOND PUBLIE.
//
// Le « relief » est ce qui permet de peindre une carte vue de dessus en gris plus clair plus
// haut, ombre, decoupee a la zone jouable, et de la couper par etage. Il tient en deux objets :
//
//   - une IMAGE `<cle>.relief.webp` (voir Relief.Image) : par pixel, la hauteur du dessus sur
//     16 bits (q = R + 256 G, pas PasHauteur), deux drapeaux dans B, A valant 0 ou 255 ;
//   - des DONNEES `<cle>.relief.json` (voir DonneesRelief) : le calage auquel l'image a ete
//     projetee, les bornes de hauteur, les etages, les sols du maillage et les traits.
//
// Les principes (hauteur par pixel, ombrage, courbes de niveau, coupe par etage) et les seuils de
// depart viennent de OpenSpartan Cartographer (Den, licence MIT) ; l'algorithme est ecrit ici.
//
// CE FICHIER porte les types, les seuils et l'enchainement de CalculeRelief. Le detail vit dans
// relief_projection.go (le rendu lu dans le cadre publie), relief_sols_maillage.go (les sols du
// maillage rasterises), relief_zone.go (la zone gardee) et relief_image.go (le codage de q).
//
// UNITES. Les hauteurs et les distances du relief sont en UNITES DU JEU, notees « u ». Le reste
// de la brique les appelle « metres » (MetresParPixel) alors qu'une unite vaut environ 3 m : on
// garde les noms, on ne convertit rien.

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

const (
	// SchemaVersionRelief est la version du format des DONNEES et de l'image du relief.
	SchemaVersionRelief = 1

	// PasHauteur : pas de quantification de la hauteur, en u. 1/256 u donne 255,99 u d'amplitude
	// sur 16 bits (la tranche de jeu du rendu borne la matiere a 40 u) et une erreur d'ombrage
	// d'au plus 1,9 niveau sur 255 ; 1/128 u irait a 4,2.
	PasHauteur = 1.0 / 256

	// ToleranceDessusSol : ecart, en u, au-dela duquel le sol le plus haut du maillage l'emporte
	// sur le dessus du rendu (le dessus n'est jamais sous un sol praticable). En deca, le
	// dessus est la hauteur du rendu. Ecrit dans DonneesRelief.Floors.Tolerance : le web
	// reconstruit la pile des sols avec la meme valeur.
	ToleranceDessusSol = 0.3

	// FermetureZone : demi-largeur, en u, de la fermeture de la zone gardee : un passage plus
	// etroit que deux fois cette valeur est referme.
	FermetureZone = 0.5

	// AireMinAnneau : aire minimale, en u2, d'une composante de la zone gardee. En dessous, la
	// composante est une miette : elle perd son drapeau DrapeauSurCarte. L'item des traits
	// l'applique aussi aux anneaux du liseré.
	AireMinAnneau = 0.05

	// ToleranceEchelleRelief : ecart relatif maximal entre l'echelle du rendu cuit et celle du
	// cadre publie. Au-dela, le relief n'est pas calcule (ErrEchelleDifferente).
	ToleranceEchelleRelief = 1e-6

	// SeuilMatierePerdue : part de la matiere cuite qui tombe hors du cadre publie au-dela de
	// laquelle CalculeRelief avertit (le cadre publie est la reference, cette matiere est perdue).
	SeuilMatierePerdue = 0.01

	// toleranceDecalageEntier : ecart, en pixels, a un decalage entier du cadre publie sur la
	// grille cuite en deca duquel la projection est une copie exacte.
	toleranceDecalageEntier = 1e-3

	// amplitudeQ est le plus grand q (16 bits).
	amplitudeQ = 1<<16 - 1
)

// Drapeaux du canal B de l'image du relief. Les bits 2 a 7 sont reserves et valent toujours 0.
const (
	// DrapeauSurCarte : la hauteur est connue ET le pixel est dans la zone gardee. Seuls ces
	// pixels se teintent ; c'est aussi le masque de la coupe.
	DrapeauSurCarte uint8 = 1
	// DrapeauSolMaillage : la hauteur vient du maillage de navigation et non de la geometrie du
	// rendu (sol sans matiere, ou sol plus haut que le dessus du rendu de plus de
	// ToleranceDessusSol). Sa part est DonneesRelief.Floors.PartMaillage.
	DrapeauSolMaillage uint8 = 2
)

// Codes de DonneesRelief.Degradations (liste fermee, jamais une phrase : le web les traduit).
const (
	// DegradationSansMaillage : la carte n'a pas de maillage de navigation (ou aucune face
	// gardee dans le cadre). La zone est l'emprise de la matiere, il n'y a ni sol ni etage.
	DegradationSansMaillage = "sans-maillage"
	// DegradationHauteursEcretees : des hauteurs depassaient l'amplitude de l'image et ont ete
	// ecretees au plus haut q.
	DegradationHauteursEcretees = "hauteurs-ecretees"
)

// ErrEchelleDifferente : l'echelle du fond publie differe de celle de la cuisson. Echantillonner
// une grille fine dans une grossiere fabriquerait des marches, donc de fausses courbes de niveau :
// le fond doit etre recuit, pas le relief force.
var ErrEchelleDifferente = errors.New("echelle du fond publie differente de la cuisson, recuire le fond")

// CalageRelief est le calage du fond publie auquel le relief a ete projete, recopie dans les
// donnees : s'il differe de celui du sidecar, le relief est perime et ne doit pas etre servi.
type CalageRelief struct {
	MetresParPixel float64 `json:"metersPerPixel"`
	OrigineX       float64 `json:"originX"`
	OrigineY       float64 `json:"originY"`
	Largeur        int     `json:"widthPx"`
	Hauteur        int     `json:"heightPx"`
}

// HauteursRelief decrit le codage et la dynamique des hauteurs.
type HauteursRelief struct {
	// Z0 et Pas : hauteur = Z0 + q * Pas, avec q = R + 256 G. Z0 est la partie entiere (vers le
	// bas) de la plus petite hauteur de la carte. Remplis par CalculeRelief.
	Z0  float64 `json:"z0"`
	Pas float64 `json:"step"`
	// Bas et Haut : 2e et 98e centiles des hauteurs de la zone, en u (bornes de la rampe de
	// clarte). Remplis par CalculeRelief.
	Bas  float64 `json:"low"`
	Haut float64 `json:"high"`
}

// EtagesRelief porte les sols du maillage et les etages qui s'en deduisent.
type EtagesRelief struct {
	// Niveaux : hauteurs des etages detectes, croissantes, en u. Rempli par l'item des sols
	// (A.3). Vide sans maillage.
	Niveaux []float64 `json:"levels"`
	// Min et Max : 1er et 99e centiles des hauteurs de sol, en u (bornes du curseur). Remplis
	// par l'item des sols ; nil sans maillage (la cle est alors absente du JSON).
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
	// Fusion : ecart, en u, sous lequel deux sols empiles sur un pixel n'en font qu'un. Rempli
	// par l'item des sols.
	Fusion float64 `json:"merge"`
	// Tolerance : ToleranceDessusSol. Remplie par CalculeRelief.
	Tolerance float64 `json:"tolerance"`
	// PartMaillage : part, entre 0 et 1, de la zone dont la hauteur vient du maillage
	// (DrapeauSolMaillage). Remplie par CalculeRelief.
	PartMaillage float64 `json:"meshShare"`
	// Sommets : x, y, z a plat, en centiemes d'u entiers, repere monde. Rempli par l'item des
	// sols (A.3).
	Sommets []int `json:"vertices"`
	// Faces : par face gardee, la liste des indices de Sommets (polygone convexe). Rempli par
	// l'item des sols (A.3).
	Faces [][]int `json:"faces"`
}

// TraitsRelief porte les polylignes du relief. Une polyligne est [x0, y0, dx1, dy1, ...] en
// dixiemes de pixel entiers du cadre publie : le premier couple est absolu, les suivants sont
// l'ecart au point precedent ; l'entier designe le CENTRE du pixel. Remplis par l'item des
// traits (A.2) ; vides tant qu'il n'a pas tourne.
type TraitsRelief struct {
	// Ruptures : sauts de hauteur entre deux pixels voisins.
	Ruptures [][]int `json:"steps"`
	// Courbes : courbes de niveau sur les pentes.
	Courbes [][]int `json:"contours"`
	// Liseres : anneaux fermes du bord de la zone, sans repeter le premier point.
	Liseres [][]int `json:"outline"`
}

// DonneesRelief est le contenu de `<cle>.relief.json` (schemaVersion 1). Les cles sont celles du
// format publie ; les champs sont remplis par :
//
//   - CalculeRelief : SchemaVersion, Calibration, Height, Floors.Tolerance, Floors.PartMaillage,
//     Degradations ;
//   - l'item des sols (A.3) : Floors.Niveaux, Min, Max, Fusion, Sommets, Faces ;
//   - l'item des traits (A.2) : Lines.
//
// Les listes sont toujours non nulles (`[]` et non `null` dans le JSON).
type DonneesRelief struct {
	SchemaVersion int            `json:"schemaVersion"`
	Calibration   CalageRelief   `json:"calibration"`
	Height        HauteursRelief `json:"height"`
	Floors        EtagesRelief   `json:"floors"`
	Lines         TraitsRelief   `json:"lines"`
	Degradations  []string       `json:"degradations"`
}

// nouvellesDonneesRelief rend des donnees dont toutes les listes sont vides et non nulles.
func nouvellesDonneesRelief(cal CalageFond) DonneesRelief {
	return DonneesRelief{
		SchemaVersion: SchemaVersionRelief,
		Calibration: CalageRelief{
			MetresParPixel: cal.MetresParPixel, OrigineX: cal.OrigineX, OrigineY: cal.OrigineY,
			Largeur: cal.Largeur, Hauteur: cal.Hauteur,
		},
		Height: HauteursRelief{Pas: PasHauteur},
		Floors: EtagesRelief{
			Niveaux: []float64{}, Sommets: []int{}, Faces: [][]int{}, Tolerance: ToleranceDessusSol,
		},
		Lines:        TraitsRelief{Ruptures: [][]int{}, Courbes: [][]int{}, Liseres: [][]int{}},
		Degradations: []string{},
	}
}

// BilanRelief est ce que CalculeRelief a mesure : de quoi journaliser une carte et tenir les
// tests. Tous les comptes sont en pixels du cadre publie, sauf mention.
type BilanRelief struct {
	// Pixels : taille du cadre. Connus : pixels dont la hauteur est connue. Zone : pixels
	// DrapeauSurCarte. SolMaillage : pixels de la zone dont la hauteur vient du maillage.
	Pixels, Connus, Zone, SolMaillage int
	// PartMaillage : SolMaillage / Zone (0 si la zone est vide).
	PartMaillage float64
	// Miettes : pixels de zone eteints parce que leur composante faisait moins de AireMinAnneau.
	Miettes int
	// Ecretes : pixels dont la hauteur depassait l'amplitude de l'image.
	Ecretes int
	// SansMaillage : aucune face gardee ne tombe dans le cadre.
	SansMaillage bool

	// DecalageX et DecalageY : position du coin haut-gauche du cadre publie dans la grille cuite,
	// en pixels (x vers la droite, y vers le bas ; negatif : le cadre publie deborde de la
	// grille). DecalageEntier : les deux sont entiers, la projection est une copie exacte.
	DecalageX, DecalageY float64
	DecalageEntier       bool
	// HorsGrille : pixels du cadre publie hors de la grille cuite (pas de hauteur du rendu).
	HorsGrille int
	// MatiereCuite : cellules de la grille cuite qui portent une altitude. MatierePerdue : celles
	// dont le centre tombe hors du cadre publie.
	MatiereCuite, MatierePerdue int

	// FondPublieMatiere et FondPublieSansHauteur : pixels de matiere du fond publie, et ceux qui
	// n'ont aucune hauteur dans le relief (trou du fond Relief face a un aplat du Rendu). Remplis
	// par Relief.MesureFondPublie, pas par CalculeRelief.
	FondPublieMatiere, FondPublieSansHauteur int
}

// Relief est le resultat de CalculeRelief : l'image, la zone et les donnees d'une carte, dans le
// calage du fond publie.
type Relief struct {
	// Image : une image de la taille du cadre publie. Par pixel : A = 0 (hauteur inconnue, R, G,
	// B nuls) ou 255 ; R = octet faible de q, G = octet fort ; B = les drapeaux.
	Image *image.NRGBA
	// Zone : le masque de la zone gardee, indice py*Largeur + px. Vrai = DrapeauSurCarte.
	Zone []bool
	// Donnees : le contenu du JSON, a completer par les items des sols et des traits.
	Donnees DonneesRelief
	// Bilan : les mesures du calcul.
	Bilan BilanRelief

	// Reserve aux calculs suivants du paquet (traits, sols), tous dans le calage `calage`, indice
	// py*Largeur + px : hauteur est la hauteur du dessus DEJA quantifiee (celle que le web
	// relit), connu dit si elle existe, depuisMaillage reprend DrapeauSolMaillage.
	calage         CalageFond
	hauteur        []float64
	connu          []bool
	depuisMaillage []bool
}

// CalculeRelief calcule le relief du rendu `r` dans le calage PUBLIE `cal`.
//
// `m` est le maillage de navigation de la carte (nil : D-7, carte sans maillage) et `garde` dit,
// par face de `m`, celles qui forment l'arene (ComposantesRetenues, calculee par l'appelant avec
// la matiere du fond publie ; nil garde toutes les faces). Le dessus de chaque pixel est :
//
//   - la hauteur du rendu, lue au plus proche voisin au centre du pixel publie (jamais
//     d'interpolation : une moyenne de part et d'autre d'un mur invente une pente) ;
//   - complete par le sol le plus haut du maillage quand le rendu n'a aucune matiere au pixel
//     (terrain comble a plat dans le fond), ou quand ce sol depasse le rendu de plus de
//     ToleranceDessusSol ; le pixel porte alors DrapeauSolMaillage.
//
// La zone gardee est le maillage elargi de MargeNavmesh, ses passages etroits refermes, ses creux
// enclos combles, ses miettes eteintes ; sans maillage, l'emprise de la matiere.
//
// Une echelle differente entre le rendu et le cadre rend ErrEchelleDifferente et rien d'autre.
// Un cadre decale ou plus grand que la grille n'est pas une erreur : ses pixels hors grille n'ont
// pas de hauteur du rendu, et le bilan les compte.
func CalculeRelief(ctx context.Context, r *Rendu, m *hinavmesh.Maillage, garde []bool, cal CalageFond) (*Relief, error) {
	if err := verifieEntreesRelief(r, m, cal); err != nil {
		return nil, err
	}
	rel := &Relief{calage: cal, Donnees: nouvellesDonneesRelief(cal)}
	rel.Bilan.Pixels = cal.Largeur * cal.Hauteur
	dessus := projetteLeRendu(r, cal, &rel.Bilan)
	sols := rasteriseLesSols(m, garde, cal)
	rel.Bilan.SansMaillage = !sols.present
	brut := rel.assembleLesHauteurs(dessus, sols)
	rel.Zone = rel.zoneGardee(sols)
	rel.encode(brut)
	rel.journalise(ctx)
	return rel, nil
}

// verifieEntreesRelief refuse un rendu absent, un cadre sans surface, une echelle differente et un
// maillage incoherent.
func verifieEntreesRelief(r *Rendu, m *hinavmesh.Maillage, cal CalageFond) error {
	if r == nil {
		return errors.New("relief : rendu absent")
	}
	if cal.Largeur <= 0 || cal.Hauteur <= 0 || cal.MetresParPixel <= 0 {
		return fmt.Errorf("relief : cadre %d x %d a %g m/px sans surface", cal.Largeur, cal.Hauteur, cal.MetresParPixel)
	}
	if r.Cell <= 0 || math.Abs(r.Cell-cal.MetresParPixel)/cal.MetresParPixel > ToleranceEchelleRelief {
		return fmt.Errorf("relief : cuisson a %g m/px, fond publie a %g m/px : %w", r.Cell, cal.MetresParPixel, ErrEchelleDifferente)
	}
	if m != nil {
		return verifieIndicesDeSommets(m)
	}
	return nil
}

// assembleLesHauteurs fond le dessus du rendu et les sols du maillage (D-6 et D-12) : rend la
// hauteur brute par pixel (NaN si inconnue) et pose `connu` et `depuisMaillage`.
func (rel *Relief) assembleLesHauteurs(dessus []float64, sols *solsMaillage) []float64 {
	n := len(dessus)
	rel.connu = make([]bool, n)
	rel.depuisMaillage = make([]bool, n)
	brut := make([]float64, n)
	for k := range brut {
		zr := dessus[k]
		zm := sols.haut[k]
		rendu := !math.IsNaN(zr)
		maillage := !math.IsInf(zm, -1)
		switch {
		case maillage && (!rendu || zm-zr > ToleranceDessusSol):
			brut[k], rel.connu[k], rel.depuisMaillage[k] = zm, true, true
		case rendu:
			brut[k], rel.connu[k] = zr, true
		default:
			brut[k] = math.NaN()
		}
	}
	return brut
}

// journalise ecrit le bilan, et avertit des deux cas ou le relief est amoindri.
func (rel *Relief) journalise(ctx context.Context) {
	b := rel.Bilan
	slog.InfoContext(ctx, "relief calcule", "pixels", b.Pixels, "connus", b.Connus, "zone", b.Zone,
		"part_maillage", b.PartMaillage, "miettes", b.Miettes, "ecretes", b.Ecretes,
		"hors_grille", b.HorsGrille, "decalage_x", b.DecalageX, "decalage_y", b.DecalageY,
		"decalage_entier", b.DecalageEntier, "matiere_perdue", b.MatierePerdue, "matiere_cuite", b.MatiereCuite)
	if b.SansMaillage {
		slog.WarnContext(ctx, "relief sans maillage : zone = emprise de la matiere, ni sol ni etage")
	}
	if b.MatiereCuite > 0 && float64(b.MatierePerdue) > SeuilMatierePerdue*float64(b.MatiereCuite) {
		slog.WarnContext(ctx, "relief : matiere cuite hors du cadre publie", "perdue", b.MatierePerdue,
			"cuite", b.MatiereCuite, "part", float64(b.MatierePerdue)/float64(b.MatiereCuite))
	}
}

// MesureFondPublie compte, dans `matiere` (le masque de matiere du fond publie : alpha non nul,
// indice py*Largeur + px, de la taille du cadre), les pixels de matiere et ceux qui n'ont aucune
// hauteur dans le relief. Le second est le trou que le fond Relief montrera la ou le Rendu a un
// aplat de sol suppose. Remplit Bilan.FondPublieMatiere et Bilan.FondPublieSansHauteur.
func (rel *Relief) MesureFondPublie(matiere []bool) error {
	if len(matiere) != len(rel.connu) {
		return fmt.Errorf("relief : masque de matiere de %d pixels, cadre de %d", len(matiere), len(rel.connu))
	}
	var avec, sans int
	for k, ok := range matiere {
		if !ok {
			continue
		}
		avec++
		if !rel.connu[k] {
			sans++
		}
	}
	rel.Bilan.FondPublieMatiere, rel.Bilan.FondPublieSansHauteur = avec, sans
	return nil
}
