package availability

import (
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// helper: build a minimal EvaluateInputs for testing.
func makeInputs(recipeID string, lines []RecipeLine, lots []LotInfo, subs []IngredientSubstitution) EvaluateInputs {
	return EvaluateInputs{RecipeID: recipeID, Lines: lines, Lots: lots, Substitutions: subs}
}

func ptrForm(f IngredientForm) *IngredientForm { return &f }
func ptrTime(t time.Time) *time.Time                         { return &t }

func TestEvaluateRecipe_ExactOnHandMatch(t *testing.T) {
	fresh := FormFresh
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "tomato", Quantity: 200, Unit: "g", PreferredForm: ptrForm(fresh)},
	}, []LotInfo{
		{ID: 1, IngredientID: "tomato", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(fresh)},
	}, nil)

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeFeasible {
		t.Errorf("status = %q, want feasible", v.Status)
	}
	if len(v.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(v.Lines))
	}
	l := v.Lines[0]
	if l.Status != StatusOnHand {
		t.Errorf("line status = %q, want on-hand", l.Status)
	}
	if l.Reason != "on-hand" {
		t.Errorf("reason = %q, want on-hand", l.Reason)
	}
	if l.Confidence != domain.ConfidenceExact {
		t.Errorf("confidence = %q, want EXACT", l.Confidence)
	}
	if l.Shortfall != 0 {
		t.Errorf("shortfall = %v, want 0", l.Shortfall)
	}
}

func TestEvaluateRecipe_FormMismatchRequiresSubstitution(t *testing.T) {
	fresh, canned := FormFresh, FormCanned
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "tomato", Quantity: 200, Unit: "g", PreferredForm: ptrForm(fresh)},
	}, []LotInfo{
		{ID: 1, IngredientID: "tomato", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(canned)},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "tomato", FromForm: ptrForm(fresh),
			ToIngredientID: "tomato", ToForm: ptrForm(canned),
			Category: TierForm, Ratio: 1.0},
	})

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeFeasibleWithSub {
		t.Errorf("status = %q, want feasible-with-substitution", v.Status)
	}
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Errorf("line status = %q, want substituted", l.Status)
	}
	if l.SubstitutionTier == nil || *l.SubstitutionTier != TierForm {
		t.Errorf("tier = %v, want FORM", l.SubstitutionTier)
	}
}

func TestEvaluateRecipe_SubstitutionAtEachTier(t *testing.T) {
	eq, good, acc, form, diet, emerg := TierEquivalent, TierGood, TierAcceptable, TierForm, TierDietary, TierEmergency
	for _, tier := range []SubstitutionTier{eq, good, acc, form, diet, emerg} {
		t.Run(string(tier), func(t *testing.T) {
			inputs := makeInputs("r1", []RecipeLine{
				{IngredientID: "chicken", Quantity: 100, Unit: "g"},
			}, []LotInfo{
				{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
			}, []IngredientSubstitution{
				{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: tier, Ratio: 1.0},
			})
			v := EvaluateRecipe(inputs)
			l := v.Lines[0]
			if l.Status != StatusSubstituted {
				t.Errorf("status = %q, want substituted", l.Status)
			}
			if l.SubstitutionTier == nil || *l.SubstitutionTier != tier {
				t.Errorf("tier = %v, want %v", l.SubstitutionTier, tier)
			}
		})
	}
}

func TestEvaluateRecipe_SubstitutionTierOrder(t *testing.T) {
	// When both EQUIVALENT and GOOD substitutions exist, EQUIVALENT should win.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "seitan", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "eq", FromIngredientID: "chicken", ToIngredientID: "seitan", Category: TierEquivalent, Ratio: 1.0},
		{ID: "good", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierGood, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Fatalf("status = %q, want substituted", l.Status)
	}
	if l.SubstitutionTier == nil || *l.SubstitutionTier != TierEquivalent {
		t.Errorf("tier = %v, want EQUIVALENT (best tier wins)", l.SubstitutionTier)
	}
}

func TestEvaluateRecipe_ExplicitRatioNotAssumed(t *testing.T) {
	// Recipe needs 100g fresh basil, substitute is dried basil with ratio 0.33
	// (1g dried per 1g fresh). On-hand dried basil is 20g.
	// Required dried = 100 * 0.33 = 33g. On-hand 20g < 33g → missing.
	dried := FormDried
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "basil", Quantity: 100, Unit: "g", PreferredForm: ptrForm(FormFresh)},
	}, []LotInfo{
		{ID: 1, IngredientID: "basil", Quantity: 20, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(dried)},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "basil", FromForm: ptrForm(FormFresh),
			ToIngredientID: "basil", ToForm: ptrForm(dried),
			Category: TierForm, Ratio: 0.33},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("status = %q, want missing (20g < 33g required)", l.Status)
	}
	if l.Shortfall == 0 {
		t.Error("expected shortfall, got 0")
	}
}

