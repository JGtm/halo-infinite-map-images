package himap

// plan_murs.go — LES MURS D'UN MAILLAGE DE NAVIGATION, EN ARETES.
//
// Une arete de face que deux faces se partagent est un passage : on la traverse en marchant.
// Une arete qu'une seule face porte est un BORD du maillage : un mur, un vide, une limite de
// la zone jouable. L'ensemble de ces bords est le plan d'etage de la carte.

import (
	"fmt"
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// PlanMurs est le plan d'un maillage : ses aretes de bord, en metres monde.
type PlanMurs struct {
	// Aretes : une ligne par arete de bord, `{x1, y1, x2, y2}`. Ordre stable : faces dans
	// l'ordre du maillage, puis aretes dans l'ordre du contour de chaque face (la derniere
	// arete referme le contour).
	Aretes [][4]float64
	// Sommets et Faces : taille du maillage dont le plan est tire.
	Sommets, Faces int
}

// paireSommets est la cle d'une arete : ses deux indices, le plus petit d'abord, pour que
// deux faces voisines qui la parcourent en sens inverse la reconnaissent.
type paireSommets struct{ a, b int32 }

func cleArete(i, j int32) paireSommets {
	if i > j {
		i, j = j, i
	}
	return paireSommets{i, j}
}

// PlanDesMurs rend les aretes de bord de `m` : celles que UNE seule face GARDEE porte. `garde`
// dit, par indice de face, celles qui comptent (voir ComposantesRetenues) ; nil les garde toutes.
// Une arete vue deux fois, dans un sens ou dans l'autre, est un passage et n'y figure pas ; une
// arete entre une face gardee et une face ecartee est donc un bord. La sortie est
// deterministe pour un maillage donne. Un maillage nil rend un plan vide.
//
// Une face de moins de trois sommets n'est pas une surface et est ignoree, comme une arete
// dont les deux indices sont egaux (sommet repete). Un indice negatif ou hors de
// `m.Sommets` est une erreur : le maillage est incoherent, pas le plan.
func PlanDesMurs(m *hinavmesh.Maillage, garde []bool) (PlanMurs, error) {
	if m == nil {
		return PlanMurs{}, nil
	}
	plan := PlanMurs{Sommets: len(m.Sommets), Faces: len(m.Faces)}
	for f, face := range m.Faces {
		for _, i := range face.Sommets {
			if i < 0 || int(i) >= len(m.Sommets) {
				return PlanMurs{}, fmt.Errorf("face %d : indice de sommet %d hors de [0, %d[", f, i, len(m.Sommets))
			}
		}
	}
	vues := make(map[paireSommets]int)
	pourChaqueArete(m, garde, func(i, j int32) { vues[cleArete(i, j)]++ })
	pourChaqueArete(m, garde, func(i, j int32) {
		if vues[cleArete(i, j)] != 1 {
			return
		}
		p, q := m.Sommets[i], m.Sommets[j]
		plan.Aretes = append(plan.Aretes, [4]float64{p.X, p.Y, q.X, q.Y})
	})
	return plan, nil
}

// pourChaqueArete parcourt, dans l'ordre des faces puis du contour, les aretes des faces gardees
// (`garde` nil : toutes) d'au moins trois sommets, sauf celles dont les deux indices sont egaux.
func pourChaqueArete(m *hinavmesh.Maillage, garde []bool, f func(i, j int32)) {
	for idx, face := range m.Faces {
		if garde != nil && (idx >= len(garde) || !garde[idx]) {
			continue
		}
		n := len(face.Sommets)
		if n < 3 {
			continue
		}
		for k, i := range face.Sommets {
			if j := face.Sommets[(k+1)%n]; i != j {
				f(i, j)
			}
		}
	}
}

// PlanRogneAuCadre rend le plan dont chaque arete est coupee au cadre du fond decrit par `cal` :
// une arete entierement dehors disparait, une arete qui sort est raccourcie a la frontiere.
// Sommets et Faces sont repris du plan. Un cadre sans surface rend un plan sans arete.
func PlanRogneAuCadre(plan PlanMurs, cal CalageFond) PlanMurs {
	out := PlanMurs{Sommets: plan.Sommets, Faces: plan.Faces}
	if cal.Largeur <= 0 || cal.Hauteur <= 0 || cal.MetresParPixel <= 0 {
		return out
	}
	xMin, xMax := cal.OrigineX, cal.OrigineX+float64(cal.Largeur)*cal.MetresParPixel
	yMax, yMin := cal.OrigineY, cal.OrigineY-float64(cal.Hauteur)*cal.MetresParPixel
	for _, a := range plan.Aretes {
		if c, ok := coupeAuCadre(a, xMin, yMin, xMax, yMax); ok {
			out.Aretes = append(out.Aretes, c)
		}
	}
	return out
}

// coupeAuCadre coupe le segment `a` au rectangle [xMin, xMax] x [yMin, yMax] (Liang-Barsky) et
// dit s'il en reste quelque chose. Un segment qui ne sort pas est rendu a l'identique.
func coupeAuCadre(a [4]float64, xMin, yMin, xMax, yMax float64) ([4]float64, bool) {
	dx, dy := a[2]-a[0], a[3]-a[1]
	t0, t1 := 0.0, 1.0
	// p/q : un couple par bord (gauche, droite, bas, haut).
	p := [4]float64{-dx, dx, -dy, dy}
	q := [4]float64{a[0] - xMin, xMax - a[0], a[1] - yMin, yMax - a[1]}
	for k := 0; k < 4; k++ {
		if p[k] == 0 {
			if q[k] < 0 {
				return a, false // parallele au bord et dehors
			}
			continue
		}
		t := q[k] / p[k]
		if p[k] < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
	}
	if t0 > t1 {
		return a, false
	}
	if t0 == 0 && t1 == 1 {
		return a, true
	}
	return [4]float64{a[0] + t0*dx, a[1] + t0*dy, a[0] + t1*dx, a[1] + t1*dy}, true
}
