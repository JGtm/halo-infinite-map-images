package himap

// relief_sols_etages.go — LES ETAGES : PICS DE L'HISTOGRAMME DES HAUTEURS DE SOL (conception §4.5).
//
// Echantillons : une hauteur par pixel et par sol empile. Les bornes sont leur 1er et leur 99e centile.
// L'histogramme part du 1er centile, au pas de PasHistogrammeEtages (une case est CENTREE sur sa
// hauteur), lisse par [0,5 ; 1 ; 0,5]. Un pic est une case au moins egale a sa voisine de gauche,
// strictement superieure a celle de droite, et au-dessus de PartMinEtage des echantillons. Les pics se
// prennent par poids decroissant : un pic est garde s'il est a plus de EcartMinEtages de tous ceux
// deja gardes, MaxEtages au plus. La hauteur de l'etage est la MEDIANE des echantillons a
// DemiFenetreEtage du pic (le pic est une case, la mediane est le plancher).

import (
	"math"
	"slices"
	"sort"
)

// etagesDeLaPile rend les hauteurs des etages (croissantes, au centieme d'u), le 1er et le 99e
// centile des hauteurs de sol (au centieme d'u), et faux si la pile est vide.
func etagesDeLaPile(p *pileDeSols) (niveaux []float64, bas, haut float64, ok bool) {
	s := p.echantillons()
	if len(s) == 0 {
		return nil, 0, 0, false
	}
	slices.Sort(s)
	zmin, zmax := centile(s, 0.01), centile(s, 0.99)
	pics := picsDeLHistogramme(s, zmin, zmax)
	niveaux = make([]float64, 0, len(pics))
	for _, pic := range pics {
		niveaux = append(niveaux, arrondiAuCentieme(medianeAutour(s, pic)))
	}
	slices.Sort(niveaux)
	return niveaux, arrondiAuCentieme(zmin), arrondiAuCentieme(zmax), true
}

// centile rend l'echantillon de rang plancher(q * n) d'une suite triee non vide.
func centile(s []float64, q float64) float64 {
	return s[min(len(s)-1, int(math.Floor(q*float64(len(s)))))]
}

// arrondiAuCentieme arrondit une hauteur, en u, au centieme d'u.
func arrondiAuCentieme(v float64) float64 {
	return math.Round(v*echelleSommets) / echelleSommets
}

// picEtage : une case de l'histogramme, sa hauteur et son poids lisse.
type picEtage struct {
	z, poids float64
}

// picsDeLHistogramme rend les hauteurs des pics retenus, du plus lourd au plus leger : sur le plus
// grand poids, a plus de EcartMinEtages de tous les pics deja pris, MaxEtages au plus.
func picsDeLHistogramme(s []float64, zmin, zmax float64) []float64 {
	cases := make([]float64, int(math.Ceil((zmax-zmin)/PasHistogrammeEtages))+1)
	for _, z := range s {
		// Une case est centree sur zmin + b * pas : l'arrondi est celui d'une demi-case vers le haut.
		if b := int(math.Floor((z-zmin)/PasHistogrammeEtages + 0.5)); b >= 0 && b < len(cases) {
			cases[b]++
		}
	}
	lisse := func(b int) float64 {
		if b < 0 || b >= len(cases) {
			return 0
		}
		return lisseDeLaCase(cases, b)
	}
	var pics []picEtage
	for b := range cases {
		v := lisse(b)
		if v >= lisse(b-1) && v > lisse(b+1) && v > float64(len(s))*PartMinEtage {
			pics = append(pics, picEtage{z: zmin + float64(b)*PasHistogrammeEtages, poids: v})
		}
	}
	sort.SliceStable(pics, func(i, j int) bool { return pics[i].poids > pics[j].poids })
	var gardes []float64
	for _, pic := range pics {
		if len(gardes) < MaxEtages && tousALEcart(gardes, pic.z) {
			gardes = append(gardes, pic.z)
		}
	}
	return gardes
}

// lisseDeLaCase rend la case b de l'histogramme lisse par [0,5 ; 1 ; 0,5] (0 hors de l'histogramme).
func lisseDeLaCase(cases []float64, b int) float64 {
	v := cases[b]
	if b > 0 {
		v += 0.5 * cases[b-1]
	}
	if b+1 < len(cases) {
		v += 0.5 * cases[b+1]
	}
	return v
}

// tousALEcart dit si z est a plus de EcartMinEtages de chacune des hauteurs `gardes`.
func tousALEcart(gardes []float64, z float64) bool {
	for _, g := range gardes {
		if math.Abs(g-z) <= EcartMinEtages {
			return false
		}
	}
	return true
}

// medianeAutour rend la mediane des echantillons de la suite triee `s` a DemiFenetreEtage de z.
func medianeAutour(s []float64, z float64) float64 {
	lo := sort.SearchFloat64s(s, z-DemiFenetreEtage)
	hi := lo + sort.Search(len(s)-lo, func(i int) bool { return s[lo+i] > z+DemiFenetreEtage })
	if hi == lo {
		return z
	}
	return s[lo+(hi-lo)/2]
}
