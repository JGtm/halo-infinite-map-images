package himap

// plan_png.go — L'IMAGE DU PLAN D'UN MAILLAGE DE NAVIGATION, SUPERPOSABLE AU FOND.
//
// PlanMurs donne les aretes de bord en metres monde ; cette image en donne la lecture : la
// silhouette EXACTE des faces gardees (sans marge, trous interieurs remplis comme en mode
// silhouette) en aplat, ses aretes de bord en trait sombre, sur un papier clair. Elle a les
// memes dimensions et le meme calage que le fond de la carte (CalageFondDuRendu), pour se
// superposer pixel a pixel. C'est une image de lecture, pas un asset publie.

import (
	"fmt"
	"image"
	"image/color"
	"math"

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

// teintePlanPapier : le papier du plan. L'aplat et le trait sont ceux de StylePlan.
var teintePlanPapier = color.RGBA{245, 245, 242, 255}

// PlanPNG dessine le plan des faces `garde` de `m` (nil : toutes) dans le cadre de `cal`. Les
// aretes de bord sont coupees au cadre. Erreur si le cadre n'a pas de surface ou si le maillage
// est incoherent (voir PlanDesMurs).
func PlanPNG(m *hinavmesh.Maillage, garde []bool, cal CalageFond) (*image.RGBA, error) {
	if cal.Largeur <= 0 || cal.Hauteur <= 0 || cal.MetresParPixel <= 0 {
		return nil, fmt.Errorf("plan : cadre %d x %d a %g m/px sans surface", cal.Largeur, cal.Hauteur, cal.MetresParPixel)
	}
	plan, err := PlanDesMurs(m, garde)
	if err != nil {
		return nil, err
	}
	plan = PlanRogneAuCadre(plan, cal)
	var tris [][3]hinavmesh.Point
	if m != nil {
		tris = m.Triangles()
		if garde != nil {
			tris = m.TrianglesDe(garde)
		}
	}
	// La grille de couverture est orientee comme le monde (j vers le haut) ; l'image descend.
	coin := [2]float64{cal.OrigineX, cal.OrigineY - float64(cal.Hauteur)*cal.MetresParPixel}
	silhouette := remplitTrous(couvreTriangles(tris, coin, cal.MetresParPixel, cal.Largeur, cal.Hauteur), cal.Largeur, cal.Hauteur)
	img := image.NewRGBA(image.Rect(0, 0, cal.Largeur, cal.Hauteur))
	for py := 0; py < cal.Hauteur; py++ {
		j := cal.Hauteur - 1 - py
		for px := 0; px < cal.Largeur; px++ {
			c := teintePlanPapier
			if silhouette[j*cal.Largeur+px] {
				c = teintePlanAplat
			}
			img.SetRGBA(px, py, c)
		}
	}
	for _, a := range plan.Aretes {
		traceArete(img, cal, a)
	}
	return img, nil
}

// traceArete trace une arete (metres monde) en trait sombre d'un pixel : un echantillon tous les
// demi-pixels le long du segment, ce qui ne laisse aucun trou, quelle que soit la pente.
func traceArete(img *image.RGBA, cal CalageFond, a [4]float64) {
	fx0, fy0 := (a[0]-cal.OrigineX)/cal.MetresParPixel, (cal.OrigineY-a[1])/cal.MetresParPixel
	fx1, fy1 := (a[2]-cal.OrigineX)/cal.MetresParPixel, (cal.OrigineY-a[3])/cal.MetresParPixel
	n := int(math.Ceil(math.Hypot(fx1-fx0, fy1-fy0)*2)) + 1
	for s := 0; s <= n; s++ {
		t := float64(s) / float64(n)
		px, py := int(math.Floor(fx0+t*(fx1-fx0))), int(math.Floor(fy0+t*(fy1-fy0)))
		if px >= 0 && px < cal.Largeur && py >= 0 && py < cal.Hauteur {
			img.SetRGBA(px, py, teintePlanContour)
		}
	}
}
