package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type fakeRetailerCredentialSvc struct {
	stored map[string]json.RawMessage
}

func (f *fakeRetailerCredentialSvc) UploadRetailerCredential(_ context.Context, retailer string, payload json.RawMessage) (RetailerCredentialResponse, error) {
	if f.stored == nil {
		f.stored = map[string]json.RawMessage{}
	}
	f.stored[retailer] = payload
	return RetailerCredentialResponse{Retailer: retailer, Payload: payload, UploadedAt: time.Now()}, nil
}

func (f *fakeRetailerCredentialSvc) GetRetailerCredential(_ context.Context, retailer string) (RetailerCredentialResponse, error) {
	payload, ok := f.stored[retailer]
	if !ok {
		return RetailerCredentialResponse{}, ErrNotFound
	}
	return RetailerCredentialResponse{Retailer: retailer, Payload: payload, UploadedAt: time.Now()}, nil
}

func TestUploadRetailerCredential_HappyPath(t *testing.T) {
	svc := &fakeRetailerCredentialSvc{}
	mux := newMux(t, Dependencies{RetailerCredentials: svc})

	body := `[{"name":"sid","value":"abc","domain":"ica.se"}]`
	rec := doPost(t, mux, "/retailers/ica/elevated-credential", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var got RetailerCredentialResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Retailer != "ica" {
		t.Errorf("Retailer = %q, want ica", got.Retailer)
	}
	if string(got.Payload) != body {
		t.Errorf("Payload = %s, want %s", got.Payload, body)
	}
}

func TestUploadThenGetRetailerCredential_RoundTrips(t *testing.T) {
	svc := &fakeRetailerCredentialSvc{}
	mux := newMux(t, Dependencies{RetailerCredentials: svc})

	body := `[{"name":"sid","value":"abc","domain":"ica.se"}]`
	if rec := doPost(t, mux, "/retailers/ica/elevated-credential", body); rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", rec.Code)
	}

	rec := doGet(t, mux, "/retailers/ica/elevated-credential")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got RetailerCredentialResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if string(got.Payload) != body {
		t.Errorf("Payload = %s, want %s", got.Payload, body)
	}
}

func TestGetRetailerCredential_NotUploadedYet(t *testing.T) {
	svc := &fakeRetailerCredentialSvc{}
	mux := newMux(t, Dependencies{RetailerCredentials: svc})

	rec := doGet(t, mux, "/retailers/ica/elevated-credential")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUploadRetailerCredential_InvalidJSON(t *testing.T) {
	svc := &fakeRetailerCredentialSvc{}
	mux := newMux(t, Dependencies{RetailerCredentials: svc})

	rec := doPost(t, mux, "/retailers/ica/elevated-credential", `not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
