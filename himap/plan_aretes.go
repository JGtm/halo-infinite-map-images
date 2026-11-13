package himap

// plan_aretes.go — LES ARETES DU PLAN A HAUTEURS.
//
// Une arete que UNE seule face gardee porte est un bord : mur, vide ou limite de la zone. Elle
// est tracee en encre, a l'epaisseur d'un trait de plan. Quand un niveau plus haut la recouvre
// au pixel (un pont au-dessus, une corniche), l'encre mentirait : on la trace en gris et en
// pointille, ce que le dessin technique appelle une arete cachee.

import (
	"image"
	"image/color"
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

var (
	// teinteAreteEncre : le trait d'une arete visible.
	teinteAreteEncre = color.RGBA{0x26, 0x32, 0x34, 255}
	// teinteAreteCachee : le trait d'une arete recouverte, plus pale pour passer au second plan.
	teinteAreteCachee = color.RGBA{0x68, 0x72, 0x72, 255}
)

const (
	// epaisseurAreteMetres : 7 cm, l'epaisseur d'un trait de plan a la plume fine ; au moins un
	// pixel, quelle que soit l'echelle.
	epaisseurAreteMetres = 0.07
	// ecartAreteCacheeMetres : une arete est recouverte quand le dessus du pixel la depasse de
	// plus que cet ecart. 1,5 m est un etage bas : en dessous, c'est la pente de la face ou le
	// bruit de la rasterisation, pas un niveau superpose.
	ecartAreteCacheeMetres = 1.5
	// pasPointille : pixels traces puis pixels sautes, le long de l'arete cachee. Six pixels se
	// lisent comme un pointille a toutes les echelles du fond (de 3 a 9 cm par pixel).
	pasPointille = 6
)

// traceLesAretes trace dans `img` les aretes de bord des faces retenues.
func (p *planHauteurs) traceLesAretes(img *image.RGBA) {
	vues := make(map[paireSommets]int)
	pourChaqueArete(p.m, p.garde, func(i, j int32) { vues[cleArete(i, j)]++ })
	ep := max(1, int(math.Round(epaisseurAreteMetres/p.cal.MetresParPixel)))
	pourChaqueArete(p.m, p.garde, func(i, j int32) {
		if vues[cleArete(i, j)] == 1 {
			p.traceArete(img, p.m.Sommets[i], p.m.Sommets[j], ep)
		}
	})
}

// traceArete trace une arete de a vers b, pixel par pixel le long de son grand axe, l'altitude
// interpolee le long du segment decidant de l'encre ou du pointille.
func (p *planHauteurs) traceArete(img *image.RGBA, a, b hinavmesh.Point, ep int) {
	cal := p.cal
	ux0, uy0 := (a.X-cal.OrigineX)/cal.MetresParPixel, (cal.OrigineY-a.Y)/cal.MetresParPixel
	ux1, uy1 := (b.X-cal.OrigineX)/cal.MetresParPixel, (cal.OrigineY-b.Y)/cal.MetresParPixel
	if math.Max(ux0, ux1) < 0 || math.Min(ux0, ux1) > float64(cal.Largeur) ||
		math.Max(uy0, uy1) < 0 || math.Min(uy0, uy1) > float64(cal.Hauteur) {
		return
	}
	// Un echantillon par pixel du grand axe, exactement : le pointille se compte en pixels.
	longueur := math.Max(math.Abs(ux1-ux0), math.Abs(uy1-uy0))
	for t := 0; t <= int(math.Ceil(longueur)); t++ {
		f := 0.0
		if longueur > 0 {
			f = math.Min(1, float64(t)/longueur)
		}
		x, y := int(math.Floor(ux0+(ux1-ux0)*f)), int(math.Floor(uy0+(uy1-uy0)*f))
		if x < 0 || y < 0 || x >= cal.Largeur || y >= cal.Hauteur {
			continue
		}
		c := teinteAreteEncre
		if p.top[y*cal.Largeur+x] > a.Z+(b.Z-a.Z)*f+ecartAreteCacheeMetres {
			if (t/pasPointille)%2 == 1 {
				continue
			}
			c = teinteAreteCachee
		}
		p.poseTrait(img, x, y, ep, c)
	}
}

// poseTrait pose un carre de `ep` pixels de cote centre sur (x, y), coupe au cadre.
func (p *planHauteurs) poseTrait(img *image.RGBA, x, y, ep int, c color.RGBA) {
	x0, y0 := x-ep/2, y-ep/2
	for dy := 0; dy < ep; dy++ {
		for dx := 0; dx < ep; dx++ {
			if px, py := x0+dx, y0+dy; px >= 0 && py >= 0 && px < p.cal.Largeur && py < p.cal.Hauteur {
				img.SetRGBA(px, py, c)
			}
		}
	}
}
