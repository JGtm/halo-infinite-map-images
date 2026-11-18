package himap

// relief_sols_pile.go — LA PILE DES SOLS PAR PIXEL.
//
// Chaque face de Floors est rasterisee, a sa hauteur interpolee, aux centres de pixel du cadre publie ;
// un pixel empile les sols distincts qui le couvrent (une passerelle au-dessus d'un sol, un etage au-
// dessus d'un autre). Deux sols a moins de FusionSols l'un de l'autre n'en font qu'un, le plus haut
// l'emporte ; au-dela de CouchesPile sols distincts, les suivants sont ecartes et comptes. La pile ne
// sert qu'a detecter les etages : le web reconstruit la sienne depuis les sommets et les faces.

import (
	"math"

	"github.com/JGtm/halo-infinite-files-reader/hinavmesh"
)

// pileDeSols : par pixel (indice py*Largeur + px), nb sols dans couches[k*CouchesPile:].
type pileDeSols struct {
	couches []float32
	nb      []uint8
	// debordes : echantillons ecartes parce que le pixel avait deja CouchesPile sols distincts.
	debordes int
}

// empileLesSols rasterise les faces indexees de Floors (sommets au centieme d'u) dans le cadre `cal`.
// L'ordre des faces fixe celui de la pose : le resultat est deterministe.
func empileLesSols(sommets []int, faces [][]int, cal CalageFond) *pileDeSols {
	n := cal.Largeur * cal.Hauteur
	p := &pileDeSols{couches: make([]float32, n*CouchesPile), nb: make([]uint8, n)}
	point := func(i int) hinavmesh.Point {
		return hinavmesh.Point{
			X: float64(sommets[3*i]) / echelleSommets, Y: float64(sommets[3*i+1]) / echelleSommets,
			Z: float64(sommets[3*i+2]) / echelleSommets,
		}
	}
	for _, f := range faces {
		p.rasterise([3]hinavmesh.Point{point(f[0]), point(f[1]), point(f[2])}, cal)
	}
	return p
}

// rasterise pose le triangle `t` : un sol a chaque pixel dont le centre y tombe.
func (p *pileDeSols) rasterise(t [3]hinavmesh.Point, cal CalageFond) {
	mpp := cal.MetresParPixel
	a, b, c := t[0], t[1], t[2]
	i0 := max(int(math.Floor((min(a.X, b.X, c.X)-cal.OrigineX)/mpp-0.5)), 0)
	i1 := min(int(math.Ceil((max(a.X, b.X, c.X)-cal.OrigineX)/mpp-0.5)), cal.Largeur-1)
	j0 := max(int(math.Floor((cal.OrigineY-max(a.Y, b.Y, c.Y))/mpp-0.5)), 0)
	j1 := min(int(math.Ceil((cal.OrigineY-min(a.Y, b.Y, c.Y))/mpp-0.5)), cal.Hauteur-1)
	ta, tb, tc := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}, [3]float64{c.X, c.Y, c.Z}
	det := (tb[1]-tc[1])*(ta[0]-tc[0]) + (tc[0]-tb[0])*(ta[1]-tc[1])
	for j := j0; j <= j1; j++ {
		y := cal.OrigineY - (float64(j)+0.5)*mpp
		for i := i0; i <= i1; i++ {
			x := cal.OrigineX + (float64(i)+0.5)*mpp
			if z, dedans := altitudeAuPoint(ta, tb, tc, det, x, y); dedans {
				p.pose(j*cal.Largeur+i, z)
			}
		}
	}
}

// pose empile le sol de hauteur z au pixel k : fondu dans un sol deja la s'il est a moins de
// FusionSols (le plus haut est garde), ajoute sinon, ecarte et compte si la pile est pleine.
func (p *pileDeSols) pose(k int, z float64) {
	base := k * CouchesPile
	for c := 0; c < int(p.nb[k]); c++ {
		if h := float64(p.couches[base+c]); math.Abs(h-z) < FusionSols {
			if z > h {
				p.couches[base+c] = float32(z)
			}
			return
		}
	}
	if int(p.nb[k]) >= CouchesPile {
		p.debordes++
		return
	}
	p.couches[base+int(p.nb[k])] = float32(z)
	p.nb[k]++
}

// echantillons rend une hauteur par pixel et par sol empile (ponderation par l'aire : un pixel, un
// echantillon), dans l'ordre des pixels.
func (p *pileDeSols) echantillons() []float64 {
	total := 0
	for _, n := range p.nb {
		total += int(n)
	}
	out := make([]float64, 0, total)
	for k, n := range p.nb {
		for c := 0; c < int(n); c++ {
			out = append(out, float64(p.couches[k*CouchesPile+c]))
		}
	}
	return out
}
