package himap

// plan_hauteurs.go — LES FACES DU PLAN A HAUTEURS : DESSUS PAR PIXEL, TEINTE, OMBRE PORTEE.
//
// Chaque face gardee est rasterisee a son altitude (la moyenne de ses sommets) ; par pixel on
// garde le dessus. La teinte suit l'altitude sur une rampe continue calee sur les centiles 2 et
// 98 des altitudes des faces gardees, pour qu'une aberration (un pylone, un fond de fosse) ne
// ecrase pas la dynamique de la carte. Un pixel dont le voisin vers le nord-ouest est nettement
// plus haut est dans l'ombre de ce niveau haut : c'est ce qui fait lire une marche comme une
// marche.

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// Rampe du bas (sombre, froid) au haut (clair, chaud) : vérifiée par simulation Machado 2009,
// clarté OKLab monotone 54 → 94, écart entre pas voisins ≥ 6,7 (×100) en protanopie,
// deutéranopie, tritanopie ; ne pas modifier sans refaire la mesure.
var rampePlanHauteurs = [...]color.RGBA{
	{0x60, 0x70, 0x8C, 255}, {0x7E, 0x8C, 0x96, 255}, {0x9E, 0xA6, 0xA0, 255},
	{0xBE, 0xBE, 0xA0, 255}, {0xDC, 0xD6, 0xA4, 255}, {0xF6, 0xEC, 0xB0, 255},
}

const (
	// ecartMinHauteurs : en dessous de cet ecart entre les centiles 2 et 98 (en metres), la
	// carte est plate pour le plan et tout recoit la teinte du milieu : une rampe etiree sur
	// quelques centimetres montrerait du bruit de maillage comme du relief.
	ecartMinHauteurs = 1.0
	// decalageOmbreMetres : portee de l'ombre vers le nord-ouest. 0,7 m est la hauteur d'une
	// marche franche : l'ombre fait un trait lisible sans manger les couloirs.
	decalageOmbreMetres = 0.7
	// decalageOmbreMinPx : a petite echelle, 0,7 m tient dans moins d'un pixel ; deux pixels
	// minimum gardent l'ombre visible.
	decalageOmbreMinPx = 2
	// ecartOmbreMetres : le niveau voisin doit etre plus haut de cet ecart pour jeter une ombre.
	// 1,2 m ecarte les pentes et les rampes (qui montent doucement) et ne garde que les marches.
	ecartOmbreMetres = 1.2
	// Facteurs d'assombrissement de l'ombre ; un peu moins sur le bleu, pour que l'ombre reste
	// neutre sur la rampe froide du bas.
	facteurOmbreRG = 0.72
	facteurOmbreB  = 0.76
)

// faceHauteur est une face gardee : son indice dans le maillage et son altitude moyenne.
type faceHauteur struct {
	idx int
	z   float64
}

// planHauteurs porte le calcul d'un plan : le maillage, la garde, les faces retenues et le
// dessus par pixel (indice py*Largeur + px, -Inf hors de toute face).
type planHauteurs struct {
	cal   CalageFond
	m     *hinavmesh.Maillage
	garde []bool
	faces []faceHauteur
	top   []float64
}

// rassembleLesFaces retient les faces gardees d'au moins trois sommets, avec leur altitude. Une
// face dont l'altitude n'est pas finie (un sommet NaN ou infini) n'est pas retenue : elle
// fausserait les centiles de la teinte, et un maillage incoherent ne doit pas faire paniquer.
func (p *planHauteurs) rassembleLesFaces() {
	for idx, f := range p.m.Faces {
		if p.garde != nil && (idx >= len(p.garde) || !p.garde[idx]) {
			continue
		}
		if len(f.Sommets) < 3 {
			continue
		}
		var somme float64
		for _, s := range f.Sommets {
			somme += p.m.Sommets[s].Z
		}
		z := somme / float64(len(f.Sommets))
		if math.IsNaN(z) || math.IsInf(z, 0) {
			continue
		}
		p.faces = append(p.faces, faceHauteur{idx: idx, z: z})
	}
}

// poseLeDessus rasterise les faces retenues dans le tampon du dessus.
func (p *planHauteurs) poseLeDessus() {
	p.top = make([]float64, p.cal.Largeur*p.cal.Hauteur)
	for k := range p.top {
		p.top[k] = math.Inf(-1)
	}
	for _, f := range p.faces {
		ct := p.m.Contour(p.m.Faces[f.idx])
		for k := 1; k+1 < len(ct); k++ {
			p.rasteriseTriangle(ct[0], ct[k], ct[k+1], f.z)
		}
	}
}

