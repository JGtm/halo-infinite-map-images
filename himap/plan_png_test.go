package himap

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// teinteMilieu : la teinte d'une carte a altitude unique, le milieu de la rampe.
var teinteMilieu = teinteDeRampe(0.5)

// calageDix : un cadre de 10 x 10 pixels d'un metre, dont le coin haut gauche est en (0, 10).
var calageDix = CalageFond{OrigineX: 0, OrigineY: 10, MetresParPixel: 1, Largeur: 10, Hauteur: 10}

func TestPlanDesMursNeLitQueLesFacesGardees(t *testing.T) {
	m := maillageTroisFaces()
	tout, err := PlanDesMurs(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	premiere, err := PlanDesMurs(m, []bool{true, false, false})
	if err != nil {
		t.Fatal(err)
	}
	if len(tout.Aretes) != 6 || len(premiere.Aretes) != 3 {
		t.Errorf("%d aretes pour tout le maillage et %d pour la premiere face ; attendu 6 et 3", len(tout.Aretes), len(premiere.Aretes))
	}
	// Une garde plus courte que le maillage ecarte les faces au-dela.
	courte, err := PlanDesMurs(m, []bool{true})
	if err != nil || !reflect.DeepEqual(courte.Aretes, premiere.Aretes) {
		t.Errorf("garde courte : %v (erreur %v), attendu %v", courte.Aretes, err, premiere.Aretes)
	}
	// Deux faces gardees qui se partagent une arete : elle est un passage.
	deux, _ := PlanDesMurs(m, []bool{true, true, false})
	if len(deux.Aretes) != 5 {
		t.Errorf("%d aretes pour deux faces voisines, attendu 5", len(deux.Aretes))
	}
}

func TestPlanRogneAuCadreCoupeLesSegmentsQuiSortent(t *testing.T) {
	plan := PlanMurs{Sommets: 4, Faces: 2, Aretes: [][4]float64{
		{2, 5, 4, 5},    // dedans : inchange
		{5, 5, 15, 5},   // sort a droite : coupe en x = 10
		{-5, 5, 5, 5},   // sort a gauche : coupe en x = 0
		{11, 0, 12, 12}, // entierement dehors
		{0, 12, 10, 12}, // parallele au bord haut, dehors
		{5, 2, 5, 20},   // sort par le haut : coupe en y = 10
	}}
	got := PlanRogneAuCadre(plan, calageDix)
	attendu := [][4]float64{{2, 5, 4, 5}, {5, 5, 10, 5}, {0, 5, 5, 5}, {5, 2, 5, 10}}
	if !reflect.DeepEqual(got.Aretes, attendu) {
		t.Errorf("aretes = %v, attendu %v", got.Aretes, attendu)
	}
	if got.Sommets != 4 || got.Faces != 2 {
		t.Errorf("taille = %d sommets, %d faces ; attendu 4 et 2", got.Sommets, got.Faces)
	}
	if vide := PlanRogneAuCadre(plan, CalageFond{}); len(vide.Aretes) != 0 {
		t.Errorf("cadre sans surface : %d aretes", len(vide.Aretes))
	}
}

// carreMaillage : une face carree de 2 a 8 m en X et en Y.
func carreMaillage() *hinavmesh.Maillage {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 2, 2, 8, 8, 0)
	return m
}

func pixelPlan(t *testing.T, m *hinavmesh.Maillage, px, py int) color.RGBA {
	t.Helper()
	img, err := PlanPNG(m, nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	return img.RGBAAt(px, py)
}

func TestPlanPNGDimensionsEtPixelsSurUnCarre(t *testing.T) {
	img, err := PlanPNG(carreMaillage(), nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("dimensions %v, attendu 10 x 10", b)
	}
	cas := []struct {
		px, py  int
		attendu color.RGBA
		nom     string
	}{
		{0, 0, teintePlanVide, "coin hors silhouette"},
		{5, 5, teinteMilieu, "interieur"},
		{4, 4, teinteMilieu, "interieur excentre"},
		{2, 5, teinteAreteEncre, "bord gauche (x = 2 m)"},
		{8, 5, teinteAreteEncre, "bord droit (x = 8 m)"},
		{5, 2, teinteAreteEncre, "bord haut (y = 8 m)"},
		{5, 8, teinteAreteEncre, "bord bas (y = 2 m)"},
		{9, 9, teintePlanVide, "coin bas droit"},
	}
	for _, c := range cas {
		if got := img.RGBAAt(c.px, c.py); got != c.attendu {
			t.Errorf("%s : pixel (%d, %d) = %v, attendu %v", c.nom, c.px, c.py, got, c.attendu)
		}
	}
}

// Le plan se pose dans le meme sens que le fond : le haut de l'image est le nord du monde.
func TestPlanPNGRespecteLeSensDuMonde(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 0, 8, 2, 10, 0) // coin haut gauche du cadre : x 0..2, y 8..10
	if got := pixelPlan(t, m, 0, 0); got == teintePlanVide {
		t.Errorf("pixel haut gauche = vide, la face est au nord-ouest du cadre")
	}
	if got := pixelPlan(t, m, 0, 9); got != teintePlanVide {
		t.Errorf("pixel bas gauche = %v, attendu le vide transparent", got)
	}
}

