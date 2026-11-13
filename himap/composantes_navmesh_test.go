package himap

import (
	"math"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// ajouteRect ajoute au maillage un rectangle horizontal d'une face, a l'altitude z, avec ses
// propres sommets (donc une composante a part), et rend l'indice de la face.
func ajouteRect(m *hinavmesh.Maillage, x0, y0, x1, y1, z float64) int {
	base := int32(len(m.Sommets))
	m.Sommets = append(m.Sommets,
		hinavmesh.Point{X: x0, Y: y0, Z: z}, hinavmesh.Point{X: x1, Y: y0, Z: z},
		hinavmesh.Point{X: x1, Y: y1, Z: z}, hinavmesh.Point{X: x0, Y: y1, Z: z})
	m.Faces = append(m.Faces, hinavmesh.Face{Sommets: []int32{base, base + 1, base + 2, base + 3}})
	return len(m.Faces) - 1
}

func dansRect(x, y, x0, y0, x1, y1 float64) bool { return x >= x0 && x <= x1 && y >= y0 && y <= y1 }

// TestComposantesRetenuesRegleD6 : une grande dalle sans ancre ni matiere est ecartee ; une
// arene portant une ancre est gardee ; un ilot de 5 m2 entierement sur la matiere est garde ;
// un ilot de 2 m2 est ecarte malgre la matiere ; une ancre a 5 m au dessus d'une dalle ne
// l'appuie pas ; un ilot de 5 m2 dont 80 % seulement est sur la matiere est ecarte.
func TestComposantesRetenuesRegleD6(t *testing.T) {
	m := &hinavmesh.Maillage{}
	dalle := ajouteRect(m, 0, 0, 100, 100, 0)   // 10 000 m2, sans ancre ni matiere
	arene := ajouteRect(m, 200, 0, 210, 10, 0)  // 100 m2, ancre appuyee
	ilot5 := ajouteRect(m, 300, 0, 302.5, 2, 0) // 5 m2, 100 % sur la matiere
	ilot2 := ajouteRect(m, 400, 0, 402, 1, 0)   // 2 m2, 100 % sur la matiere
	haut := ajouteRect(m, 500, 0, 508, 8, 0)    // 64 m2, ancre 5 m au-dessus
	partiel := ajouteRect(m, 600, 0, 605, 1, 0) // 5 m2, 80 % sur la matiere
	ancres := [][3]float64{{205, 5, 0.5}, {504, 4, 5}}
	matiere := func(x, y float64) bool {
		return dansRect(x, y, 300, 0, 302.5, 2) || dansRect(x, y, 400, 0, 402, 1) || dansRect(x, y, 600, 0, 604, 1)
	}

	res := ComposantesRetenues(m, ancres, matiere)
	for f, attendu := range map[int]bool{dalle: false, arene: true, ilot5: true, ilot2: false, haut: false, partiel: false} {
		if res.Garde[f] != attendu {
			t.Errorf("face %d : gardee = %v, attendu %v", f, res.Garde[f], attendu)
		}
	}
	if res.Gardees != 2 || res.Ecartees != 4 || res.ParAncre != 1 || res.ParMatiere != 1 || res.Repli {
		t.Errorf("bilan = %+v, attendu 2 gardees (1 par ancre, 1 par matiere), 4 ecartees, sans repli", res)
	}
	if math.Abs(res.AireGardee-105) > 1e-9 || math.Abs(res.AireEcartee-(10000+2+64+5)) > 1e-9 {
		t.Errorf("aires = %.3f gardee, %.3f ecartee ; attendu 105 et 10071", res.AireGardee, res.AireEcartee)
	}
}

// Sans matiere (selection d'avant la projection), seul l'appui d'une ancre garde une composante.
func TestComposantesRetenuesSansMatiere(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 0, 0, 100, 100, 0)
	arene := ajouteRect(m, 200, 0, 210, 10, 0)
	ajouteRect(m, 300, 0, 302.5, 2, 0)
	res := ComposantesRetenues(m, [][3]float64{{205, 5, 1}}, nil)
	if res.Gardees != 1 || !res.Garde[arene] || res.ParMatiere != 0 {
		t.Errorf("bilan = %+v, attendu la seule arene gardee", res)
	}
}

// Ni ancre ni matiere : la plus grande par aire est gardee, et le repli est signale.
func TestComposantesRetenuesRepliSurLaPlusGrande(t *testing.T) {
	m := &hinavmesh.Maillage{}
	petite := ajouteRect(m, 300, 0, 303, 3, 0)
	grande := ajouteRect(m, 0, 0, 50, 50, 0)
	res := ComposantesRetenues(m, nil, nil)
	if !res.Repli || res.Gardees != 1 || res.Ecartees != 1 || !res.Garde[grande] || res.Garde[petite] {
		t.Errorf("bilan = %+v (garde %v), attendu le repli sur la face %d", res, res.Garde, grande)
	}
}

func TestComposantesRetenuesMaillageVide(t *testing.T) {
	for _, m := range []*hinavmesh.Maillage{nil, {}} {
		res := ComposantesRetenues(m, [][3]float64{{0, 0, 0}}, nil)
		if len(res.Garde) != 0 || res.Gardees != 0 || res.Repli {
			t.Errorf("maillage vide : %+v", res)
		}
	}
}

// L'appui vertical est borne a 2 m : une ancre a 1,9 m appuie la dalle, une a 2,1 m non.
func TestAncreAppuyeeBorneVerticale(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 0, 0, 10, 10, 10)
	tris := m.Triangles()
	if !ancreAppuyee(tris, [][3]float64{{5, 5, 11.9}}) {
		t.Error("ancre a 1,9 m : appui attendu")
	}
	if ancreAppuyee(tris, [][3]float64{{5, 5, 12.1}}) {
		t.Error("ancre a 2,1 m : pas d'appui attendu")
	}
	if ancreAppuyee(tris, [][3]float64{{50, 5, 10}}) {
		t.Error("ancre hors de la dalle : pas d'appui attendu")
	}
}
