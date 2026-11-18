package himap

// relief_test.go — le relief sur des grilles synthetiques : la hauteur du dessus (D-6, D-12), la
// zone, la projection dans le cadre publie, le codage de q.

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// celluleTest : une cellule de 0,1 u donne 10 pixels par unite ; la marge de la zone (0,5 u) en fait 5
// et une miette (0,05 u2) 5 pixels.
const celluleTest = 0.1

// renduDe fabrique un rendu de nx x ny cellules dont le coin est en `min` ; h rend la hauteur d'une
// cellule (j monte avec y) et dit si elle porte de la matiere.
func renduDe(nx, ny int, cell float64, min [2]float64, h func(i, j int) (float64, bool)) *Rendu {
	r := &Rendu{Cell: cell, Min: min, NX: nx, NY: ny, z: make([]float64, nx*ny), n: make([][3]float64, nx*ny)}
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			z, ok := h(i, j)
			if !ok {
				z = math.Inf(-1)
			}
			r.z[j*nx+i] = z
		}
	}
	return r
}

// plat rend une fonction de hauteur constante sur toute la grille.
func plat(z float64) func(i, j int) (float64, bool) {
	return func(int, int) (float64, bool) { return z, true }
}

// rect : un rectangle de sol du maillage, a la hauteur z (coordonnees monde).
type rect struct{ x0, y0, x1, y1, z float64 }

// maillageDe fabrique un maillage d'une face par rectangle.
func maillageDe(rects ...rect) *hinavmesh.Maillage {
	m := &hinavmesh.Maillage{}
	for _, r := range rects {
		n := int32(len(m.Sommets))
		m.Sommets = append(m.Sommets,
			hinavmesh.Point{X: r.x0, Y: r.y0, Z: r.z}, hinavmesh.Point{X: r.x1, Y: r.y0, Z: r.z},
			hinavmesh.Point{X: r.x1, Y: r.y1, Z: r.z}, hinavmesh.Point{X: r.x0, Y: r.y1, Z: r.z})
		m.Faces = append(m.Faces, hinavmesh.Face{Sommets: []int32{n, n + 1, n + 2, n + 3}})
	}
	return m
}

// calcule lance CalculeRelief et verifie le format de l'image.
func calcule(t *testing.T, r *Rendu, m *hinavmesh.Maillage, garde []bool, cal CalageFond) *Relief {
	t.Helper()
	rel, err := CalculeRelief(context.Background(), r, m, garde, cal)
	if err != nil {
		t.Fatal(err)
	}
	verifieFormatImage(t, rel)
	return rel
}

// verifieFormatImage controle ce que le web controlera a la lecture : A vaut 0 ou 255, A = 0 impose
// R = G = B = 0, les bits 2 a 7 de B sont nuls, la taille est celle du cadre.
func verifieFormatImage(t *testing.T, rel *Relief) {
	t.Helper()
	b := rel.Image.Bounds()
	if b.Dx() != rel.calage.Largeur || b.Dy() != rel.calage.Hauteur {
		t.Fatalf("image %dx%d, cadre %dx%d", b.Dx(), b.Dy(), rel.calage.Largeur, rel.calage.Hauteur)
	}
	for k := 0; k < len(rel.Image.Pix); k += 4 {
		r, g, bl, a := rel.Image.Pix[k], rel.Image.Pix[k+1], rel.Image.Pix[k+2], rel.Image.Pix[k+3]
		if a != 0 && a != 255 {
			t.Fatalf("pixel %d : alpha %d, attendu 0 ou 255", k/4, a)
		}
		if a == 0 && (r|g|bl) != 0 {
			t.Fatalf("pixel %d : alpha 0 mais R,G,B = %d,%d,%d", k/4, r, g, bl)
		}
		if bl&^(DrapeauSurCarte|DrapeauSolMaillage) != 0 {
			t.Fatalf("pixel %d : bits reserves de B poses (%08b)", k/4, bl)
		}
		if (a == 255) != rel.connu[k/4] {
			t.Fatalf("pixel %d : alpha %d, hauteur connue %v", k/4, a, rel.connu[k/4])
		}
		if (bl&DrapeauSurCarte != 0) != rel.Zone[k/4] {
			t.Fatalf("pixel %d : drapeau sur carte %d, zone %v", k/4, bl&DrapeauSurCarte, rel.Zone[k/4])
		}
	}
}

