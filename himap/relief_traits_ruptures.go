package himap

// relief_traits_ruptures.go — LES RUPTURES : LES PAS DU TREILLIS (conception §3.1).
//
// Un PAS est l'arete du treillis entre deux pixels voisins dont les hauteurs sont connues, dont l'un au
// moins est dans la zone, et dont l'ecart depasse SeuilRupture. Les noeuds sont poses sur le treillis,
// ou deux cellules voisines les partagent par construction :
//
//	milieu du pas horizontal de p = (i, j), entre (i, j) et (i+1, j) : identifiant 2p,      position (i + 0,5 ; j)
//	milieu du pas vertical de p,            entre (i, j) et (i, j+1) : identifiant 2p + 1,  position (i ; j + 0,5)
//	centre de la cellule de p :                                        identifiant 2WH + p, position (i + 0,5 ; j + 0,5)
//
// Par cellule (coins (i, j) a (i+1, j+1)), selon le nombre n de ses quatre aretes qui sont des pas : 0
// ou 1, rien ; 2, un segment entre leurs milieux ; 3 ou 4, un segment de chaque milieu au centre (une
// JONCTION). Une marche fermee donne ainsi une boucle fermee, sans extremite pendante.

// pasDeRupture rend, par pixel p, si l'arete vers son voisin de droite (pasH) et vers celui du dessous
// (pasV) est un pas de rupture.
func (g *grilleTraits) pasDeRupture() (pasH, pasV []bool) {
	n := g.w * g.h
	pasH, pasV = make([]bool, n), make([]bool, n)
	for j := 0; j < g.h; j++ {
		for i := 0; i < g.w; i++ {
			p := j*g.w + i
			if i+1 < g.w {
				pasH[p] = g.estUnPas(p, p+1)
			}
			if j+1 < g.h {
				pasV[p] = g.estUnPas(p, p+g.w)
			}
		}
	}
	return pasH, pasV
}

// estUnPas dit si l'arete entre les pixels p et q est un pas de rupture.
func (g *grilleTraits) estUnPas(p, q int) bool {
	return g.connu[p] && g.connu[q] && (g.zone[p] || g.zone[q]) && g.ecartDeRupture(p, q)
}

// ecartDeRupture dit si l'ecart de hauteur entre p et q depasse SeuilRupture (sans regarder la zone).
func (g *grilleTraits) ecartDeRupture(p, q int) bool {
	d := g.haut[p] - g.haut[q]
	return d > SeuilRupture || -d > SeuilRupture
}

// ruptures rend la trace des ruptures et les identifiants des noeuds de jonction (centres de cellule a
// trois pas ou plus), croissants.
func (g *grilleTraits) ruptures() (trace, []int64) {
	pasH, pasV := g.pasDeRupture()
	w := g.w
	centre := int64(2 * w * g.h)
	var segs []segment
	var jonctions []int64
	for j := 0; j+1 < g.h; j++ {
		for i := 0; i+1 < w; i++ {
			p := j*w + i
			milieux, n := [4]int64{}, 0
			for _, m := range [4]struct {
				pas bool
				id  int64
			}{{pasH[p], int64(2 * p)}, {pasV[p+1], int64(2*(p+1) + 1)}, {pasH[p+w], int64(2 * (p + w))}, {pasV[p], int64(2*p + 1)}} {
				if m.pas {
					milieux[n] = m.id
					n++
				}
			}
			switch {
			case n == 2:
				segs = append(segs, segment{milieux[0], milieux[1]})
			case n >= 3:
				c := centre + int64(p)
				jonctions = append(jonctions, c)
				for _, m := range milieux[:n] {
					segs = append(segs, segment{m, c})
				}
			}
		}
	}
	return trace{segments: segs, position: g.positionDeRupture}, jonctions
}

// positionDeRupture rend la position, en pixels, d'un noeud de rupture.
func (g *grilleTraits) positionDeRupture(id int64) (x, y float64) {
	w := int64(g.w)
	if c := int64(2 * g.w * g.h); id >= c {
		p := id - c
		return float64(p%w) + 0.5, float64(p/w) + 0.5
	}
	p := id / 2
	i, j := float64(p%w), float64(p/w)
	if id%2 == 0 {
		return i + 0.5, j
	}
	return i, j + 0.5
}
