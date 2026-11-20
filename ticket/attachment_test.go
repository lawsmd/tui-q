package ticket

import "testing"

func TestRefsFindWhatRefWrites(t *testing.T) {
	a := Attachment{ID: "10001", Name: "error [2].png", MIME: "image/png"}
	b := Attachment{ID: "0b1c", Name: "notes.pdf"}
	text := "Before " + a.Ref() + " and\n\n" + b.Ref() + "\n\n![elsewhere](https://example.com/x.png)"
	refs := Refs(text)
	if len(refs) != 2 {
		t.Fatalf("%d references in %q, want 2", len(refs), text)
	}
	if refs[0].ID != a.ID || refs[0].Name != "error (2).png" || text[refs[0].Start:refs[0].End] != a.Ref() {
		t.Errorf("first reference %+v", refs[0])
	}
	if refs[1].ID != b.ID || refs[1].Name != b.Name {
		t.Errorf("second reference %+v", refs[1])
	}
}

func TestOnlyAPictureHasAnExtension(t *testing.T) {
	for mime, want := range map[string]string{
		"image/png":       ".png",
		"image/jpeg":      ".jpg",
		"image/svg+xml":   "",
		"application/pdf": "",
		"":                "",
	} {
		a := Attachment{Name: "tool.exe", MIME: mime}
		if a.Ext() != want || a.Image() != (want != "") {
			t.Errorf("%q: ext %q image %v, want %q", mime, a.Ext(), a.Image(), want)
		}
	}
	for name, want := range map[string]string{
		"a.PNG": "image/png", "b.jpeg": "image/jpeg", "c.jpg": "image/jpeg",
		"d.svg": "", "e.exe": "", "png": "",
	} {
		if got := ImageType(name); got != want {
			t.Errorf("ImageType(%q) = %q, want %q", name, got, want)
		}
	}
}
