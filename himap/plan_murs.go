package himap

// plan_murs.go — LES MURS D'UN MAILLAGE DE NAVIGATION, EN ARETES.
//
// Une arete de face que deux faces se partagent est un passage : on la traverse en marchant.
// Une arete qu'une seule face porte est un BORD du maillage : un mur, un vide, une limite de
// la zone jouable. L'ensemble de ces bords est le plan d'etage de la carte.

import "github.com/JGtm/halo-infinite-files-reader/hinavmesh"

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

// PlanDesMurs rend les aretes de bord de `m` : celles que UNE seule face porte. La sortie est
// deterministe pour un maillage donne. Un maillage nil rend un plan vide.
func PlanDesMurs(m *hinavmesh.Maillage) PlanMurs {
	if m == nil {
		return PlanMurs{}
	}
	plan := PlanMurs{Sommets: len(m.Sommets), Faces: len(m.Faces)}
	vues := make(map[paireSommets]int)
	for _, f := range m.Faces {
		if len(f.Sommets) < 3 {
			continue // pas une surface : aucune arete a compter
		}
		for k, i := range f.Sommets {
			vues[cleArete(i, f.Sommets[(k+1)%len(f.Sommets)])]++
		}
	}
	for _, f := range m.Faces {
		if len(f.Sommets) < 3 {
			continue
		}
		for k, i := range f.Sommets {
			j := f.Sommets[(k+1)%len(f.Sommets)]
			if vues[cleArete(i, j)] != 1 {
				continue
			}
			p, q := m.Sommets[i], m.Sommets[j]
			plan.Aretes = append(plan.Aretes, [4]float64{p.X, p.Y, q.X, q.Y})
		}
	}
	return plan
}
