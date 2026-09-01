package domain

import (
	"strings"
	"time"
)

// Food is a food from the Livsmedelsverket (Swedish Food Agency) database. The
// SlvNummer is the canonical nutrition key; the rest of the nutrition namespace
// (Dabas GTINs, Aridents, Matpriskollen keys) resolves through it. Mirrors the
// foods table (migration 000021).
type Food struct {
	SlvNummer        int
	Namn             string
	VetenskapligtNamn  *string
	LivsmedelsTyp    *string
	Projekt          *string
	Version          *string
	SyncedAt         time.Time
}

// Nutrient is one nutritional value for a food, per 100g edible portion;
// mirrors the nutrients table (migration 000021).
type Nutrient struct {
	FoodNummer  int
	EuroFIRKod *string
	Name        string
	Värde       float64
	Enhet       string
	Metodtyp    *string
	SyncedAt    time.Time
}

// ProductMapping resolves an external product identifier (GTIN / Dabas Arident)
// to a canonical SLV food (and, transitively, to a nutrition profile). Exactly
// one of GTIN / DabasARIdent is populated per call site; the SLVnummer is the
// resolved target. Mirrors product_mappings (migration 000021).
type ProductMapping struct {
	ID                    int
	GTIN                  *string
	DabasARIdent          *string
	FoodsSLVNummer        *int
	CanonicalIngredientID *IngredientID
	MappedAt              time.Time
}

// NutritionSyncStatus records the last successful full sync for a source, keyed
// on source; used by the sync job for incremental updates and observability.
type NutritionSyncStatus struct {
	Source      string
	LastSynced  time.Time
	RecordCount int
}

// NutritionProfile is a compact, planner-friendly summary of a food's nutrients
// (task 7.1). It is what domain.Candidate gains when enriched with nutrition data.
type NutritionProfile struct {
	SLVNummer int
	Namn      string
	// Per-100g values for the headline macros/micros. A nil entry means the
	// nutrient was not present for that food.
	Calories     *float64 // kcal
	Protein      *float64 // g
	Fat          *float64 // g
	Carbohydrate *float64 // g
	Sugar        *float64 // g
	Fibre        *float64 // g
	Natrium      *float64 // g (converted from mg where the source reports mg)
}

// NutrientValue returns the value of the named nutrient, case-insensitively
// matched against known Swedish/English nutrient names, or nil if the food does
// not carry it. Used by the preference-based scoring (task 7.3).
func (p NutritionProfile) NutrientValue(name string) *float64 {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case equalFoldAny(n, "kcal", "energi", "energy"):
		return p.Calories
	case equalFoldAny(n, "protein"):
		return p.Protein
	case equalFoldAny(n, "fett", "fat", "total fat"):
		return p.Fat
	case equalFoldAny(n, "kolhydrat", "carb", "carbohydrate", "total carbohydrate"):
		return p.Carbohydrate
	case equalFoldAny(n, "socker", "sugar", "sugars"):
		return p.Sugar
	case equalFoldAny(n, "fiber", "fibre", "dietary fiber", "fiber"):
		return p.Fibre
	case equalFoldAny(n, "natrium", "sodium", "salt"):
		return p.Natrium
	}
	return nil
}

// equalFoldAny reports whether s equals any of the candidates (case-insensitive).
func equalFoldAny(s string, candidates ...string) bool {
	for _, c := range candidates {
		if strings.EqualFold(s, c) {
			return true
		}
	}
	return false
}
