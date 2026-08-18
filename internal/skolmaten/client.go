// Package skolmaten reads the week's school lunch menu so the planner can
// avoid serving for dinner what the kids already ate at school. It talks to
// the homelab's cache-backed skolmaten service (which mirrors skolmaten.se's
// /api/4 shape).
package skolmaten

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
)

// Client fetches weekly menus from a skolmaten-compatible API.
type Client struct {
	token string // optional Client-Token header
	http  *httpclient.Client
}

// New returns a Client for the service at baseURL (e.g. "http://192.168.1.120:8787").
func New(baseURL, token string) *Client {
	return &Client{
		token: token,
		http:  httpclient.New(baseURL, "skolmaten", 30*time.Second),
	}
}

// DayMenu is one school day's meals.
type DayMenu struct {
	Date  time.Time
	Meals []string // meal names as published, e.g. "Stekt fisk med potatismos"
}

// menuResponse mirrors the /api/4 MenuResponse shape (subset we need).
type menuResponse struct {
	WeekState *struct {
		Week int `json:"week"`
		Year int `json:"year"`
		Days []struct {
			Date  string `json:"date"`
			Meals []struct {
				Name string `json:"name"`
			} `json:"Meals"`
		} `json:"Days"`
	} `json:"WeekState"`
}

// WeekMenu fetches the menu for an ISO year/week. A week with no published
// menu returns an empty slice, not an error.
func (c *Client) WeekMenu(ctx context.Context, school string, year, week int) ([]DayMenu, error) {
	path := fmt.Sprintf("/api/4/menu/school/%s?year=%d&week=%d", school, year, week)
	var body menuResponse
	headers := func(req *http.Request) {
		if c.token != "" {
			req.Header.Set("Client-Token", c.token)
		}
	}
	if err := c.http.GetJSON(ctx, path, &body, headers); err != nil {
		return nil, err
	}
	if body.WeekState == nil {
		return nil, nil // no published menu for this week
	}

	out := make([]DayMenu, 0, len(body.WeekState.Days))
	for _, d := range body.WeekState.Days {
		date, err := time.Parse(time.RFC3339, d.Date)
		if err != nil {
			// Some deployments serve plain dates.
			if date, err = time.Parse("2006-01-02", d.Date); err != nil {
				continue
			}
		}
		day := DayMenu{Date: date}
		for _, m := range d.Meals {
			if m.Name != "" {
				day.Meals = append(day.Meals, m.Name)
			}
		}
		out = append(out, day)
	}
	return out, nil
}

// Swedish filler words that carry no dish identity.
var stopwords = map[string]bool{
	"med": true, "och": true, "samt": true, "i": true, "på": true,
	"serveras": true, "till": true, "av": true, "en": true, "ett": true,
}

// TagsForDay tokenizes a day's meal names into lowercase tags comparable with
// recipe tags (the scorer's SchoolLunchTags input).
func TagsForDay(day DayMenu) []string {
	seen := map[string]bool{}
	var tags []string
	for _, meal := range day.Meals {
		for word := range strings.FieldsSeq(strings.ToLower(meal)) {
			word = strings.Trim(word, ",.()-")
			if len(word) < 3 || stopwords[word] || seen[word] {
				continue
			}
			seen[word] = true
			tags = append(tags, word)
		}
	}
	return tags
}
