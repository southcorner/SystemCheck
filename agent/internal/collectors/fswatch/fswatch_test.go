package fswatch

import "testing"

func TestFswatchSkipsTempFiles(t *testing.T) {
	if !isTempDownload("C:\\Users\\x\\Downloads\\big.zip.crdownload") {
		t.Fatal("expected .crdownload to be skipped")
	}
	if isTempDownload("report.xlsx") {
		t.Fatal("did not expect .xlsx to be skipped")
	}
}

func TestDomainOf(t *testing.T) {
	cases := map[string]string{
		"https://files.example.com/a/b.xlsx?x=1": "files.example.com",
		"http://host:8080/f":                     "host",
		"":                                        "",
		"not a url":                               "",
	}
	for in, want := range cases {
		if got := domainOf(in); got != want {
			t.Fatalf("domainOf(%q) = %q, want %q", in, got, want)
		}
	}
}
