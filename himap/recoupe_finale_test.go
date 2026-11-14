package himap

import (
	"context"
	"math"
	"testing"
)

// rogneEtPoseUnCalqueApres joue la chaine en miniature : reference et rognage sur l'anneau, puis
// de l'eau posee APRES le rognage en quatre cellules (deux hors du masque, une dans le trou
// interieur, une dans l'anneau) et une cellule de matiere peinte hors du masque.
func rogneEtPoseUnCalqueApres(t *testing.T, o OptionsNavmesh) (*Rendu, BilanCuisson) {
	t.Helper()
	ctx := context.Background()
	r := renduPleinDeMatiere()
	var b BilanCuisson
	o.Maillage, o.ancres = anneauMaillage(), [][3]float64{{3, 3, 0}}
	appliqueNavmesh(ctx, r, &b, o, etapeReference)
	appliqueNavmesh(ctx, r, &b, o, etapeRognage)
	r.eau = make([]bool, r.NX*r.NY)
	for _, c := range [][2]int{{0, 0}, {9, 9}, {4, 4}, {3, 3}} {
		r.eau[c[1]*r.NX+c[0]] = true
	}
	b.CellulesEau = 4
	r.z[0] = 0 // un calque peint hors du masque, apres le rognage
	return r, b
}

// L'eau posee hors du masque est effacee par l'etape finale ; celle du trou interieur reste en
// mode silhouette (le trou est dans la forme), celle de l'anneau aussi. La matiere peinte hors
// du masque est retiree, et le bilan compte ce qui est parti.
func TestEtapeFinaleEffaceLEauHorsDeLaSilhouette(t *testing.T) {
	r, b := rogneEtPoseUnCalqueApres(t, OptionsNavmesh{Rogne: true, Mode: ModeSilhouette, Marge: 0.2})
	if !r.Eau(0, 0) || !r.Eau(9, 9) {
		t.Fatal("l'eau hors du masque doit deborder avant l'etape finale")
	}
	appliqueNavmesh(context.Background(), r, &b, OptionsNavmesh{}, etapeFinale)
	for _, c := range []struct {
		i, j int
		eau  bool
	}{{0, 0, false}, {9, 9, false}, {4, 4, true}, {3, 3, true}} {
		if got := r.Eau(c.i, c.j); got != c.eau {
			t.Errorf("eau en (%d, %d) = %v apres l'etape finale, attendu %v", c.i, c.j, got, c.eau)
		}
	}
	if !math.IsInf(r.z[0], -1) {
		t.Error("la matiere peinte hors du masque est restee")
	}
	if b.Navmesh.EauRetiree != 2 || b.Navmesh.CellulesFinales != 1 || b.CellulesEau != 2 {
		t.Errorf("bilan : %d cellules d'eau retirees, %d de matiere, %d d'eau restantes ; attendu 2, 1 et 2",
			b.Navmesh.EauRetiree, b.Navmesh.CellulesFinales, b.CellulesEau)
	}
}

// En mode faces le masque garde son trou : l'eau du trou part aussi.
func TestEtapeFinaleModeFacesRetireAussiLeTrou(t *testing.T) {
	r, b := rogneEtPoseUnCalqueApres(t, OptionsNavmesh{Rogne: true, Mode: ModeFaces, Marge: 0.2})
	appliqueNavmesh(context.Background(), r, &b, OptionsNavmesh{}, etapeFinale)
	if r.Eau(4, 4) || !r.Eau(3, 3) {
		t.Errorf("mode faces : eau du trou %v (attendu retiree), eau de l'anneau %v (attendue gardee)", r.Eau(4, 4), r.Eau(3, 3))
	}
	if b.Navmesh.EauRetiree != 3 {
		t.Errorf("%d cellules d'eau retirees, attendu 3", b.Navmesh.EauRetiree)
	}
}

// Sans rognage (`Rogne` desarme) il n'y a pas de silhouette : l'etape finale ne coupe rien.
func TestEtapeFinaleSansRognageNeCoupeRien(t *testing.T) {
	r, b := rogneEtPoseUnCalqueApres(t, OptionsNavmesh{Mode: ModeSilhouette, Marge: 0.2})
	appliqueNavmesh(context.Background(), r, &b, OptionsNavmesh{}, etapeFinale)
	if !r.Eau(0, 0) || !r.Eau(9, 9) || math.IsInf(r.z[0], -1) {
		t.Error("l'etape finale a coupe sans silhouette de rognage")
	}
	if b.Navmesh.EauRetiree != 0 || b.CellulesEau != 4 {
		t.Errorf("bilan : %d retirees, %d restantes ; attendu 0 et 4", b.Navmesh.EauRetiree, b.CellulesEau)
	}
}
