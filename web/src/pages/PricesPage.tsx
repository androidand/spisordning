import { useQuery } from "@tanstack/react-query";
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
import { formatPrice } from "../lib/format";

type ProductPriceGroup = components["schemas"]["ProductPriceGroup"];
type StorePrice = components["schemas"]["StorePrice"];

function usePrices() {
  return useQuery({
    queryKey: ["prices"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/prices");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load prices (${response.status})`);
      return data;
    },
  });
}

function priceTone(sp: StorePrice, cheapest: StorePrice | null | undefined): "good" | "neutral" {
  if (cheapest && sp.store_id === cheapest.store_id && sp.price_kind === cheapest.price_kind) {
    return "good";
  }
  return "neutral";
}

export default function PricesPage() {
  const pricesQuery = usePrices();
  const groups: ProductPriceGroup[] = pricesQuery.data ?? [];

  if (pricesQuery.isLoading) return <Spinner label="Loading prices" />;
  if (pricesQuery.isError) return <ErrorState message={pricesQuery.error.message} />;

  return (
    <Page
      title="Prices"
      subtitle="Current price per product across stores, with the cheapest store highlighted."
    >
      {groups.length === 0 ? (
        <EmptyState>
          No price observations yet. Prices appear here once the price-intelligence sync has
          recorded store offers and observations.
        </EmptyState>
      ) : (
        <div className="price-list">
          {groups.map((g) => (
            <Card key={g.retailer_product_id} className="price-group">
              <div className="price-head">
                <span className="price-name">{g.display_name || g.retailer_product_id}</span>
                <Badge tone="neutral">{g.retailer_name || g.retailer_id}</Badge>
                {g.cheapest && (
                  <Badge tone="good">
                    cheapest: {g.cheapest.store_name || g.cheapest.store_id} @{" "}
                    {formatPrice(g.cheapest.price)}
                  </Badge>
                )}
              </div>
              <div className="price-rows">
                {g.prices.map((sp, i) => (
                  <div
                    key={`${sp.store_id}-${sp.price_kind}-${i}`}
                    className={`price-row ${priceTone(sp, g.cheapest) === "good" ? "cheapest" : ""}`}
                  >
                    <span className="price-store">{sp.store_name || sp.store_id}</span>
                    <span className="price-kind">{sp.price_kind}</span>
                    <span className="price-value">{formatPrice(sp.price)}</span>
                    <span className="price-source">{sp.source}</span>
                  </div>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
