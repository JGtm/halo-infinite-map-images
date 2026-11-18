package himap

// relief_sols_test.go — les sols du maillage et les etages, sur des maillages synthetiques. Le cadre
// est celui de renduDe (100 x 100 cellules de 0,1 u) : le monde va de (0, 0) a (10, 10).

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// solsDe calcule le relief puis les sols d'un maillage dans le cadre de test.
func solsDe(t *testing.T, m *hinavmesh.Maillage, garde []bool) *Relief {
	t.Helper()
	r := renduDe(100, 100, celluleTest, [2]float64{0, 0}, plat(0))
	rel := calcule(t, r, m, garde, CalageFondDuRendu(r))
	if err := rel.CalculeSols(context.Background(), m, garde); err != nil {
		t.Fatal(err)
	}
	return rel
}

// attendNiveaux verifie les etages, croissants, a un centieme d'u pres.
func attendNiveaux(t *testing.T, rel *Relief, want ...float64) {
	t.Helper()
	got := rel.Donnees.Floors.Niveaux
	if len(got) != len(want) {
		t.Fatalf("etages %v, attendu %v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("etages %v, attendu %v", got, want)
			return
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("etages %v : pas du bas vers le haut", got)
	}
}

// verifieCoherence controle ce que le web supposera : trois indices valides et distincts par face,
// tous les sommets cites, des sommets a la suite (x, y, z), aucun doublon de sommet.
func verifieCoherence(t *testing.T, f EtagesRelief) {
	t.Helper()
	if len(f.Sommets)%3 != 0 {
		t.Fatalf("%d entiers de sommets, pas un multiple de 3", len(f.Sommets))
	}
	n := len(f.Sommets) / 3
	cite := make([]bool, n)
	for _, face := range f.Faces {
		if len(face) != 3 || face[0] == face[1] || face[1] == face[2] || face[0] == face[2] {
			t.Fatalf("face %v : trois indices distincts attendus", face)
		}
		for _, i := range face {
			if i < 0 || i >= n {
				t.Fatalf("face %v : indice hors de [0, %d[", face, n)
			}
			cite[i] = true
		}
	}
	vus := map[[3]int]bool{}
	for i := 0; i < n; i++ {
		v := [3]int{f.Sommets[3*i], f.Sommets[3*i+1], f.Sommets[3*i+2]}
		if vus[v] {
			t.Errorf("sommet %v en double", v)
		}
		vus[v] = true
		if !cite[i] {
			t.Errorf("sommet %d cite par aucune face", i)
		}
	}
}

func TestSolsUnEtage(t *testing.T) {
	rel := solsDe(t, maillageDe(rect{1, 1, 9, 9, 2.004}), nil)
	f := rel.Donnees.Floors
	attendNiveaux(t, rel, 2)
	if f.Min == nil || f.Max == nil || *f.Min != 2 || *f.Max != 2 {
		t.Errorf("min %v max %v, attendu 2 et 2", f.Min, f.Max)
	}
	if f.Fusion != FusionSols || f.Tolerance != ToleranceDessusSol {
		t.Errorf("fusion %g tolerance %g", f.Fusion, f.Tolerance)
	}
	if len(f.Sommets) != 12 || len(f.Faces) != 2 {
		t.Fatalf("%d entiers de sommets, %d faces : attendu 4 sommets et 2 triangles", len(f.Sommets), len(f.Faces))
	}
	if !reflect.DeepEqual(f.Sommets[:3], []int{100, 100, 200}) {
		t.Errorf("premier sommet %v, attendu [100 100 200] (centiemes d'u, monde, z arrondi de 2,004)", f.Sommets[:3])
	}
	verifieCoherence(t, f)
	if len(rel.Donnees.Degradations) != 0 {
		t.Errorf("degradations %v", rel.Donnees.Degradations)
	}
}

func TestSolsDeuxEtagesSuperposes(t *testing.T) {
	rel := solsDe(t, maillageDe(rect{1, 1, 9, 9, 3.5}, rect{1, 1, 9, 9, 0.5}), nil)
	attendNiveaux(t, rel, 0.5, 3.5)
	if f := rel.Donnees.Floors; *f.Min != 0.5 || *f.Max != 3.5 {
		t.Errorf("min %g max %g", *f.Min, *f.Max)
	}
	verifieCoherence(t, rel.Donnees.Floors)
}

// Une rampe entre deux etages ne fait pas un troisieme etage : ses hauteurs sont etalees.
func TestSolsRampeEntreDeuxEtages(t *testing.T) {
	m := &hinavmesh.Maillage{}
	quad := func(pts ...hinavmesh.Point) {
		n := int32(len(m.Sommets))
		m.Sommets = append(m.Sommets, pts...)
		m.Faces = append(m.Faces, hinavmesh.Face{Sommets: []int32{n, n + 1, n + 2, n + 3}})
	}
	quad(hinavmesh.Point{X: 0, Y: 1, Z: 0}, hinavmesh.Point{X: 4, Y: 1, Z: 0}, hinavmesh.Point{X: 4, Y: 9, Z: 0}, hinavmesh.Point{X: 0, Y: 9, Z: 0})
	quad(hinavmesh.Point{X: 4, Y: 1, Z: 0}, hinavmesh.Point{X: 6, Y: 1, Z: 4}, hinavmesh.Point{X: 6, Y: 9, Z: 4}, hinavmesh.Point{X: 4, Y: 9, Z: 0})
	quad(hinavmesh.Point{X: 6, Y: 1, Z: 4}, hinavmesh.Point{X: 10, Y: 1, Z: 4}, hinavmesh.Point{X: 10, Y: 9, Z: 4}, hinavmesh.Point{X: 6, Y: 9, Z: 4})
	rel := solsDe(t, m, nil)
	attendNiveaux(t, rel, 0, 4)
	verifieCoherence(t, rel.Donnees.Floors)
}

// Deux sols a moins de FusionSols sur le meme pixel n'en font qu'un, le plus haut l'emporte ; a plus,
// ils restent deux.
func TestSolsFusionnes(t *testing.T) {
	cal := CalageFond{OrigineX: 0, OrigineY: 10, MetresParPixel: celluleTest, Largeur: 100, Hauteur: 100}
	pile := func(zBas, zHaut float64) *pileDeSols {
		s, f := sommetsEtFacesDuCadre(trianglesDesFacesGardees(maillageDe(rect{1, 1, 9, 9, zBas}, rect{1, 1, 9, 9, zHaut}), nil), cal)
		return empileLesSols(s, f, cal)
	}
	k := 50*100 + 50
	if p := pile(1, 1.39); p.nb[k] != 1 || math.Abs(float64(p.couches[k*CouchesPile])-1.39) > 1e-6 {
		t.Errorf("1,00 et 1,39 : %d sol(s) a %v, attendu un seul a 1,39", p.nb[k], p.couches[k*CouchesPile:k*CouchesPile+2])
	}
	if p := pile(1, 1.41); p.nb[k] != 2 {
		t.Errorf("1,00 et 1,41 : %d sol(s), attendu 2", p.nb[k])
	}
	rel := solsDe(t, maillageDe(rect{1, 1, 9, 9, 1}, rect{1, 1, 9, 9, 1.3}), nil)
	attendNiveaux(t, rel, 1.3)
	if f := rel.Donnees.Floors; *f.Min != 1.3 || *f.Max != 1.3 {
		t.Errorf("min %g max %g, attendu le sol fusionne a 1,3", *f.Min, *f.Max)
	}
}

// Deux sols distincts mais a moins de EcartMinEtages ne font qu'un etage ; au-dela, deux.
func TestSolsEcartMinimalEntreEtages(t *testing.T) {
	proche := solsDe(t, maillageDe(rect{1, 1, 9, 9, 0}, rect{1, 1, 9, 9, 1.2}), nil)
	if got := len(proche.Donnees.Floors.Niveaux); got != 1 {
		t.Errorf("sols a 1,2 u : %v, attendu un seul etage", proche.Donnees.Floors.Niveaux)
	}
	loin := solsDe(t, maillageDe(rect{1, 1, 9, 9, 0}, rect{1, 1, 9, 9, 1.6}), nil)
	attendNiveaux(t, loin, 0, 1.6)
}

// Un palier de moins de PartMinEtage des echantillons n'est pas un etage ; il compte pour le curseur
// seulement s'il depasse les centiles.
func TestSolsPartMinimale(t *testing.T) {
	// Sol de 8 x 8 u et plate-forme de 0,6 x 0,6 u (0,56 % des echantillons).
	rel := solsDe(t, maillageDe(rect{1, 1, 9, 9, 0}, rect{4, 4, 4.6, 4.6, 3}), nil)
	attendNiveaux(t, rel, 0)
	if *rel.Donnees.Floors.Max != 0 {
		t.Errorf("max %g : la plate-forme est sous le 99e centile", *rel.Donnees.Floors.Max)
	}
	// Mais elle est bien dans les faces : le web la verra.
	if len(rel.Donnees.Floors.Faces) != 4 {
		t.Errorf("%d faces, attendu 4 triangles", len(rel.Donnees.Floors.Faces))
	}
}

// Plus de CouchesPile sols distincts sur un pixel : la pile est tronquee, nommee, et les etages
// restent ceux des sols gardes.
func TestSolsPileTronquee(t *testing.T) {
	cinq := maillageDe(rect{1, 1, 9, 9, 0}, rect{1, 1, 9, 9, 2}, rect{1, 1, 9, 9, 4}, rect{1, 1, 9, 9, 6}, rect{1, 1, 9, 9, 8})
	rel := solsDe(t, cinq, nil)
	if !slices.Contains(rel.Donnees.Degradations, DegradationPileTronquee) {
		t.Errorf("degradations %v, attendu %q", rel.Donnees.Degradations, DegradationPileTronquee)
	}
	attendNiveaux(t, rel, 0, 2, 4, 6)
	if got := len(rel.Donnees.Floors.Faces); got != 10 {
		t.Errorf("%d faces : la troncature ne retire rien du format, attendu les 10 triangles", got)
	}
	quatre := solsDe(t, maillageDe(rect{1, 1, 9, 9, 0}, rect{1, 1, 9, 9, 2}, rect{1, 1, 9, 9, 4}, rect{1, 1, 9, 9, 6}), nil)
	attendNiveaux(t, quatre, 0, 2, 4, 6)
	if slices.Contains(quatre.Donnees.Degradations, DegradationPileTronquee) {
		t.Errorf("quatre sols : degradations %v, attendu aucune", quatre.Donnees.Degradations)
	}
}

// Au plus MaxEtages etages, du plus lourd au plus leger.
func TestSolsNombreMaximalDEtages(t *testing.T) {
	var rects []rect
	for i := 0; i < 7; i++ {
		// Sept paliers cote a cote a 2 u d'ecart, de largeur croissante : un pic par palier.
		x := 0.5 + 1.3*float64(i)
		rects = append(rects, rect{x, 1, x + 1.2, 1 + 0.5*float64(i+1), 2 * float64(i)})
	}
	rel := solsDe(t, maillageDe(rects...), nil)
	if got := len(rel.Donnees.Floors.Niveaux); got != MaxEtages {
		t.Errorf("etages %v, attendu %d", rel.Donnees.Floors.Niveaux, MaxEtages)
	}
}

// Sans maillage, ou sans face gardee dans le cadre : rien d'invente, la degradation nommee une fois.
func TestSolsSansMaillage(t *testing.T) {
	cas := map[string]struct {
		m     *hinavmesh.Maillage
		garde []bool
	}{
		"nil":        {nil, nil},
		"sans face":  {maillageDe(rect{1, 1, 4, 4, 9}), []bool{false}},
		"hors cadre": {maillageDe(rect{50, 50, 60, 60, 9}), nil},
	}
	for nom, c := range cas {
		rel := solsDe(t, c.m, c.garde)
		f := rel.Donnees.Floors
		if len(f.Niveaux) != 0 || len(f.Sommets) != 0 || len(f.Faces) != 0 || f.Min != nil || f.Max != nil || f.Fusion != 0 {
			t.Errorf("%s : sols presents sans maillage : %+v", nom, f)
		}
		if got := rel.Donnees.Degradations; len(got) != 1 || got[0] != DegradationSansMaillage {
			t.Errorf("%s : degradations %v, attendu [%s]", nom, got, DegradationSansMaillage)
		}
		j, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, cle := range []string{`"levels":[]`, `"vertices":[]`, `"faces":[]`} {
			if !strings.Contains(string(j), cle) {
				t.Errorf("%s : %s absent de %s", nom, cle, j)
			}
		}
		if strings.Contains(string(j), `"min"`) || strings.Contains(string(j), `"max"`) {
			t.Errorf("%s : min ou max presents dans %s", nom, j)
		}
	}
}

// Une face hors du cadre est ignoree ; une face a cheval sur son bord est gardee ; une face ecartee
// par `garde` l'est aussi.
func TestSolsFacesHorsDuCadre(t *testing.T) {
	m := maillageDe(rect{1, 1, 9, 9, 0}, rect{50, 50, 60, 60, 5}, rect{9, 1, 14, 9, 0}, rect{2, 2, 4, 4, 7})
	rel := solsDe(t, m, []bool{true, true, true, false})
	f := rel.Donnees.Floors
	if len(f.Faces) != 4 || len(f.Sommets) != 18 {
		t.Errorf("%d faces, %d sommets : attendu les 2 rectangles du cadre (4 triangles, 6 sommets : ils partagent un bord)", len(f.Faces), len(f.Sommets)/3)
	}
	for i := 2; i < len(f.Sommets); i += 3 {
		if f.Sommets[i] != 0 {
			t.Errorf("sommet a z=%d : la face hors cadre ou ecartee est restee", f.Sommets[i])
		}
	}
	attendNiveaux(t, rel, 0)
	verifieCoherence(t, f)
}

// Les sols sont deterministes, et un second appel refait le meme calcul.
func TestSolsDeterministes(t *testing.T) {
	var rects []rect
	for i := 0; i < 12; i++ {
		x, y := float64(i%4)*2.4+0.2, float64(i/4)*3.2+0.2
		rects = append(rects, rect{x, y, x + 2.1, y + 3, float64(i%3) * 2.2}, rect{x + 0.3, y + 0.3, x + 1.9, y + 2.7, float64(i%3)*2.2 + 2.2})
	}
	m := maillageDe(rects...)
	a, b := solsDe(t, m, nil), solsDe(t, m, nil)
	if !reflect.DeepEqual(a.Donnees, b.Donnees) {
		t.Errorf("deux calculs different :\n%+v\n%+v", a.Donnees.Floors, b.Donnees.Floors)
	}
	avant := a.Donnees
	if err := a.CalculeSols(context.Background(), m, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(avant, a.Donnees) {
		t.Errorf("un second appel change les donnees")
	}
}

// Les sols se calculent apres CalculeRelief, sur un maillage coherent.
func TestSolsRefusEntrees(t *testing.T) {
	if err := (&Relief{}).CalculeSols(context.Background(), nil, nil); err == nil {
		t.Error("CalculeSols sans CalculeRelief : erreur attendue")
	}
	r := renduDe(100, 100, celluleTest, [2]float64{0, 0}, plat(0))
	rel := calcule(t, r, nil, nil, CalageFondDuRendu(r))
	m := maillageDe(rect{1, 1, 4, 4, 0})
	m.Faces[0].Sommets[2] = 99
	if err := rel.CalculeSols(context.Background(), m, nil); err == nil {
		t.Error("maillage incoherent : erreur attendue")
	}
}

// Aller-retour JSON de Floors : les cles du format, min et max en pointeurs, listes vides non nulles.
func TestSolsAllerRetourJSON(t *testing.T) {
	rel := solsDe(t, maillageDe(rect{1, 1, 9, 9, 0}, rect{1, 1, 9, 9, 2.5}), nil)
	j, err := json.Marshal(rel.Donnees)
	if err != nil {
		t.Fatal(err)
	}
	var cles struct {
		Floors map[string]json.RawMessage `json:"floors"`
	}
	if err := json.Unmarshal(j, &cles); err != nil {
		t.Fatal(err)
	}
	for _, cle := range []string{"levels", "min", "max", "merge", "tolerance", "meshShare", "vertices", "faces"} {
		if _, ok := cles.Floors[cle]; !ok {
			t.Errorf("cle %q absente de floors : %v", cle, cles.Floors)
		}
	}
	var back DonneesRelief
	if err := json.Unmarshal(j, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Floors, rel.Donnees.Floors) {
		t.Errorf("aller-retour :\n%+v\n%+v", back.Floors, rel.Donnees.Floors)
	}
}
