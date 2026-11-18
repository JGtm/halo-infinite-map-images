package himap

// relief_projection_test.go — la projection du rendu dans le cadre publie : cadre decale, plus grand,
// plus petit, decalage non entier, echelle differente.

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestReliefCadreDecaleEtPlusGrand(t *testing.T) {
	r := renduDe(20, 20, 1, [2]float64{10, 20}, func(i, j int) (float64, bool) { return 10 + float64(i) + 0.5*float64(j), true })
	ox, oy, _ := CalageDuRendu(r)
	// Le cadre publie deborde de 2 px a gauche et de 5 px en haut : decalage (-2, -5).
	cal := CalageFond{OrigineX: ox - 2, OrigineY: oy + 5, MetresParPixel: 1, Largeur: 30, Hauteur: 30}
	rel := calcule(t, r, nil, nil, cal)
	b := rel.Bilan
	if b.DecalageX != -2 || b.DecalageY != -5 || !b.DecalageEntier {
		t.Errorf("decalage (%g, %g) entier %v, attendu (-2, -5) entier", b.DecalageX, b.DecalageY, b.DecalageEntier)
	}
	if b.HorsGrille != 500 || b.MatiereCuite != 400 || b.MatierePerdue != 0 {
		t.Errorf("hors grille %d, cuite %d, perdue %d ; attendu 500, 400, 0", b.HorsGrille, b.MatiereCuite, b.MatierePerdue)
	}
	for j := 0; j < 20; j++ {
		for i := 0; i < 20; i++ {
			// la cellule (i, j) est le pixel (i + 2, 24 - j) du cadre publie.
			attend(t, rel, i+2, 24-j, 10+float64(i)+0.5*float64(j), DrapeauSurCarte)
		}
	}
	for _, p := range [][2]int{{0, 0}, {1, 10}, {10, 4}, {25, 25}} {
		if _, _, ok := rel.lit(p[0], p[1]); ok {
			t.Errorf("pixel %v hors grille : hauteur connue", p)
		}
	}
}

func TestReliefHorsGrilleSolDuMaillage(t *testing.T) {
	r := renduDe(20, 20, 1, [2]float64{10, 20}, plat(10))
	ox, oy, _ := CalageDuRendu(r)
	cal := CalageFond{OrigineX: ox - 2, OrigineY: oy + 5, MetresParPixel: 1, Largeur: 30, Hauteur: 30}
	m := maillageDe(rect{ox - 2, oy - 25, ox + 28, oy + 5, 3})
	rel := calcule(t, r, m, nil, cal)
	attend(t, rel, 0, 0, 3, DrapeauSurCarte|DrapeauSolMaillage) // hors grille : sol du maillage
	attend(t, rel, 10, 15, 10, DrapeauSurCarte)                 // dans la grille : le rendu, plus haut
}

func TestReliefCadreRognePerdLaMatiere(t *testing.T) {
	r := renduDe(20, 20, 1, [2]float64{10, 20}, plat(1))
	ox, oy, _ := CalageDuRendu(r)
	cal := CalageFond{OrigineX: ox + 5, OrigineY: oy - 5, MetresParPixel: 1, Largeur: 10, Hauteur: 10}
	rel := calcule(t, r, nil, nil, cal)
	if rel.Bilan.MatiereCuite != 400 || rel.Bilan.MatierePerdue != 300 {
		t.Errorf("cuite %d perdue %d, attendu 400 et 300", rel.Bilan.MatiereCuite, rel.Bilan.MatierePerdue)
	}
}

func TestReliefDecalageNonEntier(t *testing.T) {
	r := renduDe(20, 20, 1, [2]float64{10, 20}, func(i, j int) (float64, bool) { return float64(i), true })
	ox, oy, _ := CalageDuRendu(r)
	cal := CalageFond{OrigineX: ox + 0.3, OrigineY: oy, MetresParPixel: 1, Largeur: 19, Hauteur: 20}
	rel := calcule(t, r, nil, nil, cal)
	if rel.Bilan.DecalageEntier || math.Abs(rel.Bilan.DecalageX-0.3) > 1e-9 {
		t.Errorf("decalage (%g, %g) entier %v, attendu (0,3 ; 0) non entier", rel.Bilan.DecalageX, rel.Bilan.DecalageY, rel.Bilan.DecalageEntier)
	}
	attend(t, rel, 7, 3, 7, DrapeauSurCarte) // plus proche voisin : le pixel 7 lit la cellule 7
}

func TestReliefEchelleDifferente(t *testing.T) {
	r := renduDe(20, 20, 1, [2]float64{0, 0}, plat(1))
	cal := CalageFondDuRendu(r)
	for _, mpp := range []float64{1.5, 1.001, 0.999} {
		cal.MetresParPixel = mpp
		if _, err := CalculeRelief(context.Background(), r, nil, nil, cal); !errors.Is(err, ErrEchelleDifferente) {
			t.Errorf("echelle %g : erreur %v, attendu ErrEchelleDifferente", mpp, err)
		}
	}
	cal.MetresParPixel = 1 + 5e-7 // sous la tolerance
	if _, err := CalculeRelief(context.Background(), r, nil, nil, cal); err != nil {
		t.Errorf("ecart de 5e-7 refuse : %v", err)
	}
}
