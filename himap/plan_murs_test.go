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
	plan, err := PlanDesMurs(maillageTroisFaces(), nil)
	if err != nil {
		t.Fatal(err)
	}
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
	a, _ := PlanDesMurs(m, nil)
	b, _ := PlanDesMurs(m, nil)
	if !reflect.DeepEqual(a, b) {
		t.Error("deux appels sur le meme maillage rendent des plans differents")
	}
}

func TestPlanDesMursMaillageVide(t *testing.T) {
	if p, err := PlanDesMurs(nil, nil); err != nil || len(p.Aretes) != 0 || p.Faces != 0 {
		t.Errorf("maillage nil : plan %+v, erreur %v", p, err)
	}
	if p, err := PlanDesMurs(&hinavmesh.Maillage{}, nil); err != nil || len(p.Aretes) != 0 {
		t.Errorf("maillage vide : %d aretes, erreur %v", len(p.Aretes), err)
	}
}

// Un sommet repete donne une arete d'un point : elle est ignoree, les autres comptent.
func TestPlanDesMursIgnoreUnSommetRepete(t *testing.T) {
	m := &hinavmesh.Maillage{
		Sommets: []hinavmesh.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		Faces:   []hinavmesh.Face{{Sommets: []int32{0, 1, 1, 2}}},
	}
	plan, err := PlanDesMurs(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	attendu := [][4]float64{{0, 0, 1, 0}, {1, 0, 1, 1}, {1, 1, 0, 0}}
	if !reflect.DeepEqual(plan.Aretes, attendu) {
		t.Errorf("aretes = %v, attendu %v", plan.Aretes, attendu)
	}
}

// Un indice negatif ou hors bornes est une erreur, jamais une panique.
func TestPlanDesMursRefuseUnIndiceHorsBornes(t *testing.T) {
	for _, indice := range []int32{3, 99, -1} {
		m := &hinavmesh.Maillage{
			Sommets: []hinavmesh.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}},
			Faces:   []hinavmesh.Face{{Sommets: []int32{0, 1, indice}}},
		}
		if _, err := PlanDesMurs(m, nil); err == nil {
			t.Errorf("indice %d : pas d'erreur", indice)
		}
	}
}

// Deux faces qui parcourent la meme arete DANS LE MEME SENS la voient deux fois : elle est un
// passage (maillage mal oriente, mais deux faces la portent) et n'est pas un bord.
func TestPlanDesMursExclutUneAretePartageeDansLeMemeSens(t *testing.T) {
	m := &hinavmesh.Maillage{
		Sommets: []hinavmesh.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}},
		Faces: []hinavmesh.Face{
			{Sommets: []int32{0, 1, 2}},
			{Sommets: []int32{0, 1, 3}},
		},
	}
	plan, err := PlanDesMurs(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Aretes) != 4 {
		t.Fatalf("%d aretes de bord, attendu 4 (l'arete 0-1 est vue deux fois)", len(plan.Aretes))
	}
	for _, a := range plan.Aretes {
		if a == [4]float64{0, 0, 1, 0} {
			t.Errorf("l'arete partagee 0-1 figure dans le plan : %v", plan.Aretes)
		}
	}
}
