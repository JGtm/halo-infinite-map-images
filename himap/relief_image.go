package himap

// relief_image.go — LE CODAGE DE L'IMAGE DU RELIEF.
//
// Par pixel : q = round((hauteur - Z0) / PasHauteur), sur 16 bits ; R = octet faible de q, G = octet
// fort (rangement mesure : 6 a 17 % de moins en WebP sans perte que l'ordre inverse) ; B = les
// drapeaux ; A = 255 quand la hauteur est connue, 0 sinon ET alors R = G = B = 0. A ne prend jamais
// de valeur intermediaire : un canevas 2D range ses pixels premultiplies, une alpha intermediaire
// ecraserait R, G et B a la relecture.
//
// Z0 est la partie entiere (vers le bas) de la plus petite hauteur de la carte : l'octet fort se lit
// alors « unites entieres au-dessus de Z0 ».

import (
	"image"
	"math"
)

// encode ecrit l'image du relief a partir des hauteurs brutes (NaN : inconnue), de `connu`, de la zone
// et des drapeaux, pose Height, Floors.PartMaillage et les comptes du bilan, et garde les hauteurs
// quantifiees dans rel.hauteur.
func (rel *Relief) encode(brut []float64) {
	cal := rel.calage
	z0 := hauteurZero(brut, rel.connu)
	img := image.NewNRGBA(image.Rect(0, 0, cal.Largeur, cal.Hauteur))
	rel.hauteur = make([]float64, len(brut))
	histo := make([]int, amplitudeQ+1)
	for k, ok := range rel.connu {
		if !ok {
			continue
		}
		q, ecrete := quantifie(brut[k], z0)
		if ecrete {
			rel.Bilan.Ecretes++
		}
		rel.hauteur[k] = decode(q, z0)
		rel.Bilan.Connus++
		var drapeaux uint8
		if rel.Zone[k] {
			drapeaux |= DrapeauSurCarte
			histo[q]++
			rel.Bilan.Zone++
			if rel.depuisMaillage[k] {
				rel.Bilan.SolMaillage++
			}
		}
		if rel.depuisMaillage[k] {
			drapeaux |= DrapeauSolMaillage
		}
		o := k * 4
		img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(q&0xFF), uint8(q>>8), drapeaux, 255
	}
	rel.Image = img
	rel.Donnees.Height.Z0 = z0
	rel.Donnees.Height.Bas, rel.Donnees.Height.Haut = centilesDeZone(histo, rel.Bilan.Zone, z0)
	if rel.Bilan.Zone > 0 {
		rel.Bilan.PartMaillage = float64(rel.Bilan.SolMaillage) / float64(rel.Bilan.Zone)
	}
	rel.Donnees.Floors.PartMaillage = rel.Bilan.PartMaillage
	if rel.Bilan.Ecretes > 0 {
		rel.Donnees.Degradations = append(rel.Donnees.Degradations, DegradationHauteursEcretees)
	}
}

// hauteurZero rend la partie entiere, vers le bas, de la plus petite hauteur connue (0 si aucune).
func hauteurZero(brut []float64, connu []bool) float64 {
	plus := math.Inf(1)
	for k, ok := range connu {
		if ok && brut[k] < plus {
			plus = brut[k]
		}
	}
	if math.IsInf(plus, 1) {
		return 0
	}
	return math.Floor(plus)
}

// quantifie rend q pour la hauteur `h` et dit si elle a ete ecretee a l'amplitude de l'image.
func quantifie(h, z0 float64) (q int, ecrete bool) {
	v := math.Round((h - z0) / PasHauteur)
	switch {
	case v < 0:
		return 0, true
	case v > amplitudeQ:
		return amplitudeQ, true
	}
	return int(v), false
}

// decode rend la hauteur de q : Z0 + q * PasHauteur (exact en flottant : PasHauteur est une
// puissance de deux).
func decode(q int, z0 float64) float64 {
	return z0 + float64(q)*PasHauteur
}

// centilesDeZone rend le 2e et le 98e centile des hauteurs de la zone, en u, lus sur l'histogramme des
// q (rang le plus proche : le plus petit q dont l'effectif cumule depasse le rang). Zone vide : 0, 0.
func centilesDeZone(histo []int, n int, z0 float64) (bas, haut float64) {
	if n == 0 {
		return 0, 0
	}
	rangBas, rangHaut := (n-1)*2/100, (n-1)*98/100
	cumul := 0
	foundBas := false
	for q, c := range histo {
		cumul += c
		if !foundBas && cumul > rangBas {
			bas, foundBas = decode(q, z0), true
		}
		if cumul > rangHaut {
			return bas, decode(q, z0)
		}
	}
	return bas, bas
}