// lit decode le pixel (px, py) de l'image comme le fera le web : hauteur, drapeaux, hauteur connue.
func (rel *Relief) lit(px, py int) (h float64, drapeaux uint8, ok bool) {
	o := (py*rel.calage.Largeur + px) * 4
	p := rel.Image.Pix[o : o+4]
	if p[3] == 0 {
		return 0, 0, false
	}
	return rel.Donnees.Height.Z0 + float64(int(p[0])+256*int(p[1]))*rel.Donnees.Height.Pas, p[2], true
}

// attend verifie la hauteur et les drapeaux d'un pixel.
func attend(t *testing.T, rel *Relief, px, py int, h float64, drapeaux uint8) {
	t.Helper()
	got, d, ok := rel.lit(px, py)
	if !ok {
		t.Fatalf("pixel (%d,%d) : hauteur inconnue, attendu %g", px, py, h)
	}
	if math.Abs(got-h) > PasHauteur/2+1e-12 || d != drapeaux {
		t.Errorf("pixel (%d,%d) : hauteur %g drapeaux %d, attendu %g et %d", px, py, got, d, h, drapeaux)
	}
}

func TestReliefMarche(t *testing.T) {
	r := renduDe(40, 20, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) {
		if i < 20 {
			return 0, true
		}
		return 1, true
	})
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	attend(t, rel, 19, 5, 0, DrapeauSurCarte)
	attend(t, rel, 20, 5, 1, DrapeauSurCarte)
	d := rel.Donnees
	if d.Height.Z0 != 0 || d.Height.Pas != PasHauteur || d.Height.Bas != 0 || d.Height.Haut != 1 {
		t.Errorf("hauteurs %+v, attendu z0 0, pas 1/256, bas 0, haut 1", d.Height)
	}
	if rel.Bilan.Zone != 800 || rel.Bilan.Connus != 800 || rel.Bilan.PartMaillage != 0 {
		t.Errorf("bilan %+v", rel.Bilan)
	}
}

func TestReliefRampe(t *testing.T) {
	r := renduDe(100, 10, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) {
		return -2.37 + 0.0123*float64(i), true
	})
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	if rel.Donnees.Height.Z0 != -3 {
		t.Errorf("z0 = %g, attendu -3 (partie entiere, vers le bas, de -2,37)", rel.Donnees.Height.Z0)
	}
	for i := 0; i < 100; i++ {
		attend(t, rel, i, 4, -2.37+0.0123*float64(i), DrapeauSurCarte)
	}
}

// Un toit au-dessus d'un sol : le dessus reste le toit (le sol est plus bas que le rendu), sans
// drapeau de maillage.
func TestReliefToitAuDessusDUnSol(t *testing.T) {
	r := renduDe(30, 30, celluleTest, [2]float64{0, 0}, plat(5))
	m := maillageDe(rect{0, 0, 3, 3, 2})
	rel := calcule(t, r, m, nil, CalageFondDuRendu(r))
	attend(t, rel, 15, 15, 5, DrapeauSurCarte)
	if rel.Bilan.SolMaillage != 0 || rel.Donnees.Floors.PartMaillage != 0 {
		t.Errorf("part du maillage %v, attendu 0", rel.Bilan.PartMaillage)
	}
	if len(rel.Donnees.Degradations) != 0 {
		t.Errorf("degradations %v, attendu aucune", rel.Donnees.Degradations)
	}
}

