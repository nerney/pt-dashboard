package settings

import "testing"

func TestMergePreservesBlankAndExistingSecret(t *testing.T) {
	fields := []Field{
		{Name: "apiKey", Secret: true},
		{Name: "token", Secret: true},
		{Name: "minimumSeeders", Default: "1", HasDefault: true},
	}
	got := Merge(fields,
		map[string]string{"apiKey": "saved-key", "token": "saved-token"},
		map[string]string{"apiKey": "", "token": ExistingSecretValue},
	)
	if got["apiKey"] != "saved-key" || got["token"] != "saved-token" {
		t.Fatalf("secrets not preserved: %#v", got)
	}
	if got["minimumSeeders"] != "1" {
		t.Fatalf("default not applied: %#v", got)
	}
}

func TestRenderMasksSecrets(t *testing.T) {
	got := Render([]Field{{Name: "apiKey", Secret: true}}, map[string]string{"apiKey": "saved"})
	if len(got) != 1 || got[0].Value != ExistingSecretValue || !got[0].HasValue {
		t.Fatalf("Render() = %#v", got)
	}
}

func TestDiffNormalizesURLBooleanAndNumber(t *testing.T) {
	fields := []Field{
		{Name: "baseUrl", URL: true},
		{Name: "enabled", Type: "checkbox"},
		{Name: "minimumSeeders", Type: "number"},
	}
	left := map[string]string{
		"baseUrl":        "https://Example.test/",
		"enabled":        "false",
		"minimumSeeders": "1.0",
	}
	right := map[string]string{
		"baseUrl":        "https://example.test",
		"enabled":        "",
		"minimumSeeders": "1",
	}
	if diff := Diff(fields, left, right); len(diff) != 0 {
		t.Fatalf("Diff() = %v, want no diff", diff)
	}
	right["enabled"] = "true"
	if diff := Diff(fields, left, right); len(diff) != 1 || diff[0] != "enabled" {
		t.Fatalf("Diff() = %v, want enabled drift", diff)
	}
}
