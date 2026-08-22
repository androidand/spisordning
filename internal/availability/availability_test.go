package availability_test

import (
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/availability"
	"github.com/androidand/spisordning/internal/domain"
)

// --- helpers ---

func milkLot(id int64, qty float64, conf domain.Confidence) availability.InventoryLotInput {
	return availability.InventoryLotInput{
		ID:           id,
		IngredientID: "milk",
		ProductID:    "p-milk",
		Quantity:     qty,
		Unit:         "dl",
		Confidence:   conf,
	}
}

func sugarLot(id int64, qty float64, conf domain.Confidence) availability.InventoryLotInput {
	return availability.InventoryLotInput{
		ID:           id,
		IngredientID: "sugar",
		Quantity:     qty,
		Unit:         "dl",
		Confidence:   conf,
	}
}

func eggLot(id int64, qty float64, conf domain.Confidence) availability.InventoryLotInput {
	return availability.InventoryLotInput{
		ID:           id,
		IngredientID: "egg",
		Quantity:     qty,
		Unit:         "stk",
		Confidence:   conf,
	}
}

func equivSub(from, to string, ratio float64) availability.Substitution {
	return availability.Substitution{
		FromIngredientID: from,
		ToIngredientID:   to,
		Category:         domain.SubstitutionEquivalent,
		Ratio:            ratio,
	}
}

func goodSub(from, to string, ratio float64) availability.Substitution {
	return availability.Substitution{
		FromIngredientID: from,
		ToIngredientID:   to,
		Category:         domain.SubstitutionGood,
		Ratio:            ratio,
	}
}

func formSub(from, to string, ratio float64) availability.Substitution {
	return availability.Substitution{
		FromIngredientID: from,
		ToIngredientID:   to,
		Category:         domain.SubstitutionForm,
		Ratio:            ratio,
	}
}

func line(ing string, qty float64, unit string) availability.RecipeIngredientLine {
	return availability.RecipeIngredientLine{
		IngredientID: ing,
		Quantity:     qty,
		Unit:         unit,
	}
}

// --- 8.1: Domain unit tests ---

func TestExactOnHandMatch(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictFeasible {
		t.Errorf("verdict = %q, want feasible", v.Verdict)
	}
	if len(v.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(v.Lines))
	}
	l := v.Lines[0]
	if l.Status != availability.StatusOnHand {
		t.Errorf("status = %q, want on-hand", l.Status)
	}
	if l.Shortfall != 0 {
		t.Errorf("shortfall = %v, want 0", l.Shortfall)
	}
	if l.IsUncertain {
		t.Error("IsUncertain = true, want false for confident lot")
	}
}

func TestSubstitutionEquivalent(t *testing.T) {
	// Recipe needs milk, we have oat milk with an EQUIVALENT substitution.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "oat-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		equivSub("milk", "oat-milk", 1.0),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	l := v.Lines[0]
	if l.Status != availability.StatusSubstituted {
		t.Errorf("status = %q, want substituted", l.Status)
	}
	if l.SubstitutionTier != string(domain.SubstitutionEquivalent) {
		t.Errorf("tier = %q, want EQUIVALENT", l.SubstitutionTier)
	}
	if l.SubstitutedFromIngredient != "milk" {
		t.Errorf("from = %q, want milk", l.SubstitutedFromIngredient)
	}
	if l.SubstitutedToIngredient != "oat-milk" {
		t.Errorf("to = %q, want oat-milk", l.SubstitutedToIngredient)
	}
}

func TestSubstitutionGood(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "oat-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		goodSub("milk", "oat-milk", 1.0),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	l := v.Lines[0]
	if l.Status != availability.StatusSubstituted {
		t.Errorf("status = %q, want substituted", l.Status)
	}
	if l.SubstitutionTier != string(domain.SubstitutionGood) {
		t.Errorf("tier = %q, want GOOD", l.SubstitutionTier)
	}
}

func TestSubstitutionTierPreference(t *testing.T) {
	// Both EQUIVALENT and GOOD substitutions exist; EQUIVALENT should win.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "oat-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "soy-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		goodSub("milk", "soy-milk", 1.0),
		equivSub("milk", "oat-milk", 1.0),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	l := v.Lines[0]
	if l.SubstitutedToIngredient != "oat-milk" {
		t.Errorf("used substitute to %q, want oat-milk (EQUIVALENT)", l.SubstitutedToIngredient)
	}
	if l.SubstitutionTier != string(domain.SubstitutionEquivalent) {
		t.Errorf("tier = %q, want EQUIVALENT", l.SubstitutionTier)
	}
}