// D-12 : un sol qui depasse le dessus du rendu de PLUS de ToleranceDessusSol devient le dessus.
func TestReliefSolPlusHautQueLeDessus(t *testing.T) {
	r := renduDe(80, 20, celluleTest, [2]float64{0, 0}, plat(0))
	m := maillageDe(rect{0, 0, 2, 2, 1}, rect{2, 0, 4, 2, 0.2}, rect{4, 0, 6, 2, 0.29}, rect{6, 0, 8, 2, 0.31})
	rel := calcule(t, r, m, nil, CalageFondDuRendu(r))
	both := DrapeauSurCarte | DrapeauSolMaillage
	attend(t, rel, 10, 10, 1, both)
	attend(t, rel, 30, 10, 0, DrapeauSurCarte) // 0,2 : sous la tolerance
	attend(t, rel, 50, 10, 0, DrapeauSurCarte) // 0,29 : sous la tolerance, a une marge que le bruit de l interpolation ne franchit pas
	attend(t, rel, 70, 10, 0.31, both)         // 0,31 : au-dela
	if got := rel.Bilan.PartMaillage; math.Abs(got-0.5) > 1e-9 {
		t.Errorf("part du maillage = %g, attendu 0,5 (deux quarts sur quatre)", got)
	}
}

// D-6 : la ou le rendu n'a aucune matiere, le sol du maillage donne la hauteur.
func TestReliefSolSansMatiere(t *testing.T) {
	r := renduDe(60, 20, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) { return 1, i < 30 })
	m := maillageDe(rect{0, 0, 6, 2, 1.2})
	rel := calcule(t, r, m, nil, CalageFondDuRendu(r))
	attend(t, rel, 10, 10, 1, DrapeauSurCarte)
	attend(t, rel, 45, 10, 1.2, DrapeauSurCarte|DrapeauSolMaillage)
	if got := rel.Bilan.PartMaillage; math.Abs(got-0.5) > 1e-9 {
		t.Errorf("part du maillage = %g, attendu 0,5", got)
	}
	if rel.Donnees.Floors.PartMaillage != rel.Bilan.PartMaillage {
		t.Errorf("meshShare %g different du bilan %g", rel.Donnees.Floors.PartMaillage, rel.Bilan.PartMaillage)
	}
}

// Un creux enclos par le maillage est dans la zone (un rocher au milieu de l'arene) ; un creux ouvert
// sur l'exterieur n'y est pas ; la matiere a plus de MargeNavmesh du maillage non plus.
func TestReliefTrouEnclos(t *testing.T) {
	r := renduDe(100, 100, celluleTest, [2]float64{0, 0}, plat(0))
	cal := CalageFondDuRendu(r)
	// Anneau de sol de 1 u autour d'un creux de 2 u de cote ; origine du cadre en haut : y monde 10 - py/10.
	anneau := maillageDe(rect{3, 3, 7, 4, 0}, rect{3, 6, 7, 7, 0}, rect{3, 4, 4, 6, 0}, rect{6, 4, 7, 6, 0})
	rel := calcule(t, r, anneau, nil, cal)
	if _, d, _ := rel.lit(50, 50); d&DrapeauSurCarte == 0 {
		t.Errorf("creux enclos hors de la zone (drapeaux %d)", d)
	}
	if _, d, ok := rel.lit(0, 0); !ok || d&DrapeauSurCarte != 0 {
		t.Errorf("matiere loin du maillage : connue %v, drapeaux %d, attendu connue et hors zone", ok, d)
	}
	// Un U ouvert au sud par un passage de 3 u : la cavite communique avec l'exterieur, le passage
	// restant ouvert apres la marge (20 pixels) et plus large que la fermeture (10 pixels).
	ouvert := maillageDe(rect{2, 3, 3.5, 6, 0}, rect{6.5, 3, 8, 6, 0}, rect{2, 6, 8, 7, 0})
	rel = calcule(t, r, ouvert, nil, cal)
	if _, d, ok := rel.lit(50, 55); !ok || d&DrapeauSurCarte != 0 { // monde (5,05 ; 4,45)
		t.Errorf("cavite ouverte : connue %v, drapeaux %d, attendu connue et hors zone", ok, d)
	}
	// Marge : un pixel a 0,3 u du bord du sol est dans la zone, un pixel a 0,8 u n'y est pas.
	plaque := maillageDe(rect{4, 4, 6, 6, 0})
	rel = calcule(t, r, plaque, nil, cal)
	if _, d, _ := rel.lit(37, 50); d&DrapeauSurCarte == 0 { // x monde 3,75 : 0,25 u du bord
		t.Errorf("pixel a 0,25 u du sol hors de la zone (marge de %g u)", MargeNavmesh)
	}
	if _, d, _ := rel.lit(32, 50); d&DrapeauSurCarte != 0 { // x monde 3,25 : 0,75 u du bord
		t.Errorf("pixel a 0,75 u du sol dans la zone (marge de %g u)", MargeNavmesh)
	}
}

