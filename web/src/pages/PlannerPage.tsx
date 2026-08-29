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
} from "../components/ui";
import { quantityLabel, weekdayOf } from "../lib/format";

type MealPlan = components["schemas"]["MealPlan"];
type MealPlanView = components["schemas"]["MealPlanView"];
type Candidate = components["schemas"]["MealPlanCandidate"];
type Requirement = components["schemas"]["ShoppingRequirement"];

function usePlans() {
  return useQuery({
    queryKey: ["plans"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/plans");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load plans (${response.status})`);
      return data;
    },
  });
}

function usePlanView(planId: number | undefined) {
  return useQuery({
    queryKey: ["plan", planId],
    enabled: planId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/plans/{planId}", {
        params: { path: { planId: planId as number } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load plan (${response.status})`);
      return data;
    },
  });
}

function useRequirements(planId: number | undefined) {
  return useQuery({
    queryKey: ["shopping-requirements", planId],
    enabled: planId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/plans/{planId}/shopping-requirements", {
        params: { path: { planId: planId as number } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load requirements (${response.status})`);
      return data;
    },
  });
}

function statusTone(status: MealPlan["status"]) {
  return status === "approved" ? "approved" : status === "archived" ? "archived" : "draft";
}

function mondayOf(weekStart: string): string[] {
  const base = new Date(weekStart + "T00:00:00Z");
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(base);
    d.setUTCDate(d.getUTCDate() + i);
    return d.toISOString().slice(0, 10);
  });
}