// Les trous interieurs (rochers, piliers) n'ont pas de face : ils restent transparents, et leurs
// aretes sont tracees.
func TestPlanPNGLaisseLesTrousTransparentsEtTraceLeursBords(t *testing.T) {
	img, err := PlanPNG(anneauMaillage(), nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(5, 5); got != teintePlanVide {
		t.Errorf("centre du trou = %v, attendu le vide transparent", got)
	}
	if got := img.RGBAAt(4, 5); got != teinteAreteEncre {
		t.Errorf("bord du trou = %v, attendu le trait", got)
	}
}

func TestPlanPNGCadreSansSurfaceEtMaillageIncoherent(t *testing.T) {
	if _, err := PlanPNG(carreMaillage(), nil, CalageFond{}); err == nil {
		t.Error("cadre sans surface : pas d'erreur")
	}
	incoherent := &hinavmesh.Maillage{
		Sommets: []hinavmesh.Point{{X: 0, Y: 0}, {X: 1, Y: 0}},
		Faces:   []hinavmesh.Face{{Sommets: []int32{0, 1, 9}}},
	}
	if _, err := PlanPNG(incoherent, nil, calageDix); err == nil {
		t.Error("maillage incoherent : pas d'erreur")
	}
	if img, err := PlanPNG(nil, nil, calageDix); err != nil || img.RGBAAt(5, 5) != teintePlanVide {
		t.Errorf("maillage nil : vide transparent attendu (erreur %v)", err)
	}
}

func TestCalageFondDuRenduDitLaMemeChoseQueCalageDuRendu(t *testing.T) {
	r := NewRendu([2]float64{-10, 20}, [2]float64{-5, 26}, 0.5)
	x0, y0, mpp := CalageDuRendu(r)
	cal := CalageFondDuRendu(r)
	if cal.OrigineX != x0 || cal.OrigineY != y0 || cal.MetresParPixel != mpp || cal.Largeur != r.NX || cal.Hauteur != r.NY {
		t.Errorf("calage %+v, attendu %v, %v, %v et %d x %d", cal, x0, y0, mpp, r.NX, r.NY)
	}
}

// Le vide du plan est transparent : hors faces et dans les trous l'alpha est 0 (et la couleur 0,
// seule forme valide en alpha premultiplie), partout ailleurs 255, sans alpha partiel, et aucun
// pixel n'a la teinte de l'ancien papier opaque. Mutation jouee : papier (240, 241, 236, 255).
func TestPlanPNGVideTransparentSansAlphaPartiel(t *testing.T) {
	ancienPapier := color.RGBA{240, 241, 236, 255}
	for nom, m := range map[string]*hinavmesh.Maillage{"carre": carreMaillage(), "anneau": anneauMaillage(), "nil": nil} {
		img, err := PlanPNG(m, nil, calageDix)
		if err != nil {
			t.Fatal(err)
		}
		transparents := 0
		for py := 0; py < 10; py++ {
			for px := 0; px < 10; px++ {
				c := img.RGBAAt(px, py)
				switch {
				case c == ancienPapier:
					t.Fatalf("%s : pixel (%d, %d) a la teinte de l'ancien papier opaque", nom, px, py)
				case c.A == 0:
					transparents++
					if c != (color.RGBA{}) {
						t.Errorf("%s : pixel (%d, %d) transparent de couleur %v, attendu 0", nom, px, py, c)
					}
				case c.A != 255:
					t.Errorf("%s : pixel (%d, %d) d'alpha partiel %d", nom, px, py, c.A)
				}
			}
		}
		if nom == "nil" && transparents != 100 {
			t.Errorf("maillage nil : %d pixels transparents, attendu 100", transparents)
		}
		if nom == "carre" && transparents != 51 {
			t.Errorf("carre : %d pixels transparents, attendu 51", transparents)
		}
	}
	// Un trou du maillage est transparent, et son bord (le trait) reste opaque.
	img, err := PlanPNG(anneauMaillage(), nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(5, 5); got.A != 0 {
		t.Errorf("centre du trou = %v, attendu alpha 0", got)
	}
	if got := img.RGBAAt(4, 5); got.A != 255 {
		t.Errorf("bord du trou = %v, attendu opaque", got)
	}
}
