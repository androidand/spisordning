package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/service"
)

// discoveryStore is an in-memory Store focused on the recipe-discovery and
// recipe-family surface used by service.Discovery. It embeds fakeStore for the
// unrelated Store methods and overrides the discovery/family methods so tests
// can observe created rows.
type discoveryStore struct {
	*fakeStore
	sources   map[string]persistence.ExternalRecipeSource
	cands     map[domain.RecipeImportCandidateID]persistence.ImportCandidate
	candIngs  map[domain.RecipeImportCandidateID][]persistence.ImportCandidateIngredient
	families  map[domain.RecipeFamilyID]persistence.RecipeFamily
	variants  map[domain.RecipeVariantID]persistence.RecipeVariant
	revisions map[domain.RecipeRevisionID]persistence.RecipeRevision
}

func newDiscoveryStore() *discoveryStore {
	d := &discoveryStore{
		fakeStore: &fakeStore{},
		sources:   map[string]persistence.ExternalRecipeSource{},
		cands:     map[domain.RecipeImportCandidateID]persistence.ImportCandidate{},
		candIngs:  map[domain.RecipeImportCandidateID][]persistence.ImportCandidateIngredient{},
		families:  map[domain.RecipeFamilyID]persistence.RecipeFamily{},
		variants:  map[domain.RecipeVariantID]persistence.RecipeVariant{},
		revisions: map[domain.RecipeRevisionID]persistence.RecipeRevision{},
	}
	d.sources["web-jsonld"] = persistence.ExternalRecipeSource{
		ID: "web-jsonld", Name: "Web JSON-LD", Kind: "jsonld_web",
		Decision: "integrate_now", Enabled: true,
	}
	return d
}

func (d *discoveryStore) GetExternalRecipeSource(ctx context.Context, id string) (persistence.ExternalRecipeSource, error) {
	src, ok := d.sources[id]
	if !ok {
		return persistence.ExternalRecipeSource{}, persistence.ErrNoRows
	}
	return src, nil
}

func (d *discoveryStore) SaveImportCandidate(ctx context.Context, c persistence.ImportCandidate) (domain.RecipeImportCandidateID, error) {
	if c.ID == (domain.RecipeImportCandidateID{}) {
		c.ID = domain.NewRecipeImportCandidateID()
	}
	if c.ImportedAt.IsZero() {
		c.ImportedAt = time.Now()
	}
	d.cands[c.ID] = c
	return c.ID, nil
}

func (d *discoveryStore) SaveCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID, lines []persistence.ImportCandidateIngredient) error {
	for i := range lines {
		lines[i].CandidateID = candidateID
	}
	d.candIngs[candidateID] = lines
	return nil
}

func (d *discoveryStore) GetImportCandidate(ctx context.Context, id domain.RecipeImportCandidateID) (persistence.ImportCandidate, error) {
	c, ok := d.cands[id]
	if !ok {
		return persistence.ImportCandidate{}, persistence.ErrNoRows
	}
	return c, nil
}