export default function PlannerPage() {
  const queryClient = useQueryClient();
  const plansQuery = usePlans();
  const plans = plansQuery.data ?? [];

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const planId = selectedId ?? plans.find((p) => p.status === "approved")?.id ?? plans.at(-1)?.id ?? null;
  const viewQuery = usePlanView(planId ?? undefined);
  const reqQuery = useRequirements(planId ?? undefined);

  const [picked, setPicked] = useState<Record<string, string>>({});

  const runMutation = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/plans/run", {
        body: { create_wishlist: false },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Plan run failed (${response.status})`);
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["plans"] }),
  });

  const approveMutation = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.PATCH("/plans/{planId}", {
        params: { path: { planId: planId as number } },
        body: { status: "approved" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to approve (${response.status})`);
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["plan", planId] }),
  });

  const decideMutation = useMutation({
    mutationFn: async () => {
      const entries = Object.entries(picked);
      if (entries.length === 0) throw new Error("Pick at least one dinner first");
      const { data, error, response } = await apiClient.POST("/plans/{planId}/decisions", {
        params: { path: { planId: planId as number } },
        body: entries.map(([slotDate, mealieRecipeId]) => ({ slot_date: slotDate, mealie_recipe_id: mealieRecipeId })),
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to commit decisions (${response.status})`);
      return data;
    },
    onSuccess: () => {
      setPicked({});
      queryClient.invalidateQueries({ queryKey: ["plan", planId] });
      queryClient.invalidateQueries({ queryKey: ["shopping-requirements", planId] });
    },
  });

  const view: MealPlanView | undefined = viewQuery.data;
  const requirements: Requirement[] = reqQuery.data ?? [];
  const days = view ? mondayOf(view.plan.week_start) : [];

  const candidatesByDate = useMemo(() => {
    const map = new Map<string, Candidate>();
    for (const c of view?.candidates ?? []) {
      const existing = map.get(c.slot_date);
      if (!existing || c.rank < existing.rank) map.set(c.slot_date, c);
    }
    return map;
  }, [view]);

  const decisions = view?.decisions ?? [];
  const decisionByDate = useMemo(() => {
    const map = new Map<string, string>();
    for (const d of decisions) map.set(d.slot_date, d.mealie_recipe_id);
    return map;
  }, [decisions]);

  const recipeTitle = (id: string): string => {
    for (const c of view?.candidates ?? []) {
      if (c.recipe.mealie_recipe_id === id && c.recipe.title) return c.recipe.title;
    }
    return id;
  };

  if (plansQuery.isLoading) return <Spinner label="Loading plans" />;
  if (plansQuery.isError) return <ErrorState message={plansQuery.error.message} />;

  return (
    <Page
      title="Weekly planner"
      subtitle="Plan dinners, commit decisions, and see what to buy."
      actions={
        <>
          {plans.length > 0 && (
            <select
              className="input"
              value={planId ?? ""}
              onChange={(e) => setSelectedId(Number(e.target.value))}
            >
              {plans.map((p) => (
                <option key={p.id} value={p.id}>
                  Week of {p.week_start} · {p.status}
                </option>
              ))}
            </select>
          )}
          <Button onClick={() => runMutation.mutate()} disabled={runMutation.isPending}>
            {runMutation.isPending ? "Running…" : "Run planner"}
          </Button>
        </>
      }
    >
      {runMutation.data && (
        <Card className="notice">
          Planner: {runMutation.data.message}
          {runMutation.data.week_start ? ` (week of ${runMutation.data.week_start})` : ""}
        </Card>
      )}
      {runMutation.isError && <ErrorState message={runMutation.error.message} />}

      {!view ? (
        <EmptyState>No meal plan yet. Run the planner to create one.</EmptyState>
      ) : (
        <>
          <div className="plan-meta">
            <Badge tone={statusTone(view.plan.status)}>{view.plan.status}</Badge>
            {view.plan.status === "draft" && (
              <Button
                variant="ghost"
                onClick={() => approveMutation.mutate()}
                disabled={approveMutation.isPending}
              >
                {approveMutation.isPending ? "Approving…" : "Approve plan"}
              </Button>
            )}
            {approveMutation.isError && (
              <ErrorState message={approveMutation.error.message} />
            )}
          </div>

          <SectionTitle>Dinner plan</SectionTitle>
          {viewQuery.isLoading ? (
            <Spinner label="Loading plan" />
          ) : viewQuery.isError ? (
            <ErrorState message={viewQuery.error.message} />
          ) : (
            <div className="days">
              {days.map((slotDate) => {
                const decision = decisionByDate.get(slotDate);
                const candidate = candidatesByDate.get(slotDate);
                const isDecided = Boolean(decision);
                return (
                  <Card key={slotDate} className={`day ${isDecided ? "day-decided" : ""}`}>
                    <span className="day-weekday">{weekdayOf(slotDate)}</span>
                    {isDecided ? (
                      <>
                        <span className="day-title">{recipeTitle(decision!)}</span>
                        <span className="day-subtitle">committed</span>
                      </>
                    ) : (
                      <>
                        <span className="day-title">
                          {candidate ? candidate.recipe.title || candidate.recipe.mealie_recipe_id : "—"}
                        </span>
                        <span className="day-subtitle">
                          {candidate
                            ? `rank ${candidate.rank + 1} · score ${candidate.score.toFixed(1)}`
                            : "no candidate"}
                        </span>
                        {candidate && (
                          <select
                            className="input day-pick"
                            value={picked[slotDate] ?? candidate.recipe.mealie_recipe_id}
                            onChange={(e) =>
                              setPicked((prev) => ({ ...prev, [slotDate]: e.target.value }))
                            }
                          >
                            {(view.candidates
                              .filter((c) => c.slot_date === slotDate)
                              .sort((a, b) => a.rank - b.rank) || [])
                              .map((c) => (
                                <option key={c.id} value={c.recipe.mealie_recipe_id}>
                                  {c.recipe.title || c.recipe.mealie_recipe_id} (rank {c.rank + 1})
                                </option>
                              ))}
                          </select>
                        )}
                      </>
                    )}
                  </Card>
                );
              })}
            </div>
          )}

          {view.plan.status === "approved" && (
            <div className="row">
              <Button
                onClick={() => decideMutation.mutate()}
                disabled={decideMutation.isPending || Object.keys(picked).length === 0}
              >
                {decideMutation.isPending ? "Committing…" : "Commit picks"}
              </Button>
              {decideMutation.isError && <ErrorState message={decideMutation.error.message} />}
            </div>
          )}

          <SectionTitle>Shopping list</SectionTitle>
          {reqQuery.isLoading ? (
            <Spinner label="Loading shopping list" />
          ) : reqQuery.isError ? (
            <ErrorState message={reqQuery.error.message} />
          ) : requirements.length === 0 ? (
            <EmptyState>No shopping requirements yet (approve the plan and commit decisions).</EmptyState>
          ) : (
            <div className="items">
              {requirements.map((req) => (
                <Card key={req.id} className="item">
                  <span className="item-qty">{quantityLabel(req.quantity, req.unit)}</span>
                  <span className="item-name">{req.preferred_form ?? req.ingredient_id}</span>
                  {req.acceptable_forms.length > 1 && (
                    <span className="item-forms">or {req.acceptable_forms.join(", ")}</span>
                  )}
                </Card>
              ))}
            </div>
          )}
        </>
      )}
    </Page>
  );
}
