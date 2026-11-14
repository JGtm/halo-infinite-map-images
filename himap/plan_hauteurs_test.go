package himap

import (
	"image/color"
	"math"
	"testing"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// luminanceRelative rend le Y relatif (WCAG) d'une couleur sRGB.
func luminanceRelative(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// La rampe est la rampe verifiee pour les daltoniens : six pas dont la clarte croit strictement.
func TestRampePlanHauteursClarteStrictementCroissante(t *testing.T) {
	attendue := [...]color.RGBA{
		{0x60, 0x70, 0x8C, 255}, {0x7E, 0x8C, 0x96, 255}, {0x9E, 0xA6, 0xA0, 255},
		{0xBE, 0xBE, 0xA0, 255}, {0xDC, 0xD6, 0xA4, 255}, {0xF6, 0xEC, 0xB0, 255},
	}
	if rampePlanHauteurs != attendue {
		t.Fatalf("la rampe a change : %v, attendu %v (a ne modifier qu'apres une nouvelle mesure)", rampePlanHauteurs, attendue)
	}
	for i := 1; i < len(rampePlanHauteurs); i++ {
		avant, apres := luminanceRelative(rampePlanHauteurs[i-1]), luminanceRelative(rampePlanHauteurs[i])
		if apres <= avant {
			t.Errorf("pas %d : clarte %.4f, pas %d : %.4f ; la rampe doit s'eclaircir", i-1, avant, i, apres)
		}
	}
}

func TestTeinteDeRampeBornesEtMilieu(t *testing.T) {
	if got := teinteDeRampe(0); got != rampePlanHauteurs[0] {
		t.Errorf("t = 0 : %v, attendu le bas de la rampe", got)
	}
	if got := teinteDeRampe(1); got != rampePlanHauteurs[5] {
		t.Errorf("t = 1 : %v, attendu le haut de la rampe", got)
	}
	if teinteDeRampe(-3) != teinteDeRampe(0) || teinteDeRampe(7) != teinteDeRampe(1) {
		t.Error("t hors de [0, 1] n'est pas borne")
	}
	// Le milieu est a mi-chemin du troisieme et du quatrieme pas.
	if got, attendu := teinteDeRampe(0.5), (color.RGBA{174, 178, 160, 255}); got != attendu {
		t.Errorf("t = 0,5 : %v, attendu %v", got, attendu)
	}
}

// Une carte a altitude unique, ou dont les centiles sont a moins d'un metre, est toute au milieu.
func TestPlanPNGCarteAAltitudeUniqueEstAuMilieu(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 1, 1, 4, 4, 7)
	ajouteRect(m, 6, 6, 9, 9, 7.5)
	img, err := PlanPNG(m, nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]int{{2, 7}, {7, 2}} {
		if got := img.RGBAAt(p[0], p[1]); got != teinteMilieu {
			t.Errorf("pixel %v = %v, attendu la teinte du milieu %v", p, got, teinteMilieu)
		}
	}
}

// deuxNiveaux : un grand carre bas (z = 0, de 4 a 36 m) et, par-dessus, un carre haut (z = 4) de
// 2 a 20 m en X et de 2 a 38 m en Y, dans un cadre de 40 x 40 pixels d'un metre. Le carre haut
// couvre donc toute l'arete ouest du carre bas.
func deuxNiveaux() (*hinavmesh.Maillage, CalageFond) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 4, 4, 36, 36, 0)
	ajouteRect(m, 2, 2, 20, 38, 4)
	return m, CalageFond{OrigineX: 0, OrigineY: 40, MetresParPixel: 1, Largeur: 40, Hauteur: 40}
}

func TestPlanPNGDeuxNiveauxLeHautEstPlusClair(t *testing.T) {
	m, cal := deuxNiveaux()
	img, err := PlanPNG(m, nil, cal)
	if err != nil {
		t.Fatal(err)
	}
	bas, haut := img.RGBAAt(30, 20), img.RGBAAt(12, 20)
	if bas != rampePlanHauteurs[0] || haut != rampePlanHauteurs[5] {
		t.Errorf("bas %v et haut %v, attendu les deux bouts de la rampe %v et %v", bas, haut, rampePlanHauteurs[0], rampePlanHauteurs[5])
	}
	if luminanceRelative(haut) <= luminanceRelative(bas) {
		t.Errorf("le haut (%v) n'est pas plus clair que le bas (%v)", haut, bas)
	}
}

