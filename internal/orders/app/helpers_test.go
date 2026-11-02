package app

import "testing"

func TestSniffImage(t *testing.T) {
	cases := map[string][]byte{
		"image/jpeg": {0xFF, 0xD8, 0xFF, 0xE0},
		"image/png":  {0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
		"image/webp": []byte("RIFF\x00\x00\x00\x00WEBP"),
		"":           []byte("GIF89a"),
	}
	for want, b := range cases {
		if got := sniffImage(b); got != want {
			t.Errorf("sniffImage(%q) = %q, quería %q", b, got, want)
		}
	}
}

func TestDescribeAndJoin(t *testing.T) {
	brand, line := "Yamaha", "NMAX"
	for _, c := range []struct {
		brand, line *string
		kind, want  string
	}{
		{&brand, &line, "moto", "Yamaha NMAX"},
		{nil, &line, "moto", "NMAX"},
		{nil, nil, "moto", "moto"},
		{nil, nil, "carro", "carro"},
	} {
		if got := describe(c.brand, c.line, c.kind); got != c.want {
			t.Errorf("describe = %q, quería %q", got, c.want)
		}
	}
	if got := join([]string{"fotos", "kilometraje"}); got != "fotos; kilometraje" {
		t.Errorf("join = %q", got)
	}
}
