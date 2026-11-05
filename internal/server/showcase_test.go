package server_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// putPhoto sube la foto de un repuesto (PUT multipart, campo "foto").
func (c *client) putPhoto(partID string, photo []byte) int {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("foto", "foto.jpg")
	_, _ = fw.Write(photo)
	_ = mw.Close()
	req, _ := http.NewRequest("PUT", c.e.srv.URL+"/api/v1/parts/"+partID+"/photo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", c.e.srv.URL)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

// ADR-0029: el dueño marca un repuesto para la vitrina de WhatsApp y le sube la foto. Una foto que no es imagen se
// rechaza, y la foto se descarga con su tipo.
func TestInventory_ShowcasePhoto(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550191", "Óscar Ruiz")
	p := c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{"referencia": "AC-20W50", "nombre": "Aceite 20W50 moto",
		"tipoVehiculo": "moto", "costo": 18000, "precioVenta": 32000, "existencia": 6, "minimo": 1}))
	id := p.str("id")
	if p.body["ofrecerWhatsapp"] != false || p.body["tieneFoto"] != false {
		t.Fatalf("nuevo: %v", p.body)
	}
	r := c.must(http.StatusOK, c.do("PATCH", "/api/v1/parts/"+id, map[string]any{"ofrecerWhatsapp": true, "version": p.body["version"]}))
	if r.body["ofrecerWhatsapp"] != true {
		t.Fatalf("vitrina: %v", r.body)
	}
	if s := c.putPhoto(id, []byte("no soy una imagen")); s != http.StatusBadRequest {
		t.Fatalf("foto inválida: %d", s)
	}
	if s := c.putPhoto(id, jpg); s != http.StatusOK {
		t.Fatalf("foto: %d", s)
	}
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/parts/"+id+"/photo", nil)
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || !bytes.Equal(body, jpg) {
		t.Fatalf("descarga: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
}
