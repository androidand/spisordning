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
  Field,
  Page,
  SectionTitle,
  Spinner,
  TextInput,
} from "../components/ui";
import { formatDate, quantityLabel } from "../lib/format";

type Family = components["schemas"]["RecipeFamily"];
type Variant = components["schemas"]["RecipeVariant"];
type Revision = components["schemas"]["RecipeRevision"];
type Ingredient = components["schemas"]["RecipeIngredient"];

function useFamilies() {
  return useQuery({
    queryKey: ["recipe-families"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/recipe-families");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load families (${response.status})`);
      return data;
    },
  });
}

function useVariants(familyId: string | undefined) {
  return useQuery({
    queryKey: ["recipe-variants", familyId],
    enabled: familyId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET(
        "/recipe-families/{familyId}/variants",
        { params: { path: { familyId: familyId as string } } },
      );
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load variants (${response.status})`);
      return data;
    },
  });
}

function useRevisions(familyId: string | undefined, variantId: string | undefined) {
  return useQuery({
    queryKey: ["recipe-revisions", familyId, variantId],
    enabled: familyId !== undefined && variantId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET(
        "/recipe-families/{familyId}/variants/{variantId}/revisions",
        { params: { path: { familyId: familyId as string, variantId: variantId as string } } },
      );
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load revisions (${response.status})`);
      return data;
    },
  });
}

