//go:build gamefiles

package himap

// maillage_partage_gamefiles_test.go — le maillage de Live Fire, lu dans le module `any` partage
// de l'installation. Mesures du 2026-11-18.

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// moduleCommun rend le module de geometrie partage `pc/globals/common-rtx-new.module`.
func moduleCommun(racine string) string {
	return filepath.Join(racine, "pc", "globals", "common-rtx-new.module")
}

// Live Fire : son module `any` est un talon sans maillage, la geometrie et le maillage vivent
// dans le module partage ; la ressource 20390 (arene de Live Fire) l'emporte sur la 20391
// (arene voisine). Les 100 candidates illisibles sont des ressources dont les octets ne sont pas
// dans le fichier du module (cf. le compte rendu du 2026-11-18) : elles restent comptees.
func TestChargeMaillageNatifGeometriePartageeLiveFire(t *testing.T) {
	racine := racineDuJeu(t)
	m, bilan, err := ChargeMaillageNatifGeometriePartagee(context.Background(), racine, "sgh_interlock",
		moduleCommun(racine), ancresLiveFire())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Live Fire : %d sommets, %d faces, emprise %v -> %v, bilan %+v", len(m.Sommets), len(m.Faces), m.Min, m.Max, bilan)
	if bilan.Ressource != 20390 {
		t.Errorf("ressource = %d, attendu 20390", bilan.Ressource)
	}
	if len(m.Faces) != 786 || len(m.Sommets) != 1566 {
		t.Errorf("%d sommets et %d faces, attendu 1566 et 786", len(m.Sommets), len(m.Faces))
	}
	if bilan.Secondaires != 1 {
		t.Errorf("secondaires = %d, attendu 1 (l'arene voisine, ressource 20391)", bilan.Secondaires)
	}
	if bilan.Illisibles != 100 {
		t.Errorf("illisibles = %d, attendu 100", bilan.Illisibles)
	}
	if bilan.Octets != 493356 {
		t.Errorf("octets = %d, attendu 493356", bilan.Octets)
	}
	// Tout le maillage tombe dans l'arene de Live Fire (bornes du bsp retenu).
	arene := areneLiveFire().Bounds
	if part := partDansArene(m, arene); part != 1 {
		t.Errorf("part des sommets dans l'arene = %.3f, attendu 1", part)
	}
}

// Sans ancre, la part de sommets dans l'arene suffit a designer la meme ressource.
func TestChargeMaillageNatifGeometriePartageeLiveFireSansAncre(t *testing.T) {
	racine := racineDuJeu(t)
	// Sans ancre, ChoisitBSP retient le bsp le plus peuple, qui est l'arene de Live Fire.
	_, bilan, err := ChargeMaillageNatifGeometriePartagee(context.Background(), racine, "sgh_interlock",
		moduleCommun(racine), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bilan.Ressource != 20390 {
		t.Errorf("ressource = %d, attendu 20390", bilan.Ressource)
	}
}

// Une carte dont le module `any` porte un maillage ne change pas de comportement : meme
// ressource, meme bilan que ChargeMaillageNatif, quel que soit le module de geometrie passe.
func TestChargeMaillageNatifGeometriePartageeCarteAvecMaillage(t *testing.T) {
	racine := racineDuJeu(t)
	ctx := context.Background()
	attendu, bilanAttendu, err := ChargeMaillageNatif(ctx, racine, "ctf_bazaar")
	if err != nil {
		t.Fatal(err)
	}
	for _, geometrie := range []string{"", moduleCommun(racine)} {
		m, bilan, err := ChargeMaillageNatifGeometriePartagee(ctx, racine, "ctf_bazaar", geometrie, ancresBazaar)
		if err != nil {
			t.Fatalf("geometrie %q : %v", geometrie, err)
		}
		if bilan != bilanAttendu || len(m.Sommets) != len(attendu.Sommets) || len(m.Faces) != len(attendu.Faces) {
			t.Errorf("geometrie %q : bilan %+v (%d faces), attendu %+v (%d faces)",
				geometrie, bilan, len(m.Faces), bilanAttendu, len(attendu.Faces))
		}
	}
}

// Une carte sans maillage dont la geometrie n'est pas dans un module partage reste sans maillage.
func TestChargeMaillageNatifGeometriePartageeSansModulePartage(t *testing.T) {
	racine := racineDuJeu(t)
	for _, geometrie := range []string{"", filepath.Join(racine, "pc", "levels", "multi", "sgh_interlock", "sgh_interlock-rtx-new.module")} {
		m, _, err := ChargeMaillageNatifGeometriePartagee(context.Background(), racine, "sgh_interlock", geometrie, ancresLiveFire())
		if !errors.Is(err, ErrSansMaillage) || m != nil {
			t.Errorf("geometrie %q : (%v, %v), attendu ErrSansMaillage", geometrie, m, err)
		}
	}
}
