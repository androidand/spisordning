import { useState } from "react";
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
import { formatDate, formatPrice } from "../lib/format";

type Order = components["schemas"]["Order"];
type OrderView = components["schemas"]["OrderView"];

function useOrders(retailer?: string) {
  return useQuery({
    queryKey: ["orders", retailer],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/orders", {
        params: { query: retailer ? { retailer } : undefined },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load orders (${response.status})`);
      return data;
    },
  });
}

function useOrderDetail(orderId: number | undefined) {
  return useQuery({
    queryKey: ["order", orderId],
    enabled: orderId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/orders/{orderId}", {
        params: { path: { orderId: orderId as number } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load order (${response.status})`);
      return data;
    },
  });
}

export default function OrdersPage() {
  const ordersQuery = useOrders();
  const orders: Order[] = ordersQuery.data ?? [];
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const detailQuery = useOrderDetail(selectedId ?? undefined);
  const detail: OrderView | undefined = detailQuery.data;

  if (ordersQuery.isLoading) return <Spinner label="Loading orders" />;
  if (ordersQuery.isError) return <ErrorState message={ordersQuery.error.message} />;

  return (
    <Page title="Orders" subtitle="Orders placed with retailers.">
      {orders.length === 0 ? (
        <EmptyState>No orders yet.</EmptyState>
      ) : (
        <div className="items">
          {orders.map((o) => (
            <Card key={o.id} className="item">
              <span className="item-qty">#{o.id}</span>
              <span className="item-name">{formatDate(o.ordered_at)}</span>
              <Badge tone="neutral">{o.retailer}</Badge>
              <span className="item-forms">{formatPrice(o.total_price)}</span>
              <button className="link-btn" onClick={() => setSelectedId(o.id)}>
                view
              </button>
            </Card>
          ))}
        </div>
      )}

      {detailQuery.isLoading && <Spinner label="Loading order" />}
      {detailQuery.isError && <ErrorState message={detailQuery.error.message} />}
      {detail && (
        <div className="items order-detail">
          {detail.items.map((it) => (
            <Card key={it.id} className="item">
              <span className="item-name">{it.retailer_product_id}</span>
              <span className="item-qty">×{it.quantity}</span>
              <span className="item-forms">{formatPrice(it.total_price)}</span>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
