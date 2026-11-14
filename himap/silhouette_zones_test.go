package himap

import (
	"context"
	"math"
	"testing"
)

// zonesSurLesDeuxPremieresColonnes : un masque de zones qui ne couvre que les cellules d'indice
// i < 3 de la grille de renduPleinDeMatiere.
func zonesSurLesDeuxPremieresColonnes(r *Rendu) []bool {
	m := make([]bool, r.NX*r.NY)
	for k := range m {
		m[k] = k%r.NX < 3
	}
	return m
}

// Le plan est le masque de reference : une cellule hors des zones mais dans la silhouette du plan
// survit au rognage aux zones ; une cellule dans les zones survit ; une cellule hors des deux est
// effacee. Sans rognage au maillage, le rognage aux zones est inchange.
func TestSilhouettePrimeSurLeRognageAuxZones(t *testing.T) {
	cas := []struct {
		nom                              string
		rogne                            bool
		dansZones, dansPlan, horsDesDeux bool // survie attendue
		sauvees                          int
	}{
		{"avec rognage au maillage", true, true, true, false, 30},
		{"sans rognage au maillage", false, true, false, false, 0},
	}
	for _, c := range cas {
		r := renduPleinDeMatiere()
		var b BilanCuisson
		o := OptionsNavmesh{Maillage: anneauMaillage(), Rogne: c.rogne, Mode: ModeSilhouette, Marge: 0.2,
			ancres: [][3]float64{{3, 3, 0}}}
		appliqueNavmesh(context.Background(), r, &b, o, etapePlan)
		r.rogneAuxZonesEtSilhouette(context.Background(), &b, zonesSurLesDeuxPremieresColonnes(r))
		survit := func(i, j int) bool { return !math.IsInf(r.z[j*r.NX+i], -1) }
		// (0, 0) : dans les zones, hors du plan ; (5, 5) : dans le plan (le trou est rempli),
		// hors des zones ; (9, 9) : hors des deux.
		if got := survit(0, 0); got != c.dansZones {
			t.Errorf("%s : cellule dans les zones survit = %v, attendu %v", c.nom, got, c.dansZones)
		}
		if got := survit(5, 5); got != c.dansPlan {
			t.Errorf("%s : cellule dans le plan hors des zones survit = %v, attendu %v", c.nom, got, c.dansPlan)
		}
		if got := survit(9, 9); got != c.horsDesDeux {
			t.Errorf("%s : cellule hors des deux survit = %v, attendu %v", c.nom, got, c.horsDesDeux)
		}
		if b.CellulesSauveesParPlan != c.sauvees {
			t.Errorf("%s : %d cellules sauvees par le plan, attendu %d", c.nom, b.CellulesSauveesParPlan, c.sauvees)
		}
	}
}

// Sans silhouette posee (etape non jouee), l'union rend le masque des zones tel quel.
func TestAvecLaSilhouetteDuPlanSansSilhouetteRendLesZones(t *testing.T) {
	r := renduPleinDeMatiere()
	zones := zonesSurLesDeuxPremieresColonnes(r)
	union, sauvees := r.avecLaSilhouetteDuPlan(context.Background(), zones)
	if sauvees != 0 || &union[0] != &zones[0] {
		t.Errorf("sans silhouette : %d sauvees, masque recopie %v ; attendu 0 et le meme masque", sauvees, &union[0] != &zones[0])
	}
}
