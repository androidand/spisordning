import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
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
import { formatPrice, quantityLabel } from "../lib/format";

type Requirement = components["schemas"]["ShoppingRequirement"];
type CompareReq = components["schemas"]["CompareRequirement"];
type ItemComparison = components["schemas"]["ItemComparison"];

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

function retailerLabel(r: string): string {
  return r === "willys" ? "Willys" : r === "ica" ? "ICA" : r;
}

export default function ComparePage() {
  const plansQuery = usePlans();
  const plans = plansQuery.data ?? [];
  const [planId, setPlanId] = useState<number | null>(null);
  const activePlanId = planId ?? plans.find((p) => p.status === "approved")?.id ?? plans.at(-1)?.id ?? null;
  const reqQuery = useRequirements(activePlanId ?? undefined);
  const requirements: Requirement[] = reqQuery.data ?? [];

  const [manual, setManual] = useState<CompareReq[]>([]);
  const [mIng, setMIng] = useState("");
  const [mQty, setMQty] = useState("1");
  const [mUnit, setMUnit] = useState("pcs");

  const source: CompareReq[] = useMemo(() => {
    if (requirements.length > 0) {
      return requirements.map((r) => ({
        ingredient: r.preferred_form ?? r.ingredient_id,
        quantity: r.quantity,
        unit: r.unit,
        acceptable_forms: r.acceptable_forms,
        preferred_form: r.preferred_form ?? undefined,
      }));
    }
    return manual;
  }, [requirements, manual]);

  const compareMutation = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/compare", {
        body: { requirements: source },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Comparison failed (${response.status})`);
      return data;
    },
  });

  const items: ItemComparison[] = compareMutation.data?.items ?? [];
  const totals = useMemo(() => {
    const t: Record<string, number> = {};
    for (const item of items) {
      for (const r of item.results) {
        if (r.available && r.price_value !== null && r.price_value !== undefined) {
          t[r.retailer] = (t[r.retailer] ?? 0) + r.price_value;
        }
      }
    }
    return t;
  }, [items]);
  const cheapestStore = Object.entries(totals).sort((a, b) => a[1] - b[1])[0]?.[0];

  return (
    <Page
      title="Compare prices"
      subtitle="Cheapest grocery bag across Willys and ICA."
      actions={
        <>
          {plans.length > 0 && (
            <select className="input" value={activePlanId ?? ""} onChange={(e) => setPlanId(Number(e.target.value))}>
              {plans.map((p) => (
                <option key={p.id} value={p.id}>
                  Week of {p.week_start}
                </option>
              ))}
            </select>
          )}
          <Button
            onClick={() => compareMutation.mutate()}
            disabled={compareMutation.isPending || source.length === 0}
          >
            {compareMutation.isPending ? "Comparing…" : "Compare"}
          </Button>
        </>
      }
    >
      {reqQuery.isLoading && <Spinner label="Loading requirements" />}
      {reqQuery.isError && <ErrorState message={reqQuery.error.message} />}

      {requirements.length === 0 && (
        <>
          <SectionTitle>Manual items</SectionTitle>
          <div className="row add-row">
            <TextInput placeholder="Ingredient" value={mIng} onChange={(e) => setMIng(e.target.value)} />
            <TextInput type="number" className="input-narrow" value={mQty} onChange={(e) => setMQty(e.target.value)} />
            <TextInput className="input-narrow" value={mUnit} onChange={(e) => setMUnit(e.target.value)} />
            <Button
              onClick={() => {
                if (!mIng.trim()) return;
                setManual((prev) => [
                  ...prev,
                  { ingredient: mIng.trim(), quantity: Number(mQty) || 1, unit: mUnit },
                ]);
                setMIng("");
                setMQty("1");
              }}
            >
              Add
            </Button>
          </div>
          {manual.length > 0 && (
            <div className="items">
              {manual.map((m, i) => (
                <Card key={i} className="item">
                  <span className="item-qty">{quantityLabel(m.quantity, m.unit)}</span>
                  <span className="item-name">{m.ingredient}</span>
                  <Button variant="danger" onClick={() => setManual((prev) => prev.filter((_, j) => j !== i))}>
                    Remove
                  </Button>
                </Card>
              ))}
            </div>
          )}
        </>
      )}

      {compareMutation.isError && <ErrorState message={compareMutation.error.message} />}

      {items.length > 0 && (
        <>
          <SectionTitle>Per-item comparison</SectionTitle>
          <div className="compare-table">
            {items.map((item, i) => (
              <Card key={i} className="compare-item">
                <div className="compare-head">
                  <span className="item-name">{item.ingredient}</span>
                  {item.unresolved ? (
                    <Badge tone="warn">unresolved</Badge>
                  ) : item.cheapest ? (
                    <Badge tone="good">
                      cheapest: {retailerLabel(item.cheapest.retailer)} {formatPrice(item.cheapest.price_value)}
                    </Badge>
                  ) : null}
                </div>
                <div className="compare-results">
                  {item.results.map((r) => (
                    <div key={r.retailer} className={`compare-cell ${r.available ? "" : "compare-unavailable"}`}>
                      <span className="compare-retailer">{retailerLabel(r.retailer)}</span>
                      {r.available ? (
                        <>
                          <span className="compare-product">{r.product_name ?? r.product_id ?? "product"}</span>
                          <span className="compare-price">{formatPrice(r.price_value)}</span>
                        </>
                      ) : (
                        <span className="compare-error">{r.error ?? "unavailable"}</span>
                      )}
                    </div>
                  ))}
                </div>
              </Card>
            ))}
          </div>

          <SectionTitle>Bag totals</SectionTitle>
          <div className="totals">
            {Object.entries(totals).map(([store, total]) => (
              <Card key={store} className={`total ${store === cheapestStore ? "total-cheapest" : ""}`}>
                <span className="total-store">{retailerLabel(store)}</span>
                <span className="total-price">{formatPrice(total)}</span>
                {store === cheapestStore && <Badge tone="good">cheapest</Badge>}
              </Card>
            ))}
          </div>
        </>
      )}

      {items.length === 0 && !compareMutation.isPending && source.length === 0 && (
        <EmptyState>
          Select a plan with shopping requirements, or add manual items, then compare.
        </EmptyState>
      )}
    </Page>
  );
}
