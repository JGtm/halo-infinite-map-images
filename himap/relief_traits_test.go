package himap

// relief_traits_test.go — les traits du relief sur des grilles synthetiques : convention du demi-pixel,
// chainage des ruptures, jonctions, courbes sur pente et sur sol plat, lisere, miettes, tolerance de la
// simplification, encodage, determinisme.

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"
)

// mppTraits : l'echelle de Recharge, pour que les seuils en u et en pixels jouent comme sur une carte.
const mppTraits = 0.029

// reliefDeGrille fabrique un relief de w x h pixels sans passer par la cuisson : `haut` rend la hauteur
// d'un pixel (py vers le bas) et si elle est connue ; `zone` dit s'il est dans la zone (nil : la zone
// est tout ce qui est connu). Les hauteurs sont quantifiees comme celles de CalculeRelief.
func reliefDeGrille(w, h int, mpp float64, haut func(px, py int) (float64, bool), zone func(px, py int) bool) *Relief {
	n := w * h
	rel := &Relief{
		calage:  CalageFond{MetresParPixel: mpp, Largeur: w, Hauteur: h},
		hauteur: make([]float64, n), connu: make([]bool, n), Zone: make([]bool, n),
	}
	rel.Donnees = nouvellesDonneesRelief(rel.calage)
	z0 := math.Inf(1)
	for k := 0; k < n; k++ {
		if v, ok := haut(k%w, k/w); ok {
			z0 = math.Min(z0, v)
		}
	}
	z0 = math.Floor(z0)
	rel.Donnees.Height.Z0 = z0
	for k := 0; k < n; k++ {
		v, ok := haut(k%w, k/w)
		if !ok {
			continue
		}
		q, _ := quantifie(v, z0)
		rel.hauteur[k], rel.connu[k] = decode(q, z0), true
		rel.Zone[k] = zone == nil || zone(k%w, k/w)
	}
	return rel
}