func TestEvaluateRecipe_MissingIngredientNoSubstitute(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "saffron", Quantity: 5, Unit: "g"},
	}, nil, nil)

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeInfeasible {
		t.Errorf("status = %q, want infeasible", v.Status)
	}
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("line status = %q, want missing", l.Status)
	}
	if l.Reason != "missing" {
		t.Errorf("reason = %q, want missing", l.Reason)
	}
}

func TestEvaluateRecipe_UnknownConfidenceFlagged(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "milk", Quantity: 500, Unit: "ml"},
	}, []LotInfo{
		{ID: 1, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceUnknown},
	}, nil)

	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusOnHandUncertain {
		t.Errorf("line status = %q, want on-hand-uncertain", l.Status)
	}
	if l.Confidence != domain.ConfidenceUnknown {
		t.Errorf("confidence = %q, want UNKNOWN", l.Confidence)
	}
	if l.Reason != "on-hand-uncertain" {
		t.Errorf("reason = %q, want on-hand-uncertain", l.Reason)
	}
	// Recipe should not be "feasible" — UNKNOWN lots downgrade it.
	if v.Status == RecipeFeasible {
		t.Errorf("recipe status = %q, want feasible-with-substitution (UNKNOWN downgrades)", v.Status)
	}
	if v.Status != RecipeFeasibleWithSub {
		t.Errorf("recipe status = %q, want feasible-with-substitution", v.Status)
	}
}

func TestEvaluateRecipe_RecipeLevelAggregation(t *testing.T) {
	// Mixed: one on-hand, one substituted, one missing → infeasible.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "flour", Quantity: 200, Unit: "g"},
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
		{IngredientID: "saffron", Quantity: 5, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "flour", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierEquivalent, Ratio: 1.0},
	})

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeInfeasible {
		t.Errorf("status = %q, want infeasible (saffron missing)", v.Status)
	}
	if len(v.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(v.Lines))
	}
	if v.Lines[0].Status != StatusOnHand {
		t.Errorf("line 0 status = %q, want on-hand", v.Lines[0].Status)
	}
	if v.Lines[1].Status != StatusSubstituted {
		t.Errorf("line 1 status = %q, want substituted", v.Lines[1].Status)
	}
	if v.Lines[2].Status != StatusMissing {
		t.Errorf("line 2 status = %q, want missing", v.Lines[2].Status)
	}
}

func TestEvaluateRecipe_AllOnHandFeasible(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "flour", Quantity: 200, Unit: "g"},
		{IngredientID: "eggs", Quantity: 3, Unit: "piece"},
	}, []LotInfo{
		{ID: 1, IngredientID: "flour", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "eggs", Quantity: 6, Unit: "piece", Confidence: domain.ConfidenceLikely},
	}, nil)

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeFeasible {
		t.Errorf("status = %q, want feasible", v.Status)
	}
}

func TestEvaluateRecipe_SubstitutionWithUnknownConfidence(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceUnknown},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierEquivalent, Ratio: 1.0},
	})

	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Errorf("status = %q, want substituted", l.Status)
	}
	if l.Confidence != domain.ConfidenceUnknown {
		t.Errorf("confidence = %q, want UNKNOWN", l.Confidence)
	}
	if l.Reason != "substituted-EQUIVALENT-uncertain" {
		t.Errorf("reason = %q, want substituted-EQUIVALENT-uncertain", l.Reason)
	}
}

