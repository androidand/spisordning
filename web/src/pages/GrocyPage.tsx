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
import { formatDate } from "../lib/format";

type GrocyStatus = components["schemas"]["GrocyStatus"];

function useGrocyStatus() {
  return useQuery({
    queryKey: ["grocy", "status"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/grocy/status");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load grocy status (${response.status})`);
      return data;
    },
  });
}

function useGrocyStock(enabled: boolean) {
  return useQuery({
    queryKey: ["grocy", "stock"],
    enabled,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/grocy/stock");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load grocy stock (${response.status})`);
      return data;
    },
  });
}

function useGrocyShoppingList(enabled: boolean) {
  return useQuery({
    queryKey: ["grocy", "shopping-list"],
    enabled,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/grocy/shopping-list");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load grocy shopping list (${response.status})`);
      return data;
    },
  });
}

export default function GrocyPage() {
  const queryClient = useQueryClient();
  const statusQuery = useGrocyStatus();
  const status: GrocyStatus | undefined = statusQuery.data;
  const connected = !!status?.configured && !!status?.reachable;

  const stockQuery = useGrocyStock(connected);
  const shoppingQuery = useGrocyShoppingList(connected);

  const [note, setNote] = useState("");
  const [amount, setAmount] = useState("1");

  const addShoppingItem = useMutation({
    mutationFn: async () => {
      const { error, response } = await apiClient.POST("/grocy/shopping-list/items", {
        body: { product_id: 0, note, amount: Number(amount) || 1 },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to add shopping item (${response.status})`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["grocy", "shopping-list"] });
      setNote("");
      setAmount("1");
    },
  });

  if (statusQuery.isLoading) return <Spinner label="Checking Grocy" />;
  if (statusQuery.isError) return <ErrorState message={statusQuery.error.message} />;

  if (!status?.configured) {
    return (
      <Page title="Grocy" subtitle="Integrate a self-hosted Grocy inventory.">
        <EmptyState>
          Grocy is not configured. Set <code>GROCY_BASE_URL</code> (and optionally
          <code> GROCY_API_KEY</code>) on the <code>food-brain serve</code> process to connect a
          running Grocy instance.
        </EmptyState>
      </Page>
    );
  }

  if (!status.reachable) {
    return (
      <Page title="Grocy" subtitle="Integrate a self-hosted Grocy inventory.">
        <ErrorState message={`Grocy is configured at ${status.base_url} but is not reachable.`} />
      </Page>
    );
  }

  return (
    <Page
      title="Grocy"
      subtitle={`Connected to Grocy ${status.version ?? ""} at ${status.base_url}.`}
    >
      <SectionTitle>Stock</SectionTitle>
      {stockQuery.isLoading ? (
        <Spinner label="Loading stock" />
      ) : stockQuery.isError ? (
        <ErrorState message={stockQuery.error.message} />
      ) : (stockQuery.data?.length ?? 0) === 0 ? (
        <EmptyState>No stock recorded in Grocy.</EmptyState>
      ) : (
        <div className="items">
          {(stockQuery.data ?? []).map((s) => (
            <Card key={s.id} className="item">
              <span className="item-name">{s.product_name || `Product ${s.product_id}`}</span>
              <Badge tone="neutral">{s.amount} (qu {s.qu_id})</Badge>
              {s.best_before && <span className="item-forms">best before {formatDate(s.best_before)}</span>}
            </Card>
          ))}
        </div>
      )}

      <SectionTitle>Shopping list</SectionTitle>
      {shoppingQuery.isLoading ? (
        <Spinner label="Loading shopping list" />
      ) : shoppingQuery.isError ? (
        <ErrorState message={shoppingQuery.error.message} />
      ) : (shoppingQuery.data?.length ?? 0) === 0 ? (
        <EmptyState>Nothing on the Grocy shopping list.</EmptyState>
      ) : (
        <div className="items">
          {(shoppingQuery.data ?? []).map((it) => (
            <Card key={it.id} className="item">
              <span className="item-name">{it.note || `Product ${it.product_id}`}</span>
              <Badge tone={it.done ? "good" : "neutral"}>{it.done ? "done" : `${it.amount}`}</Badge>
            </Card>
          ))}
        </div>
      )}

      <SectionTitle>Add a free-text item</SectionTitle>
      <div className="row">
        <TextInput placeholder="Item (e.g. napkins)" value={note} onChange={(e) => setNote(e.target.value)} />
        <TextInput
          placeholder="Amount"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          type="number"
        />
        <Button
          onClick={() => addShoppingItem.mutate()}
          disabled={!note.trim() || addShoppingItem.isPending}
        >
          Add
        </Button>
      </div>
      {addShoppingItem.isError && <ErrorState message={addShoppingItem.error.message} />}
    </Page>
  );
}
