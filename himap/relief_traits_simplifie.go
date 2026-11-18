package himap

// relief_traits_simplifie.go — SIMPLIFICATION ET ENCODAGE DES TRAITS.
//
// Simplification : Douglas-Peucker iteratif (une pile, aucune recursion), tolerance en pixels,
// extremites gardees. Une chaine fermee est coupee en deux au point le plus loin du premier, garde lui
// aussi : chaque moitie est une polyligne ouverte. Les extremites d'une chaine de rupture sont ses
// jonctions : elles restent exactes, et les chaines qui s'y rejoignent s'y rejoignent encore apres.
//
// Encodage (conception §3.5) : chaque point est arrondi au dixieme de pixel AVANT la difference
// (dx = arrondi(10 x_k) - arrondi(10 x_k-1)), ce qui n'accumule aucune derive ; un point confondu avec
// le precedent apres arrondi est saute. Les noeuds des ruptures et du lisere tombent sur des demi-pixels,
// multiples de cinq dixiemes : ils sont exacts.

import "math"

// point : une position en pixels du cadre.
type point struct{ x, y float64 }

// simplifieChaine rend les points simplifies d'une chaine. Chaine fermee : l'anneau, sans repeter le
// premier point, sauf si `repete`.
func simplifieChaine(g *graphe, c chaine, tol float64, repete bool) []point {
	pts := make([]point, len(c.noeuds))
	for k, n := range c.noeuds {
		pts[k] = point{g.x[n], g.y[n]}
	}
	if !c.fermee || len(pts) < 3 {
		return douglasPeucker(pts, tol)
	}
	loin := pointLePlusLoin(pts)
	premiere := douglasPeucker(pts[:loin+1], tol)
	seconde := douglasPeucker(append(slicesClone(pts[loin:]), pts[0]), tol)
	out := append(premiere, seconde[1:len(seconde)-1]...)
	if repete {
		out = append(out, pts[0])
	}
	return out
}

// slicesClone copie une tranche de points (l'ajout du point de fermeture ne doit pas ecrire dans pts).
func slicesClone(p []point) []point {
	return append(make([]point, 0, len(p)+1), p...)
}

// pointLePlusLoin rend l'indice du point le plus loin du premier (le premier rencontre a egalite).
func pointLePlusLoin(pts []point) int {
	loin, d := 0, -1.0
	for k, p := range pts {
		if e := math.Hypot(p.x-pts[0].x, p.y-pts[0].y); e > d {
			loin, d = k, e
		}
	}
	return loin
}

// douglasPeucker rend les points gardes d'une polyligne ouverte : ses extremites, et tout point dont
// l'ecart au segment qui le remplacerait depasse `tol`.
func douglasPeucker(pts []point, tol float64) []point {
	n := len(pts)
	if n <= 2 {
		return append([]point(nil), pts...)
	}
	garde := make([]bool, n)
	garde[0], garde[n-1] = true, true
	pile := [][2]int{{0, n - 1}}
	for len(pile) > 0 {
		r := pile[len(pile)-1]
		pile = pile[:len(pile)-1]
		m, d := ecartMax(pts, r[0], r[1])
		if d > tol {
			garde[m] = true
			pile = append(pile, [2]int{r[0], m}, [2]int{m, r[1]})
		}
	}
	out := make([]point, 0, n)
	for k, ok := range garde {
		if ok {
			out = append(out, pts[k])
		}
	}
	return out
}

// ecartMax rend le point de ]i, j[ le plus loin du segment pts[i]-pts[j], et son ecart (-1 sans point).
func ecartMax(pts []point, i, j int) (int, float64) {
	m, d := -1, -1.0
	for k := i + 1; k < j; k++ {
		if e := distanceAuSegment(pts[k], pts[i], pts[j]); e > d {
			m, d = k, e
		}
	}
	return m, d
}

// distanceAuSegment rend la distance du point p au segment [a, b] (a lui-meme si le segment est nul).
func distanceAuSegment(p, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(p.x-a.x, p.y-a.y)
	}
	t := math.Max(0, math.Min(1, ((p.x-a.x)*dx+(p.y-a.y)*dy)/l2))
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

// encodePolyligne rend [x0, y0, dx1, dy1, ...] en dixiemes de pixel entiers, les points confondus apres
// arrondi sautes ; nil s'il reste moins de deux points.
func encodePolyligne(pts []point) []int {
	out := make([]int, 0, 2*len(pts))
	var px, py int
	for k, p := range pts {
		x, y := int(math.Round(p.x*dixiemesParPixel)), int(math.Round(p.y*dixiemesParPixel))
		switch {
		case k == 0:
			out = append(out, x, y)
		case x == px && y == py:
			continue
		default:
			out = append(out, x-px, y-py)
		}
		px, py = x, y
	}
	if len(out) < 4 {
		return nil
	}
	return out
}