func (d *discoveryStore) ListImportCandidates(ctx context.Context, status *string) ([]persistence.ImportCandidate, error) {
	out := make([]persistence.ImportCandidate, 0, len(d.cands))
	for _, c := range d.cands {
		if status != nil && *status != "" && c.Status != *status {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *discoveryStore) ListCandidateIngredients(ctx context.Context, candidateID domain.RecipeImportCandidateID) ([]persistence.ImportCandidateIngredient, error) {
	return d.candIngs[candidateID], nil
}

func (d *discoveryStore) SetCandidateStatus(ctx context.Context, id domain.RecipeImportCandidateID, status string) error {
	c, ok := d.cands[id]
	if !ok {
		return persistence.ErrNoRows
	}
	c.Status = status
	d.cands[id] = c
	return nil
}

func (d *discoveryStore) SetCandidatePromoted(ctx context.Context, id domain.RecipeImportCandidateID, variantID domain.RecipeVariantID) error {
	c, ok := d.cands[id]
	if !ok {
		return persistence.ErrNoRows
	}
	c.Status = "promoted"
	c.PromotedVariantID = &variantID
	d.cands[id] = c
	return nil
}

func (d *discoveryStore) CreateRecipeFamily(ctx context.Context, fam persistence.RecipeFamily) error {
	if fam.ID == (domain.RecipeFamilyID{}) {
		fam.ID = domain.NewRecipeFamilyID()
	}
	d.families[fam.ID] = fam
	return nil
}

func (d *discoveryStore) GetRecipeFamily(ctx context.Context, id domain.RecipeFamilyID) (persistence.RecipeFamily, error) {
	f, ok := d.families[id]
	if !ok {
		return persistence.RecipeFamily{}, persistence.ErrNoRows
	}
	return f, nil
}

func (d *discoveryStore) SetRecipeFamilyDefaultVariant(ctx context.Context, familyID domain.RecipeFamilyID, variantID domain.RecipeVariantID) error {
	f, ok := d.families[familyID]
	if !ok {
		return persistence.ErrNoRows
	}
	f.DefaultVariantID = variantID
	d.families[familyID] = f
	return nil
}

func (d *discoveryStore) CreateRecipeVariant(ctx context.Context, v persistence.RecipeVariant) error {
	if v.ID == (domain.RecipeVariantID{}) {
		v.ID = domain.NewRecipeVariantID()
	}
	d.variants[v.ID] = v
	return nil
}

func (d *discoveryStore) GetRecipeVariant(ctx context.Context, id domain.RecipeVariantID) (persistence.RecipeVariant, error) {
	v, ok := d.variants[id]
	if !ok {
		return persistence.RecipeVariant{}, persistence.ErrNoRows
	}
	return v, nil
}

func (d *discoveryStore) CreateRecipeRevision(ctx context.Context, r persistence.RecipeRevision) (domain.RecipeRevisionID, error) {
	if r.ID == (domain.RecipeRevisionID{}) {
		r.ID = domain.NewRecipeRevisionID()
	}
	d.revisions[r.ID] = r
	return r.ID, nil
}

func (d *discoveryStore) ListRecipeRevisions(ctx context.Context, variantID domain.RecipeVariantID) ([]persistence.RecipeRevision, error) {
	var out []persistence.RecipeRevision
	for _, r := range d.revisions {
		if r.VariantID == variantID {
			out = append(out, r)
		}
	}
	return out, nil
}

func intPtr(n int) *int { return &n }

const recipeJSONLD = `{"@context":"https://schema.org","@type":"Recipe","name":"Köttfärssås","description":"En test-sås","recipeYield":"4 portioner","totalTime":"PT30M","author":{"@type":"Person","name":"Test Author"},"recipeIngredient":["200 g köttfärs","1 dl mjölk","salt och peppar"],"recipeInstructions":[{"@type":"HowToStep","text":"Blanda allt i en kastrull."}]}`

const recipeHTML = `<html><head><script type="application/ld+json">` + recipeJSONLD + `</script></head><body></body></html>`

func TestDiscovery_DiscoverFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(recipeHTML))
	}))
	defer srv.Close()

	d := newDiscoveryStore()
	svc := service.NewDiscovery(d, nil, nil)

	resp, err := svc.DiscoverFromURL(context.Background(), dto.DiscoverRecipeInput{URL: srv.URL})
	if err != nil {
		t.Fatalf("DiscoverFromURL: %v", err)
	}
	if resp.Title != "Köttfärssås" {
		t.Errorf("Title = %q, want %q", resp.Title, "Köttfärssås")
	}
	if resp.Status != "candidate" {
		t.Errorf("Status = %q, want %q", resp.Status, "candidate")
	}
	if resp.SourceID != "web-jsonld" {
		t.Errorf("SourceID = %q, want %q", resp.SourceID, "web-jsonld")
	}
	if resp.SourceURL != srv.URL {
		t.Errorf("SourceURL = %q, want %q", resp.SourceURL, srv.URL)
	}
	if resp.Servings == nil || *resp.Servings != 4 {
		t.Errorf("Servings = %v, want 4", resp.Servings)
	}
	if resp.TotalTimeSec == nil || *resp.TotalTimeSec != 1800 {
		t.Errorf("TotalTimeSec = %v, want 1800", resp.TotalTimeSec)
	}
	if resp.Attribution == nil || *resp.Attribution != "Test Author" {
		t.Errorf("Attribution = %v, want %q", resp.Attribution, "Test Author")
	}
	if len(resp.Ingredients) != 3 {
		t.Fatalf("len(Ingredients) = %d, want 3", len(resp.Ingredients))
	}
	if got := resp.Ingredients[0]; got.RawText != "200 g köttfärs" || got.Quantity == nil || *got.Quantity != 200 || got.Unit != "g" {
		t.Errorf("Ingredients[0] = %+v, want 200 g köttfärs", got)
	}
	if got := resp.Ingredients[1]; got.RawText != "1 dl mjölk" || got.Quantity == nil || *got.Quantity != 1 || got.Unit != "dl" {
		t.Errorf("Ingredients[1] = %+v, want 1 dl mjölk", got)
	}
	if got := resp.Ingredients[2]; got.RawText != "salt och peppar" || !got.NeedsReview {
		t.Errorf("Ingredients[2] = %+v, want salt och peppar (needs review)", got)
	}

	// The candidate must be persisted under the returned id.
	if len(d.cands) != 1 {
		t.Fatalf("len(cands) = %d, want 1", len(d.cands))
	}
	for id, c := range d.cands {
		if c.ID.String() != resp.ID {
			t.Errorf("stored candidate id %s != response id %s", c.ID, resp.ID)
		}
		if len(d.candIngs[id]) != 3 {
			t.Errorf("stored ingredients for %s = %d, want 3", id, len(d.candIngs[id]))
		}
	}
}

