package himap

import (
	"reflect"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// maillageTroisFaces : un triangle, un quadrilatere et un triangle alignes. Le triangle de
// gauche et le quadrilatere se partagent l'arete 1-2 (parcourue en sens inverse), le
// quadrilatere et le triangle de droite l'arete 3-4 (idem). Reste six aretes de bord.
func maillageTroisFaces() *hinavmesh.Maillage {
	return &hinavmesh.Maillage{
		Sommets: []hinavmesh.Point{
			{X: 0, Y: 0, Z: 5}, {X: 1, Y: 0, Z: 6}, {X: 1, Y: 1, Z: 7},
			{X: 2, Y: 0, Z: 8}, {X: 2, Y: 1, Z: 9}, {X: 3, Y: 0.5, Z: 10},
		},
		Faces: []hinavmesh.Face{
			{Sommets: []int32{0, 1, 2}},
			{Sommets: []int32{1, 3, 4, 2}},
			{Sommets: []int32{3, 5, 4}},
		},
	}
}

func TestPlanDesMursExclutLesAretesPartagees(t *testing.T) {
	plan := PlanDesMurs(maillageTroisFaces())
	attendu := [][4]float64{
		{0, 0, 1, 0}, {1, 1, 0, 0},
		{1, 0, 2, 0}, {2, 1, 1, 1},
		{2, 0, 3, 0.5}, {3, 0.5, 2, 1},
	}
	if !reflect.DeepEqual(plan.Aretes, attendu) {
		t.Errorf("aretes de bord = %v, attendu %v", plan.Aretes, attendu)
	}
	if plan.Sommets != 6 || plan.Faces != 3 {
		t.Errorf("taille = %d sommets, %d faces ; attendu 6 et 3", plan.Sommets, plan.Faces)
	}
}

func TestPlanDesMursEstDeterministe(t *testing.T) {
	m := maillageTroisFaces()
	a, b := PlanDesMurs(m), PlanDesMurs(m)
	if !reflect.DeepEqual(a, b) {
		t.Error("deux appels sur le meme maillage rendent des plans differents")
	}
}

func TestPlanDesMursMaillageVide(t *testing.T) {
	if p := PlanDesMurs(nil); len(p.Aretes) != 0 || p.Faces != 0 {
		t.Errorf("maillage nil : plan non vide %+v", p)
	}
	if p := PlanDesMurs(&hinavmesh.Maillage{}); len(p.Aretes) != 0 {
		t.Errorf("maillage vide : %d aretes", len(p.Aretes))
	}
}
