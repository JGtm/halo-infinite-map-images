package himap

// relief_traits_lisere.go — LE LISERE : LE BORD DE LA ZONE GARDEE (conception §3.3).
//
// Marche des carres sur la zone (hors du cadre : dehors), cellules de i = -1 a W-1 et de j = -1 a H-1,
// noeuds au MILIEU des aretes du treillis ou la zone change. Une cellule a 0, 2 ou 4 aretes qui changent ;
// a 4 (deux coins opposes dans la zone), chaque segment entoure son coin : la zone ne se relie pas en
// diagonale. Chaque noeud a donc deux segments, et les chaines sont toutes des anneaux fermes.
//
// Le treillis du lisere est celui d'une grille elargie d'un pixel de chaque cote (largeur W+2) : le
// pixel (i, j) du cadre y est le pixel (i+1, j+1).
//
// Les miettes de zone (composantes de moins de AireMinAnneau) sont deja eteintes par CalculeRelief : la
// zone et le lisere concordent.

// lisere rend la trace du bord de la zone.
func (g *grilleTraits) lisere() trace {
	we := g.w + 2
	dedans := func(i, j int) float64 {
		if i < 0 || j < 0 || i >= g.w || j >= g.h || !g.zone[j*g.w+i] {
			return 0
		}
		return 1
	}
	var segs []segment
	for j := -1; j < g.h; j++ {
		for i := -1; i < g.w; i++ {
			idx := indiceDeCellule(dedans(i, j), dedans(i+1, j), dedans(i+1, j+1), dedans(i, j+1), 0.5)
			paires := casMarcheDesCarres[idx]
			pe := (j+1)*we + i + 1
			for s := 0; s+1 < len(paires); s += 2 {
				segs = append(segs, segment{areteDuTreillis(paires[s], pe, we), areteDuTreillis(paires[s+1], pe, we)})
			}
		}
	}
	return trace{segments: segs, position: g.positionDeLisere}
}

// positionDeLisere rend la position, en pixels du cadre, d'un noeud du lisere : le milieu de son arete.
func (g *grilleTraits) positionDeLisere(id int64) (x, y float64) {
	we := int64(g.w + 2)
	pe := id / 2
	i, j := float64(pe%we-1), float64(pe/we-1)
	if id%2 == 0 {
		return i + 0.5, j
	}
	return i, j + 0.5
}