// L'ombre du niveau haut tombe vers le sud-est : un pixel du bas dont le voisin a deux pixels au
// nord-ouest est sur le haut est assombri ; plus loin, il ne l'est plus.
func TestPlanPNGOmbrePorteeAuSudEstDuBordHaut(t *testing.T) {
	m, cal := deuxNiveaux()
	img, err := PlanPNG(m, nil, cal)
	if err != nil {
		t.Fatal(err)
	}
	// Le bord est du carre haut est a x = 20 m (colonne 20, trait d'encre ; le haut couvre les
	// colonnes jusqu'a 19) : la colonne 21 du bas a le haut deux pixels au nord-ouest, la 22 non.
	ombre := assombritOmbre(rampePlanHauteurs[0])
	if got := img.RGBAAt(21, 20); got != ombre {
		t.Errorf("colonne 21 = %v, attendu l'ombre %v", got, ombre)
	}
	if got := img.RGBAAt(22, 20); got != rampePlanHauteurs[0] {
		t.Errorf("colonne 22 = %v, attendu le bas sans ombre %v", got, rampePlanHauteurs[0])
	}
	if got := img.RGBAAt(24, 20); got != rampePlanHauteurs[0] {
		t.Errorf("colonne 24 = %v, attendu le bas sans ombre %v", got, rampePlanHauteurs[0])
	}
	if ombre.R >= rampePlanHauteurs[0].R || ombre.G >= rampePlanHauteurs[0].G || ombre.B >= rampePlanHauteurs[0].B {
		t.Errorf("l'ombre %v n'assombrit pas %v", ombre, rampePlanHauteurs[0])
	}
	// Pas d'ombre sur le haut lui-meme, ni a son nord-ouest.
	if got := img.RGBAAt(12, 20); got != rampePlanHauteurs[5] {
		t.Errorf("le haut porte une ombre : %v", got)
	}
}

// L'arete ouest du carre bas (x = 4 m) est sous le carre haut : elle se trace en gris et en
// pointille, six pixels traces puis six sautes le long de l'arete ; l'arete est du haut, elle,
// reste d'encre.
func TestPlanPNGAreteRecouverteEnPointille(t *testing.T) {
	m, cal := deuxNiveaux()
	img, err := PlanPNG(m, nil, cal)
	if err != nil {
		t.Fatal(err)
	}
	// L'arete ouest est parcourue du nord (ligne 4) au sud ; l'indice d'echantillon est la ligne - 4.
	traces, sautes := 0, 0
	for py := 6; py <= 33; py++ {
		got := img.RGBAAt(4, py)
		if got == teinteAreteEncre {
			t.Fatalf("ligne %d : arete recouverte tracee en encre", py)
		}
		if (py-4)/pasPointille%2 == 0 {
			traces++
			if got != teinteAreteCachee {
				t.Errorf("ligne %d : %v, attendu le gris de l'arete cachee %v", py, got, teinteAreteCachee)
			}
		} else {
			sautes++
			if got != rampePlanHauteurs[5] {
				t.Errorf("ligne %d : %v, attendu le haut %v (trait saute)", py, got, rampePlanHauteurs[5])
			}
		}
	}
	if traces == 0 || sautes == 0 {
		t.Errorf("pas d'alternance : %d pixels traces, %d sautes", traces, sautes)
	}
	// L'arete est du carre haut (colonne 20) n'est recouverte par rien : encre.
	if got := img.RGBAAt(20, 20); got != teinteAreteEncre {
		t.Errorf("arete du haut = %v, attendu l'encre %v", got, teinteAreteEncre)
	}
}