// Sans maillage (D-7), ou avec un maillage dont aucune face n'est gardee : la zone est l'emprise de
// la matiere, la degradation est nommee, aucun etage.
func TestReliefSansMaillage(t *testing.T) {
	r := renduDe(40, 40, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) { return 2, i >= 10 })
	cal := CalageFondDuRendu(r)
	cas := map[string]struct {
		m     *hinavmesh.Maillage
		garde []bool
	}{
		"nil":          {nil, nil},
		"sans face":    {maillageDe(rect{0, 0, 4, 4, 9}), []bool{false}},
		"hors cadre":   {maillageDe(rect{50, 50, 60, 60, 9}), nil},
		"garde courte": {maillageDe(rect{0, 0, 4, 4, 9}), []bool{}},
	}
	for nom, c := range cas {
		rel := calcule(t, r, c.m, c.garde, cal)
		if got := rel.Donnees.Degradations; len(got) != 1 || got[0] != DegradationSansMaillage {
			t.Errorf("%s : degradations %v, attendu [%s]", nom, got, DegradationSansMaillage)
		}
		if !rel.Bilan.SansMaillage || rel.Bilan.Zone != 30*40 || rel.Bilan.SolMaillage != 0 {
			t.Errorf("%s : bilan %+v, attendu zone = matiere (1200 pixels), aucun sol du maillage", nom, rel.Bilan)
		}
		attend(t, rel, 20, 20, 2, DrapeauSurCarte)
		if _, _, ok := rel.lit(5, 20); ok {
			t.Errorf("%s : hauteur connue la ou le rendu est vide", nom)
		}
		if rel.Donnees.Floors.Min != nil || rel.Donnees.Floors.Max != nil || len(rel.Donnees.Floors.Niveaux) != 0 {
			t.Errorf("%s : etages presents sans maillage : %+v", nom, rel.Donnees.Floors)
		}
	}
}

// Une miette de zone s'eteint (elle garde sa hauteur) ; une composante assez grande reste.
func TestReliefMiettes(t *testing.T) {
	r := renduDe(60, 20, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) {
		petit := i >= 50 && i < 52 && j >= 5 && j < 7 // 4 pixels : sous 5
		moyen := i >= 54 && i < 57 && j >= 5 && j < 7 // 6 pixels : assez
		grand := i < 40
		return 1, petit || moyen || grand
	})
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	if rel.Bilan.Miettes != 4 {
		t.Errorf("miettes = %d, attendu 4", rel.Bilan.Miettes)
	}
	// y monde 2,0 - py/10 : la rangee j = 5 est py = 14.
	attend(t, rel, 50, 14, 1, 0)
	attend(t, rel, 55, 14, 1, DrapeauSurCarte)
	attend(t, rel, 10, 10, 1, DrapeauSurCarte)
}

