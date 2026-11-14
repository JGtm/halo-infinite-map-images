package himap

import (
	"context"
	"math"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// anneauEtIlot : l'anneau de anneauMaillage (face 0 a 7) et un ilot de 5 m2 dans le coin de la
// grille, hors de l'anneau, sans ancre (face 8). Ancre posee dans l'anneau.
func anneauEtIlot() (*hinavmesh.Maillage, [][3]float64) {
	m := anneauMaillage()
	ajouteRect(m, 0, 0, 2.5, 2, 0)
	return m, [][3]float64{{3, 3, 0}}
}

// A l'etape de reference, une composante ecartee n'a pas de reference : elle reste a NaN.
// Mutation jouee : passer `nil` au lieu de `res.Garde` a ArmeReferenceDepuisNavmesh.
func TestEtapeReferenceLaisseNaNLaComposanteEcartee(t *testing.T) {
	m, ancres := anneauEtIlot()
	r := renduPleinDeMatiere()
	var b BilanCuisson
	appliqueNavmesh(context.Background(), r, &b, OptionsNavmesh{Maillage: m, ancres: ancres}, etapeReference)
	if b.Navmesh.Gardees != 1 || b.Navmesh.Ecartees != 1 {
		t.Fatalf("composantes %+v, attendu 1 gardee (l'anneau) et 1 ecartee (l'ilot)", b.Navmesh)
	}
	if v := r.referenceNavmesh[0]; !math.IsNaN(v) {
		t.Errorf("reference de la cellule de l'ilot ecarte = %v, attendu NaN", v)
	}
	if v := r.referenceNavmesh[2*r.NX+2]; math.IsNaN(v) {
		t.Error("reference d'une cellule de l'anneau retenu = NaN")
	}
}

// Sans `Rogne`, l'etape de rognage pose quand meme la couverture finale : un ilot sans ancre,
// entierement sur la matiere, est couvert, et aucune matiere n'est effacee.
// Mutation jouee : ne faire la selection de l'etape de rognage que si `Rogne` est armee.
func TestCouvertureFinaleSansRognage(t *testing.T) {
	m, ancres := anneauEtIlot()
	r := renduPleinDeMatiere()
	var b BilanCuisson
	o := OptionsNavmesh{Maillage: m, ancres: ancres}
	ctx := context.Background()
	appliqueNavmesh(ctx, r, &b, o, etapeReference)
	if r.couvertureNavmesh[0] {
		t.Fatal("apres la reference seule, l'ilot sans ancre ne devrait pas etre couvert")
	}
	appliqueNavmesh(ctx, r, &b, o, etapeRognage)
	if !r.couvertureNavmesh[0] {
		t.Error("apres l'etape de rognage sans Rogne, l'ilot garde par la matiere n'est pas couvert")
	}
	if b.Navmesh.Gardees != 2 || b.Navmesh.ParMatiere != 1 {
		t.Errorf("composantes %+v, attendu 2 gardees dont 1 par matiere", b.Navmesh)
	}
	if got := matiereRestante(r); got != 100 || b.CellulesHorsNavmesh != 0 {
		t.Errorf("%d cellules de matiere, %d effacees : sans Rogne rien ne doit etre efface", got, b.CellulesHorsNavmesh)
	}
	if b.Navmesh.Mode != "" || b.Navmesh.Marge != 0 {
		t.Errorf("mode %q et marge %g ecrits sans rognage", b.Navmesh.Mode, b.Navmesh.Marge)
	}
}

// PlanPNG avec un masque qui ecarte une face : ses pixels restent du papier.
// Mutation jouee : ignorer `garde` dans PlanPNG.
func TestPlanPNGIgnoreLesFacesEcartees(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 1, 1, 4, 4, 0) // en bas a gauche
	ajouteRect(m, 6, 6, 9, 9, 0) // en haut a droite
	img, err := PlanPNG(m, []bool{true, false}, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(2, 7); got != teinteMilieu { // x = 2,5 ; y = 2,5
		t.Errorf("face gardee : %v, attendu la teinte du milieu", got)
	}
	for _, p := range [][2]int{{7, 2}, {6, 1}, {8, 3}} { // dedans, au bord et sur le contour de la face ecartee
		if got := img.RGBAAt(p[0], p[1]); got != teintePlanPapier {
			t.Errorf("pixel %v de la face ecartee = %v, attendu le papier", p, got)
		}
	}
}

// Carre de 6 m (de 2 a 8 m) a 1 m/px dans un cadre de 10 x 10 : 25 pixels de face hors trait (le
// bloc 3..7 x 3..7), 24 de trait (le cadre de 7 x 7 pixels de 2 a 8), 51 de papier.
// Mutation jouee : aplat decale d'un pixel.
func TestPlanPNGCompteExactSurUnCarre(t *testing.T) {
	img, err := PlanPNG(carreMaillage(), nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	var aplat, trait, papier int
	minX, minY, maxX, maxY := 99, 99, -1, -1
	for py := 0; py < 10; py++ {
		for px := 0; px < 10; px++ {
			switch img.RGBAAt(px, py) {
			case teinteMilieu:
				aplat++
				minX, maxX = min(minX, px), max(maxX, px)
				minY, maxY = min(minY, py), max(maxY, py)
			case teinteAreteEncre:
				trait++
			case teintePlanPapier:
				papier++
			}
		}
	}
	if aplat != 25 || trait != 24 || papier != 51 {
		t.Errorf("%d pixels d'aplat, %d de trait, %d de papier ; attendu 25, 24 et 51", aplat, trait, papier)
	}
	if minX != 3 || maxX != 7 || minY != 3 || maxY != 7 {
		t.Errorf("aplat de (%d, %d) a (%d, %d), attendu de (3, 3) a (7, 7)", minX, minY, maxX, maxY)
	}
}
