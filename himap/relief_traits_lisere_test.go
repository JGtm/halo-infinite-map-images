package himap

// relief_traits_lisere_test.go — le lisere du relief : anneau d'un carre, trou enclos, petit trou sans
// anneau, deux composantes, concordance avec les miettes de zone.

import "testing"

// Un carre de zone donne un anneau ferme, sans point repete, simplifie a ses 4 coins (ou 8 chanfreines).
func TestTraitsLisereDUnCarre(t *testing.T) {
	rel := reliefDeGrille(60, 60, mppTraits, func(int, int) (float64, bool) { return 0, true }, func(px, py int) bool {
		return px >= 10 && px < 50 && py >= 10 && py < 50
	})
	traits(t, rel)
	o := rel.Donnees.Lines.Liseres
	if len(o) != 1 {
		t.Fatalf("%d anneaux, attendu 1", len(o))
	}
	pts := decodePolyligne(o[0])
	if n := len(pts); n != 4 && n != 8 {
		t.Errorf("%d sommets, attendu 4 ou 8 : %v", n, pts)
	}
	vus := map[point]bool{}
	for _, p := range pts {
		if vus[p] {
			t.Errorf("point %v repete", p)
		}
		vus[p] = true
	}
}

// Un trou enclos dans la zone donne deux anneaux (le bord et le trou) ; un trou de moins de
// AireMinAnneau reste un trou, sans anneau ; deux composantes donnent deux anneaux.
func TestTraitsLisereTrouEtComposantes(t *testing.T) {
	sol := func(int, int) (float64, bool) { return 0, true }
	cas := []struct {
		nom     string
		zone    func(px, py int) bool
		anneaux int
	}{
		{"trou enclos", func(px, py int) bool {
			return px >= 10 && px < 90 && py >= 10 && py < 90 && !(px >= 40 && px < 60 && py >= 40 && py < 60)
		}, 2},
		{"trou de 9 pixels", func(px, py int) bool {
			return px >= 10 && px < 90 && py >= 10 && py < 90 && !(px >= 50 && px < 53 && py >= 50 && py < 53)
		}, 1},
		{"deux composantes", func(px, py int) bool {
			return py >= 10 && py < 90 && ((px >= 10 && px < 40) || (px >= 60 && px < 90))
		}, 2},
	}
	for _, c := range cas {
		rel := reliefDeGrille(100, 100, mppTraits, sol, c.zone)
		b := traits(t, rel)
		if got := len(rel.Donnees.Lines.Liseres); got != c.anneaux {
			t.Errorf("%s : %d anneaux, attendu %d (miettes %d)", c.nom, got, c.anneaux, b.Liseres.Miettes)
		}
	}
}

// Une composante de zone juste assez grande pour survivre a eteintLesMiettes garde son anneau.
func TestTraitsLisereConcordeAvecLesMiettesDeZone(t *testing.T) {
	nMin := int(pixelsMinDUnAnneau(mppTraits)) // 60 pixels a 0,029 u
	rel := reliefDeGrille(40, 40, mppTraits, func(int, int) (float64, bool) { return 0, true }, func(px, py int) bool {
		return py == 20 && px >= 5 && px < 5+nMin/2 || py == 21 && px >= 5 && px < 5+nMin-nMin/2
	})
	zone, eteints := eteintLesMiettes(rel.Zone, rel.calage)
	if eteints != 0 || !egauxBool(zone, rel.Zone) {
		t.Fatalf("la composante de %d pixels est une miette de zone", nMin)
	}
	traits(t, rel)
	if n := len(rel.Donnees.Lines.Liseres); n != 1 {
		t.Errorf("%d anneaux, attendu 1 : une composante gardee garde son anneau", n)
	}
}

func egauxBool(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}
