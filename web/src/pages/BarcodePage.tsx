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

type IngredientProduct = components["schemas"]["IngredientProduct"];

function useProductByGtin(gtin: string) {
  return useQuery({
    queryKey: ["product-by-gtin", gtin],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/products/by-gtin", {
        params: { query: { gtin } },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to look up product (${response.status})`);
      return data;
    },
    enabled: gtin.trim() !== "",
  });
}

export default function BarcodePage() {
  const [gtin, setGtin] = useState("");
  const [submitted, setSubmitted] = useState("");
  const [scanError, setScanError] = useState<string | null>(null);

  const resultsQuery = useProductByGtin(submitted);
  const results: IngredientProduct[] = resultsQuery.data ?? [];

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setScanError(null);
    setSubmitted(gtin.trim());
  }

  function scanBarcode() {
    setScanError(null);
    // BarcodeDetector is available in Chromium-based browsers.
    const BD = (window as unknown as { BarcodeDetector?: new (opts?: { formats?: string[] }) => { detect: (source: CanvasImageSource) => Promise<{ rawValue: string }[]> } }).BarcodeDetector;
    if (!BD || !("mediaDevices" in navigator)) {
      setScanError("Barcode scanning is not available in this browser. Enter the GTIN manually.");
      return;
    }
    const detector = new BD({ formats: ["ean_13", "ean_8", "upc_a", "upc_e"] });
    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: "environment" } })
      .then((stream) => {
        const video = document.createElement("video");
        video.srcObject = stream;
        video.play();
        let cancelled = false;
        const tick = () => {
          if (cancelled) return;
          detector
            .detect(video)
            .then((codes) => {
              if (cancelled) return;
              if (codes.length > 0) {
                cancelled = true;
                stream.getTracks().forEach((t) => t.stop());
                setGtin(codes[0].rawValue);
                setSubmitted(codes[0].rawValue);
              } else {
                requestAnimationFrame(tick);
              }
            })
            .catch(() => {
              if (!cancelled) requestAnimationFrame(tick);
            });
        };
        requestAnimationFrame(tick);
      })
      .catch((err: unknown) => {
        setScanError(`Could not access the camera: ${err instanceof Error ? err.message : String(err)}`);
      });
  }

  return (
    <Page
      title="Barcode"
      subtitle="Look up a retail product by its GTIN (barcode) via Matpriskollen."
    >
      <SectionTitle>Scan or enter a GTIN</SectionTitle>
      <form className="row" onSubmit={submit}>
        <TextInput
          placeholder="GTIN (e.g. 7390946115206)"
          value={gtin}
          onChange={(e) => setGtin(e.target.value)}
          inputMode="numeric"
        />
        <Button type="submit" disabled={gtin.trim() === ""}>
          Look up
        </Button>
        <Button type="button" variant="ghost" onClick={scanBarcode}>
          Scan with camera
        </Button>
      </form>
      {scanError && <ErrorState message={scanError} />}

      <SectionTitle>Results</SectionTitle>
      {resultsQuery.isLoading ? (
        <Spinner label="Looking up product" />
      ) : resultsQuery.isError ? (
        <ErrorState message={resultsQuery.error.message} />
      ) : submitted.trim() === "" ? (
        <EmptyState>Enter a GTIN (or scan a barcode) to look up a product.</EmptyState>
      ) : results.length === 0 ? (
        <EmptyState>No product found for GTIN {submitted}.</EmptyState>
      ) : (
        <div className="barcode-results">
          {results.map((p) => (
            <Card key={p.key} className="barcode-result">
              <div className="barcode-head">
                <span className="barcode-name">{p.name}</span>
                {p.brand && <Badge tone="neutral">{p.brand}</Badge>}
              </div>
              {p.amount && <div className="barcode-amount">{p.amount}</div>}
              {p.gtin && <div className="barcode-gtin">GTIN {p.gtin}</div>}
              {p.description && <div className="barcode-desc">{p.description}</div>}
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
