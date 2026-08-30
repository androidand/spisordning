import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient, errorMessage } from "../api/client";
import type { components } from "../generated/spisordning";
import {
  Button,
  Card,
  EmptyState,
  ErrorState,
  Page,
  SectionTitle,
  Spinner,
  TextInput,
} from "../components/ui";
import { daysUntil, expiryLabel, formatDate, quantityLabel } from "../lib/format";

type Location = components["schemas"]["InventoryLocation"];
type Lot = components["schemas"]["InventoryLot"];

function useExpiring() {
  return useQuery({
    queryKey: ["pantry-expiring"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/pantry/expiring");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load expiring lots (${response.status})`);
      return data;
    },
  });
}

function useLocations() {
  return useQuery({
    queryKey: ["pantry-locations"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/pantry/locations");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load locations (${response.status})`);
      return data;
    },
  });
}

function useLots(locationId: string | undefined) {
  return useQuery({
    queryKey: ["pantry-lots", locationId],
    enabled: locationId !== undefined,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/pantry/locations/{id}/lots", {
        params: { path: { id: locationId as string } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load lots (${response.status})`);
      return data;
    },
  });
}

export default function PantryPage() {
  const queryClient = useQueryClient();
  const locationsQuery = useLocations();
  const locations: Location[] = locationsQuery.data ?? [];

  const [locationId, setLocationId] = useState<string | null>(null);
  const activeId = locationId ?? locations.at(0)?.id ?? null;
  const lotsQuery = useLots(activeId ?? undefined);
  const lots: Lot[] = lotsQuery.data ?? [];
  const expiringQuery = useExpiring();
  const expiring: Lot[] = expiringQuery.data ?? [];

  const [newLoc, setNewLoc] = useState("");
  const [pIng, setPIng] = useState("");
  const [pQty, setPQty] = useState("1");
  const [pUnit, setPUnit] = useState("pcs");
  const [actionLot, setActionLot] = useState<Lot | null>(null);
  const [discardQty, setDiscardQty] = useState("");
  const [adjustQty, setAdjustQty] = useState("");
  const [transferLocation, setTransferLocation] = useState("");
  const [transferQty, setTransferQty] = useState("");

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["pantry-locations"] });
    queryClient.invalidateQueries({ queryKey: ["pantry-lots", activeId] });
    queryClient.invalidateQueries({ queryKey: ["pantry-expiring"] });
  };

  const closeActions = () => {
    setActionLot(null);
    setDiscardQty("");
    setAdjustQty("");
    setTransferLocation("");
    setTransferQty("");
  };

  const createLocation = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/pantry/locations", {
        body: { name: newLoc, household_id: "", location_type: "pantry", parent_location_id: "" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to create location (${response.status})`);
      return data;
    },
    onSuccess: (l) => {
      setNewLoc("");
      setLocationId(l.id);
      invalidate();
    },
  });

  const purchase = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/pantry/lots/purchase", {
        body: {
          ingredient_id: pIng,
          product_id: "",
          location_id: activeId as string,
          quantity: Number(pQty) || 1,
          unit: pUnit,
          source: "manual",
        },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to record purchase (${response.status})`);
      return data;
    },
    onSuccess: () => {
      setPIng("");
      setPQty("1");
      invalidate();
    },
  });

  const consume = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/consume", {
        params: { path: { id: lot.id } },
        body: { quantity: lot.quantity, estimated: false, source: "manual" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to consume (${response.status})`);
    },
    onSuccess: () => invalidate(),
  });

  const discard = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/discard", {
        params: { path: { id: lot.id } },
        body: { quantity: Number(discardQty) || 0, estimated: false, source: "manual" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to discard (${response.status})`);
    },
    onSuccess: () => {
      closeActions();
      invalidate();
    },
  });

  const adjust = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/adjust", {
        params: { path: { id: lot.id } },
        body: { quantity: Number(adjustQty) || 0, estimated: false, source: "manual" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to adjust (${response.status})`);
    },
    onSuccess: () => {
      closeActions();
      invalidate();
    },
  });

  const markEmpty = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/mark-empty", {
        params: { path: { id: lot.id } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to mark empty (${response.status})`);
    },
    onSuccess: () => {
      closeActions();
      invalidate();
    },
  });

  const openLot = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/open", {
        params: { path: { id: lot.id } },
        body: { source: "manual" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to open lot (${response.status})`);
    },
    onSuccess: () => {
      closeActions();
      invalidate();
    },
  });

  const transfer = useMutation({
    mutationFn: async (lot: Lot) => {
      const { error, response } = await apiClient.POST("/pantry/lots/{id}/transfer", {
        params: { path: { id: lot.id } },
        body: { location_id: transferLocation, quantity: Number(transferQty) || 0, source: "manual" },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok) throw new Error(`Failed to transfer (${response.status})`);
    },
    onSuccess: () => {
      closeActions();
      invalidate();
    },
  });

  const actionError =
    discard.error?.message ?? adjust.error?.message ?? markEmpty.error?.message ?? openLot.error?.message ?? transfer.error?.message;

  if (locationsQuery.isLoading) return <Spinner label="Loading pantry" />;
  if (locationsQuery.isError) return <ErrorState message={locationsQuery.error.message} />;

  return (
    <Page
      title="Pantry"
      subtitle="What you have on hand, by location."
      actions={
        locations.length > 0 ? (
          <select className="input" value={activeId ?? ""} onChange={(e) => setLocationId(e.target.value)}>
            {locations.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        ) : undefined
      }
    >
      <SectionTitle>Best before</SectionTitle>
      {expiringQuery.isLoading ? (
        <Spinner label="Checking expiring items" />
      ) : expiringQuery.isError ? (
        <ErrorState message={expiringQuery.error.message} />
      ) : expiring.length === 0 ? (
        <EmptyState>Nothing expiring soon.</EmptyState>
      ) : (
        <div className="items">
          {expiring.map((lot) => {
            const days = daysUntil(lot.best_before);
            const expired = days !== null && days < 0;
            return (
              <Card key={`exp-${lot.id}`} className={`item ${expired ? "expired" : "expiring"}`}>
                <span className="item-qty">{quantityLabel(lot.quantity, lot.unit)}</span>
                <span className="item-name">{lot.ingredient_id}</span>
                <span className="item-forms">{expiryLabel(lot.best_before)}</span>
              </Card>
            );
          })}
        </div>
      )}

      <div className="row">
        <TextInput placeholder="New location name" value={newLoc} onChange={(e) => setNewLoc(e.target.value)} />
        <Button onClick={() => createLocation.mutate()} disabled={!newLoc.trim() || createLocation.isPending}>
          Add location
        </Button>
      </div>
      {createLocation.isError && <ErrorState message={createLocation.error.message} />}

      {!activeId ? (
        <EmptyState>No pantry locations yet. Add one above.</EmptyState>
      ) : (
        <>
          <SectionTitle>Record a purchase</SectionTitle>
          <div className="row add-row">
            <TextInput placeholder="Ingredient id" value={pIng} onChange={(e) => setPIng(e.target.value)} />
            <TextInput type="number" className="input-narrow" value={pQty} onChange={(e) => setPQty(e.target.value)} />
            <TextInput className="input-narrow" value={pUnit} onChange={(e) => setPUnit(e.target.value)} />
            <Button onClick={() => purchase.mutate()} disabled={!pIng.trim() || purchase.isPending}>
              Purchase
            </Button>
          </div>
          {purchase.isError && <ErrorState message={purchase.error.message} />}

          <SectionTitle>In stock</SectionTitle>
          {lotsQuery.isLoading ? (
            <Spinner label="Loading stock" />
          ) : lotsQuery.isError ? (
            <ErrorState message={lotsQuery.error.message} />
          ) : lots.length === 0 ? (
            <EmptyState>Nothing in stock here.</EmptyState>
          ) : (
            <div className="items">
              {lots.map((lot) => {
                const managing = actionLot?.id === lot.id;
                return (
                  <Card key={lot.id} className="item">
                    <span className="item-qty">{quantityLabel(lot.quantity, lot.unit)}</span>
                    <span className="item-name">{lot.ingredient_id}</span>
                    <span className="item-forms">
                      {lot.best_before ? `best before ${formatDate(lot.best_before)}` : "no expiry"}
                    </span>
                    <Button variant="danger" onClick={() => consume.mutate(lot)} disabled={consume.isPending || lot.quantity <= 0}>
                      Consume
                    </Button>
                    <Button onClick={() => (managing ? closeActions() : setActionLot(lot))}>
                      {managing ? "Close" : "Manage"}
                    </Button>
                    {managing && (
                      <div className="lot-actions">
                        <div className="row">
                          <TextInput
                            type="number"
                            className="input-narrow"
                            placeholder="Discard"
                            value={discardQty}
                            onChange={(e) => setDiscardQty(e.target.value)}
                          />
                          <Button
                            variant="danger"
                            onClick={() => discard.mutate(lot)}
                            disabled={discard.isPending || !(Number(discardQty) > 0) || Number(discardQty) > lot.quantity}
                          >
                            Discard
                          </Button>
                        </div>
                        <div className="row">
                          <TextInput
                            type="number"
                            className="input-narrow"
                            placeholder="New quantity"
                            value={adjustQty}
                            onChange={(e) => setAdjustQty(e.target.value)}
                          />
                          <Button onClick={() => adjust.mutate(lot)} disabled={adjust.isPending || adjustQty.trim() === ""}>
                            Adjust
                          </Button>
                        </div>
                        <div className="row">
                          <select
                            className="input"
                            value={transferLocation}
                            onChange={(e) => setTransferLocation(e.target.value)}
                          >
                            <option value="">Move to…</option>
                            {locations
                              .filter((l) => l.id !== lot.location_id)
                              .map((l) => (
                                <option key={l.id} value={l.id}>
                                  {l.name}
                                </option>
                              ))}
                          </select>
                          <TextInput
                            type="number"
                            className="input-narrow"
                            placeholder="Qty"
                            value={transferQty}
                            onChange={(e) => setTransferQty(e.target.value)}
                          />
                          <Button
                            onClick={() => transfer.mutate(lot)}
                            disabled={
                              transfer.isPending ||
                              !transferLocation ||
                              !(Number(transferQty) > 0) ||
                              Number(transferQty) > lot.quantity
                            }
                          >
                            Transfer
                          </Button>
                        </div>
                        <div className="row">
                          <Button onClick={() => openLot.mutate(lot)} disabled={openLot.isPending || !!lot.opened_at}>
                            Open
                          </Button>
                          <Button
                            variant="danger"
                            onClick={() => markEmpty.mutate(lot)}
                            disabled={markEmpty.isPending || lot.quantity <= 0}
                          >
                            Mark empty
                          </Button>
                        </div>
                        {actionError && <ErrorState message={actionError} />}
                      </div>
                    )}
                  </Card>
                );
              })}
            </div>
          )}
        </>
      )}
    </Page>
  );
}