// rasteriseTriangle pose l'altitude `z` sur les pixels dont le centre tombe dans le triangle,
// la ou elle depasse le dessus deja pose. Le triangle est coupe au cadre par sa boite.
func (p *planHauteurs) rasteriseTriangle(a, b, c hinavmesh.Point, z float64) {
	cal := p.cal
	mpp := cal.MetresParPixel
	i0 := int(math.Floor((min(a.X, b.X, c.X)-cal.OrigineX)/mpp - 0.5))
	i1 := int(math.Ceil((max(a.X, b.X, c.X)-cal.OrigineX)/mpp - 0.5))
	j0 := int(math.Floor((cal.OrigineY-max(a.Y, b.Y, c.Y))/mpp - 0.5))
	j1 := int(math.Ceil((cal.OrigineY-min(a.Y, b.Y, c.Y))/mpp - 0.5))
	i0, i1 = max(i0, 0), min(i1, cal.Largeur-1)
	j0, j1 = max(j0, 0), min(j1, cal.Hauteur-1)
	ta, tb, tc := [3]float64{a.X, a.Y, 0}, [3]float64{b.X, b.Y, 0}, [3]float64{c.X, c.Y, 0}
	det := (tb[1]-tc[1])*(ta[0]-tc[0]) + (tc[0]-tb[0])*(ta[1]-tc[1])
	for j := j0; j <= j1; j++ {
		y := cal.OrigineY - (float64(j)+0.5)*mpp
		for i := i0; i <= i1; i++ {
			x := cal.OrigineX + (float64(i)+0.5)*mpp
			if _, dedans := altitudeAuPoint(ta, tb, tc, det, x, y); !dedans {
				continue
			}
			if k := j*cal.Largeur + i; z > p.top[k] {
				p.top[k] = z
			}
		}
	}
}

// bornesDeTeinte rend les centiles 2 et 98 des altitudes des faces retenues.
func (p *planHauteurs) bornesDeTeinte() (bas, haut float64) {
	zs := make([]float64, len(p.faces))
	for i, f := range p.faces {
		zs[i] = f.z
	}
	sort.Float64s(zs)
	return zs[len(zs)/50], zs[len(zs)-1-len(zs)/50]
}

// teinteDeRampe interpole continument sur la rampe : t = 0 est le bas, t = 1 le haut (borne). Un
// t indetermine (NaN) prend la teinte du milieu, celle d'une altitude inconnue.
func teinteDeRampe(t float64) color.RGBA {
	if math.IsNaN(t) {
		t = 0.5
	}
	t = math.Max(0, math.Min(1, t)) * float64(len(rampePlanHauteurs)-1)
	i := min(len(rampePlanHauteurs)-2, int(math.Floor(t)))
	f := t - float64(i)
	a, b := rampePlanHauteurs[i], rampePlanHauteurs[i+1]
	mele := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*f)) }
	return color.RGBA{mele(a.R, b.R), mele(a.G, b.G), mele(a.B, b.B), 255}
}

// assombritOmbre applique l'ombre portee a une teinte.
func assombritOmbre(c color.RGBA) color.RGBA {
	return color.RGBA{
		uint8(float64(c.R) * facteurOmbreRG), uint8(float64(c.G) * facteurOmbreRG),
		uint8(float64(c.B) * facteurOmbreB), 255}
}

// peintLesFaces rend l'image : papier hors de toute face, teinte de l'altitude du dessus
// ailleurs, assombrie si un niveau nettement plus haut est au nord-ouest.
func (p *planHauteurs) peintLesFaces() *image.RGBA {
	cal := p.cal
	img := image.NewRGBA(image.Rect(0, 0, cal.Largeur, cal.Hauteur))
	var bas, haut float64
	if len(p.faces) > 0 {
		bas, haut = p.bornesDeTeinte()
	}
	plat := haut-bas < ecartMinHauteurs
	d := max(decalageOmbreMinPx, int(math.Round(decalageOmbreMetres/cal.MetresParPixel)))
	for y := 0; y < cal.Hauteur; y++ {
		for x := 0; x < cal.Largeur; x++ {
			z := math.Inf(-1)
			if p.top != nil {
				z = p.top[y*cal.Largeur+x]
			}
			if math.IsInf(z, -1) {
				img.SetRGBA(x, y, teintePlanPapier)
				continue
			}
			t := 0.5
			if !plat {
				t = (z - bas) / (haut - bas)
			}
			c := teinteDeRampe(t)
			if x >= d && y >= d && p.top[(y-d)*cal.Largeur+x-d] > z+ecartOmbreMetres {
				c = assombritOmbre(c)
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}