func TestDiscovery_DiscoverFromURL_Validation(t *testing.T) {
	d := newDiscoveryStore()
	svc := service.NewDiscovery(d, nil, nil)
	ctx := context.Background()

	if _, err := svc.DiscoverFromURL(ctx, dto.DiscoverRecipeInput{URL: ""}); err == nil {
		t.Error("empty URL: expected error, got nil")
	}
	if _, err := svc.DiscoverFromURL(ctx, dto.DiscoverRecipeInput{URL: "ftp://example.com"}); err == nil {
		t.Error("non-http URL: expected error, got nil")
	}
	if len(d.cands) != 0 {
		t.Errorf("len(cands) = %d, want 0 after validation failures", len(d.cands))
	}
}

func seedPromotableCandidate(d *discoveryStore) domain.RecipeImportCandidateID {
	cid := domain.NewRecipeImportCandidateID()
	d.cands[cid] = persistence.ImportCandidate{
		ID:        cid,
		SourceID:  "web-jsonld",
		SourceURL: "https://example.com/kottfarsass",
		Title:     "Köttfärssås",
		Status:    "candidate",
		Servings:  intPtr(4),
		RawJSONLD: []byte(recipeJSONLD),
	}
	return cid
}

func TestDiscovery_PromoteCandidate(t *testing.T) {
	d := newDiscoveryStore()
	cid := seedPromotableCandidate(d)
	svc := service.NewDiscovery(d, nil, nil)

	resp, err := svc.PromoteCandidate(context.Background(), cid.String(), dto.PromoteCandidateInput{})
	if err != nil {
		t.Fatalf("PromoteCandidate: %v", err)
	}
	if resp.CandidateStatus != "promoted" {
		t.Errorf("CandidateStatus = %q, want %q", resp.CandidateStatus, "promoted")
	}
	if len(d.families) != 1 || len(d.variants) != 1 || len(d.revisions) != 1 {
		t.Fatalf("families=%d variants=%d revisions=%d, want 1/1/1", len(d.families), len(d.variants), len(d.revisions))
	}
	var famID domain.RecipeFamilyID
	var fam persistence.RecipeFamily
	for id, f := range d.families {
		famID, fam = id, f
	}
	var varID domain.RecipeVariantID
	var variant persistence.RecipeVariant
	for id, v := range d.variants {
		varID, variant = id, v
	}
	var revID domain.RecipeRevisionID
	var rev persistence.RecipeRevision
	for id, r := range d.revisions {
		revID, rev = id, r
	}

	if resp.FamilyID != famID.String() {
		t.Errorf("FamilyID = %q, want %q", resp.FamilyID, famID)
	}
	if resp.VariantID != varID.String() {
		t.Errorf("VariantID = %q, want %q", resp.VariantID, varID)
	}
	if resp.RevisionID != revID.String() {
		t.Errorf("RevisionID = %q, want %q", resp.RevisionID, revID)
	}
	if fam.Name != "Köttfärssås" {
		t.Errorf("family.Name = %q, want %q", fam.Name, "Köttfärssås")
	}
	if variant.FamilyID != famID {
		t.Errorf("variant.FamilyID = %s, want %s", variant.FamilyID, famID)
	}
	if fam.DefaultVariantID != varID {
		t.Errorf("family.DefaultVariantID = %s, want %s", fam.DefaultVariantID, varID)
	}
	if rev.VariantID != varID {
		t.Errorf("revision.VariantID = %s, want %s", rev.VariantID, varID)
	}
	if len(rev.Ingredients) != 3 {
		t.Errorf("len(revision.Ingredients) = %d, want 3", len(rev.Ingredients))
	}
	if len(rev.Steps) != 1 {
		t.Errorf("len(revision.Steps) = %d, want 1", len(rev.Steps))
	}

	// The candidate must be marked promoted and linked to the variant.
	c := d.cands[cid]
	if c.Status != "promoted" {
		t.Errorf("candidate.Status = %q, want %q", c.Status, "promoted")
	}
	if c.PromotedVariantID == nil || *c.PromotedVariantID != varID {
		t.Errorf("candidate.PromotedVariantID = %v, want %s", c.PromotedVariantID, varID)
	}
}