// Cadre plus grand et decale : copie exacte des hauteurs, pixels hors grille sans hauteur.
// Le sol du maillage donne la hauteur aux pixels du cadre publie hors de la grille cuite.
func TestReliefEntreesInvalides(t *testing.T) {
	r := renduDe(4, 4, 1, [2]float64{0, 0}, plat(1))
	cal := CalageFondDuRendu(r)
	cas := map[string]func() error{
		"rendu nil": func() error { _, err := CalculeRelief(context.Background(), nil, nil, nil, cal); return err },
		"cadre vide": func() error {
			c := cal
			c.Largeur = 0
			_, err := CalculeRelief(context.Background(), r, nil, nil, c)
			return err
		},
		"indice de sommet": func() error {
			m := maillageDe(rect{0, 0, 1, 1, 0})
			m.Faces[0].Sommets[2] = 99
			_, err := CalculeRelief(context.Background(), r, m, nil, cal)
			return err
		},
	}
	for nom, f := range cas {
		if f() == nil {
			t.Errorf("%s : aucune erreur", nom)
		}
	}
}

// L'aller-retour du codage sur toute la plage : 65 536 pixels, chacun a son q.
func TestReliefAllerRetourQ(t *testing.T) {
	const n = amplitudeQ + 1
	r := renduDe(n, 1, celluleTest, [2]float64{0, 0}, func(i, _ int) (float64, bool) { return float64(i) * PasHauteur, true })
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	if rel.Donnees.Height.Z0 != 0 || rel.Bilan.Ecretes != 0 {
		t.Fatalf("z0 %g, ecretes %d", rel.Donnees.Height.Z0, rel.Bilan.Ecretes)
	}
	for q := 0; q < n; q++ {
		o := q * 4
		if got := int(rel.Image.Pix[o]) + 256*int(rel.Image.Pix[o+1]); got != q {
			t.Fatalf("pixel %d : q relu %d (R=%d G=%d)", q, got, rel.Image.Pix[o], rel.Image.Pix[o+1])
		}
		if rel.hauteur[q] != float64(q)*PasHauteur {
			t.Fatalf("pixel %d : hauteur %g, attendu %g", q, rel.hauteur[q], float64(q)*PasHauteur)
		}
	}
}

func TestReliefHauteursEcretees(t *testing.T) {
	r := renduDe(10, 1, celluleTest, [2]float64{0, 0}, func(i, _ int) (float64, bool) { return float64(i) * 40, true })
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	if rel.Bilan.Ecretes != 3 { // 280, 320 et 360 depassent 255,996
		t.Errorf("ecretes = %d, attendu 3", rel.Bilan.Ecretes)
	}
	if got := rel.Donnees.Degradations; len(got) != 2 || got[1] != DegradationHauteursEcretees {
		t.Errorf("degradations %v, attendu [sans-maillage hauteurs-ecretees]", got)
	}
	if _, _, ok := rel.lit(9, 0); !ok || rel.hauteur[9] != decode(amplitudeQ, 0) {
		t.Errorf("hauteur ecretee %g, attendu %g", rel.hauteur[9], decode(amplitudeQ, 0))
	}
}

func TestQuantifie(t *testing.T) {
	cas := []struct {
		h, z0  float64
		q      int
		ecrete bool
	}{
		{-3.5, -4, 128, false}, {0, 0, 0, false}, {1, 0, 256, false},
		{0.001, 0, 0, false}, {0.002, 0, 1, false}, {255.99609375, 0, amplitudeQ, false},
		{256, 0, amplitudeQ, true}, {-1, 0, 0, true},
	}
	for _, c := range cas {
		if q, e := quantifie(c.h, c.z0); q != c.q || e != c.ecrete {
			t.Errorf("quantifie(%g, %g) = %d, %v ; attendu %d, %v", c.h, c.z0, q, e, c.q, c.ecrete)
		}
	}
}