func TestEvaluateRecipe_NearExpirySurface(t *testing.T) {
	now := time.Now()
	threeDays := now.AddDate(0, 0, 3)
	twoWeeks := now.AddDate(0, 0, 14)
	inputs := EvaluateInputs{
		RecipeID: "r1",
		Lines: []RecipeLine{
			{IngredientID: "milk", Quantity: 500, Unit: "ml"},
		},
		Lots: []LotInfo{
			{ID: 1, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceExact, BestBefore: &threeDays},
			{ID: 2, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceExact, BestBefore: &twoWeeks},
		},
		Now: now,
	}

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeFeasible {
		t.Errorf("status = %q, want feasible", v.Status)
	}
	// Should prefer the near-expiry lot and surface it.
	if len(v.NearExpiryLotIDs) != 1 || v.NearExpiryLotIDs[0] != 1 {
		t.Errorf("near-expiry lots = %v, want [1]", v.NearExpiryLotIDs)
	}
}

func TestEvaluateRecipe_NearExpiryNoSurfaceWhenNowZero(t *testing.T) {
	// When Now is zero, expiry is not surfaced.
	inputs := EvaluateInputs{
		RecipeID: "r1",
		Lines: []RecipeLine{
			{IngredientID: "milk", Quantity: 500, Unit: "ml"},
		},
		Lots: []LotInfo{
			{ID: 1, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceExact,
				BestBefore: ptrTime(time.Now().AddDate(0, 0, 3))},
		},
	}

	v := EvaluateRecipe(inputs)
	if len(v.NearExpiryLotIDs) != 0 {
		t.Errorf("near-expiry lots = %v, want empty when Now is zero", v.NearExpiryLotIDs)
	}
}

func TestEvaluateRecipe_RetiredSubstitutionIgnored(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierEquivalent, Ratio: 1.0, Retired: true},
	})

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeInfeasible {
		t.Errorf("status = %q, want infeasible (retired sub ignored)", v.Status)
	}
}

func TestEvaluateRecipe_PartialQuantityIsUnmet(t *testing.T) {
	// On-hand quantity is less than required → missing, not partially satisfied.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "sugar", Quantity: 200, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "sugar", Quantity: 50, Unit: "g", Confidence: domain.ConfidenceExact},
	}, nil)

	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("status = %q, want missing (partial = unmet)", l.Status)
	}
	if l.Shortfall != 150 {
		t.Errorf("shortfall = %v, want 150", l.Shortfall)
	}
}

func TestEvaluateRecipe_AcceptableFormsAllowMismatch(t *testing.T) {
	// Recipe accepts both fresh and canned tomato. On-hand is canned.
	// Should match directly, not require FORM substitution.
	canned := FormCanned
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "tomato", Quantity: 200, Unit: "g", PreferredForm: ptrForm(FormFresh), AcceptableForms: []IngredientForm{canned}},
	}, []LotInfo{
		{ID: 1, IngredientID: "tomato", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(canned)},
	}, nil)

	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusOnHand {
		t.Errorf("status = %q, want on-hand (acceptable form matched)", l.Status)
	}
}

func TestEvaluateRecipe_SubstitutionShortfallDoesNotBlockLowerTier(t *testing.T) {
	// EQUIVALENT sub exists but target lot is short; GOOD sub fully covers.
	// The line should be satisfied via GOOD, not reported as missing.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "seitan", Quantity: 10, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "eq", FromIngredientID: "chicken", ToIngredientID: "seitan", Category: TierEquivalent, Ratio: 1.0},
		{ID: "good", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierGood, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Errorf("status = %q, want substituted", l.Status)
	}
	if l.SubstitutionTier == nil || *l.SubstitutionTier != TierGood {
		t.Errorf("tier = %v, want GOOD (EQUIVALENT short lot should not block)", l.SubstitutionTier)
	}
	if l.Shortfall != 0 {
		t.Errorf("shortfall = %v, want 0", l.Shortfall)
	}
}

