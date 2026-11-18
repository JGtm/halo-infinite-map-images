package himap

// relief_traits_simplifie_test.go — simplification et encodage des traits : tolerance de Douglas-Peucker,
// tolerance de ce qui est publie, encodage delta au dixieme de pixel.

import (
	"math"
	"testing"
)

// decodePolyligne rend les points, en pixels, d'une polyligne encodee.
func decodePolyligne(p []int) []point {
	out := make([]point, 0, len(p)/2)
	x, y := 0, 0
	for k := 0; k+1 < len(p); k += 2 {
		x, y = x+p[k], y+p[k+1]
		out = append(out, point{float64(x) / dixiemesParPixel, float64(y) / dixiemesParPixel})
	}
	return out
}

// Douglas-Peucker : chaque point d'origine est a moins de la tolerance de la polyligne simplifiee, et
// les extremites sont gardees.
func TestTraitsToleranceDeSimplification(t *testing.T) {
	var pts []point
	for k := 0; k <= 400; k++ {
		x := float64(k) * 0.5
		pts = append(pts, point{x, 30*math.Sin(x/17) + 0.4*math.Sin(x*3)})
	}
	s := douglasPeucker(pts, ToleranceTraits)
	if s[0] != pts[0] || s[len(s)-1] != pts[len(pts)-1] {
		t.Fatal("extremites perdues")
	}
	if len(s) >= len(pts)/2 {
		t.Errorf("%d points gardes sur %d : la simplification ne simplifie pas", len(s), len(pts))
	}
	for _, p := range pts {
		if d := distanceALaPolyligne(p, s); d > ToleranceTraits+1e-12 {
			t.Errorf("point %v a %.3f px de la polyligne simplifiee, tolerance %.2f", p, d, ToleranceTraits)
		}
	}
}

// Sur une carte synthetique complete, chaque noeud brut des traits est a moins de la tolerance plus
// l'arrondi au dixieme (0,05 px par axe) de la polyligne publiee.
func TestTraitsTolerancePubliee(t *testing.T) {
	rel := reliefDeGrille(160, 160, mppTraits, func(px, py int) (float64, bool) {
		dx, dy := float64(px-80)*mppTraits, float64(py-80)*mppTraits
		z := 0.5 * (dx*dx + dy*dy)
		if px > 100 && py > 30 && py < 70 {
			z += 1
		}
		return z, true
	}, nil)
	g, err := rel.grilleDesTraits()
	if err != nil {
		t.Fatal(err)
	}
	traits(t, rel)
	marge := ToleranceTraits + math.Hypot(0.05, 0.05) + 1e-9
	for nom, c := range map[string]struct {
		t     trace
		polys [][]int
	}{
		"courbes":  {g.courbes(), rel.Donnees.Lines.Courbes},
		"ruptures": {traceDesRuptures(g), rel.Donnees.Lines.Ruptures},
	} {
		var publies [][]point
		for _, p := range c.polys {
			publies = append(publies, decodePolyligne(p))
		}
		gr := construitGraphe(c.t)
		for _, ch := range gr.chaines() {
			if len(ch.noeuds) < 2 || gr.longueur(ch)*mppTraits < LongueurMinCourbe {
				continue
			}
			for _, n := range ch.noeuds {
				p := point{gr.x[n], gr.y[n]}
				if d := distanceAuxPolylignes(p, publies); d > marge {
					t.Errorf("%s : noeud %v a %.3f px du trait publie", nom, p, d)
				}
			}
		}
	}
}

// Encodage : le decodage rend les points d'origine au dixieme de pixel pres, sans derive ; un point
// confondu avec le precedent est saute ; moins de deux points : rien.
func TestTraitsEncodageDelta(t *testing.T) {
	var pts []point
	for k := 0; k < 1000; k++ {
		pts = append(pts, point{12.3456 + float64(k)*0.137, 900.01 - float64(k)*0.0731})
	}
	dec := decodePolyligne(encodePolyligne(pts))
	if len(dec) != len(pts) {
		t.Fatalf("%d points decodes, attendu %d", len(dec), len(pts))
	}
	for k := range pts {
		if math.Abs(dec[k].x-pts[k].x) > 0.05+1e-9 || math.Abs(dec[k].y-pts[k].y) > 0.05+1e-9 {
			t.Fatalf("point %d : %v decode en %v", k, pts[k], dec[k])
		}
	}
	if got := encodePolyligne([]point{{1, 1}, {1.01, 1.02}, {2, 2}}); !egaux(got, []int{10, 10, 10, 10}) {
		t.Errorf("point confondu non saute : %v", got)
	}
	if got := encodePolyligne([]point{{1, 1}, {1.01, 1.02}}); got != nil {
		t.Errorf("polyligne d'un seul point encodee : %v", got)
	}
}

// distanceALaPolyligne rend la distance d'un point a une polyligne.
func distanceALaPolyligne(p point, s []point) float64 {
	d := math.Inf(1)
	for k := 1; k < len(s); k++ {
		d = math.Min(d, distanceAuSegment(p, s[k-1], s[k]))
	}
	if len(s) == 1 {
		d = math.Hypot(p.x-s[0].x, p.y-s[0].y)
	}
	return d
}

// distanceAuxPolylignes rend la distance d'un point a la plus proche de plusieurs polylignes.
func distanceAuxPolylignes(p point, polys [][]point) float64 {
	d := math.Inf(1)
	for _, s := range polys {
		d = math.Min(d, distanceALaPolyligne(p, s))
	}
	return d
}

// traceDesRuptures rend la trace des ruptures seule.
func traceDesRuptures(g *grilleTraits) trace {
	tr, _ := g.ruptures()
	return tr
}
