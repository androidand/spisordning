import { useQuery } from "@tanstack/react-query";
import { apiClient, errorMessage } from "../api/client";
import type { components } from "../generated/spisordning";
import {
  Badge,
  Card,
  EmptyState,
  ErrorState,
  Page,
  SectionTitle,
  Spinner,
} from "../components/ui";
import { expiryLabel, formatDate, quantityLabel } from "../lib/format";

type Dashboard = components["schemas"]["Dashboard"];

function useDashboard() {
  return useQuery({
    queryKey: ["dashboard"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/widgets/dashboard");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load dashboard (${response.status})`);
      return data;
    },
  });
}

export default function DashboardPage() {
  const dashboardQuery = useDashboard();
  const d: Dashboard | undefined = dashboardQuery.data;

  if (dashboardQuery.isLoading) return <Spinner label="Loading dashboard" />;
  if (dashboardQuery.isError) return <ErrorState message={dashboardQuery.error.message} />;

  return (
    <Page
      title="Dashboard"
      subtitle="Tonight's meal, pantry status, and what's expiring — in one place."
    >
      <SectionTitle>Tonight</SectionTitle>
      {d?.tonight ? (
        <Card className="tonight-card">
          <h3>{d.tonight.recipe.title || d.tonight.recipe.mealie_recipe_id}</h3>
          <p className="tonight-reactions">Served {formatDate(d.tonight.served_on)}</p>
        </Card>
      ) : (
        <EmptyState>No meal planned for tonight.</EmptyState>
      )}

      <SectionTitle>Pantry</SectionTitle>
      <div className="dashboard-stats">
        <Card className="stat">
          <span className="stat-value">{d?.pantry.locations ?? 0}</span>
          <span className="stat-label">locations</span>
        </Card>
        <Card className="stat">
          <span className="stat-value">{d?.pantry.lots ?? 0}</span>
          <span className="stat-label">lots in stock</span>
        </Card>
        <Card className="stat">
          <span className="stat-value">{d?.pantry.expiring ?? 0}</span>
          <span className="stat-label">expiring soon</span>
        </Card>
      </div>

      <SectionTitle>Expiring soon</SectionTitle>
      {d && d.expiring.length === 0 ? (
        <EmptyState>Nothing expiring in the next week.</EmptyState>
      ) : (
        <div className="items">
          {(d?.expiring ?? []).map((lot, i) => (
            <Card key={`${lot.ingredient_id}-${i}`} className="item expiring">
              <span className="item-qty">{quantityLabel(lot.quantity, lot.unit)}</span>
              <span className="item-name">{lot.ingredient_id}</span>
              <span className="item-forms">
                {lot.best_before ? expiryLabel(lot.best_before) : "no best-before"}
              </span>
            </Card>
          ))}
        </div>
      )}

      {d && d.pantry.expiring > 0 && (
        <div className="dashboard-hint">
          <Badge tone="neutral">
            {d.pantry.expiring} item{d.pantry.expiring > 1 ? "s" : ""} need attention
          </Badge>
        </div>
      )}
    </Page>
  );
}
