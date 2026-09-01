// Package coverage computes whether a filled shopping list satisfies the
// ingredient needs of a meal plan. It is pure: it knows nothing about
// persistence or MCP, so it can be unit-tested in isolation and reused by any
// presentation layer (MCP, REST, CLI).
package coverage

import "fmt"

// Status is the coverage status of a single required line.
type Status string

const (
	// StatusCovered means the list supplies at least the required quantity.
	StatusCovered Status = "covered"
	// StatusShort means the list supplies some but less than the required quantity.
	StatusShort Status = "short"
	// StatusMissing means the list supplies nothing on the line.
	StatusMissing Status = "missing"
)

// Key groups requirements and supply by (ingredient_id, unit). Two lines that
// differ only by unit are distinct keys — mirroring planning.BuildRequirements.
type Key struct {
	IngredientID string
	Unit         string
}

func (k Key) String() string { return fmt.Sprintf("%s@%s", k.IngredientID, k.Unit) }

// StatusString renders a Status as its stable wire string, so presentation
// layers (MCP, REST) share a single canonical spelling.
func StatusString(s Status) string {
	switch s {
	case StatusShort:
		return "short"
	case StatusMissing:
		return "missing"
	default:
		return "covered"
	}
}

// Requirement is one line of a plan's "what to buy" list.
type Requirement struct {
	Key      Key
	Name     string
	Quantity float64
}

// Supply is the quantity the list has for one (ingredient_id, unit) key.
type Supply struct {
	Key        Key
	Quantity   float64
	NotPlanDerivable bool // true when the item carried no ingredient_id (e.g. a checklist label)
}

// Line is the coverage verdict for one required line.
type Line struct {
	Key       Key
	Name      string
	Status    Status
	Required  float64
	Supplied  float64
	Shortfall float64 // positive when supplied < required; zero when covered
}

// Report is the coverage verdict for a plan vs. a list.
type Report struct {
	Lines       []Line
	ShortCount  int
	MissingCount int
	NotPlanDerived int
}

// Check compares a set of requirements against the quantities supplied per key.
// A line is covered when supplied >= required, short when supplied is positive
// but below required, and missing when supplied is zero. Shortfall is the
// positive gap between required and supplied (zero when covered).
func Check(reqs []Requirement, supplied map[Key]float64) Report {
	var rep Report
	for _, r := range reqs {
		s := supplied[r.Key]
		line := Line{
			Key:   r.Key,
			Name:  r.Name,
			Status: StatusCovered,
			Required: r.Quantity,
			Supplied: s,
		}
		if s >= r.Quantity {
			line.Shortfall = 0
		} else {
			line.Status = StatusShort
			line.Shortfall = r.Quantity - s
			if s == 0 {
				line.Status = StatusMissing
				line.Shortfall = r.Quantity
			}
		}
		rep.Lines = append(rep.Lines, line)
		if line.Status == StatusShort {
			rep.ShortCount++
		}
		if line.Status == StatusMissing {
			rep.MissingCount++
		}
	}
	return rep
}

// Aggregate sums the supplied quantities per (ingredient_id, unit) key,
// ignoring items that are not plan-derived.
func Aggregate(supplies []Supply) map[Key]float64 {
	out := map[Key]float64{}
	for _, s := range supplies {
		if s.NotPlanDerivable {
			continue
		}
		out[s.Key] += s.Quantity
	}
	return out
}

// Summarize attaches the counts to a lines slice. Check already records the
// counts, but this is exposed so callers that build lines differently can
// aggregate the same way.
func Summarize(lines []Line) Report {
	var rep Report
	for _, l := range lines {
		rep.Lines = append(rep.Lines, l)
		if l.Status == StatusShort {
			rep.ShortCount++
		}
		if l.Status == StatusMissing {
			rep.MissingCount++
		}
	}
	return rep
}
