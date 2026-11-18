package himap

// relief_traits_courbes.go — LES COURBES DE NIVEAU : MARCHE DES CARRES SUR LES HAUTEURS LISSEES
// (conception §3.2).
//
// Hauteur lissee : moyenne des seuls pixels connus du carre de 2 RayonLissageCourbes + 1 pixels de cote,
// calculee sur les q entiers de l'image (sommes exactes). Une cellule de coins a = (i, j), b = (i+1, j),
// c = (i+1, j+1), d = (i, j+1) porte des courbes si ses quatre coins sont connus, l'un au moins dans la
// zone, si aucun de ses coins n'est a moins de RayonSansCourbePresDUneRupture d'une rupture, et si sa
// pente lissee atteint PenteMinCourbe. Les niveaux sont t = k EquidistanceCourbes, k entier, avec
// min(a, b, c, d) <= t < max(a, b, c, d) (un coin pose sur t compte « dessous »).
//
// Pourquoi l'ecart a la rupture : une courbe de niveau dit une PENTE. Pres d'une marche, le carre de
// lissage melange les hauteurs des deux cotes et fabrique une pente qui n'existe pas : une a quatre
// courbes paralleles au mur, a un ou deux pixels de lui, que la rupture trace deja. Sans cette regle,
// 96,5 % de la longueur des courbes de Recharge etait une telle doublure.
//
// Un noeud de courbe est identifie par (arete du treillis, niveau) : deux cellules voisines le calculent
// a partir des deux memes pixels, dans le meme ordre, donc au meme flottant.

import "math"

// RayonSansCourbePresDUneRupture : distance, en pixels (carre, comme le lissage), en deca de laquelle
// d'un pixel touche par une rupture (extremite d'une arete dont l'ecart depasse SeuilRupture) une
// cellule ne porte aucune courbe. C'est le rayon de lissage : au-dela, le carre de lissage d'un coin ne
// voit plus la marche, et la pente lissee est celle du terrain. Il vaut 0,06 u (Recharge) a 0,10 u
// (Behemoth) : une vraie pente qui longe un mur n'y perd que cette bande.
const RayonSansCourbePresDUneRupture = RayonLissageCourbes

// casMarcheDesCarres : pour l'indice 8 (a > t) + 4 (b > t) + 2 (c > t) + 1 (d > t), les paires d'aretes
// de la cellule reliees par un segment. Aretes : 0 = haut (a-b), 1 = droite (b-c), 2 = bas (d-c),
// 3 = gauche (a-d). Aux deux cas de selle (5 et 10), chaque segment entoure son coin « dessus » : la
// zone du lisere ne se relie jamais en diagonale.
var casMarcheDesCarres = [16][]int8{
	{}, {3, 2}, {2, 1}, {3, 1}, {0, 1}, {0, 1, 3, 2}, {0, 2}, {0, 3},
	{0, 3}, {0, 2}, {0, 3, 2, 1}, {0, 1}, {3, 1}, {2, 1}, {3, 2}, {},
}

// areteDuTreillis rend l'identifiant de treillis de l'arete `arete` (0 a 3) de la cellule dont le coin
// haut-gauche est le pixel p d'une grille de largeur w : 2q pour l'arete horizontale de q vers q+1,
// 2q + 1 pour l'arete verticale de q vers q+w.
func areteDuTreillis(arete int8, p, w int) int64 {
	switch arete {
	case 0:
		return int64(2 * p)
	case 1:
		return int64(2*(p+1) + 1)
	case 2:
		return int64(2 * (p + w))
	}
	return int64(2*p + 1)
}

// indiceDeCellule rend l'indice de la marche des carres pour les coins a, b, c, d et le niveau t.
func indiceDeCellule(a, b, c, d, t float64) int {
	k := 0
	for bit, v := range [4]float64{d, c, b, a} {
		if v > t {
			k |= 1 << bit
		}
	}
	return k
}