func TestEvaluateRecipe_ConsumedLotIDsPopulated(t *testing.T) {
	now := time.Now()
	threeDays := now.AddDate(0, 0, 3)
	inputs := EvaluateInputs{
		RecipeID: "r1",
		Lines: []RecipeLine{
			{IngredientID: "milk", Quantity: 500, Unit: "ml"},
		},
		Lots: []LotInfo{
			{ID: 1, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceExact, BestBefore: &threeDays},
		},
		Now: now,
	}
	v := EvaluateRecipe(inputs)
	if len(v.ConsumedLotIDs) != 1 || v.ConsumedLotIDs[0] != 1 {
		t.Errorf("consumed lot ids = %v, want [1]", v.ConsumedLotIDs)
	}
	if len(v.NearExpiryLotIDs) != 1 || v.NearExpiryLotIDs[0] != 1 {
		t.Errorf("near-expiry lot ids = %v, want [1]", v.NearExpiryLotIDs)
	}
}

func TestEvaluateRecipe_ConsumedLotIDsForSubstitution(t *testing.T) {
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierEquivalent, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Fatalf("status = %q, want substituted", l.Status)
	}
	if len(l.ConsumedLotIDs) != 1 || l.ConsumedLotIDs[0] != 1 {
		t.Errorf("consumed lot ids = %v, want [1]", l.ConsumedLotIDs)
	}
}

func TestEvaluateRecipe_SubstitutionToFormEnforced(t *testing.T) {
	// Sub says "chicken → tofu (frozen)". On-hand tofu is fresh only.
	// Should not match; line should be missing.
	frozen, fresh := FormFrozen, FormFresh
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(fresh)},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", ToForm: ptrForm(frozen), Category: TierEquivalent, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("status = %q, want missing (toForm=frozen, on-hand is fresh)", l.Status)
	}
}

func TestEvaluateRecipe_SubstitutionToFormAllowsMatch(t *testing.T) {
	// Sub says "chicken → tofu (frozen)". On-hand tofu is frozen.
	// Should match via substitution.
	frozen := FormFrozen
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 100, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "tofu", Quantity: 100, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(frozen)},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", ToForm: ptrForm(frozen), Category: TierEquivalent, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Errorf("status = %q, want substituted (toForm matched)", l.Status)
	}
	if l.SubstitutionTier == nil || *l.SubstitutionTier != TierEquivalent {
		t.Errorf("tier = %v, want EQUIVALENT", l.SubstitutionTier)
	}
}

func TestEvaluateRecipe_MultiLotAggregation(t *testing.T) {
	// Two lots of the same ingredient in different locations.
	// 500g + 500g = 1000g total, recipe needs 800g → feasible.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "flour", Quantity: 800, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "flour", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "flour", Quantity: 500, Unit: "g", Confidence: domain.ConfidenceExact},
	}, nil)

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeFeasible {
		t.Errorf("status = %q, want feasible (500+500 >= 800)", v.Status)
	}
	l := v.Lines[0]
	if l.Status != StatusOnHand {
		t.Errorf("line status = %q, want on-hand", l.Status)
	}
	if l.OnHandQuantity != 1000 {
		t.Errorf("on-hand quantity = %v, want 1000 (aggregated)", l.OnHandQuantity)
	}
	if l.Shortfall != 0 {
		t.Errorf("shortfall = %v, want 0", l.Shortfall)
	}
	if len(l.ConsumedLotIDs) != 2 {
		t.Errorf("consumed lot ids = %v, want 2 lots", l.ConsumedLotIDs)
	}
}

func TestEvaluateRecipe_MultiLotAggregationStillShort(t *testing.T) {
	// Two lots: 300g + 400g = 700g, recipe needs 800g → missing with shortfall 100.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "flour", Quantity: 800, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "flour", Quantity: 300, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "flour", Quantity: 400, Unit: "g", Confidence: domain.ConfidenceExact},
	}, nil)

	v := EvaluateRecipe(inputs)
	if v.Status != RecipeInfeasible {
		t.Errorf("status = %q, want infeasible", v.Status)
	}
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("line status = %q, want missing", l.Status)
	}
	if l.Shortfall != 100 {
		t.Errorf("shortfall = %v, want 100", l.Shortfall)
	}
}