export default function RecipeFamilyPage() {
  const queryClient = useQueryClient();
  const familiesQuery = useFamilies();
  const families: Family[] = familiesQuery.data ?? [];

  const [familyId, setFamilyId] = useState<string | null>(null);
  const activeFamilyId = familyId ?? families.at(0)?.id ?? null;
  const variantsQuery = useVariants(activeFamilyId ?? undefined);
  const variants: Variant[] = variantsQuery.data ?? [];

  const [variantId, setVariantId] = useState<string | null>(null);
  const activeVariantId = variantId ?? variants.at(0)?.id ?? null;
  const revisionsQuery = useRevisions(activeFamilyId ?? undefined, activeVariantId ?? undefined);
  const revisions: Revision[] = revisionsQuery.data ?? [];

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["recipe-families"] });
    queryClient.invalidateQueries({ queryKey: ["recipe-variants", activeFamilyId] });
    queryClient.invalidateQueries({ queryKey: ["recipe-revisions", activeFamilyId, activeVariantId] });
  };

  // Create family.
  const [newFamilyName, setNewFamilyName] = useState("");
  const createFamily = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/recipe-families", {
        body: { name: newFamilyName },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to create family (${response.status})`);
      return data;
    },
    onSuccess: (f) => {
      setNewFamilyName("");
      setFamilyId(f.id);
      invalidate();
    },
  });

  // Create variant.
  const [newVariantTitle, setNewVariantTitle] = useState("");
  const createVariant = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST(
        "/recipe-families/{familyId}/variants",
        {
          params: { path: { familyId: activeFamilyId as string } },
          body: { title: newVariantTitle },
        },
      );
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to create variant (${response.status})`);
      return data;
    },
    onSuccess: (v) => {
      setNewVariantTitle("");
      setVariantId(v.id);
      invalidate();
    },
  });

  // Create revision.
  const [revDesc, setRevDesc] = useState("");
  const [revServings, setRevServings] = useState("4");
  const [revIngredients, setRevIngredients] = useState("");
  const [revSteps, setRevSteps] = useState("");
  const createRevision = useMutation({
    mutationFn: async () => {
      const ingredients: Ingredient[] = revIngredients
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => {
          const [qty, unit, ...rest] = line.split(/\s+/);
          return {
            ingredient_id: rest.join(" ") || line,
            quantity: Number(qty) || 1,
            unit: unit && !Number.isNaN(Number(unit)) ? unit : "pcs",
            raw_text: line,
          };
        });
      const steps = revSteps
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean);
      const { data, error, response } = await apiClient.POST(
        "/recipe-families/{familyId}/variants/{variantId}/revisions",
        {
          params: {
            path: { familyId: activeFamilyId as string, variantId: activeVariantId as string },
          },
          body: {
            servings: Number(revServings) || 0,
            description: revDesc,
            ingredients,
            steps,
          },
        },
      );
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to create revision (${response.status})`);
      return data;
    },
    onSuccess: () => {
      setRevDesc("");
      setRevIngredients("");
      setRevSteps("");
      invalidate();
    },
  });

  const setDefault = useMutation({
    mutationFn: async (vid: string) => {
      const { error, response } = await apiClient.POST(
        "/recipe-families/{familyId}/variants/{variantId}/default",
        { params: { path: { familyId: activeFamilyId as string, variantId: vid } } },
      );
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to set default (${response.status})`);
    },
    onSuccess: () => invalidate(),
  });

  if (familiesQuery.isLoading) return <Spinner label="Loading recipe families" />;
  if (familiesQuery.isError) return <ErrorState message={familiesQuery.error.message} />;

  const activeFamily = families.find((f) => f.id === activeFamilyId);

  return (
    <Page
      title="Recipe families"
      subtitle="Git-like recipe hierarchy: family → variant → revision."
      actions={
        families.length > 0 ? (
          <select
            className="input"
            value={activeFamilyId ?? ""}
            onChange={(e) => {
              setFamilyId(e.target.value);
              setVariantId(null);
            }}
          >
            {families.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name}
              </option>
            ))}
          </select>
        ) : undefined
      }
    >
      <div className="row">
        <TextInput
          placeholder="New family name (e.g. Korvstroganoff)"
          value={newFamilyName}
          onChange={(e) => setNewFamilyName(e.target.value)}
        />
        <Button
          onClick={() => createFamily.mutate()}
          disabled={!newFamilyName.trim() || createFamily.isPending}
        >
          Add family
        </Button>
      </div>
      {createFamily.isError && <ErrorState message={createFamily.error.message} />}

      {!activeFamily ? (
        <EmptyState>No recipe families yet. Add one above.</EmptyState>
      ) : (
        <>
          <Card className="family-card">
            <div className="family-head">
              <span className="family-name">{activeFamily.name}</span>
              {activeFamily.default_variant_id && (
                <Badge tone="approved">
                  default: {variants.find((v) => v.id === activeFamily.default_variant_id)?.title ?? "—"}
                </Badge>
              )}
            </div>
            {activeFamily.description && <p className="family-desc">{activeFamily.description}</p>}
          </Card>

          <SectionTitle>Variants</SectionTitle>
          <div className="row">
            <TextInput
              placeholder="New variant title (e.g. Andreas version)"
              value={newVariantTitle}
              onChange={(e) => setNewVariantTitle(e.target.value)}
            />
            <Button
              onClick={() => createVariant.mutate()}
              disabled={!newVariantTitle.trim() || createVariant.isPending}
            >
              Add variant
            </Button>
          </div>
          {createVariant.isError && <ErrorState message={createVariant.error.message} />}

          {variants.length === 0 ? (
            <EmptyState>No variants in this family yet.</EmptyState>
          ) : (
            <div className="variant-list">
              {variants.map((v) => (
                <Card
                  key={v.id}
                  className={`variant ${v.id === activeVariantId ? "active" : ""}`}
                >
                  <button
                    type="button"
                    className="variant-select"
                    onClick={() => setVariantId(v.id)}
                  >
                    <span className="variant-title">{v.title}</span>
                    {v.source_attribution && (
                      <span className="variant-source">{v.source_attribution}</span>
                    )}
                  </button>
                  {v.id !== activeFamily.default_variant_id && (
                    <Button
                      variant="ghost"
                      onClick={() => setDefault.mutate(v.id)}
                      disabled={setDefault.isPending}
                    >
                      Set default
                    </Button>
                  )}
                </Card>
              ))}
            </div>
          )}
          {setDefault.isError && <ErrorState message={setDefault.error.message} />}

          {activeVariantId && (
            <>
              <SectionTitle>Revisions</SectionTitle>
              {revisionsQuery.isLoading ? (
                <Spinner label="Loading revisions" />
              ) : revisionsQuery.isError ? (
                <ErrorState message={revisionsQuery.error.message} />
              ) : revisions.length === 0 ? (
                <EmptyState>No revisions yet. Add the first one below.</EmptyState>
              ) : (
                <div className="revision-list">
                  {revisions.map((rev) => (
                    <Card key={rev.id} className="revision">
                      <div className="revision-head">
                        <span className="revision-id">#{rev.id}</span>
                        {rev.servings ? <Badge tone="neutral">{rev.servings} servings</Badge> : null}
                        <span className="revision-date">{formatDate(rev.created_at)}</span>
                        {rev.parents && rev.parents.length > 0 && (
                          <Badge tone="draft">from #{rev.parents.join(", #")}</Badge>
                        )}
                      </div>
                      {rev.description && <p className="revision-desc">{rev.description}</p>}
                      {rev.ingredients.length > 0 && (
                        <ul className="revision-ingredients">
                          {rev.ingredients.map((ing, i) => (
                            <li key={i}>
                              {quantityLabel(ing.quantity, ing.unit)} {ing.ingredient_id}
                            </li>
                          ))}
                        </ul>
                      )}
                      {rev.steps.length > 0 && (
                        <ol className="revision-steps">
                          {rev.steps.map((s, i) => (
                            <li key={i}>{s}</li>
                          ))}
                        </ol>
                      )}
                    </Card>
                  ))}
                </div>
              )}

              <SectionTitle>New revision</SectionTitle>
              <div className="revision-form">
                <Field label="Description">
                  <TextInput
                    value={revDesc}
                    onChange={(e) => setRevDesc(e.target.value)}
                    placeholder="What changed?"
                  />
                </Field>
                <Field label="Servings">
                  <TextInput
                    type="number"
                    className="input-narrow"
                    value={revServings}
                    onChange={(e) => setRevServings(e.target.value)}
                  />
                </Field>
                <Field label="Ingredients (one per line: qty unit name)">
                  <textarea
                    className="input textarea"
                    rows={4}
                    value={revIngredients}
                    onChange={(e) => setRevIngredients(e.target.value)}
                    placeholder={"250 g mjölk\n2 st ägg\n100 g smör"}
                  />
                </Field>
                <Field label="Steps (one per line)">
                  <textarea
                    className="input textarea"
                    rows={4}
                    value={revSteps}
                    onChange={(e) => setRevSteps(e.target.value)}
                    placeholder={"Smält smöret\nVispa äggen"}
                  />
                </Field>
                <Button
                  onClick={() => createRevision.mutate()}
                  disabled={createRevision.isPending || !revIngredients.trim()}
                >
                  Add revision
                </Button>
              </div>
              {createRevision.isError && <ErrorState message={createRevision.error.message} />}
            </>
          )}
        </>
      )}
    </Page>
  );
}