func TestDiscovery_PromoteCandidate_Idempotent(t *testing.T) {
	d := newDiscoveryStore()

	famID := domain.NewRecipeFamilyID()
	varID := domain.NewRecipeVariantID()
	revID := domain.NewRecipeRevisionID()
	d.families[famID] = persistence.RecipeFamily{ID: famID, Slug: "kottfarsass", Name: "Köttfärssås", DefaultVariantID: varID}
	d.variants[varID] = persistence.RecipeVariant{ID: varID, Slug: "kottfarsass", FamilyID: famID, Title: "Köttfärssås"}
	d.revisions[revID] = persistence.RecipeRevision{ID: revID, VariantID: varID, Servings: 4, Steps: []string{"Blanda allt i en kastrull."}}

	cid := domain.NewRecipeImportCandidateID()
	d.cands[cid] = persistence.ImportCandidate{
		ID:                cid,
		SourceID:          "web-jsonld",
		SourceURL:         "https://example.com/kottfarsass",
		Title:             "Köttfärssås",
		Status:            "promoted",
		PromotedVariantID: &varID,
		RawJSONLD:         []byte(recipeJSONLD),
	}

	svc := service.NewDiscovery(d, nil, nil)
	resp, err := svc.PromoteCandidate(context.Background(), cid.String(), dto.PromoteCandidateInput{})
	if err != nil {
		t.Fatalf("PromoteCandidate (idempotent): %v", err)
	}
	if resp.CandidateStatus != "promoted" {
		t.Errorf("CandidateStatus = %q, want %q", resp.CandidateStatus, "promoted")
	}
	if resp.FamilyID != famID.String() {
		t.Errorf("FamilyID = %q, want %q", resp.FamilyID, famID)
	}
	if resp.VariantID != varID.String() {
		t.Errorf("VariantID = %q, want %q", resp.VariantID, varID)
	}
	if resp.RevisionID != revID.String() {
		t.Errorf("RevisionID = %q, want %q", resp.RevisionID, revID)
	}
	// No new rows must be created on re-promotion.
	if len(d.families) != 1 || len(d.variants) != 1 || len(d.revisions) != 1 {
		t.Errorf("after re-promote: families=%d variants=%d revisions=%d, want 1/1/1", len(d.families), len(d.variants), len(d.revisions))
	}
}