func TestEvaluateRecipe_MultiLotSubstitutionForResidual(t *testing.T) {
	// Direct lots: 300g, need 800g → residual 500g.
	// EQUIVALENT sub target has 600g → covers residual.
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "chicken", Quantity: 800, Unit: "g"},
	}, []LotInfo{
		{ID: 1, IngredientID: "chicken", Quantity: 300, Unit: "g", Confidence: domain.ConfidenceExact},
		{ID: 2, IngredientID: "tofu", Quantity: 600, Unit: "g", Confidence: domain.ConfidenceExact},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "chicken", ToIngredientID: "tofu", Category: TierEquivalent, Ratio: 1.0},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusSubstituted {
		t.Errorf("status = %q, want substituted (direct short, sub covers residual)", l.Status)
	}
	if l.SubstitutionTier == nil || *l.SubstitutionTier != TierEquivalent {
		t.Errorf("tier = %v, want EQUIVALENT", l.SubstitutionTier)
	}
	if l.Shortfall != 0 {
		t.Errorf("shortfall = %v, want 0", l.Shortfall)
	}
}

func TestEvaluateRecipe_SubstitutionShortfallCorrect(t *testing.T) {
	// Recipe needs 100g fresh basil, substitute is dried basil with ratio 0.33.
	// Required dried = 33g. On-hand dried basil is 20g → shortfall = 13.
	dried := FormDried
	inputs := makeInputs("r1", []RecipeLine{
		{IngredientID: "basil", Quantity: 100, Unit: "g", PreferredForm: ptrForm(FormFresh)},
	}, []LotInfo{
		{ID: 1, IngredientID: "basil", Quantity: 20, Unit: "g", Confidence: domain.ConfidenceExact, Form: ptrForm(dried)},
	}, []IngredientSubstitution{
		{ID: "s1", FromIngredientID: "basil", FromForm: ptrForm(FormFresh),
			ToIngredientID: "basil", ToForm: ptrForm(dried),
			Category: TierForm, Ratio: 0.33},
	})
	v := EvaluateRecipe(inputs)
	l := v.Lines[0]
	if l.Status != StatusMissing {
		t.Errorf("status = %q, want missing (20g < 33g required)", l.Status)
	}
	if l.Shortfall != 13 {
		t.Errorf("shortfall = %v, want 13 (33 - 20)", l.Shortfall)
	}
	if l.OnHandQuantity != 20 {
		t.Errorf("on-hand quantity = %v, want 20", l.OnHandQuantity)
	}
}

func TestEvaluateRecipe_MultiLotDeduplication(t *testing.T) {
	// Two recipe lines consume the same lot id — should be deduped at recipe level.
	now := time.Now()
	threeDays := now.AddDate(0, 0, 3)
	inputs := EvaluateInputs{
		RecipeID: "r1",
		Lines: []RecipeLine{
			{IngredientID: "milk", Quantity: 200, Unit: "ml"},
			{IngredientID: "milk", Quantity: 200, Unit: "ml"},
		},
		Lots: []LotInfo{
			{ID: 1, IngredientID: "milk", Quantity: 1000, Unit: "ml", Confidence: domain.ConfidenceExact, BestBefore: &threeDays},
		},
		Now: now,
	}
	v := EvaluateRecipe(inputs)
	// Both lines consume lot 1, but recipe-level should dedup.
	if len(v.ConsumedLotIDs) != 1 || v.ConsumedLotIDs[0] != 1 {
		t.Errorf("consumed lot ids = %v, want [1] (deduped)", v.ConsumedLotIDs)
	}
	if len(v.NearExpiryLotIDs) != 1 || v.NearExpiryLotIDs[0] != 1 {
		t.Errorf("near-expiry lot ids = %v, want [1] (deduped)", v.NearExpiryLotIDs)
	}
}

func TestSubstitutionTierOrder(t *testing.T) {
	order := SubstitutionTierOrder()
	expected := []SubstitutionTier{
		TierEquivalent, TierGood, TierAcceptable,
		TierForm, TierDietary, TierEmergency,
	}
	if len(order) != len(expected) {
		t.Fatalf("order length = %d, want %d", len(order), len(expected))
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], expected[i])
		}
	}
}
