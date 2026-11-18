package himap

// relief_projection.go — LE RENDU CUIT, LU DANS LE CADRE DU FOND PUBLIE.
//
// Le relief se calcule dans la grille du cadre PUBLIE, pas dans celle de la cuisson : le fond publie
// est la reference, et une recuisson peut sortir un cadre plus petit, plus grand ou decale (Recharge
// recuite : 1605 x 1736 px contre 1605 x 1771 publies, decale de 35 px en Y). Le rendu n'est lu
// qu'une fois par pixel publie, au plus proche voisin, au centre du pixel :
//
//	x = OrigineX + (px + 0,5) * mpp ;  y = OrigineY - (py + 0,5) * mpp
//	i = plancher((x - r.Min[0]) / r.Cell) ;  j = plancher((y - r.Min[1]) / r.Cell)
//
// Le j du rendu monte avec y, le py de l'image descend : pas d'inversion a faire, la formule part du
// monde. Jamais d'interpolation. Un decalage entier est une copie exacte ; un decalage non entier
// erre d'au plus un demi-pixel et se lit au bilan ; une echelle differente est refusee en amont.

import (
	"math"
)

// projetteLeRendu rend, par pixel du cadre `cal`, la hauteur du rendu `r` au centre du pixel, NaN
// la ou le rendu n'a pas de matiere ou n'a pas de grille. Remplit au bilan le decalage, les
// pixels hors grille et la matiere cuite perdue.
func projetteLeRendu(r *Rendu, cal CalageFond, b *BilanRelief) []float64 {
	w, h, mpp := cal.Largeur, cal.Hauteur, cal.MetresParPixel
	mesureLeDecalage(r, cal, b)
	colonnes := make([]int, w)
	for px := range colonnes {
		x := cal.OrigineX + (float64(px)+0.5)*mpp
		colonnes[px] = int(math.Floor((x - r.Min[0]) / r.Cell))
	}
	out := make([]float64, w*h)
	for py := 0; py < h; py++ {
		y := cal.OrigineY - (float64(py)+0.5)*mpp
		j := int(math.Floor((y - r.Min[1]) / r.Cell))
		for px, i := range colonnes {
			k := py*w + px
			if i < 0 || i >= r.NX || j < 0 || j >= r.NY {
				b.HorsGrille++
				out[k] = math.NaN()
				continue
			}
			if z, ok := r.Altitude(i, j); ok {
				out[k] = z
			} else {
				out[k] = math.NaN()
			}
		}
	}
	comptePerdue(r, cal, b)
	return out
}

// mesureLeDecalage pose le coin haut-gauche du cadre publie dans la grille cuite, en pixels.
func mesureLeDecalage(r *Rendu, cal CalageFond, b *BilanRelief) {
	ox, oy, _ := CalageDuRendu(r)
	b.DecalageX = (cal.OrigineX - ox) / cal.MetresParPixel
	b.DecalageY = (oy - cal.OrigineY) / cal.MetresParPixel
	b.DecalageEntier = estEntier(b.DecalageX) && estEntier(b.DecalageY)
}

// estEntier dit si v est un entier a toleranceDecalageEntier pres.
func estEntier(v float64) bool {
	return math.Abs(v-math.Round(v)) <= toleranceDecalageEntier
}

// comptePerdue compte la matiere cuite et celle dont le centre tombe hors du cadre publie.
func comptePerdue(r *Rendu, cal CalageFond, b *BilanRelief) {
	mpp := cal.MetresParPixel
	colonnes := make([]int, r.NX)
	for i := range colonnes {
		x := r.Min[0] + (float64(i)+0.5)*r.Cell
		colonnes[i] = int(math.Floor((x - cal.OrigineX) / mpp))
	}
	for j := 0; j < r.NY; j++ {
		y := r.Min[1] + (float64(j)+0.5)*r.Cell
		py := int(math.Floor((cal.OrigineY - y) / mpp))
		for i, px := range colonnes {
			if _, ok := r.Altitude(i, j); !ok {
				continue
			}
			b.MatiereCuite++
			if px < 0 || px >= cal.Largeur || py < 0 || py >= cal.Hauteur {
				b.MatierePerdue++
			}
		}
	}
}