// traits lance CalculeTraits et rend le bilan.
func traits(t *testing.T, rel *Relief) BilanTraits {
	t.Helper()
	b, err := rel.CalculeTraits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bouts compte, sur toutes les polylignes d'une famille, combien de fois chaque point est une extremite.
func bouts(polys [][]int) map[point]int {
	out := map[point]int{}
	for _, p := range polys {
		pts := decodePolyligne(p)
		out[pts[0]]++
		out[pts[len(pts)-1]]++
	}
	return out
}

// marcheCarree : un plateau de 1 u sur [20, 40[ x [20, 40[ dans une grille de 60 x 60 a 0.
func marcheCarree(px, py int) (float64, bool) {
	if px >= 20 && px < 40 && py >= 20 && py < 40 {
		return 1, true
	}
	return 0, true
}

// Convention du demi-pixel : une marche entre les colonnes 10 et 11 donne un trait a u = 105, d'un bout
// a l'autre de la grille, et aucun autre trait.
func TestTraitsMarcheDroiteAuDemiPixel(t *testing.T) {
	rel := reliefDeGrille(30, 40, mppTraits, func(px, _ int) (float64, bool) {
		if px <= 10 {
			return 0, true
		}
		return 1, true
	}, nil)
	b := traits(t, rel)
	r := rel.Donnees.Lines.Ruptures
	if len(r) != 1 {
		t.Fatalf("%d ruptures, attendu 1 : %v", len(r), r)
	}
	if want := []int{105, 0, 0, 390}; !egaux(r[0], want) {
		t.Errorf("rupture %v, attendu %v (colonne 10,5 de la ligne 0 a la ligne 39)", r[0], want)
	}
	if len(rel.Donnees.Lines.Courbes) != 0 || b.Jonctions != 0 {
		t.Errorf("courbes %d, jonctions %d : attendu aucune", len(rel.Donnees.Lines.Courbes), b.Jonctions)
	}
}

// Une marche fermee (un plateau) donne une seule rupture FERMEE : elle se termine sur son premier point,
// aucune extremite pendante ; simplifiee a ses huit coins chanfreines.
func TestTraitsMarcheFermeeSansExtremitePendante(t *testing.T) {
	rel := reliefDeGrille(60, 60, mppTraits, marcheCarree, nil)
	traits(t, rel)
	r := rel.Donnees.Lines.Ruptures
	if len(r) != 1 {
		t.Fatalf("%d ruptures, attendu une boucle", len(r))
	}
	pts := decodePolyligne(r[0])
	if pts[0] != pts[len(pts)-1] {
		t.Errorf("boucle ouverte : %v ... %v", pts[0], pts[len(pts)-1])
	}
	if n := len(pts) - 1; n != 4 && n != 8 {
		t.Errorf("%d sommets, attendu 4 ou 8 (coins chanfreines) : %v", n, pts)
	}
	for _, p := range pts {
		if p.x < 19 || p.x > 40 || p.y < 19 || p.y > 40 {
			t.Errorf("sommet %v loin du bord du plateau", p)
		}
	}
}

// Une marche en escalier (trois paliers en diagonale) : les ruptures se chainent sans extremite pendante
// a l'interieur de la grille ; chaque bout est au bord de la grille ou sur une jonction partagee.
func TestTraitsMarcheEnEscalierChainee(t *testing.T) {
	rel := reliefDeGrille(60, 60, mppTraits, func(px, py int) (float64, bool) {
		return float64((px+py)/20) * 0.6, true
	}, nil)
	traits(t, rel)
	r := rel.Donnees.Lines.Ruptures
	if len(r) == 0 {
		t.Fatal("aucune rupture")
	}
	for p, n := range bouts(r) {
		auBord := p.x <= 0.5 || p.y <= 0.5 || p.x >= 58.5 || p.y >= 58.5
		if !auBord && n < 2 {
			t.Errorf("extremite pendante en %v (%d chaines)", p, n)
		}
	}
}

// Une jonction en T donne trois chaines qui partagent le centre de la cellule.
func TestTraitsJonctionEnT(t *testing.T) {
	rel := reliefDeGrille(40, 40, mppTraits, func(px, py int) (float64, bool) {
		switch {
		case py < 20:
			return 0, true
		case px < 20:
			return 1, true
		}
		return 2, true
	}, nil)
	b := traits(t, rel)
	if b.Jonctions != 1 || b.JonctionsGardees != 1 {
		t.Fatalf("jonctions %d (gardees %d), attendu 1", b.Jonctions, b.JonctionsGardees)
	}
	r := rel.Donnees.Lines.Ruptures
	if len(r) != 3 {
		t.Fatalf("%d ruptures, attendu 3", len(r))
	}
	centre := point{19.5, 19.5}
	if n := bouts(r)[centre]; n != 3 {
		t.Errorf("le centre %v termine %d chaines, attendu 3", centre, n)
	}
}

// Une marche franche ne se double d'aucune courbe : le lissage y fabrique une pente que la rupture
// trace deja. Le sol bas est pose sur le niveau 0, le pire cas. Une rampe loin de toute marche garde
// ses courbes : 2,5, 3 et 3,5 u, a px = 100 + (t - 2,2) / (0,6 x 0,029).
func TestTraitsCourbesLoinDesRuptures(t *testing.T) {
	const pente = 0.6
	rel := reliefDeGrille(200, 60, mppTraits, func(px, _ int) (float64, bool) {
		switch {
		case px <= 40:
			return 0, true
		case px < 100:
			return 2.2, true
		}
		return 2.2 + pente*float64(px-100)*mppTraits, true
	}, nil)
	traits(t, rel)
	l := rel.Donnees.Lines
	if len(l.Ruptures) != 1 {
		t.Errorf("%d ruptures, attendu la marche seule", len(l.Ruptures))
	}
	if len(l.Courbes) != 3 {
		t.Fatalf("%d courbes, attendu les 3 de la rampe : %v", len(l.Courbes), l.Courbes)
	}
	for k, c := range l.Courbes {
		attendu := 100 + (2.5+0.5*float64(k)-2.2)/(pente*mppTraits)
		for _, p := range decodePolyligne(c) {
			if math.Abs(p.x-40.5) <= RayonSansCourbePresDUneRupture+1 {
				t.Errorf("courbe %d : point %v dans le rayon de la marche", k, p)
			}
			if math.Abs(p.x-attendu) > 0.6 {
				t.Errorf("courbe %d : point %v, attendu x = %.2f", k, p, attendu)
			}
		}
	}
}

// Une rampe douce : aucune rupture, des courbes paralleles tous les EquidistanceCourbes.
func TestTraitsRampeDouce(t *testing.T) {
	const pente = 0.3 // u par u : 0,0087 u par pixel, loin de SeuilRupture
	rel := reliefDeGrille(200, 60, mppTraits, func(px, _ int) (float64, bool) {
		return pente * float64(px) * mppTraits, true
	}, nil)
	traits(t, rel)
	l := rel.Donnees.Lines
	if len(l.Ruptures) != 0 {
		t.Errorf("%d ruptures sur une rampe douce", len(l.Ruptures))
	}
	// La rampe monte de 0 a 1,73 u : niveaux 0,5, 1 et 1,5.
	if len(l.Courbes) != 3 {
		t.Fatalf("%d courbes, attendu 3", len(l.Courbes))
	}
	for k, c := range l.Courbes {
		pts := decodePolyligne(c)
		attendu := float64(k+1) * EquidistanceCourbes / (pente * mppTraits)
		for _, p := range pts {
			if math.Abs(p.x-attendu) > 0.6 {
				t.Errorf("courbe %d : point %v, attendu x = %.2f", k, p, attendu)
			}
		}
	}
}

// Un sol plat a 2,00 u exactement, bruite de plus ou moins 0,01 u, ne trace aucune courbe ; un sol plat
// sans bruit non plus.
func TestTraitsSolPlatSansCourbe(t *testing.T) {
	graine := uint32(12345)
	bruit := make([]float64, 120*120)
	for k := range bruit {
		graine = graine*1664525 + 1013904223
		bruit[k] = (float64(graine>>8)/float64(1<<24))*0.02 - 0.01
	}
	for _, c := range []struct {
		nom  string
		haut func(px, py int) (float64, bool)
	}{
		{"plat", func(int, int) (float64, bool) { return 2, true }},
		{"bruite", func(px, py int) (float64, bool) { return 2 + bruit[py*120+px], true }},
	} {
		rel := reliefDeGrille(120, 120, mppTraits, c.haut, nil)
		traits(t, rel)
		if n := len(rel.Donnees.Lines.Courbes); n != 0 {
			t.Errorf("%s : %d courbes sur un sol plat", c.nom, n)
		}
		if n := len(rel.Donnees.Lines.Ruptures); n != 0 {
			t.Errorf("%s : %d ruptures sur un sol plat", c.nom, n)
		}
	}
}

// Une cuvette (paraboloide) : des courbes fermees, concentriques, qui se terminent sur leur premier point.
func TestTraitsCuvetteCourbesFermees(t *testing.T) {
	rel := reliefDeGrille(160, 160, mppTraits, func(px, py int) (float64, bool) {
		dx, dy := float64(px-80)*mppTraits, float64(py-80)*mppTraits
		return 0.5 * (dx*dx + dy*dy), true
	}, nil)
	traits(t, rel)
	l := rel.Donnees.Lines
	if len(l.Ruptures) != 0 {
		t.Errorf("%d ruptures dans une cuvette douce", len(l.Ruptures))
	}
	fermees := 0
	for _, c := range l.Courbes {
		if pts := decodePolyligne(c); pts[0] == pts[len(pts)-1] {
			fermees++
		}
	}
	// Rayon 2,32 u au bord : niveaux 0,5 (r = 1), 1 (r = 1,41), 1,5 (r = 1,73), 2 (r = 2) entiers dans la grille.
	if fermees < 4 {
		t.Errorf("%d courbes fermees sur %d, attendu au moins 4", fermees, len(l.Courbes))
	}
}

// Miettes de rupture : une bosse isolee de 3 pixels (moins de 0,5 u de pourtour) s'ote ; un plateau de
// 20 pixels de cote reste.
func TestTraitsMiettesDeRupture(t *testing.T) {
	rel := reliefDeGrille(80, 80, mppTraits, func(px, py int) (float64, bool) {
		if px >= 10 && px < 13 && py >= 10 && py < 11 {
			return 1, true
		}
		return marcheCarree(px, py)
	}, nil)
	b := traits(t, rel)
	if n := len(rel.Donnees.Lines.Ruptures); n != 1 || b.Ruptures.Miettes != 1 {
		t.Errorf("%d ruptures, %d miettes ; attendu le plateau seul et une miette", n, b.Ruptures.Miettes)
	}
}

// Determinisme : deux executions sur la meme entree donnent le meme JSON a l'octet.
func TestTraitsDeterministes(t *testing.T) {
	haut := func(px, py int) (float64, bool) {
		dx, dy := float64(px-70)*mppTraits, float64(py-50)*mppTraits
		z := 0.4*(dx*dx+dy*dy) + float64((px/25+py/30)%3)*0.5
		return z, (px-70)*(px-70)+(py-50)*(py-50) < 60*60
	}
	var sorties [2][]byte
	for k := range sorties {
		rel := reliefDeGrille(140, 100, mppTraits, haut, nil)
		traits(t, rel)
		b, err := json.Marshal(rel.Donnees.Lines)
		if err != nil {
			t.Fatal(err)
		}
		sorties[k] = b
	}
	if !bytes.Equal(sorties[0], sorties[1]) {
		t.Error("deux executions, deux sorties")
	}
	if len(sorties[0]) < 100 {
		t.Errorf("sortie trop maigre pour juger : %s", sorties[0])
	}
}

// De bout en bout : CalculeRelief puis CalculeTraits sur un rendu a une marche.
func TestTraitsApresCalculeRelief(t *testing.T) {
	r := renduDe(40, 20, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) {
		if i < 20 {
			return 0, true
		}
		return 1, true
	})
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	traits(t, rel)
	l := rel.Donnees.Lines
	if len(l.Ruptures) != 1 || !egaux(l.Ruptures[0], []int{195, 0, 0, 190}) {
		t.Errorf("ruptures %v, attendu [[195 0 0 190]]", l.Ruptures)
	}
	if len(l.Liseres) != 1 {
		t.Errorf("%d anneaux, attendu le bord du cadre", len(l.Liseres))
	}
}

func TestTraitsReliefNonCalcule(t *testing.T) {
	if _, err := (&Relief{}).CalculeTraits(context.Background()); err == nil {
		t.Error("relief vide accepte")
	}
	var rel *Relief
	if _, err := rel.CalculeTraits(context.Background()); err == nil {
		t.Error("relief nil accepte")
	}
}

func egaux(a, b []int) bool {
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
