package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDecideBotReplyHandoff(t *testing.T) {
	cases := []string{
		"Quiero hablar con un asesor",
		"necesito un AGENTE",
		"Quiero una TASACIÓN de mi departamento",
		"tengo un reclamo",
		"quiero coordinar visita para el sábado",
		"pasame con una persona real",
	}
	for _, text := range cases {
		d := decideBotReply(text)
		if !d.Handoff {
			t.Errorf("expected handoff for %q", text)
		}
		if d.Reply != handoffReplyMsg {
			t.Errorf("expected handoff reply for %q, got %q", text, d.Reply)
		}
	}
}

func TestDecideBotReplyNoHandoff(t *testing.T) {
	cases := []string{
		"hola",
		"busco alquilar un departamento en Palermo",
		"quiero comprar una casa",
		"cuánto cuesta el de 3 ambientes",
		"gracias",
		"asdfgh",
	}
	for _, text := range cases {
		d := decideBotReply(text)
		if d.Handoff {
			t.Errorf("did not expect handoff for %q", text)
		}
		if d.Reply == "" {
			t.Errorf("expected a reply for %q", text)
		}
	}
}

func TestNormalizeIntentText(t *testing.T) {
	if got := normalizeIntentText("TASACIÓN Señal"); got != "tasacion senal" {
		t.Errorf("got %q", got)
	}
}

func TestWhatsAppParse(t *testing.T) {
	p := &whatsappProvider{}
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"1","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp",
		"contacts":[{"wa_id":"5491112345678","profile":{"name":"María Gómez"}}],
		"messages":[
			{"from":"5491112345678","id":"wamid.A","timestamp":"1700000000","type":"text","text":{"body":"hola"}},
			{"from":"5491112345678","id":"wamid.B","timestamp":"1700000001","type":"image"}
		]}}]}]}`)
	msgs, err := p.Parse(body)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 text message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.ExternalID != "5491112345678" || m.MessageID != "wamid.A" || m.Text != "hola" {
		t.Errorf("unexpected message: %+v", m)
	}
	if m.DisplayName != "María Gómez" {
		t.Errorf("unexpected display name %q", m.DisplayName)
	}

	ignored, err := p.Parse([]byte(`{"object":"page","entry":[]}`))
	if err != nil || len(ignored) != 0 {
		t.Errorf("expected no messages for other objects, got %v (err %v)", ignored, err)
	}

	statusOnly, err := p.Parse([]byte(`{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"messages","value":{"statuses":[{"id":"x","status":"read"}]}}]}]}`))
	if err != nil || len(statusOnly) != 0 {
		t.Errorf("expected no messages for status updates, got %v (err %v)", statusOnly, err)
	}
}

func TestWhatsAppReply(t *testing.T) {
	var gotPath, gotAuth string
	var gotPayload map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.X"}]}`))
	}))
	defer ts.Close()

	p := &whatsappProvider{
		accessToken:   "wa-token",
		phoneNumberID: "12345",
		baseURL:       ts.URL + "/v21.0",
		client:        &http.Client{Timeout: 10 * time.Second},
	}
	if err := p.Reply("5491112345678", "hola de vuelta"); err != nil {
		t.Fatalf("unexpected reply error: %v", err)
	}
	if gotPath != "/v21.0/12345/messages" {
		t.Errorf("unexpected path %q", gotPath)
	}
	if gotAuth != "Bearer wa-token" {
		t.Errorf("unexpected auth header %q", gotAuth)
	}
	if gotPayload["to"] != "5491112345678" || gotPayload["messaging_product"] != "whatsapp" {
		t.Errorf("unexpected payload %v", gotPayload)
	}
	text, _ := gotPayload["text"].(map[string]interface{})
	if text["body"] != "hola de vuelta" {
		t.Errorf("unexpected text %v", text)
	}

	errTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer errTS.Close()
	bad := &whatsappProvider{accessToken: "x", phoneNumberID: "1", baseURL: errTS.URL, client: &http.Client{Timeout: 10 * time.Second}}
	if err := bad.Reply("1", "t"); err == nil {
		t.Error("expected error on HTTP 401")
	}

	unconfigured := &whatsappProvider{}
	if err := unconfigured.Reply("1", "t"); err == nil {
		t.Error("expected error when not configured")
	}
}
