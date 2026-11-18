package himap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// arbreDeploy cree sous un dossier temporaire les fichiers donnes (chemins relatifs a la racine).
func arbreDeploy(t *testing.T, fichiers ...string) string {
	t.Helper()
	racine := t.TempDir()
	for _, f := range fichiers {
		p := filepath.Join(racine, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return racine
}

// Un module partage `pc/globals/<nom>` a son homonyme `<variante>/globals/<nom>`, et non
// `<variante>/levels/multi/globals/<nom>` (le chemin ancien, qui n'existe pas).
func TestModuleVarianteDUnModulePartage(t *testing.T) {
	racine := arbreDeploy(t, "pc/globals/common-rtx-new.module", "any/globals/common-rtx-new.module")
	got, err := ModuleVariante(filepath.Join(racine, "pc", "globals", "common-rtx-new.module"), "any")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(racine, "any", "globals", "common-rtx-new.module"); got != want {
		t.Errorf("ModuleVariante = %q, attendu %q", got, want)
	}
	// Absent de la variante : erreur qui nomme le dossier, jamais un chemin suppose.
	_, err = ModuleVariante(filepath.Join(racine, "pc", "globals", "common-rtx-new.module"), "ds")
	if err == nil || !strings.Contains(err.Error(), "module ds/globals introuvable") {
		t.Errorf("erreur = %v, attendu « module ds/globals introuvable »", err)
	}
}

func TestCheminVarianteGlobale(t *testing.T) {
	racine := filepath.Join("jeu", "deploy")
	cas := []struct {
		nom, module, attendu string
		ok                   bool
	}{
		{"module partage", filepath.Join(racine, "pc", "globals", "common-rtx-new.module"),
			filepath.Join(racine, "any", "globals", "common-rtx-new.module"), true},
		{"depuis la variante any", filepath.Join(racine, "any", "globals", "globals-rtx-new.module"),
			filepath.Join(racine, "any", "globals", "globals-rtx-new.module"), true},
		{"module de carte", filepath.Join(racine, "pc", "levels", "multi", "ctf_bazaar", "ctf_bazaar-rtx-new.module"), "", false},
		{"dossier globals d'une carte", filepath.Join(racine, "pc", "levels", "multi", "globals", "x-rtx-new.module"), "", false},
		{"sous-dossier de globals", filepath.Join(racine, "pc", "globals", "forge", "forge_objects-rtx-new.module"), "", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			got, ok := cheminVarianteGlobale(c.module, "any")
			if ok != c.ok || got != c.attendu {
				t.Errorf("cheminVarianteGlobale = (%q, %v), attendu (%q, %v)", got, ok, c.attendu, c.ok)
			}
		})
	}
}

// Une carte garde son emplacement : `<variante>/levels/multi/<dossier>/<nom>`.
func TestCheminVarianteCarteInchange(t *testing.T) {
	dir := filepath.Join("jeu", "deploy", "any", "levels", "multi")
	got := cheminVarianteCarte(dir, filepath.Join("jeu", "deploy", "pc", "levels", "multi", "ctf_bazaar", "ctf_bazaar-rtx-new.module"))
	if want := filepath.Join(dir, "ctf_bazaar", "ctf_bazaar-rtx-new.module"); got != want {
		t.Errorf("cheminVarianteCarte = %q, attendu %q", got, want)
	}
}

// Le sddt d'un module partage n'est ni lu ni applique : la cuisson inscrit la degradation
// mesuree, pour la frontiere comme pour l'eau.
func TestSddtDUnModulePartageNEstPasApplique(t *testing.T) {
	racine := arbreDeploy(t, "pc/globals/common-rtx-new.module", "any/globals/common-rtx-new.module")
	module := filepath.Join(racine, "pc", "globals", "common-rtx-new.module")
	ctx := context.Background()
	var b BilanCuisson
	if _, ok := frontiereDuModule(ctx, &b, module); ok {
		t.Error("frontiereDuModule a rendu une frontiere pour un module partage")
	}
	PoseEauDepuisModule(ctx, nil, &b, module)
	want := []string{raisonFrontierePartagee, raisonEauPartagee}
	if len(b.Degradations) != 2 || b.Degradations[0] != want[0] || b.Degradations[1] != want[1] {
		t.Errorf("degradations = %q, attendu %q", b.Degradations, want)
	}
}

// Un module de carte n'est pas un module partage : la voie ordinaire est suivie (ici, le module
// `any` homonyme est introuvable et la degradation le dit).
func TestSddtDUneCarteSuitLaVoieOrdinaire(t *testing.T) {
	racine := arbreDeploy(t, "pc/levels/multi/ctf_x/ctf_x-rtx-new.module")
	var b BilanCuisson
	_, ok := frontiereDuModule(context.Background(), &b, filepath.Join(racine, "pc", "levels", "multi", "ctf_x", "ctf_x-rtx-new.module"))
	if ok || len(b.Degradations) != 1 || b.Degradations[0] == raisonFrontierePartagee {
		t.Errorf("ok=%v degradations=%q, attendu une degradation ordinaire", ok, b.Degradations)
	}
}
