import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
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

type Store = components["schemas"]["Store"];

function useStores(latitude?: number, longitude?: number) {
  return useQuery({
    queryKey: ["stores", latitude ?? null, longitude ?? null],
    queryFn: async () => {
      const params: { latitude?: number; longitude?: number } = {};
      if (latitude !== undefined) params.latitude = latitude;
      if (longitude !== undefined) params.longitude = longitude;
      const { data, error, response } = await apiClient.GET("/stores", {
        params: { query: params },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load stores (${response.status})`);
      return data;
    },
    enabled: latitude !== undefined || longitude !== undefined,
  });
}

function distanceLabel(km: number | null | undefined): string {
  if (km === null || km === undefined) return "no position";
  if (km < 1) return `${Math.round(km * 1000)} m`;
  return `${km.toFixed(1)} km`;
}

export default function StoreLocatorPage() {
  const [lat, setLat] = useState("");
  const [lon, setLon] = useState("");
  const [geoError, setGeoError] = useState<string | null>(null);

  const hasOrigin = lat.trim() !== "" && lon.trim() !== "";
  const parsedLat = hasOrigin ? Number(lat) : undefined;
  const parsedLon = hasOrigin ? Number(lon) : undefined;

  const storesQuery = useStores(parsedLat, parsedLon);
  const stores: Store[] = storesQuery.data ?? [];

  function useMyLocation() {
    setGeoError(null);
    if (!("geolocation" in navigator)) {
      setGeoError("Geolocation is not available in this browser.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(pos.coords.latitude.toFixed(6));
        setLon(pos.coords.longitude.toFixed(6));
      },
      (err) => setGeoError(`Could not get your location: ${err.message}`),
    );
  }

  return (
    <Page
      title="Store locator"
      subtitle="Every store with its position, ranked nearest-first when you supply an origin."
    >
      <SectionTitle>Origin</SectionTitle>
      <div className="row">
        <TextInput
          placeholder="Latitude (e.g. 59.3293)"
          value={lat}
          onChange={(e) => setLat(e.target.value)}
        />
        <TextInput
          placeholder="Longitude (e.g. 18.0686)"
          value={lon}
          onChange={(e) => setLon(e.target.value)}
        />
        <Button variant="ghost" onClick={useMyLocation}>
          Use my location
        </Button>
      </div>
      {geoError && <ErrorState message={geoError} />}

      <SectionTitle>Stores</SectionTitle>
      {storesQuery.isLoading ? (
        <Spinner label="Locating stores" />
      ) : storesQuery.isError ? (
        <ErrorState message={storesQuery.error.message} />
      ) : stores.length === 0 ? (
        <EmptyState>
          {hasOrigin
            ? "No stores found."
            : "Enter an origin (or use your location) to rank stores by distance."}
        </EmptyState>
      ) : (
        <div className="store-list">
          {stores.map((s) => (
            <Card key={s.id} className="store-row">
              <div className="store-main">
                <span className="store-name">{s.name || s.id}</span>
                <Badge tone="neutral">{s.retailer_name || s.retailer_id}</Badge>
              </div>
              <span className="store-distance">
                {distanceLabel(s.distance_km)}
              </span>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
