package himap

import (
	"path/filepath"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// LES TEMOINS DU CHOIX DU MAILLAGE D'UNE ARENE — la regle de `choisitMaillageArene`, rejouee sans
// le jeu installe. Les boites sont celles MESUREES le 2026-11-18 sur `any/globals/common-rtx-new`
// (ressources 20390 et 20391), la boite d'arene et les ancres celles de choix_bsp_test.go.

// maillageTemoin fabrique un maillage de sommets donnes ; seuls les sommets et l'emprise
// mesuree entrent dans la decision.
func maillageTemoin(sommets ...hinavmesh.Point) *hinavmesh.Maillage {
	m := &hinavmesh.Maillage{Sommets: sommets, Min: sommets[0], Max: sommets[0]}
	for _, s := range sommets {
		m.Min = hinavmesh.Point{X: min(m.Min.X, s.X), Y: min(m.Min.Y, s.Y), Z: min(m.Min.Z, s.Z)}
		m.Max = hinavmesh.Point{X: max(m.Max.X, s.X), Y: max(m.Max.Y, s.Y), Z: max(m.Max.Z, s.Z)}
	}
	return m
}

// Le maillage de l'arene de Live Fire : X [-11,1 ; 29,6] Y [15,0 ; 48,0], tout dans la boite.
func maillageAreneLiveFire() *hinavmesh.Maillage {
	return maillageTemoin(
		hinavmesh.Point{X: -11.1, Y: 15, Z: -1}, hinavmesh.Point{X: 29.6, Y: 48, Z: 3.8},
		hinavmesh.Point{X: 0, Y: 30, Z: 1}, hinavmesh.Point{X: 20, Y: 20, Z: 0})
}

// Le maillage de l'arene voisine : X [38,8 ; 66,0] Y [28,1 ; 98,1], dont un quart des sommets
// mord sur la boite de l'arene de Live Fire (X <= 46,5, Y <= 53,7).
func maillageAreneVoisine() *hinavmesh.Maillage {
	return maillageTemoin(
		hinavmesh.Point{X: 38.8, Y: 28.1}, hinavmesh.Point{X: 66, Y: 98.1},
		hinavmesh.Point{X: 50, Y: 50}, hinavmesh.Point{X: 60, Y: 90})
}

func TestChoisitMaillageArenePrendLAreneDeLiveFire(t *testing.T) {
	arene, ancres := areneLiveFire().Bounds, ancresLiveFire()
	arenes := []*hinavmesh.Maillage{maillageAreneLiveFire(), maillageAreneVoisine()}
	// Les deux ordres : la regle ne depend pas de l'ordre de lecture (taille decroissante).
	if got := choisitMaillageArene(arenes, arene, ancres); got != 0 {
		t.Errorf("choix = %d, attendu 0 (l'arene de Live Fire)", got)
	}
	inverse := []*hinavmesh.Maillage{arenes[1], arenes[0]}
	if got := choisitMaillageArene(inverse, arene, ancres); got != 1 {
		t.Errorf("choix (ordre inverse) = %d, attendu 1 (l'arene de Live Fire)", got)
	}
}

// Les ancres tranchent entre deux maillages assez dans l'arene : le voisin, rendu eligible, perd
// parce qu'aucune ancre n'est dans son emprise.
func TestChoisitMaillageAreneLesAncresTranchent(t *testing.T) {
	arene, ancres := areneLiveFire().Bounds, ancresLiveFire()
	// Un voisin dont 4 sommets sur 5 tombent dans la boite : eligible, mais sans ancre.
	voisin := maillageTemoin(
		hinavmesh.Point{X: 38.8, Y: 28.1}, hinavmesh.Point{X: 66, Y: 98.1},
		hinavmesh.Point{X: 40, Y: 50}, hinavmesh.Point{X: 44, Y: 40}, hinavmesh.Point{X: 45, Y: 45})
	if got := partDansArene(voisin, arene); got < fractionMinDansArene {
		t.Fatalf("le temoin n'est pas eligible : part %.2f", got)
	}
	if got := choisitMaillageArene([]*hinavmesh.Maillage{voisin, maillageAreneLiveFire()}, arene, ancres); got != 1 {
		t.Errorf("choix = %d, attendu 1 : le maillage qui porte les ancres", got)
	}
}

// Sans ancre, la part de sommets dans l'arene departage, puis l'emprise la plus petite.
func TestChoisitMaillageAreneSansAncreDepartageParPartPuisEmprise(t *testing.T) {
	arene := areneLiveFire().Bounds
	complet := maillageAreneLiveFire()
	partiel := maillageTemoin(hinavmesh.Point{X: 0, Y: 0}, hinavmesh.Point{X: 40, Y: 40},
		hinavmesh.Point{X: 50, Y: 60}, hinavmesh.Point{X: 5, Y: 5})
	if got := choisitMaillageArene([]*hinavmesh.Maillage{partiel, complet}, arene, nil); got != 1 {
		t.Errorf("choix = %d, attendu 1 (100 %% des sommets dans l'arene)", got)
	}
	petit := maillageTemoin(hinavmesh.Point{X: 0, Y: 0}, hinavmesh.Point{X: 5, Y: 5})
	if got := choisitMaillageArene([]*hinavmesh.Maillage{complet, petit}, arene, nil); got != 1 {
		t.Errorf("choix = %d, attendu 1 (a part egale, l'emprise la plus petite)", got)
	}
}

// Aucun maillage dans l'arene (ou aucun maillage) : -1, jamais « le moins mauvais ».
func TestChoisitMaillageAreneAucunDansLArene(t *testing.T) {
	arene, ancres := areneLiveFire().Bounds, ancresLiveFire()
	if got := choisitMaillageArene([]*hinavmesh.Maillage{maillageAreneVoisine()}, arene, ancres); got != -1 {
		t.Errorf("choix = %d, attendu -1 : l'arene voisine n'est pas celle de la carte", got)
	}
	if got := choisitMaillageArene(nil, arene, ancres); got != -1 {
		t.Errorf("choix sans candidate = %d, attendu -1", got)
	}
	if got := choisitMaillageArene([]*hinavmesh.Maillage{{}}, arene, ancres); got != -1 {
		t.Errorf("choix d'un maillage vide = %d, attendu -1", got)
	}
}

func TestModuleAnyPartage(t *testing.T) {
	racine := filepath.Join("jeu", "deploy")
	cas := []struct {
		nom, geometrie, attendu string
		ok                      bool
	}{
		{"module partage", filepath.Join(racine, "pc", "globals", "common-rtx-new.module"),
			filepath.Join(racine, "any", "globals", "common-rtx-new.module"), true},
		{"module d'une carte", filepath.Join(racine, "pc", "levels", "multi", "ctf_bazaar", "ctf_bazaar-rtx-new.module"), "", false},
		{"autre variante", filepath.Join(racine, "ds", "globals", "common-rtx-new.module"), "", false},
		{"sous-dossier de globals", filepath.Join(racine, "pc", "globals", "forge", "forge_objects-rtx-new.module"), "", false},
		{"hors de la racine", filepath.Join("ailleurs", "pc", "globals", "common-rtx-new.module"), "", false},
		{"vide", "", "", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			got, ok := moduleAnyPartage(racine, c.geometrie)
			if ok != c.ok || got != c.attendu {
				t.Errorf("moduleAnyPartage = (%q, %v), attendu (%q, %v)", got, ok, c.attendu, c.ok)
			}
		})
	}
}
