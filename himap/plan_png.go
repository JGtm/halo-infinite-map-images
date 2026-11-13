package himap

// plan_png.go — L'IMAGE DU PLAN D'UN MAILLAGE DE NAVIGATION : LE PLAN A HAUTEURS.
//
// PlanDesMurs donne les aretes de bord en metres monde ; cette image en donne la lecture : les
// faces gardees teintees par leur altitude (rampe froid -> chaud, du bas au haut), l'ombre
// portee des niveaux hauts sur les bas, les aretes de bord en trait d'encre (en pointille gris
// quand un niveau plus haut les recouvre), sur un papier clair. Elle a les memes dimensions et
// le meme calage que le fond de la carte (CalageFondDuRendu), pour se superposer pixel a pixel.
// C'est une image de lecture, pas un asset publie.
//
// Le dessus l'emporte pixel par pixel (z-buffer) : un etage superpose a un autre le cache, comme
// vu du ciel. Les trous du maillage (un rocher, un pilier) n'ont pas de face : ils restent du
// papier, ce sont des obstacles. Le detail du dessin vit dans plan_hauteurs.go (teinte, ombre)
// et plan_aretes.go (aretes).

import (
	"fmt"
	"image"
	"image/color"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// CalageFond est le calage monde -> pixel d'une image de fond, avec ses dimensions. Meme
// convention que CalageDuRendu : xMonde = OrigineX + (px+0,5)*MetresParPixel et
// yMonde = OrigineY - (py+0,5)*MetresParPixel.
type CalageFond struct {
	OrigineX, OrigineY, MetresParPixel float64
	Largeur, Hauteur                   int
}

// CalageFondDuRendu rend le calage et les dimensions de l'image qu'un rendu produit.
func CalageFondDuRendu(r *Rendu) CalageFond {
	x0, y0, mpp := CalageDuRendu(r)
	return CalageFond{OrigineX: x0, OrigineY: y0, MetresParPixel: mpp, Largeur: r.NX, Hauteur: r.NY}
}

// teintePlanPapier : le papier du plan, ce que ne couvre aucune face gardee.
var teintePlanPapier = color.RGBA{240, 241, 236, 255}

// PlanPNG dessine le plan a hauteurs des faces `garde` de `m` (nil : toutes) dans le cadre de
// `cal`. Les faces et leurs aretes sont coupees au cadre. Erreur si le cadre n'a pas de surface
// ou si le maillage est incoherent (voir PlanDesMurs). Un maillage nil rend le papier seul.
func PlanPNG(m *hinavmesh.Maillage, garde []bool, cal CalageFond) (*image.RGBA, error) {
	if cal.Largeur <= 0 || cal.Hauteur <= 0 || cal.MetresParPixel <= 0 {
		return nil, fmt.Errorf("plan : cadre %d x %d a %g m/px sans surface", cal.Largeur, cal.Hauteur, cal.MetresParPixel)
	}
	p := &planHauteurs{cal: cal, m: m, garde: garde}
	if m != nil {
		if err := verifieIndicesDeSommets(m); err != nil {
			return nil, err
		}
		p.rassembleLesFaces()
		p.poseLeDessus()
	}
	img := p.peintLesFaces()
	if m != nil {
		p.traceLesAretes(img)
	}
	return img, nil
}
