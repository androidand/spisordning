package planning

import "testing"

func TestIsAssumedStaple(t *testing.T) {
	staples := []string{"salt", "peppar", "socker", "olja", "smör", "vatten", "salt "}
	for _, s := range staples {
		if !IsAssumedStaple(s) {
			t.Errorf("%q should be an assumed staple", s)
		}
	}
	realFood := []string{"lök", "falukorv", "matlagningsgrädde", "krossade tomater", "japansk soja", "ägg", "mjölk"}
	for _, f := range realFood {
		if IsAssumedStaple(f) {
			t.Errorf("%q must NOT be dropped as a staple", f)
		}
	}
}

func TestPartitionStaples(t *testing.T) {
	reqs := []ShoppingRequirement{
		{IngredientID: "falukorv", Quantity: 550, Unit: "g"},
		{IngredientID: "salt", Quantity: 1, Unit: "tsk"},
		{IngredientID: "matlagningsgrädde", Quantity: 2.5, Unit: "dl"},
		{IngredientID: "peppar", Quantity: 1, Unit: "krm"},
	}
	buy, dropped := PartitionStaples(reqs)

	if len(buy) != 2 {
		t.Fatalf("expected 2 items to buy, got %d: %+v", len(buy), buy)
	}
	if len(dropped) != 2 {
		t.Fatalf("expected 2 staples dropped, got %d: %+v", len(dropped), dropped)
	}
	for _, r := range buy {
		if r.IngredientID == "salt" || r.IngredientID == "peppar" {
			t.Errorf("staple %q leaked into the buy list", r.IngredientID)
		}
	}
}
