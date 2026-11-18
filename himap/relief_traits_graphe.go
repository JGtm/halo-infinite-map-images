package himap

// relief_traits_graphe.go — LE CHAINAGE DES TRAITS : UN GRAPHE DE SEGMENTS, PARCOURU PAR IDENTIFIANT.
//
// Une famille de traits arrive en segments entre noeuds a identifiant entier (int64), partages par
// construction entre cellules voisines. Le graphe range les noeuds par identifiant croissant, et chaque
// noeud ses segments par indice croissant : le parcours ne depend jamais de l'ordre d'une map, la sortie
// est la meme a l'octet d'une execution a l'autre (conception §3.4) :
//
//  1. de chaque noeud de degre different de 2 (bout, jonction), pour chaque segment non visite : suivre
//     les noeuds de degre 2 jusqu'a un noeud de degre different de 2 ;
//  2. les segments restants forment des boucles : partir du plus petit noeud non visite, tourner
//     jusqu'au retour.

import (
	"cmp"
	"math"
	"slices"
)

// segment : un segment de trait entre deux noeuds.
type segment struct{ a, b int64 }

// trace : les segments d'une famille et la position, en pixels du cadre, de chacun de ses noeuds.
type trace struct {
	segments []segment
	position func(noeud int64) (x, y float64)
}

// graphe : les noeuds d'une trace en indices compacts, croissants avec leur identifiant.
type graphe struct {
	ids   []int64    // identifiant de chaque noeud, croissant
	x, y  []float64  // position de chaque noeud, en pixels
	debut []int32    // segments du noeud k : incid[debut[k]:debut[k+1]]
	incid []int32    // indices de segments, croissants par noeud
	bouts [][2]int32 // noeuds compacts de chaque segment
}

// chaine : une suite de noeuds compacts. Fermee : le dernier noeud rejoint le premier, qui n'est pas
// repete.
type chaine struct {
	noeuds []int32
	fermee bool
}

// bout : une extremite de segment, pour le tri qui construit le graphe.
type bout struct {
	noeud int64
	seg   int32
	cote  int32
}

// construitGraphe range les extremites des segments par (noeud, segment) et en tire le graphe.
func construitGraphe(t trace) *graphe {
	bs := make([]bout, 0, 2*len(t.segments))
	for s, sg := range t.segments {
		bs = append(bs, bout{sg.a, int32(s), 0}, bout{sg.b, int32(s), 1})
	}
	slices.SortFunc(bs, func(u, v bout) int {
		if c := cmp.Compare(u.noeud, v.noeud); c != 0 {
			return c
		}
		if c := cmp.Compare(u.seg, v.seg); c != 0 {
			return c
		}
		return cmp.Compare(u.cote, v.cote)
	})
	g := &graphe{bouts: make([][2]int32, len(t.segments)), incid: make([]int32, len(bs))}
	for k, b := range bs {
		if k == 0 || b.noeud != bs[k-1].noeud {
			g.debut = append(g.debut, int32(k))
			g.ids = append(g.ids, b.noeud)
			x, y := t.position(b.noeud)
			g.x, g.y = append(g.x, x), append(g.y, y)
		}
		g.incid[k] = b.seg
		g.bouts[b.seg][b.cote] = int32(len(g.ids) - 1)
	}
	g.debut = append(g.debut, int32(len(bs)))
	return g
}

// degre rend le nombre de segments du noeud k.
func (g *graphe) degre(k int32) int32 { return g.debut[k+1] - g.debut[k] }

// indice rend le noeud compact d'un identifiant, et s'il existe.
func (g *graphe) indice(id int64) (int32, bool) {
	k, ok := slices.BinarySearch(g.ids, id)
	return int32(k), ok
}