func TestSubstitutionRatioApplied(t *testing.T) {
	// Recipe needs 100g flour, substitute is potato starch at ratio 0.5,
	// so we need 50g potato starch. We have 60g — should satisfy.
	lines := []availability.RecipeIngredientLine{
		{IngredientID: "flour", Quantity: 100, Unit: "g"},
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "potato-starch", Quantity: 60, Unit: "g", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		equivSub("flour", "potato-starch", 0.5),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusSubstituted {
		t.Errorf("status = %q, want substituted", v.Lines[0].Status)
	}
}

func TestSubstitutionRatioInsufficient(t *testing.T) {
	// Recipe needs 100g flour, substitute ratio 0.5 → need 50g potato starch.
	// We only have 30g — should be unmet.
	lines := []availability.RecipeIngredientLine{
		{IngredientID: "flour", Quantity: 100, Unit: "g"},
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "potato-starch", Quantity: 30, Unit: "g", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		equivSub("flour", "potato-starch", 0.5),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusMissing {
		t.Errorf("status = %q, want missing", v.Lines[0].Status)
	}
}

func TestMissingIngredientNoSubstitute(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("saffron", 1, "g"),
	}
	lots := []availability.InventoryLotInput{}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusMissing {
		t.Errorf("status = %q, want missing", v.Lines[0].Status)
	}
	if v.Lines[0].Shortfall != 1 {
		t.Errorf("shortfall = %v, want 1", v.Lines[0].Shortfall)
	}
}

func TestUnknownConfidenceLotFlagged(t *testing.T) {
	// Only on-hand lot has UNKNOWN confidence.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceUnknown),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	// Line is satisfied but flagged.
	l := v.Lines[0]
	if l.Status != availability.StatusUnknown {
		t.Errorf("status = %q, want unknown", l.Status)
	}
	if !l.IsUncertain {
		t.Error("IsUncertain = false, want true")
	}
	// Recipe cannot be "feasible" when a line is uncertain.
	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
}

func TestPartialQuantityIsUnmet(t *testing.T) {
	// Recipe needs 5dl milk, we have 2dl — shortfall, not partial satisfaction.
	lines := []availability.RecipeIngredientLine{
		line("milk", 5, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 2, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	l := v.Lines[0]
	if l.Status != availability.StatusMissing {
		t.Errorf("status = %q, want missing (partial is unmet)", l.Status)
	}
	if l.Shortfall != 3 {
		t.Errorf("shortfall = %v, want 3", l.Shortfall)
	}
	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
}

func TestUnitMismatchIsUnmet(t *testing.T) {
	// Recipe needs milk in "dl", we have it in "ml" — unit mismatch.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "milk", Quantity: 200, Unit: "ml", Confidence: domain.ConfidenceExact},
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusMissing {
		t.Errorf("status = %q, want missing", v.Lines[0].Status)
	}
}

// --- 8.2: Recipe-level aggregation ---

func TestAllOnHandIsFeasible(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("sugar", 1, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
		sugarLot(2, 2, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictFeasible {
		t.Errorf("verdict = %q, want feasible", v.Verdict)
	}
	for i := range v.Lines {
		if v.Lines[i].Status != availability.StatusOnHand {
			t.Errorf("line %d status = %q, want on-hand", i, v.Lines[i].Status)
		}
	}
}

func TestOneSubstitutionMakesFeasibleWithSub(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("sugar", 1, "dl"),
	}
	lots := []availability.InventoryLotInput{
		sugarLot(1, 2, domain.ConfidenceExact),
		{ID: 2, IngredientID: "oat-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
	}
	subs := []availability.Substitution{
		equivSub("milk", "oat-milk", 1.0),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, subs)

	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusSubstituted {
		t.Errorf("line 0 status = %q, want substituted", v.Lines[0].Status)
	}
	if v.Lines[1].Status != availability.StatusOnHand {
		t.Errorf("line 1 status = %q, want on-hand", v.Lines[1].Status)
	}
}

func TestOneMissingMakesInfeasible(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("saffron", 1, "g"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
}

func TestAllUncertainIsFeasibleWithSub(t *testing.T) {
	// Every line is satisfied by an UNKNOWN-confidence lot.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceUnknown),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	// Not "feasible" because not confidently on hand.
	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	if !v.Lines[0].IsUncertain {
		t.Error("IsUncertain = false, want true")
	}
}

func TestMixedOnHandAndMissing(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("saffron", 1, "g"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
	if v.Lines[0].Status != availability.StatusOnHand {
		t.Errorf("line 0 status = %q, want on-hand", v.Lines[0].Status)
	}
	if v.Lines[1].Status != availability.StatusMissing {
		t.Errorf("line 1 status = %q, want missing", v.Lines[1].Status)
	}
}

func TestExplainabilityReasons(t *testing.T) {
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("saffron", 1, "g"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Lines[0].Reason == "" {
		t.Error("line 0 reason is empty")
	}
	if v.Lines[1].Reason == "" {
		t.Error("line 1 reason is empty")
	}
	// The recipe-level verdict must be derivable from the per-line results.
	if v.Verdict != availability.VerdictInfeasible {
		t.Errorf("verdict = %q, want infeasible", v.Verdict)
	}
}

func TestGreedyLotConsumption(t *testing.T) {
	// Two recipe lines need the same ingredient; first line consumes part,
	// second line sees the remainder.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
		line("milk", 3, "dl"),
	}
	lots := []availability.InventoryLotInput{
		milkLot(1, 4, domain.ConfidenceExact),
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if v.Lines[0].Status != availability.StatusOnHand {
		t.Errorf("line 0 status = %q, want on-hand", v.Lines[0].Status)
	}
	// Second line needs 3dl but only 2dl remain — should be missing.
	if v.Lines[1].Status != availability.StatusMissing {
		t.Errorf("line 1 status = %q, want missing", v.Lines[1].Status)
	}
	if v.Lines[1].Shortfall != 1 {
		t.Errorf("line 1 shortfall = %v, want 1", v.Lines[1].Shortfall)
	}
}

func TestDeterministicLotOrder(t *testing.T) {
	// Same lots in reverse ID order should produce the same result because
	// matchingLots sorts by ID internally.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	lotsA := []availability.InventoryLotInput{
		milkLot(2, 3, domain.ConfidenceExact),
		milkLot(1, 3, domain.ConfidenceExact),
	}
	lotsB := []availability.InventoryLotInput{
		milkLot(1, 3, domain.ConfidenceExact),
		milkLot(2, 3, domain.ConfidenceExact),
	}
	vA := availability.ComputeRecipeAvailability("r-1", lines, lotsA, nil)
	vB := availability.ComputeRecipeAvailability("r-1", lines, lotsB, nil)

	if vA.Verdict != vB.Verdict {
		t.Errorf("verdicts differ: A=%q B=%q", vA.Verdict, vB.Verdict)
	}
	if vA.Lines[0].Status != vB.Lines[0].Status {
		t.Errorf("line statuses differ: A=%q B=%q", vA.Lines[0].Status, vB.Lines[0].Status)
	}
}

func TestFormFilterOnSubstitution(t *testing.T) {
	// Recipe needs "milk" with default_form="fresh". There's an EQUIVALENT
	// substitution from milk→oat-milk with from_form="fresh" and another
	// with from_form=nil (any form). The form-filtered one should be chosen.
	lines := []availability.RecipeIngredientLine{
		{IngredientID: "milk", Quantity: 2, Unit: "dl", DefaultForm: "fresh"},
	}
	lots := []availability.InventoryLotInput{
		{ID: 1, IngredientID: "oat-milk", Quantity: 2, Unit: "dl", Confidence: domain.ConfidenceExact},
	}
	freshSub := availability.Substitution{
		FromIngredientID: "milk",
		FromForm:         ptrString("fresh"),
		ToIngredientID:   "oat-milk",
		Category:         domain.SubstitutionEquivalent,
		Ratio:            1.0,
	}
	anyFormSub := availability.Substitution{
		FromIngredientID: "milk",
		ToIngredientID:   "oat-milk",
		Category:         domain.SubstitutionEquivalent,
		Ratio:            1.0,
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, []availability.Substitution{anyFormSub, freshSub})

	if v.Verdict != availability.VerdictFeasibleWithSub {
		t.Errorf("verdict = %q, want feasible-with-substitution", v.Verdict)
	}
	// The form-matching substitution should be preferred when default_form is set.
	if v.Lines[0].SubstitutedToIngredient != "oat-milk" {
		t.Errorf("substituted to %q, want oat-milk", v.Lines[0].SubstitutedToIngredient)
	}
}

func TestEmptyRecipeIsFeasible(t *testing.T) {
	v := availability.ComputeRecipeAvailability("r-empty", nil, nil, nil)
	if v.Verdict != availability.VerdictFeasible {
		t.Errorf("verdict = %q, want feasible for empty recipe", v.Verdict)
	}
	if len(v.Lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(v.Lines))
	}
}

func TestBestBeforeExposure(t *testing.T) {
	// The verdict should surface which lots would be consumed.
	lines := []availability.RecipeIngredientLine{
		line("milk", 2, "dl"),
	}
	bb := time.Now().Add(2 * 24 * time.Hour) // 2 days from now
	lots := []availability.InventoryLotInput{
		{
			ID:           1,
			IngredientID: "milk",
			Quantity:     3,
			Unit:         "dl",
			Confidence:   domain.ConfidenceExact,
			BestBefore:   &bb,
		},
	}
	v := availability.ComputeRecipeAvailability("r-1", lines, lots, nil)

	if len(v.Lines[0].ConsumedLotIDs) != 1 {
		t.Errorf("expected 1 consumed lot, got %d", len(v.Lines[0].ConsumedLotIDs))
	}
}

// --- ptr helpers ---

func ptrString(s string) *string { return &s }