// lisseLesHauteurs rend la hauteur lissee de chaque pixel connu (moyenne des pixels connus du carre de
// lissage), NaN ailleurs. Sommes des q entiers : exactes, independantes de l'ordre.
func (g *grilleTraits) lisseLesHauteurs() []float64 {
	n := g.w * g.h
	q := make([]int64, n)
	c := make([]int64, n)
	for k, ok := range g.connu {
		if ok {
			q[k], c[k] = int64(math.Round((g.haut[k]-g.z0)/PasHauteur)), 1
		}
	}
	q, c = sommeGlissante(q, g.w, g.h, 1), sommeGlissante(c, g.w, g.h, 1)
	q, c = sommeGlissante(q, g.w, g.h, g.w), sommeGlissante(c, g.w, g.h, g.w)
	out := make([]float64, n)
	for k, ok := range g.connu {
		out[k] = math.NaN()
		if ok {
			out[k] = g.z0 + PasHauteur*float64(q[k])/float64(c[k])
		}
	}
	return out
}

// sommeGlissante rend, par pixel, la somme de v sur la fenetre de 2 RayonLissageCourbes + 1 pixels le
// long d'un axe : `pas` vaut 1 (lignes) ou w (colonnes). La fenetre s'arrete au bord de la grille.
func sommeGlissante(v []int64, w, h, pas int) []int64 {
	out := make([]int64, len(v))
	long, lignes := w, h
	if pas != 1 {
		long, lignes = h, w
	}
	debutDeLigne := func(l int) int {
		if pas == 1 {
			return l * w
		}
		return l
	}
	r := RayonLissageCourbes
	for l := 0; l < lignes; l++ {
		o := debutDeLigne(l)
		var s int64
		for k := 0; k < min(r, long); k++ {
			s += v[o+k*pas]
		}
		for k := 0; k < long; k++ {
			if e := k + r; e < long {
				s += v[o+e*pas]
			}
			if x := k - r - 1; x >= 0 {
				s -= v[o+x*pas]
			}
			out[o+k*pas] = s
		}
	}
	return out
}

// niveauxDesCourbes : la plage des niveaux k d'une carte, pour l'identifiant (arete, niveau).
type niveauxDesCourbes struct{ kmin, nombre int64 }

// courbes rend la trace des courbes de niveau.
func (g *grilleTraits) courbes() trace {
	hl := g.lisseLesHauteurs()
	niv := niveauxDe(hl)
	proche := g.procheDUneRupture()
	var segs []segment
	w := g.w
	for j := 0; j+1 < g.h; j++ {
		for i := 0; i+1 < w; i++ {
			p := j*w + i
			if !g.porteDesCourbes(p, hl, proche) {
				continue
			}
			segs = g.segmentsDeCourbe(segs, p, hl, niv)
		}
	}
	return trace{segments: segs, position: func(id int64) (float64, float64) { return g.positionDeCourbe(id, hl, niv) }}
}

// niveauxDe rend la plage des niveaux que les hauteurs lissees peuvent croiser, une marge de chaque cote.
func niveauxDe(hl []float64) niveauxDesCourbes {
	bas, haut := math.Inf(1), math.Inf(-1)
	for _, v := range hl {
		if !math.IsNaN(v) {
			bas, haut = math.Min(bas, v), math.Max(haut, v)
		}
	}
	if bas > haut {
		return niveauxDesCourbes{nombre: 1}
	}
	kmin := int64(math.Floor(bas/EquidistanceCourbes)) - 1
	return niveauxDesCourbes{kmin: kmin, nombre: int64(math.Ceil(haut/EquidistanceCourbes)) + 2 - kmin}
}