// Une arete decouverte du carre bas (son bord sud) reste en encre.
func TestPlanPNGAreteDecouverteEnEncre(t *testing.T) {
	m, cal := deuxNiveaux()
	img, err := PlanPNG(m, nil, cal)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(30, 36); got != teinteAreteEncre { // bord sud du bas (y = 4 m), hors du haut
		t.Errorf("bord decouvert = %v, attendu l'encre %v", got, teinteAreteEncre)
	}
}

// L'epaisseur du trait suit l'echelle : 7 cm, au moins un pixel.
func TestPlanPNGEpaisseurDuTraitSuitLEchelle(t *testing.T) {
	m := &hinavmesh.Maillage{}
	ajouteRect(m, 1, 1, 9, 9, 0)
	cal := CalageFond{OrigineX: 0, OrigineY: 10, MetresParPixel: 0.035, Largeur: 286, Hauteur: 286}
	img, err := PlanPNG(m, nil, cal)
	if err != nil {
		t.Fatal(err)
	}
	// 0,07 / 0,035 = 2 pixels : l'arete ouest (x = 1 m, colonne 28) fait 2 colonnes d'encre.
	n := 0
	for px := 0; px < 60; px++ {
		if img.RGBAAt(px, 143) == teinteAreteEncre {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d colonnes d'encre sur l'arete ouest, attendu 2", n)
	}
}

// Une face a altitude non finie est ecartee : pas de panique, la face normale garde une teinte
// valide, la face ecartee ne laisse ni teinte ni arete.
func TestPlanPNGIgnoreLesFacesAAltitudeNonFinie(t *testing.T) {
	for _, mauvaise := range []float64{math.NaN(), math.Inf(1)} {
		m := &hinavmesh.Maillage{}
		ajouteRect(m, 1, 1, 4, 4, 5)
		ajouteRect(m, 6, 6, 9, 9, mauvaise)
		img, err := PlanPNG(m, nil, calageDix)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.RGBAAt(2, 7); got != teinteMilieu {
			t.Errorf("z = %v : face normale = %v, attendu la teinte du milieu %v", mauvaise, got, teinteMilieu)
		}
		for _, p := range [][2]int{{7, 2}, {6, 3}, {8, 3}} {
			if got := img.RGBAAt(p[0], p[1]); got != teintePlanPapier {
				t.Errorf("z = %v : pixel %v de la face ecartee = %v, attendu le papier", mauvaise, p, got)
			}
		}
	}
	if got := teinteDeRampe(math.NaN()); got != teinteDeRampe(0.5) {
		t.Errorf("t NaN = %v, attendu la teinte du milieu", got)
	}
}

// Une face seule en pente (de 0 a 6 m) ne recouvre pas son propre bord bas : tous ses bords sont
// en encre pleine, sans pointille ni gris.
func TestPlanPNGFaceEnPenteNeRecouvrePasSesBords(t *testing.T) {
	m := &hinavmesh.Maillage{
		Sommets: []hinavmesh.Point{{X: 2, Y: 2, Z: 0}, {X: 8, Y: 2, Z: 0}, {X: 8, Y: 8, Z: 6}, {X: 2, Y: 8, Z: 6}},
		Faces:   []hinavmesh.Face{{Sommets: []int32{0, 1, 2, 3}}},
	}
	img, err := PlanPNG(m, nil, calageDix)
	if err != nil {
		t.Fatal(err)
	}
	for py := 0; py < 10; py++ {
		for px := 0; px < 10; px++ {
			if img.RGBAAt(px, py) == teinteAreteCachee {
				t.Fatalf("pixel (%d, %d) en gris d'arete cachee : la face se recouvre elle-meme", px, py)
			}
		}
	}
	// Le bord bas (y = 2 m, ligne 8) et le bord haut (ligne 2) sont d'encre sur toute leur longueur.
	for px := 2; px <= 8; px++ {
		for _, py := range []int{2, 8} {
			if got := img.RGBAAt(px, py); got != teinteAreteEncre {
				t.Errorf("pixel (%d, %d) = %v, attendu l'encre %v", px, py, got, teinteAreteEncre)
			}
		}
	}
}
