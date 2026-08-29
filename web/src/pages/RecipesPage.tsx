import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient, errorMessage } from "../api/client";
import type { components } from "../generated/spisordning";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorState,
  Page,
  SectionTitle,
  Spinner,
  TextInput,
} from "../components/ui";
import { formatDate } from "../lib/format";

type RecipeRef = components["schemas"]["RecipeRef"];
type Favorite = components["schemas"]["Favorite"];
type RecipeRating = components["schemas"]["RecipeRating"];

function useRecipes() {
  return useQuery({
    queryKey: ["recipes"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/recipes");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load recipes (${response.status})`);
      return data;
    },
  });
}

function useFavorites(recipeId: string) {
  return useQuery({
    queryKey: ["favorites", recipeId],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/recipes/{id}/favorites", {
        params: { path: { id: recipeId } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load favorites (${response.status})`);
      return data;
    },
  });
}

function useRating(recipeId: string) {
  return useQuery({
    queryKey: ["rating", recipeId],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/recipes/{id}/rating", {
        params: { path: { id: recipeId } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load rating (${response.status})`);
      return data;
    },
  });
}

function effortLabel(effort: number): string {
  if (effort <= 1) return "quick";
  if (effort === 2) return "medium";
  return "intense";
}

/** A recipe card with a favorite toggle and its rating. */
function RecipeCard({ recipe }: { recipe: RecipeRef }) {
  const queryClient = useQueryClient();
  const favoritesQuery = useFavorites(recipe.mealie_recipe_id);
  const ratingQuery = useRating(recipe.mealie_recipe_id);
  const favorites: Favorite[] = favoritesQuery.data ?? [];
  const rating: RecipeRating | undefined = ratingQuery.data;
  const isFavorite = favorites.length > 0;

  const toggleFavorite = useMutation({
    mutationFn: async () => {
      const body = { person_id: "household" };
      if (isFavorite) {
        const { error, response } = await apiClient.DELETE("/recipes/{id}/favorites", {
          params: { path: { id: recipe.mealie_recipe_id } },
          body,
        });
        if (error) throw new Error(errorMessage(error));
        if (!response.ok) throw new Error(`Failed to unset favorite (${response.status})`);
      } else {
        const { error, response } = await apiClient.POST("/recipes/{id}/favorites", {
          params: { path: { id: recipe.mealie_recipe_id } },
          body,
        });
        if (error) throw new Error(errorMessage(error));
        if (!response.ok) throw new Error(`Failed to set favorite (${response.status})`);
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["favorites", recipe.mealie_recipe_id] });
    },
  });

  return (
    <Card className="recipe">
      <div className="recipe-head">
        <span className="recipe-title">{recipe.title || recipe.mealie_recipe_id}</span>
        <Badge tone="neutral">{effortLabel(recipe.effort)}</Badge>
      </div>
      {recipe.tags.length > 0 && (
        <div className="recipe-tags">
          {recipe.tags.map((t) => (
            <span key={t} className="tag">
              {t}
            </span>
          ))}
        </div>
      )}
      <div className="recipe-meta">
        <span>synced {formatDate(recipe.last_synced_at)}</span>
        {rating && rating.review_count > 0 && (
          <span className="recipe-rating">
            {"★".repeat(Math.round(rating.average))}
            {"☆".repeat(Math.max(0, 5 - Math.round(rating.average)))} {rating.average.toFixed(1)} ({rating.review_count})
          </span>
        )}
      </div>
      <div className="recipe-actions">
        <Button
          variant={isFavorite ? "primary" : "ghost"}
          onClick={() => toggleFavorite.mutate()}
          disabled={toggleFavorite.isPending}
        >
          {isFavorite ? "★ Favorited" : "☆ Favorite"}
        </Button>
      </div>
      {toggleFavorite.isError && <ErrorState message={toggleFavorite.error.message} />}
    </Card>
  );
}

export default function RecipesPage() {
  const recipesQuery = useRecipes();
  const [q, setQ] = useState("");

  const recipes: RecipeRef[] = recipesQuery.data ?? [];
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if (!needle) return recipes;
    return recipes.filter(
      (r) =>
        r.title.toLowerCase().includes(needle) ||
        r.tags.some((t) => t.toLowerCase().includes(needle)),
    );
  }, [recipes, q]);

  if (recipesQuery.isLoading) return <Spinner label="Loading recipes" />;
  if (recipesQuery.isError) return <ErrorState message={recipesQuery.error.message} />;

  return (
    <Page
      title="Recipes"
      subtitle="Your recipe library (Mealie-backed)."
      actions={
        <TextInput
          placeholder="Filter by title or tag…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      }
    >
      {filtered.length === 0 ? (
        <EmptyState>No recipes found.</EmptyState>
      ) : (
        <div className="recipe-grid">
          {filtered.map((r) => (
            <RecipeCard key={r.mealie_recipe_id} recipe={r} />
          ))}
        </div>
      )}

      <SectionTitle>Recipe families</SectionTitle>
      <EmptyState>
        For the git-like recipe hierarchy (families, variants, and revisions), see the{" "}
        <a href="#/recipe-families">Recipe families</a> page.
      </EmptyState>
    </Page>
  );
}