// porteDesCourbes dit si la cellule de coin haut-gauche p porte des courbes. `proche` : les pixels a
// moins de RayonSansCourbePresDUneRupture d'une rupture (procheDUneRupture).
func (g *grilleTraits) porteDesCourbes(p int, hl []float64, proche []bool) bool {
	w := g.w
	a, b, c, d := p, p+1, p+w+1, p+w
	if !g.cellulePleineDansLaZone(a, b, c, d) {
		return false
	}
	if proche[a] || proche[b] || proche[c] || proche[d] {
		return false
	}
	ha, hb, hc, hd := hl[a], hl[b], hl[c], hl[d]
	pente := math.Hypot((hb-ha+hc-hd)/2, (hd-ha+hc-hb)/2) / g.mpp
	return pente >= PenteMinCourbe
}

// procheDUneRupture rend le masque des pixels a moins de RayonSansCourbePresDUneRupture (carre) d'un
// pixel touche par une rupture : une extremite connue d'une arete dont l'ecart depasse SeuilRupture,
// zone ou non (une marche hors de la zone fausse aussi le lissage). Il contient en particulier les coins
// de toute cellule que traverse une rupture.
func (g *grilleTraits) procheDUneRupture() []bool {
	touche := make([]bool, g.w*g.h)
	marque := func(p, q int) {
		if g.connu[p] && g.connu[q] && g.ecartDeRupture(p, q) {
			touche[p], touche[q] = true, true
		}
	}
	for j := 0; j < g.h; j++ {
		for i := 0; i < g.w; i++ {
			p := j*g.w + i
			if i+1 < g.w {
				marque(p, p+1)
			}
			if j+1 < g.h {
				marque(p, p+g.w)
			}
		}
	}
	return dilate(touche, g.w, g.h, RayonSansCourbePresDUneRupture)
}

// cellulePleineDansLaZone dit si les quatre coins d'une cellule ont une hauteur connue et si l'un au
// moins est dans la zone.
func (g *grilleTraits) cellulePleineDansLaZone(a, b, c, d int) bool {
	if !g.connu[a] || !g.connu[b] || !g.connu[c] || !g.connu[d] {
		return false
	}
	return g.zone[a] || g.zone[b] || g.zone[c] || g.zone[d]
}

// segmentsDeCourbe ajoute a segs les segments de la cellule p, pour chaque niveau qu'elle croise.
func (g *grilleTraits) segmentsDeCourbe(segs []segment, p int, hl []float64, niv niveauxDesCourbes) []segment {
	w := g.w
	a, b, c, d := hl[p], hl[p+1], hl[p+w+1], hl[p+w]
	bas, haut := math.Min(math.Min(a, b), math.Min(c, d)), math.Max(math.Max(a, b), math.Max(c, d))
	for k := int64(math.Ceil(bas / EquidistanceCourbes)); float64(k)*EquidistanceCourbes < haut; k++ {
		t := float64(k) * EquidistanceCourbes
		paires := casMarcheDesCarres[indiceDeCellule(a, b, c, d, t)]
		for s := 0; s+1 < len(paires); s += 2 {
			segs = append(segs, segment{
				areteDuTreillis(paires[s], p, w)*niv.nombre + k - niv.kmin,
				areteDuTreillis(paires[s+1], p, w)*niv.nombre + k - niv.kmin,
			})
		}
	}
	return segs
}

// positionDeCourbe rend la position, en pixels, d'un noeud de courbe : sur l'arete du treillis, a
// r = (t - u) / (v - u) du premier pixel, u et v les hauteurs lissees de ses deux pixels.
func (g *grilleTraits) positionDeCourbe(id int64, hl []float64, niv niveauxDesCourbes) (float64, float64) {
	arete, k := id/niv.nombre, id%niv.nombre+niv.kmin
	t := float64(k) * EquidistanceCourbes
	p := int(arete / 2)
	i, j := float64(p%g.w), float64(p/g.w)
	if arete%2 == 0 {
		u, v := hl[p], hl[p+1]
		return i + (t-u)/(v-u), j
	}
	u, v := hl[p], hl[p+g.w]
	return i, j + (t-u)/(v-u)
}
