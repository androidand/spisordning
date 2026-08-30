package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

// fakeDiscoverySvc is an in-memory DiscoveryService for handler unit tests.
type fakeDiscoverySvc struct {
	candidate   dto.ImportCandidateResponse
	err         error
	discover    dto.ImportCandidateResponse
	discoverErr error
	promote     dto.PromoteCandidateResponse
	promoteErr  error
	rejectErr   error
}

func (f *fakeDiscoverySvc) DiscoverFromURL(ctx context.Context, in dto.DiscoverRecipeInput) (dto.ImportCandidateResponse, error) {
	if f.discoverErr != nil {
		return dto.ImportCandidateResponse{}, f.discoverErr
	}
	return f.discover, nil
}

func (f *fakeDiscoverySvc) ListCandidates(ctx context.Context, status *string) ([]dto.ImportCandidateResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []dto.ImportCandidateResponse{f.candidate}, nil
}

func (f *fakeDiscoverySvc) GetCandidate(ctx context.Context, id string) (dto.ImportCandidateResponse, error) {
	if f.err != nil {
		return dto.ImportCandidateResponse{}, f.err
	}
	return f.candidate, nil
}

func (f *fakeDiscoverySvc) RejectCandidate(ctx context.Context, id string) error {
	return f.rejectErr
}

func (f *fakeDiscoverySvc) PromoteCandidate(ctx context.Context, id string, in dto.PromoteCandidateInput) (dto.PromoteCandidateResponse, error) {
	if f.promoteErr != nil {
		return dto.PromoteCandidateResponse{}, f.promoteErr
	}
	return f.promote, nil
}

func TestDiscoverRecipe_HappyPath(t *testing.T) {
	svc := &fakeDiscoverySvc{discover: dto.ImportCandidateResponse{
		ID: "c1", Title: "Pasta", Status: "candidate", ImportedAt: time.Now(),
	}}
	mux := newMux(t, Dependencies{Discovery: svc})

	rec := doPost(t, mux, "/recipes/discover", `{"url":"https://example.com/pasta"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got dto.ImportCandidateResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Title != "Pasta" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
}

func TestDiscoverRecipe_MissingURL(t *testing.T) {
	mux := newMux(t, Dependencies{Discovery: &fakeDiscoverySvc{}})
	rec := doPost(t, mux, "/recipes/discover", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDiscoveryListCandidates_HappyPath(t *testing.T) {
	svc := &fakeDiscoverySvc{candidate: dto.ImportCandidateResponse{ID: "c1", Title: "Pasta", Status: "candidate"}}
	mux := newMux(t, Dependencies{Discovery: svc})

	rec := doGet(t, mux, "/recipes/discovery/candidates")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []dto.ImportCandidateResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Title != "Pasta" {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestDiscoveryGetCandidate_HappyPath(t *testing.T) {
	svc := &fakeDiscoverySvc{candidate: dto.ImportCandidateResponse{ID: "c1", Title: "Pasta", Status: "candidate"}}
	mux := newMux(t, Dependencies{Discovery: svc})

	rec := doGet(t, mux, "/recipes/discovery/candidates/c1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
}

func TestDiscoveryGetCandidate_NotFound(t *testing.T) {
	svc := &fakeDiscoverySvc{err: dto.ErrNotFound}
	mux := newMux(t, Dependencies{Discovery: svc})

	rec := doGet(t, mux, "/recipes/discovery/candidates/missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message in 404 body")
	}
}

func TestDiscoveryRejectCandidate_HappyPath(t *testing.T) {
	mux := newMux(t, Dependencies{Discovery: &fakeDiscoverySvc{}})

	rec := doPost(t, mux, "/recipes/discovery/candidates/c1/reject", ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
}

func TestDiscoveryRejectCandidate_NotFound(t *testing.T) {
	mux := newMux(t, Dependencies{Discovery: &fakeDiscoverySvc{rejectErr: dto.ErrNotFound}})

	rec := doPost(t, mux, "/recipes/discovery/candidates/missing/reject", ``)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDiscoveryPromoteCandidate_HappyPath(t *testing.T) {
	svc := &fakeDiscoverySvc{promote: dto.PromoteCandidateResponse{
		FamilyID: "f1", VariantID: "v1", RevisionID: "r1", CandidateStatus: "promoted",
	}}
	mux := newMux(t, Dependencies{Discovery: svc})

	rec := doPost(t, mux, "/recipes/discovery/candidates/c1/promote", ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got dto.PromoteCandidateResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.FamilyID != "f1" || got.CandidateStatus != "promoted" {
		t.Fatalf("unexpected promote response: %+v", got)
	}
}

func TestDiscoveryPromoteCandidate_NotFound(t *testing.T) {
	mux := newMux(t, Dependencies{Discovery: &fakeDiscoverySvc{promoteErr: dto.ErrNotFound}})

	rec := doPost(t, mux, "/recipes/discovery/candidates/missing/promote", ``)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
