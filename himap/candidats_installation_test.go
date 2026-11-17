package himap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fauxJeu cree un faux `deploy` (variante pc) sous `racine`/<sous-chemin du jeu> et rend la
// racine deploy. Le sous-chemin vient des constantes de deploy_root.go, pas d'une copie.
func fauxJeu(t *testing.T, racine, sousChemin string) string {
	t.Helper()
	deploy := filepath.Join(racine, filepath.FromSlash(sousChemin))
	if err := os.MkdirAll(dossierCartes(deploy, "pc"), 0o755); err != nil {
		t.Fatal(err)
	}
	return deploy
}

// echappeVdf double les antislashs, comme Steam les ecrit dans libraryfolders.vdf.
func echappeVdf(chemin string) string { return strings.ReplaceAll(chemin, `\`, `\\`) }

// ecritVdf pose un libraryfolders.vdf (une entree `"path"` par bibliotheque) sous le
// dossier de Steam factice.
func ecritVdf(t *testing.T, steam string, biblios ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("\"libraryfolders\"\n{\n")
	for i, p := range biblios {
		b.WriteString("\t\"" + string(rune('0'+i)) + "\"\n\t{\n\t\t\"path\"\t\t\"" + echappeVdf(p) + "\"\n")
		b.WriteString("\t\t\"label\"\t\t\"\"\n\t\t\"apps\"\n\t\t{\n\t\t\t\"1240440\"\t\t\"123\"\n\t\t}\n\t}\n")
	}
	b.WriteString("}\n")
	dir := filepath.Join(steam, "steamapps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "libraryfolders.vdf"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// registreFaux : une table (ruche/valeur) -> chaine ; le reste est absent.
func registreFaux(valeurs map[string]string) func(string, string, string) (string, error) {
	return func(ruche, _, valeur string) (string, error) {
		if v, ok := valeurs[ruche+"/"+valeur]; ok {
			return v, nil
		}
		return "", errRegistreAbsent
	}
}

func sansLecteurs() ([]string, error) { return nil, nil }

func cheminSteam(racine string) string {
	return filepath.Join(racine, filepath.FromSlash(sousCheminSteam))
}

func cheminXbox(racine string) string {
	return filepath.Join(racine, filepath.FromSlash(sousCheminXbox))
}

func egal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCandidatsSteamDeuxBibliotheques — le dossier de Steam puis chaque bibliotheque du vdf,
// sans doublon (la bibliotheque « 0 » EST le dossier de Steam) ; une bibliotheque sans le jeu
// reste candidate (rien n'est valide ici), ValideDeploy la departage.
func TestCandidatsSteamDeuxBibliotheques(t *testing.T) {
	steam, biblio2, biblio3 := t.TempDir(), t.TempDir(), t.TempDir()
	deploy3 := fauxJeu(t, biblio3, sousCheminSteam)
	ecritVdf(t, steam, steam, biblio2, biblio3)

	src := sourcesInstallation{
		Registre:      registreFaux(map[string]string{"HKCU/SteamPath": filepath.ToSlash(steam)}),
		LecteursFixes: sansLecteurs,
	}
	got, err := candidatsInstallation(BoutiqueSteam, src)
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	want := []string{cheminSteam(steam), cheminSteam(biblio2), cheminSteam(biblio3)}
	if !egal(got, want) {
		t.Fatalf("candidats = %q, attendu %q", got, want)
	}
	var valides []string
	for _, c := range got {
		if ValideDeploy(c) == nil {
			valides = append(valides, c)
		}
	}
	if !egal(valides, []string{deploy3}) {
		t.Errorf("valides = %q, attendu seulement %q", valides, deploy3)
	}
}

// TestCandidatsSteamRepliMachine — la ruche de la machine ne sert que si celle de
// l'utilisateur n'a rien donne.
func TestCandidatsSteamRepliMachine(t *testing.T) {
	steam := t.TempDir()
	ecritVdf(t, steam, steam)
	src := sourcesInstallation{
		Registre:      registreFaux(map[string]string{"HKLM/InstallPath": steam}),
		LecteursFixes: sansLecteurs,
	}
	got, err := candidatsInstallation(BoutiqueSteam, src)
	if err != nil || !egal(got, []string{cheminSteam(steam)}) {
		t.Fatalf("got %q, err %v", got, err)
	}

	// Les deux ruches renseignees : l'utilisateur prime.
	autre := t.TempDir()
	src.Registre = registreFaux(map[string]string{"HKCU/SteamPath": autre, "HKLM/InstallPath": steam})
	got, _ = candidatsInstallation(BoutiqueSteam, src)
	if len(got) == 0 || got[0] != cheminSteam(autre) {
		t.Errorf("la ruche utilisateur doit primer : %q", got)
	}
}

// TestCandidatsSteamVdfAbsent — le dossier de Steam reste candidat, l'absence du vdf est
// rendue a l'appelant.
func TestCandidatsSteamVdfAbsent(t *testing.T) {
	steam := t.TempDir()
	src := sourcesInstallation{
		Registre:      registreFaux(map[string]string{"HKCU/SteamPath": steam}),
		LecteursFixes: sansLecteurs,
	}
	got, err := candidatsInstallation(BoutiqueSteam, src)
	if err == nil {
		t.Error("un vdf absent doit etre rendu a l'appelant")
	}
	if !egal(got, []string{cheminSteam(steam)}) {
		t.Errorf("candidats = %q", got)
	}
}

// TestCandidatsSteamVdfMalForme — erreur rendue, dossier de Steam et chemins lus avant la
// rupture conserves.
func TestCandidatsSteamVdfMalForme(t *testing.T) {
	steam, biblio := t.TempDir(), t.TempDir()
	dir := filepath.Join(steam, "steamapps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"" + echappeVdf(biblio) + "\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\n"
	if err := os.WriteFile(filepath.Join(dir, "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	src := sourcesInstallation{
		Registre:      registreFaux(map[string]string{"HKCU/SteamPath": steam}),
		LecteursFixes: sansLecteurs,
	}
	got, err := candidatsInstallation(BoutiqueSteam, src)
	if err == nil {
		t.Fatal("un vdf mal forme doit rendre une erreur")
	}
	want := []string{cheminSteam(steam), cheminSteam(biblio)}
	if !egal(got, want) {
		t.Errorf("candidats = %q, attendu %q", got, want)
	}
}

// TestCandidatsSteamErreurRegistre — une erreur de registre ne coupe pas le repli sur la
// ruche de la machine, et elle est rendue.
func TestCandidatsSteamErreurRegistre(t *testing.T) {
	steam := t.TempDir()
	ecritVdf(t, steam, steam)
	panne := errors.New("acces refuse")
	src := sourcesInstallation{
		Registre: func(ruche, _, _ string) (string, error) {
			if ruche == rucheUtilisateur {
				return "", panne
			}
			return steam, nil
		},
		LecteursFixes: sansLecteurs,
	}
	got, err := candidatsInstallation(BoutiqueSteam, src)
	if !errors.Is(err, panne) {
		t.Errorf("l'erreur de registre doit etre rendue : %v", err)
	}
	if !egal(got, []string{cheminSteam(steam)}) {
		t.Errorf("candidats = %q", got)
	}
}

// TestCandidatsAucun — rien a proposer : liste vide, pas d'erreur (Steam absent, hors Windows).
func TestCandidatsAucun(t *testing.T) {
	src := sourcesInstallation{Registre: registreFaux(nil), LecteursFixes: sansLecteurs}
	for _, b := range []Boutique{BoutiqueSteam, BoutiqueXbox} {
		got, err := candidatsInstallation(b, src)
		if len(got) != 0 || err != nil {
			t.Errorf("%s : got %q, err %v", b, got, err)
		}
	}
	if _, err := candidatsInstallation("gog", src); !errors.Is(err, ErrBoutiqueInconnue) {
		t.Errorf("boutique inconnue : %v", err)
	}
}

// TestCandidatsXboxDeuxLecteurs — un candidat par lecteur fixe, dans l'ordre fourni, sans
// doublon, sans validation.
func TestCandidatsXboxDeuxLecteurs(t *testing.T) {
	c, d := t.TempDir(), t.TempDir()
	fauxJeu(t, d, sousCheminXbox)
	src := sourcesInstallation{
		Registre:      registreFaux(nil),
		LecteursFixes: func() ([]string, error) { return []string{c, d, d}, nil },
	}
	got, err := candidatsInstallation(BoutiqueXbox, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{cheminXbox(c), cheminXbox(d)}; !egal(got, want) {
		t.Errorf("candidats = %q, attendu %q", got, want)
	}

	panne := errors.New("lecteurs illisibles")
	src.LecteursFixes = func() ([]string, error) { return []string{c}, panne }
	got, err = candidatsInstallation(BoutiqueXbox, src)
	if !errors.Is(err, panne) || len(got) != 1 {
		t.Errorf("got %q, err %v", got, err)
	}
}

// TestAnalyseVdfBarresDoublees — deux antislashs dans le fichier valent un antislash.
func TestAnalyseVdfBarresDoublees(t *testing.T) {
	vdf := `"libraryfolders"
{
	"0" { "path"	"D:\\SteamLibrary" // commentaire
	      "PATH"	"C:\\Program Files (x86)\\Steam" }
	"1" { "path"	"" }
}`
	got, err := analyseBibliothequesVdf(vdf)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`D:\SteamLibrary`, `C:\Program Files (x86)\Steam`}; !egal(got, want) {
		t.Errorf("chemins = %q, attendu %q", got, want)
	}
}

// TestAnalyseVdfMalForme — chaque rupture de structure est une erreur.
func TestAnalyseVdfMalForme(t *testing.T) {
	cas := map[string]string{
		"vide":                   "",
		"texte libre":            "n importe quoi",
		"chaine non fermee":      `"libraryfolders" { "path" "D:\\x`,
		"accolade non fermee":    `"libraryfolders" { "0" { "path" "D:\\x" }`,
		"accolade en trop":       `"libraryfolders" { } }`,
		"accolade sans nom":      `{ }`,
		"cle sans valeur":        `"libraryfolders" { "path" }`,
		"cle finale sans valeur": `"libraryfolders" { } "path"`,
	}
	for nom, vdf := range cas {
		if _, err := analyseBibliothequesVdf(vdf); err == nil {
			t.Errorf("%s : erreur attendue", nom)
		}
	}
}

// TestValideDeploy — un vrai deploy simule passe ; un dossier vide, un parent du deploy, un
// fichier et un chemin absent sont refuses, avec ErrDeployInvalide et ce qui manque.
func TestValideDeploy(t *testing.T) {
	racine := t.TempDir()
	deploy := fauxJeu(t, racine, sousCheminSteam)
	if err := ValideDeploy(deploy); err != nil {
		t.Errorf("un deploy simule doit passer : %v", err)
	}

	// Variante autre que pc : `ds` seul suffit.
	ds := t.TempDir()
	if err := os.MkdirAll(dossierCartes(ds, "ds"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ValideDeploy(ds); err != nil {
		t.Errorf("une variante ds seule doit passer : %v", err)
	}

	fichier := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(fichier, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	refus := map[string]string{
		"dossier vide":      t.TempDir(),
		"parent du deploy":  filepath.Dir(deploy),
		"racine d'install":  racine,
		"fichier":           fichier,
		"chemin inexistant": filepath.Join(racine, "absent"),
	}
	for nom, chemin := range refus {
		err := ValideDeploy(chemin)
		if !errors.Is(err, ErrDeployInvalide) {
			t.Errorf("%s : erreur attendue enveloppant ErrDeployInvalide, recu %v", nom, err)
			continue
		}
		if nom != "fichier" && nom != "chemin inexistant" && !strings.Contains(err.Error(), dossierCartes(chemin, "pc")) {
			t.Errorf("%s : le message doit dire ce qui manque : %v", nom, err)
		}
	}
}
