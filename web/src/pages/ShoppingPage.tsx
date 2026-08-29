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
import { quantityLabel } from "../lib/format";

type Item = components["schemas"]["ShoppingListItem"];
type Ingredient = components["schemas"]["Ingredient"];

function useLists() {
  return useQuery({
    queryKey: ["shopping-lists"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/shopping-lists");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load lists (${response.status})`);
      return data;
    },
  });
}

function useItems(listId: number | undefined) {
  return useQuery({
    queryKey: ["shopping-items", listId],
    enabled: listId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/shopping-lists/{listId}/items", {
        params: { path: { listId: listId as number } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load items (${response.status})`);
      return data;
    },
  });
}

function useSearch(q: string) {
  return useQuery({
    queryKey: ["ingredient-search", q],
    enabled: q.trim().length > 1,
    staleTime: 60_000,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/ingredients/search", {
        params: { query: { q } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Search failed (${response.status})`);
      return data;
    },
  });
}

export default function ShoppingPage() {
  const queryClient = useQueryClient();
  const listsQuery = useLists();
  const lists = (listsQuery.data ?? []).filter((l) => l.status === "active");

  const [listId, setListId] = useState<number | null>(null);
  const activeId = listId ?? lists.at(0)?.id ?? null;
  const itemsQuery = useItems(activeId ?? undefined);

  const [newListName, setNewListName] = useState("");
  const [search, setSearch] = useState("");
  const searchQuery = useSearch(search);
  const [addLabel, setAddLabel] = useState("");
  const [addQty, setAddQty] = useState("1");
  const [addUnit, setAddUnit] = useState("pcs");
  const [retailer, setRetailer] = useState("willys");

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["shopping-lists"] });
    queryClient.invalidateQueries({ queryKey: ["shopping-items", activeId] });
  };

  const createList = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/shopping-lists", {
        body: { name: newListName },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to create list (${response.status})`);
      return data;
    },
    onSuccess: (l) => {
      setNewListName("");
      setListId(l.id);
      invalidate();
    },
  });

  const addItem = useMutation({
    mutationFn: async (payload: { label: string; quantity: number; unit: string }) => {
      const { data, error, response } = await apiClient.POST("/shopping-lists/{listId}/items", {
        params: { path: { listId: activeId as number } },
        body: payload,
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to add item (${response.status})`);
      return data;
    },
    onSuccess: () => {
      setAddLabel("");
      setAddQty("1");
      invalidate();
    },
  });

  const toggleItem = useMutation({
    mutationFn: async (item: Item) => {
      const { data, error, response } = await apiClient.PATCH("/shopping-lists/{listId}/items/{itemId}", {
        params: { path: { listId: activeId as number, itemId: item.id } },
        body: { checked: !item.checked },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to toggle item (${response.status})`);
      return data;
    },
    onSuccess: () => invalidate(),
  });

  const deleteItem = useMutation({
    mutationFn: async (item: Item) => {
      const { error, response } = await apiClient.DELETE("/shopping-lists/{listId}/items/{itemId}", {
        params: { path: { listId: activeId as number, itemId: item.id } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to delete item (${response.status})`);
    },
    onSuccess: () => invalidate(),
  });

  const pushList = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/shopping-lists/{listId}/push", {
        params: { path: { listId: activeId as number }, query: { retailer } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to push (${response.status})`);
      return data;
    },
  });

  const items: Item[] = itemsQuery.data ?? [];
  const results: Ingredient[] = searchQuery.data ?? [];

  if (listsQuery.isLoading) return <Spinner label="Loading shopping lists" />;
  if (listsQuery.isError) return <ErrorState message={listsQuery.error.message} />;

  return (
    <Page
      title="Shopping"
      subtitle="Build lists, check items off, and push to a retailer."
      actions={
        lists.length > 0 ? (
          <select className="input" value={activeId ?? ""} onChange={(e) => setListId(Number(e.target.value))}>
            {lists.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        ) : undefined
      }
    >
      <div className="row">
        <TextInput
          placeholder="New list name"
          value={newListName}
          onChange={(e) => setNewListName(e.target.value)}
        />
        <Button
          onClick={() => createList.mutate()}
          disabled={!newListName.trim() || createList.isPending}
        >
          New list
        </Button>
      </div>
      {createList.isError && <ErrorState message={createList.error.message} />}

      {!activeId ? (
        <EmptyState>No active shopping lists. Create one above.</EmptyState>
      ) : (
        <>
          <SectionTitle>Add from search</SectionTitle>
          <div className="row">
            <TextInput
              placeholder="Search ingredients…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {searchQuery.isLoading && <Spinner label="Searching" />}
          {searchQuery.isError && <ErrorState message={searchQuery.error.message} />}
          {results.length > 0 && (
            <div className="results">
              {results.slice(0, 8).map((ing) => (
                <button
                  key={ing.id}
                  className="result-btn"
                  onClick={() => {
                    setAddLabel(ing.display);
                    setSearch("");
                  }}
                >
                  {ing.display}
                </button>
              ))}
            </div>
          )}
          <div className="row add-row">
            <TextInput
              placeholder="Item label"
              value={addLabel}
              onChange={(e) => setAddLabel(e.target.value)}
            />
            <TextInput
              type="number"
              className="input-narrow"
              value={addQty}
              onChange={(e) => setAddQty(e.target.value)}
            />
            <TextInput
              className="input-narrow"
              value={addUnit}
              onChange={(e) => setAddUnit(e.target.value)}
            />
            <Button
              onClick={() =>
                addItem.mutate({
                  label: addLabel,
                  quantity: Number(addQty) || 1,
                  unit: addUnit,
                })
              }
              disabled={!addLabel.trim() || addItem.isPending}
            >
              Add
            </Button>
          </div>
          {addItem.isError && <ErrorState message={addItem.error.message} />}

          <SectionTitle>Items</SectionTitle>
          {itemsQuery.isLoading ? (
            <Spinner label="Loading items" />
          ) : itemsQuery.isError ? (
            <ErrorState message={itemsQuery.error.message} />
          ) : items.length === 0 ? (
            <EmptyState>This list is empty.</EmptyState>
          ) : (
            <div className="items">
              {items.map((item) => (
                <Card key={item.id} className={`item ${item.checked ? "item-checked" : ""}`}>
                  <input
                    type="checkbox"
                    checked={item.checked}
                    onChange={() => toggleItem.mutate(item)}
                  />
                  <span className="item-qty">{quantityLabel(item.quantity, item.unit)}</span>
                  <span className="item-name">{item.label ?? item.ingredient_id ?? "—"}</span>
                  <Button variant="danger" onClick={() => deleteItem.mutate(item)}>
                    Remove
                  </Button>
                </Card>
              ))}
            </div>
          )}

          <SectionTitle>Push to retailer</SectionTitle>
          <div className="row">
            <select className="input" value={retailer} onChange={(e) => setRetailer(e.target.value)}>
              <option value="willys">Willys</option>
              <option value="ica">ICA</option>
            </select>
            <Button onClick={() => pushList.mutate()} disabled={pushList.isPending}>
              {pushList.isPending ? "Pushing…" : "Push list"}
            </Button>
          </div>
          {pushList.data && (
            <Card className="notice">
              Pushed to {pushList.data.retailer} · list {pushList.data.external_list_id}
              {pushList.data.last_push_status ? (
                <Badge tone={pushList.data.last_push_status === "success" ? "good" : "bad"}>
                  {pushList.data.last_push_status}
                </Badge>
              ) : null}
            </Card>
          )}
          {pushList.isError && <ErrorState message={pushList.error.message} />}
        </>
      )}
    </Page>
  );
}
