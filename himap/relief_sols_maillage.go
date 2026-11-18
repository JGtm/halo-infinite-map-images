package himap

// relief_sols_maillage.go — LES SOLS DU MAILLAGE, RASTERISES DANS LE CADRE PUBLIE.
//
// Chaque face gardee est rasterisee a sa hauteur INTERPOLEE (pas la moyenne de ses sommets, que
// plan_hauteurs.go utilise pour sa teinte) aux centres de pixel du cadre. Par pixel on retient le
// sol le plus haut, et que le maillage y est ou non : c'est ce qui complete le dessus du rendu
// (D-6, D-12) et ce qui, elargi, dessine la zone gardee.

import (
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// solsMaillage est le maillage rasterise dans le cadre publie. Indice : py*Largeur + px.
type solsMaillage struct {
	// haut : le sol le plus haut au centre du pixel, -Inf s'il n'y en a pas.
	haut []float64
	// couvre : un sol gardé couvre le centre du pixel.
	couvre []bool
	// present : au moins un pixel du cadre est couvert (sinon la carte est traitee sans maillage).
	present bool
}

// trianglesDesFacesGardees rend les triangles (eventail depuis le premier sommet) des faces gardees
// d'au moins trois sommets. Un triangle dont un sommet n'est pas fini est ecarte : il fausserait la
// hauteur, et un maillage incoherent ne doit pas faire paniquer. `garde` nil garde toutes les
// faces ; sinon une face dont l'indice depasse `garde` est ecartee.
func trianglesDesFacesGardees(m *hinavmesh.Maillage, garde []bool) [][3]hinavmesh.Point {
	if m == nil {
		return nil
	}
	tris := make([][3]hinavmesh.Point, 0, len(m.Faces))
	for idx, f := range m.Faces {
		if garde != nil && (idx >= len(garde) || !garde[idx]) {
			continue
		}
		for k := 1; k+1 < len(f.Sommets); k++ {
			t := [3]hinavmesh.Point{m.Sommets[f.Sommets[0]], m.Sommets[f.Sommets[k]], m.Sommets[f.Sommets[k+1]]}
			if triangleFini(t) {
				tris = append(tris, t)
			}
		}
	}
	return tris
}

// triangleFini dit si les neuf coordonnees du triangle sont des nombres finis.
func triangleFini(t [3]hinavmesh.Point) bool {
	for _, p := range t {
		for _, v := range [3]float64{p.X, p.Y, p.Z} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
		}
	}
	return true
}

// rasteriseLesSols rasterise les faces gardees de `m` dans le cadre `cal`. Un maillage nil, ou sans
// face gardee dans le cadre, rend une couverture vide (`present` faux).
func rasteriseLesSols(m *hinavmesh.Maillage, garde []bool, cal CalageFond) *solsMaillage {
	n := cal.Largeur * cal.Hauteur
	s := &solsMaillage{haut: make([]float64, n), couvre: make([]bool, n)}
	for k := range s.haut {
		s.haut[k] = math.Inf(-1)
	}
	for _, t := range trianglesDesFacesGardees(m, garde) {
		s.pose(t, cal)
	}
	return s
}

// pose pose le triangle `t` : pour chaque pixel dont le centre y tombe, le sol le plus haut gagne.
// Le triangle est coupe au cadre par sa boite.
func (s *solsMaillage) pose(t [3]hinavmesh.Point, cal CalageFond) {
	mpp := cal.MetresParPixel
	a, b, c := t[0], t[1], t[2]
	i0 := int(math.Floor((min(a.X, b.X, c.X)-cal.OrigineX)/mpp - 0.5))
	i1 := int(math.Ceil((max(a.X, b.X, c.X)-cal.OrigineX)/mpp - 0.5))
	j0 := int(math.Floor((cal.OrigineY-max(a.Y, b.Y, c.Y))/mpp - 0.5))
	j1 := int(math.Ceil((cal.OrigineY-min(a.Y, b.Y, c.Y))/mpp - 0.5))
	i0, i1 = max(i0, 0), min(i1, cal.Largeur-1)
	j0, j1 = max(j0, 0), min(j1, cal.Hauteur-1)
	ta, tb, tc := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}, [3]float64{c.X, c.Y, c.Z}
	det := (tb[1]-tc[1])*(ta[0]-tc[0]) + (tc[0]-tb[0])*(ta[1]-tc[1])
	for j := j0; j <= j1; j++ {
		y := cal.OrigineY - (float64(j)+0.5)*mpp
		for i := i0; i <= i1; i++ {
			x := cal.OrigineX + (float64(i)+0.5)*mpp
			z, dedans := altitudeAuPoint(ta, tb, tc, det, x, y)
			if !dedans {
				continue
			}
			k := j*cal.Largeur + i
			s.couvre[k], s.present = true, true
			if z > s.haut[k] {
				s.haut[k] = z
			}
		}
	}
}
