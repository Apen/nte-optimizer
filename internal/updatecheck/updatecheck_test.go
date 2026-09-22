package updatecheck

import "testing"

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"v0.1.3", "0.1.2", 1},
		{"0.1.3", "v0.1.3", 0},
		{"v0.2.0", "v0.10.0", -1},
	} {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestReleaseDownloadURLPrefersWindowsArchive(t *testing.T) {
	value := release{HTMLURL: "https://example.test/release"}
	value.Assets = append(value.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: "NTE-Optimizer-Windows.zip", BrowserDownloadURL: "https://example.test/app.zip"})
	if got := releaseDownloadURL(value); got != "https://example.test/app.zip" {
		t.Fatalf("unexpected download URL %q", got)
	}
}
