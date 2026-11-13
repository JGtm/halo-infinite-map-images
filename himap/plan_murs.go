package himap

// plan_murs.go — LES MURS D'UN MAILLAGE DE NAVIGATION, EN ARETES.
//
// Une arete de face que deux faces se partagent est un passage : on la traverse en marchant.
// Une arete qu'une seule face porte est un BORD du maillage : un mur, un vide, une limite de
// la zone jouable. L'ensemble de ces bords est le plan d'etage de la carte.

import (
	"fmt"

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

// PlanDesMurs rend les aretes de bord de `m` : celles que UNE seule face porte. Une arete vue
// deux fois, dans un sens ou dans l'autre, est un passage et n'y figure pas. La sortie est
// deterministe pour un maillage donne. Un maillage nil rend un plan vide.
//
// Une face de moins de trois sommets n'est pas une surface et est ignoree, comme une arete
// dont les deux indices sont egaux (sommet repete). Un indice negatif ou hors de
// `m.Sommets` est une erreur : le maillage est incoherent, pas le plan.
func PlanDesMurs(m *hinavmesh.Maillage) (PlanMurs, error) {
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
	pourChaqueArete(m, func(i, j int32) { vues[cleArete(i, j)]++ })
	pourChaqueArete(m, func(i, j int32) {
		if vues[cleArete(i, j)] != 1 {
			return
		}
		p, q := m.Sommets[i], m.Sommets[j]
		plan.Aretes = append(plan.Aretes, [4]float64{p.X, p.Y, q.X, q.Y})
	})
	return plan, nil
}

// pourChaqueArete parcourt, dans l'ordre des faces puis du contour, les aretes de toutes les
// faces d'au moins trois sommets, sauf celles dont les deux indices sont egaux.
func pourChaqueArete(m *hinavmesh.Maillage, f func(i, j int32)) {
	for _, face := range m.Faces {
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
