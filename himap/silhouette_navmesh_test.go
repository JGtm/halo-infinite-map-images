package himap

import (
	"context"
	"math"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// anneauMaillage : une grille de 3 x 3 cases de 2 m (de 2 a 8 m en X et en Y), sommets partages,
// dont la case du milieu (4 a 6 m) manque : un anneau de huit faces entourant un trou.
func anneauMaillage() *hinavmesh.Maillage {
	m := &hinavmesh.Maillage{}
	for b := 0; b < 4; b++ {
		for a := 0; a < 4; a++ {
			m.Sommets = append(m.Sommets, hinavmesh.Point{X: 2 + 2*float64(a), Y: 2 + 2*float64(b)})
		}
	}
	for b := int32(0); b < 3; b++ {
		for a := int32(0); a < 3; a++ {
			if a == 1 && b == 1 {
				continue
			}
			m.Faces = append(m.Faces, hinavmesh.Face{Sommets: []int32{b*4 + a, b*4 + a + 1, (b+1)*4 + a + 1, (b+1)*4 + a}})
		}
	}
	return m
}

// renduPleinDeMatiere : une grille de 10 x 10 cellules de 1 m, de la matiere partout.
func renduPleinDeMatiere() *Rendu {
	r := NewRendu([2]float64{0, 0}, [2]float64{9, 9}, 1)
	for k := range r.z {
		r.z[k] = 0
	}
	return r
}

func matiereRestante(r *Rendu) int {
	n := 0
	for _, z := range r.z {
		if !math.IsInf(z, -1) {
			n++
		}
	}
	return n
}

func TestRemplitTrousGardeLeVideOuvertEtRempliLeVideFerme(t *testing.T) {
	const nx, ny = 7, 7
	grille := func(ouverture bool) []bool {
		g := make([]bool, nx*ny)
		for k := 1; k <= 5; k++ {
			g[1*nx+k], g[5*nx+k], g[k*nx+1], g[k*nx+5] = true, true, true, true
		}
		if ouverture {
			g[3*nx+5] = false // une breche dans le cote droit
		}
		return g
	}
	ferme := remplitTrous(grille(false), nx, ny)
	for j := 2; j <= 4; j++ {
		for i := 2; i <= 4; i++ {
			if !ferme[j*nx+i] {
				t.Errorf("trou ferme : cellule (%d, %d) non remplie", i, j)
			}
		}
	}
	if ferme[0] || ferme[6*nx+6] {
		t.Error("l'exterieur de l'anneau a ete rempli")
	}
	ouvert := remplitTrous(grille(true), nx, ny)
	if ouvert[3*nx+3] {
		t.Error("trou ouvert sur l'exterieur : la cellule centrale ne doit pas etre remplie")
	}
}

func TestRemplitTrousGrilleVide(t *testing.T) {
	if got := remplitTrous(nil, 0, 0); len(got) != 0 {
		t.Errorf("grille vide : %v", got)
	}
}

// Le rognage en mode silhouette garde la matiere qui occupe le trou d'un anneau ; le mode
// faces l'efface.
func TestSilhouetteGardeLeTrouInterieurDUnAnneau(t *testing.T) {
	m := anneauMaillage()

	faces := renduPleinDeMatiere()
	faces.ArmeReferenceDepuisNavmesh(m, nil, false)
	if n := faces.EffaceHorsNavmesh(0); n != 68 {
		t.Errorf("mode faces : %d cellules effacees, attendu 68 (100 moins l'anneau de 32)", n)
	}
	if !math.IsInf(faces.z[4*faces.NX+4], -1) {
		t.Error("mode faces : la cellule du trou devrait etre effacee")
	}

	sil := renduPleinDeMatiere()
	sil.ArmeReferenceDepuisNavmesh(m, nil, false)
	if n := sil.EffaceHorsSilhouetteNavmesh(0); n != 64 {
		t.Errorf("mode silhouette : %d cellules effacees, attendu 64 (100 moins le carre de 36)", n)
	}
	if math.IsInf(sil.z[4*sil.NX+4], -1) {
		t.Error("mode silhouette : la cellule du trou devrait rester dessinee")
	}
	if got := matiereRestante(sil); got != 36 {
		t.Errorf("mode silhouette : %d cellules de matiere restantes, attendu 36", got)
	}
}

// La reference ne lit que les faces gardees : les autres laissent la reference a NaN.
func TestReferenceNeLitQueLesFacesGardees(t *testing.T) {
	m := anneauMaillage()
	garde := make([]bool, len(m.Faces))
	garde[0] = true // la case (2..4, 2..4)
	r := renduPleinDeMatiere()
	r.ArmeReferenceDepuisNavmesh(m, garde, false)
	couvertes := 0
	for _, c := range r.couvertureNavmesh {
		if c {
			couvertes++
		}
	}
	if couvertes != 4 {
		t.Errorf("%d cellules couvertes, attendu 4 (une case de 2 m sur 2 m)", couvertes)
	}
	if r.couvertureNavmesh[7*r.NX+7] {
		t.Error("une face ecartee couvre une cellule")
	}
}

// appliqueNavmesh au rognage : mode, marge et composantes arrivent au bilan ; un mode inconnu
// retombe sur le mode faces.
func TestAppliqueNavmeshRognageEcritLeBilan(t *testing.T) {
	ctx := context.Background()
	cas := []struct {
		mode        ModeNavmesh
		marge       float64
		attendu     ModeNavmesh
		margeEcrite float64
		efface      int
	}{
		{ModeSilhouette, 0.2, ModeSilhouette, 0.2, 64},
		{ModeFaces, 0.2, ModeFaces, 0.2, 68},
		{"", 0, ModeFaces, MargeNavmesh, -1},
		{"inconnu", 0.2, ModeFaces, 0.2, 68},
	}
	for _, c := range cas {
		r := renduPleinDeMatiere()
		var b BilanCuisson
		o := OptionsNavmesh{Maillage: anneauMaillage(), Rogne: true, Mode: c.mode, Marge: c.marge,
			ancres: [][3]float64{{3, 3, 0}}}
		appliqueNavmesh(ctx, r, &b, o, etapeReference)
		appliqueNavmesh(ctx, r, &b, o, etapeRognage)
		if b.Navmesh.Mode != c.attendu || b.Navmesh.Marge != c.margeEcrite {
			t.Errorf("mode %q : bilan %q a %g m, attendu %q a %g m", c.mode, b.Navmesh.Mode, b.Navmesh.Marge, c.attendu, c.margeEcrite)
		}
		if b.Navmesh.Gardees != 1 || b.Navmesh.Ecartees != 0 || b.Navmesh.ParAncre != 1 {
			t.Errorf("mode %q : composantes %+v, attendu 1 gardee par ancre", c.mode, b.Navmesh)
		}
		if c.efface >= 0 && b.CellulesHorsNavmesh != c.efface {
			t.Errorf("mode %q : %d cellules effacees, attendu %d", c.mode, b.CellulesHorsNavmesh, c.efface)
		}
	}
}

// Le rognage rechoisit les composantes avec la matiere : un ilot de 5 m2 sans ancre, venu avec
// la matiere, est gardee au rognage alors que la reference l'avait ignore.
func TestRognageGardeUnIlotSurLaMatiere(t *testing.T) {
	m := anneauMaillage()
	ajouteRect(m, 0, 0, 2.5, 2, 0) // un ilot de 5 m2 dans le coin, hors de l'anneau
	r := renduPleinDeMatiere()
	var b BilanCuisson
	o := OptionsNavmesh{Maillage: m, Rogne: true, Marge: 0.2, ancres: [][3]float64{{3, 3, 0}}}
	appliqueNavmesh(context.Background(), r, &b, o, etapeReference)
	if b.Navmesh.Gardees != 1 {
		t.Fatalf("reference : %d gardees, attendu 1 (l'anneau, par son ancre)", b.Navmesh.Gardees)
	}
	appliqueNavmesh(context.Background(), r, &b, o, etapeRognage)
	if b.Navmesh.Gardees != 2 || b.Navmesh.ParMatiere != 1 {
		t.Errorf("rognage : %+v, attendu 2 gardees dont 1 par matiere", b.Navmesh)
	}
	if math.IsInf(r.z[0], -1) {
		t.Error("la matiere de l'ilot a ete effacee")
	}
}
