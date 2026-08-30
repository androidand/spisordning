package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/androidand/spisordning/internal/ambient"
	"github.com/androidand/spisordning/internal/domain"
)

// runTonight is the ambient surface for task 5.2: it shows tonight's meal and
// records one-tap reactions. Home Assistant (via the homeops MCP) drives this to
// populate a dashboard and to fold reactions back into the preference model.
//
//	food-brain tonight                          — show tonight's dinner
//	food-brain tonight --json                   — emit the HA payload as JSON
//	food-brain tonight --person kid --sentiment loves
//	                                          — record a reaction, update preferences
func runTonight(args []string) error {
	fs := flag.NewFlagSet("tonight", flag.ExitOnError)
	file := fs.String("file", "tonight.json", "plan projection written by `plan --write-tonight`")
	date := fs.String("date", time.Now().Format("2006-01-02"), "date to surface (default: today)")
	person := fs.String("person", "", "person id reacting (with --sentiment)")
	sentiment := fs.String("sentiment", "", "reaction: loves|likes|neutral|dislikes|hates")
	family := fs.String("family", "family.json", "path to the family config JSON")
	jsonOut := fs.Bool("json", false, "emit the HA payload as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	buf, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("tonight: %w (run `food-brain plan --write-tonight` first)", err)
	}
	var plan ambient.PlanFile
	if err := json.Unmarshal(buf, &plan); err != nil {
		return fmt.Errorf("tonight: %w", err)
	}

	slot, ok := plan.Tonight(*date)
	if !ok {
		return fmt.Errorf("tonight: no planned dinner found in %s", *file)
	}

	if *person != "" && *sentiment != "" {
		s, ok := parseSentiment(*sentiment)
		if !ok {
			return fmt.Errorf("tonight: unknown sentiment %q (want loves|likes|neutral|dislikes|hates)", *sentiment)
		}
		fam, err := loadFamily(*family)
		if err != nil {
			return err
		}
		var personID domain.PersonID
		for _, p := range fam.People {
			if p.Name == *person {
				personID = p.ID
				break
			}
		}
		if personID.String() == "" {
			return fmt.Errorf("tonight: person %q not found in family", *person)
		}
		fam.Preferences = ambient.RecordReaction(fam.Preferences, personID, slot.Tags, s)
		if err := saveFamily(*family, fam); err != nil {
			return err
		}
		fmt.Printf("Recorded %s's %s reaction to %q — preferences updated.\n", *person, *sentiment, slot.Title)
		return nil
	}

	if *jsonOut {
		payload := map[string]string{
			"date":   slot.Date,
			"title":  slot.Title,
			"reason": slot.Reason,
			"render": ambient.Render(slot),
		}
		out, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Println("Tonight's dinner:")
	fmt.Printf("  %s\n", ambient.Render(slot))
	fmt.Println("\nReact with:")
	fmt.Println("  food-brain tonight --person <id> --sentiment <loves|likes|neutral|dislikes|hates>")
	return nil
}

func parseSentiment(s string) (domain.Sentiment, bool) {
	switch s {
	case "loves":
		return domain.Loves, true
	case "likes":
		return domain.Likes, true
	case "neutral":
		return domain.Neutral, true
	case "dislikes":
		return domain.Dislikes, true
	case "hates":
		return domain.Hates, true
	}
	return 0, false
}

// saveFamily writes the family config back, preserving the family.json shape
// (the domain types carry matching json tags).
func saveFamily(path string, fam *familyConfig) error {
	buf, err := json.MarshalIndent(fam, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(buf, '\n'), 0o644)
}
