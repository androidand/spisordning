import { useState } from "react";
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

type IngredientAlias = components["schemas"]["IngredientAlias"];

function useAliases() {
  return useQuery({
    queryKey: ["ingredient-aliases"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/ingredient-aliases");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load aliases (${response.status})`);
      return data;
    },
  });
}

export default function AliasesPage() {
  const queryClient = useQueryClient();
  const aliasesQuery = useAliases();
  const aliases: IngredientAlias[] = aliasesQuery.data ?? [];

  const [alias, setAlias] = useState("");
  const [ingredientId, setIngredientId] = useState("");

  const create = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/ingredient-aliases", {
        body: { alias, ingredient_id: ingredientId },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to add alias (${response.status})`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ingredient-aliases"] });
      setAlias("");
      setIngredientId("");
    },
  });

  const remove = useMutation({
    mutationFn: async (a: IngredientAlias) => {
      const { error, response } = await apiClient.DELETE("/ingredient-aliases/{alias}", {
        params: { path: { alias: a.alias } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to remove alias (${response.status})`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ingredient-aliases"] }),
  });

  return (
    <Page
      title="Ingredient nicknames"
      subtitle="Teach Spisordning your household's names for ingredients, so recipes and lists match up."
    >
      <SectionTitle>Add a nickname</SectionTitle>
      <div className="row">
        <TextInput
          placeholder="Nickname (e.g. potatis)"
          value={alias}
          onChange={(e) => setAlias(e.target.value)}
        />
        <TextInput
          placeholder="Canonical ingredient id (e.g. potato)"
          value={ingredientId}
          onChange={(e) => setIngredientId(e.target.value)}
        />
        <Button
          onClick={() => create.mutate()}
          disabled={!alias.trim() || !ingredientId.trim() || create.isPending}
        >
          Add
        </Button>
      </div>
      {create.isError && <ErrorState message={create.error.message} />}

      <SectionTitle>Known nicknames</SectionTitle>
      {aliasesQuery.isLoading ? (
        <Spinner label="Loading nicknames" />
      ) : aliasesQuery.isError ? (
        <ErrorState message={aliasesQuery.error.message} />
      ) : aliases.length === 0 ? (
        <EmptyState>
          No nicknames yet. Add one above — e.g. map "potatis" to "potato" so both resolve to the
          same ingredient.
        </EmptyState>
      ) : (
        <div className="alias-list">
          {aliases.map((a) => (
            <Card key={a.id} className="alias-row">
              <div className="alias-main">
                <span className="alias-name">{a.alias}</span>
                <span className="alias-arrow">→</span>
                <span className="alias-target">{a.ingredient_id}</span>
                {a.household_id && <Badge tone="neutral">{a.household_id}</Badge>}
              </div>
              <Button
                variant="ghost"
                onClick={() => remove.mutate(a)}
                disabled={remove.isPending}
              >
                Remove
              </Button>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