// chaines rend les chaines du graphe : d'abord celles qui partent des noeuds de degre different de 2,
// puis les boucles, chaque fois par noeud croissant.
func (g *graphe) chaines() []chaine {
	vu := make([]bool, len(g.bouts))
	var out []chaine
	for pass := 0; pass < 2; pass++ {
		for k := int32(0); k < int32(len(g.ids)); k++ {
			if pass == 0 && g.degre(k) == 2 {
				continue
			}
			for _, s := range g.incid[g.debut[k]:g.debut[k+1]] {
				if !vu[s] {
					out = append(out, g.suit(k, s, vu))
				}
			}
		}
	}
	return out
}

// suit parcourt la chaine qui part du noeud `depart` par le segment `s`, jusqu'a un noeud de degre
// different de 2 ou au retour sur `depart`, et marque ses segments vus.
func (g *graphe) suit(depart, s int32, vu []bool) chaine {
	noeuds := []int32{depart}
	cur := depart
	for {
		vu[s] = true
		nxt := g.bouts[s][0]
		if nxt == cur {
			nxt = g.bouts[s][1]
		}
		if nxt == depart && g.degre(depart) == 2 {
			return chaine{noeuds: noeuds, fermee: true}
		}
		noeuds = append(noeuds, nxt)
		if g.degre(nxt) != 2 {
			return chaine{noeuds: noeuds}
		}
		suivant := g.incid[g.debut[nxt]]
		if suivant == s {
			suivant = g.incid[g.debut[nxt]+1]
		}
		if vu[suivant] {
			return chaine{noeuds: noeuds}
		}
		cur, s = nxt, suivant
	}
}

// distance rend la distance, en pixels, entre les noeuds a et b.
func (g *graphe) distance(a, b int32) float64 {
	return math.Hypot(g.x[b]-g.x[a], g.y[b]-g.y[a])
}

// longueur rend la longueur d'une chaine, en pixels (le segment de fermeture compris).
func (g *graphe) longueur(c chaine) float64 {
	l := 0.0
	for k := 1; k < len(c.noeuds); k++ {
		l += g.distance(c.noeuds[k-1], c.noeuds[k])
	}
	if c.fermee && len(c.noeuds) > 1 {
		l += g.distance(c.noeuds[len(c.noeuds)-1], c.noeuds[0])
	}
	return l
}

// longueurTotale rend la longueur de tous les segments, en pixels.
func (g *graphe) longueurTotale() float64 {
	l := 0.0
	for _, b := range g.bouts {
		l += g.distance(b[0], b[1])
	}
	return l
}

// aire rend l'aire, en pixels carres et sans signe, qu'entoure une chaine fermee (0 sinon).
func (g *graphe) aire(c chaine) float64 {
	if !c.fermee {
		return 0
	}
	s := 0.0
	n := len(c.noeuds)
	for k := 0; k < n; k++ {
		a, b := c.noeuds[k], c.noeuds[(k+1)%n]
		s += g.x[a]*g.y[b] - g.x[b]*g.y[a]
	}
	return math.Abs(s) / 2
}

// composantes rend, par noeud, l'indice de sa composante connexe, et la longueur de chaque composante en
// u (`mpp` : u par pixel). Union-find sur les segments, racine = plus petit noeud : deterministe.
func (g *graphe) composantes(mpp float64) ([]int32, []float64) {
	parent := make([]int32, len(g.ids))
	for k := range parent {
		parent[k] = int32(k)
	}
	racine := func(k int32) int32 {
		for parent[k] != k {
			parent[k] = parent[parent[k]]
			k = parent[k]
		}
		return k
	}
	for _, b := range g.bouts {
		ra, rb := racine(b[0]), racine(b[1])
		if ra != rb {
			parent[max(ra, rb)] = min(ra, rb)
		}
	}
	composante := make([]int32, len(g.ids))
	for k := range composante {
		composante[k] = racine(int32(k))
	}
	longueurs := make([]float64, len(g.ids))
	for _, b := range g.bouts {
		longueurs[composante[b[0]]] += g.distance(b[0], b[1]) * mpp
	}
	return composante, longueurs
}