// Deux executions sur la meme entree donnent la meme sortie, a l'octet.
func TestReliefDeterministe(t *testing.T) {
	r := renduDe(60, 60, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) { return float64(i%7) * 0.4, (i+j)%9 != 0 })
	m := maillageDe(rect{1, 1, 5, 5, 0.2}, rect{2, 2, 4, 4, 1.5})
	a := calcule(t, r, m, nil, CalageFondDuRendu(r))
	b := calcule(t, r, m, nil, CalageFondDuRendu(r))
	if string(a.Image.Pix) != string(b.Image.Pix) {
		t.Error("images differentes")
	}
	ja, _ := json.Marshal(a.Donnees)
	jb, _ := json.Marshal(b.Donnees)
	if string(ja) != string(jb) {
		t.Error("donnees differentes")
	}
}

// Les cles du JSON sont celles du format publie, les listes sont `[]` et jamais `null`.
func TestReliefFormatJSON(t *testing.T) {
	d := nouvellesDonneesRelief(CalageFond{OrigineX: -10.5, OrigineY: 28.25, MetresParPixel: 0.029, Largeur: 1605, Hauteur: 1771})
	got, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	const attendu = `{"schemaVersion":1,"calibration":{"metersPerPixel":0.029,"originX":-10.5,"originY":28.25,` +
		`"widthPx":1605,"heightPx":1771},"height":{"z0":0,"step":0.00390625,"low":0,"high":0},` +
		`"floors":{"levels":[],"merge":0,"tolerance":0.3,"meshShare":0,"vertices":[],"faces":[]},` +
		`"lines":{"steps":[],"contours":[],"outline":[]},"degradations":[]}`
	if string(got) != attendu {
		t.Errorf("JSON :\n%s\nattendu :\n%s", got, attendu)
	}
	bas, haut := 0.5, 4.25
	d.Floors.Min, d.Floors.Max = &bas, &haut
	got, _ = json.Marshal(d)
	if !strings.Contains(string(got), `"min":0.5,"max":4.25,"merge"`) {
		t.Errorf("min et max absents ou mal places : %s", got)
	}
}

func TestReliefMesureFondPublie(t *testing.T) {
	r := renduDe(20, 10, celluleTest, [2]float64{0, 0}, func(i, j int) (float64, bool) { return 1, i < 10 })
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	matiere := make([]bool, 200)
	for k := range matiere {
		matiere[k] = k%20 < 15 // 150 pixels de matiere, dont 50 sans hauteur (colonnes 10 a 14)
	}
	if err := rel.MesureFondPublie(matiere); err != nil {
		t.Fatal(err)
	}
	if rel.Bilan.FondPublieMatiere != 150 || rel.Bilan.FondPublieSansHauteur != 50 {
		t.Errorf("fond publie : matiere %d, sans hauteur %d ; attendu 150 et 50", rel.Bilan.FondPublieMatiere, rel.Bilan.FondPublieSansHauteur)
	}
	if rel.MesureFondPublie(matiere[:10]) == nil {
		t.Error("un masque de la mauvaise taille est accepte")
	}
}

func TestErodeMasque(t *testing.T) {
	// Un carre plein de 5 x 5 dans une grille de 9 x 9, erode de 1 : reste le carre de 3 x 3.
	m := make([]bool, 81)
	for j := 2; j < 7; j++ {
		for i := 2; i < 7; i++ {
			m[j*9+i] = true
		}
	}
	e := erodeMasque(m, 9, 9, 1)
	n := 0
	for k, v := range e {
		if v {
			n++
			if i, j := k%9, k/9; i < 3 || i > 5 || j < 3 || j > 5 {
				t.Errorf("pixel (%d,%d) garde par l'erosion", i, j)
			}
		}
	}
	if n != 9 {
		t.Errorf("%d pixels apres erosion, attendu 9", n)
	}
}
