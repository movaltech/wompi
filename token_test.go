package wompi

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestTokenCreateCard(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/tokens/cards", testPublicKey)
		body := readBody(t, r)
		if body["number"] != "4242424242424242" || body["exp_year"] != "29" || body["card_holder"] != "Ana Gómez" {
			t.Errorf("body = %v", body)
		}
		writeJSON(t, w, 201, map[string]any{"status": "CREATED", "data": map[string]any{
			"id": "tok_test_1", "brand": "VISA", "last_four": "4242", "bin": "424242", "exp_month": "12", "exp_year": "29",
		}})
	})
	tok, err := c.Tokens.CreateCard(context.Background(), CardParams{
		Number: "4242 4242 4242 4242", CVC: "123", ExpMonth: "12", ExpYear: "29", CardHolder: "Ana Gómez",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if tok.ID != "tok_test_1" || tok.Brand != "VISA" || tok.LastFour != "4242" {
		t.Errorf("token = %+v", tok)
	}
	if _, err := c.Tokens.CreateCard(context.Background(), CardParams{Number: "4242"}); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("incomplete card: err = %v", err)
	}
}

func TestTokenNequi(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			assertRequest(t, r, http.MethodPost, "/tokens/nequi", testPublicKey)
			if body := readBody(t, r); body["phone_number"] != "3991111111" {
				t.Errorf("body = %v", body)
			}
			writeJSON(t, w, 201, map[string]any{"data": map[string]any{"id": "nequi_test_1", "status": "PENDING", "phone_number": "3991111111"}})
		default:
			assertRequest(t, r, http.MethodGet, "/tokens/nequi/nequi_test_1", testPublicKey)
			writeJSON(t, w, 200, map[string]any{"data": map[string]any{"id": "nequi_test_1", "status": "APPROVED", "name": "Ana"}})
		}
	})
	tok, err := c.Tokens.CreateNequi(context.Background(), "3991111111")
	if err != nil || tok.Status != TokenPending {
		t.Fatalf("CreateNequi: %+v, %v", tok, err)
	}
	tok, err = c.Tokens.GetNequi(context.Background(), tok.ID)
	if err != nil || tok.Status != TokenApproved || tok.Name != "Ana" {
		t.Fatalf("GetNequi: %+v, %v", tok, err)
	}
	if _, err := c.Tokens.CreateNequi(context.Background(), ""); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("empty phone: err = %v", err)
	}
}

func TestTokenBancolombiaTransfer(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			assertRequest(t, r, http.MethodPost, "/tokens/bancolombia_transfer", testPublicKey)
			body := readBody(t, r)
			if body["type_auth"] != "TOKEN" || body["redirect_url"] != "https://shop.example/back" {
				t.Errorf("body = %v", body)
			}
			writeJSON(t, w, 201, map[string]any{"data": map[string]any{
				"id": "bt_1", "status": "PENDING", "authorization_url": "https://bancolombia.example/auth",
			}})
		default:
			assertRequest(t, r, http.MethodGet, "/tokens/bancolombia_transfer/bt_1", testPublicKey)
			writeJSON(t, w, 200, map[string]any{"data": map[string]any{"id": "bt_1", "status": "APPROVED"}})
		}
	})
	tok, err := c.Tokens.CreateBancolombiaTransfer(context.Background(), BancolombiaTransferTokenParams{RedirectURL: "https://shop.example/back"})
	if err != nil || tok.AuthorizationURL == "" {
		t.Fatalf("CreateBancolombiaTransfer: %+v, %v", tok, err)
	}
	tok, err = c.Tokens.GetBancolombiaTransfer(context.Background(), "bt_1")
	if err != nil || tok.Status != TokenApproved {
		t.Fatalf("GetBancolombiaTransfer: %+v, %v", tok, err)
	}
}
