import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { apiClient, errorMessage } from "../api/client";
import type { components } from "../generated/spisordning";
import {
  Badge,
  Card,
  EmptyState,
  ErrorState,
  Page,
  Spinner,
} from "../components/ui";

type Suggestion = components["schemas"]["InspirationSuggestion"];

function useInspiration() {
  return useQuery({
    queryKey: ["inspiration"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/inspiration");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load inspiration (${response.status})`);
      return data;
    },
  });
}

function matchTone(ratio: number): "good" | "warn" | "neutral" {
  if (ratio >= 1) return "good";
  if (ratio >= 0.5) return "warn";
  return "neutral";
}

export default function InspirationPage() {
  const query = useInspiration();
  const suggestions: Suggestion[] = query.data ?? [];

  if (query.isLoading) return <Spinner label="Thinking about dinner" />;
  if (query.isError) return <ErrorState message={query.error.message} />;

  return (
    <Page
      title="Inspiration"
      subtitle="Recipes ranked by how much of them you already have in the pantry."
    >
      {suggestions.length === 0 ? (
        <EmptyState>
          Nothing to suggest yet. Add some ingredients to your pantry (or sync recipes from
          Mealie) and ideas will appear here, ranked by how cookable they are from what you own.
        </EmptyState>
      ) : (
        <div className="items">
          {suggestions.map((s) => (
            <Card key={s.mealie_recipe_id} className="item">
              <div className="item-main">
                <Link className="item-name" to={`/recipes/${s.mealie_recipe_id}`}>
                  {s.title}
                </Link>
                <Badge tone={matchTone(s.match_ratio)}>
                  {Math.round(s.match_ratio * 100)}% on hand
                </Badge>
                {s.tags.map((t) => (
                  <Badge key={t} tone="neutral">
                    {t}
                  </Badge>
                ))}
              </div>
              <div className="item-forms">
                {s.matched_ingredient_ids.length}/{s.total_ingredients} ingredients in pantry
                {s.missing_ingredient_ids.length > 0 &&
                  ` · still need ${s.missing_ingredient_ids.length}`}
              </div>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
