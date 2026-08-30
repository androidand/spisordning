import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  discoverRecipe,
  getImportCandidate,
  listImportCandidates,
  promoteImportCandidate,
  rejectImportCandidate,
  type ImportCandidate,
} from "../api/client";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorState,
  Field,
  Page,
  SectionTitle,
  Select,
  Spinner,
  TextInput,
} from "../components/ui";
import { formatDate } from "../lib/format";

const STATUS_FILTERS = ["all", "candidate", "promoted", "rejected"] as const;
type StatusFilter = (typeof STATUS_FILTERS)[number];

function statusTone(status: ImportCandidate["status"]): "warn" | "good" | "bad" {
  if (status === "promoted") return "good";
  if (status === "rejected") return "bad";
  return "warn";
}

function useCandidates(status: StatusFilter) {
  return useQuery({
    queryKey: ["discovery", "candidates", status],
    queryFn: () => listImportCandidates(status === "all" ? undefined : status),
  });
}

/** Provenance + metadata for a staged candidate. */
function Provenance({ c }: { c: ImportCandidate }) {
  const rows: [string, string | undefined | null][] = [
    ["Source", c.source_id],
    ["External id", c.external_id],
    ["Category", c.category],
    ["Cuisine", c.cuisine],
    ["Servings", c.servings != null ? String(c.servings) : null],
    ["Total time", c.total_time_sec != null ? `${c.total_time_sec}s` : null],
    ["License", c.license_note],
    ["Attribution", c.attribution],
  ];
  return (
    <dl className="discovery-provenance">
      {rows.map(([label, value]) =>
        value ? (
          <div key={label} className="discovery-prov-row">
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ) : null,
      )}
      <div className="discovery-prov-row">
        <dt>Source URL</dt>
        <dd>
          <a href={c.source_url} target="_blank" rel="noreferrer">
            {c.source_url}
          </a>
        </dd>
      </div>
    </dl>
  );
}

/** Ingredient lines with needs-review flags. */
function Ingredients({ c }: { c: ImportCandidate }) {
  if (c.ingredients.length === 0) return <EmptyState>No ingredients were parsed.</EmptyState>;
  return (
    <ul className="discovery-ingredients">
      {c.ingredients.map((ing) => (
        <li key={ing.line_no} className={ing.needs_review ? "needs-review" : undefined}>
          <span className="discovery-ing-text">{ing.raw_text}</span>
          {ing.needs_review && <Badge tone="warn">needs review</Badge>}
        </li>
      ))}
    </ul>
  );
}

/** The detail panel for the selected candidate, with reject/promote actions. */
function CandidateDetail({ candidateId }: { candidateId: string }) {
  const queryClient = useQueryClient();
  const detail = useQuery({
    queryKey: ["discovery", "candidate", candidateId],
    queryFn: () => getImportCandidate(candidateId),
  });

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["discovery"] });

  const reject = useMutation({
    mutationFn: () => rejectImportCandidate(candidateId),
    onSuccess: invalidate,
  });
  const promote = useMutation({
    mutationFn: () => promoteImportCandidate(candidateId),
    onSuccess: invalidate,
  });

  if (detail.isLoading) return <Spinner label="Loading candidate" />;
  if (detail.isError) return <ErrorState message={detail.error.message} />;
  const c = detail.data;
  if (!c) return <EmptyState>Candidate not found.</EmptyState>;

  return (
    <Card className="discovery-detail">
      <div className="discovery-detail-head">
        <span className="recipe-title">{c.title}</span>
        <Badge tone={statusTone(c.status)}>{c.status}</Badge>
      </div>
      {c.description && <p className="discovery-description">{c.description}</p>}
      <Provenance c={c} />
      <SectionTitle>Ingredients</SectionTitle>
      <Ingredients c={c} />
      {c.status === "candidate" && (
        <div className="recipe-actions">
          <Button variant="danger" onClick={() => reject.mutate()} disabled={reject.isPending}>
            Reject
          </Button>
          <Button variant="primary" onClick={() => promote.mutate()} disabled={promote.isPending}>
            Promote to recipe family
          </Button>
        </div>
      )}
      {c.status === "promoted" && c.promoted_variant_id && (
        <p className="state">
          Promoted as variant{" "}
          <a href={`#/recipe-families?variant=${c.promoted_variant_id}`}>{c.promoted_variant_id}</a>
        </p>
      )}
      {(reject.isError || promote.isError) && (
        <ErrorState message={reject.error?.message ?? promote.error?.message ?? ""} />
      )}
    </Card>
  );
}

export default function RecipeDiscoveryPage() {
  const [url, setUrl] = useState("");
  const [sourceId, setSourceId] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const candidatesQuery = useCandidates(statusFilter);
  const candidates: ImportCandidate[] = useMemo(
    () => candidatesQuery.data ?? [],
    [candidatesQuery.data],
  );

  const discover = useMutation({
    mutationFn: () => discoverRecipe(url.trim(), sourceId.trim() || undefined),
    onSuccess: (c) => {
      setUrl("");
      setSourceId("");
      setSelectedId(c.id);
    },
  });

  return (
    <Page
      title="Recipe discovery"
      subtitle="Fetch an external recipe URL, stage it as a candidate, then review and promote it."
    >
      <Card className="discovery-form">
        <form
          className="discovery-form-row"
          onSubmit={(e) => {
            e.preventDefault();
            if (url.trim()) discover.mutate();
          }}
        >
          <Field label="Recipe URL">
            <TextInput
              placeholder="https://example.com/recipe"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </Field>
          <Field label="Source (optional)">
            <TextInput
              placeholder="web-jsonld"
              value={sourceId}
              onChange={(e) => setSourceId(e.target.value)}
            />
          </Field>
          <div className="discovery-form-actions">
            <Button type="submit" variant="primary" disabled={discover.isPending || !url.trim()}>
              {discover.isPending ? "Discovering…" : "Discover"}
            </Button>
          </div>
        </form>
        {discover.isError && <ErrorState message={discover.error.message} />}
      </Card>

      <div className="discovery-toolbar">
        <Field label="Status">
          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
          >
            {STATUS_FILTERS.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </Select>
        </Field>
      </div>

      {candidatesQuery.isLoading ? (
        <Spinner label="Loading candidates" />
      ) : candidatesQuery.isError ? (
        <ErrorState message={candidatesQuery.error.message} />
      ) : candidates.length === 0 ? (
        <EmptyState>No candidates yet. Paste a recipe URL above to stage one.</EmptyState>
      ) : (
        <div className="discovery-list">
          {candidates.map((c) => (
            <Card
              key={c.id}
              className={`discovery-item ${selectedId === c.id ? "selected" : ""}`}
            >
              <button
                type="button"
                className="discovery-item-btn"
                onClick={() => setSelectedId(c.id)}
              >
                <div className="discovery-item-head">
                  <span className="recipe-title">{c.title}</span>
                  <Badge tone={statusTone(c.status)}>{c.status}</Badge>
                </div>
                <div className="discovery-item-meta">
                  <span>{c.source_id}</span>
                  <span>imported {formatDate(c.imported_at)}</span>
                  {c.ingredients.some((i) => i.needs_review) && (
                    <Badge tone="warn">needs review</Badge>
                  )}
                </div>
              </button>
            </Card>
          ))}
        </div>
      )}

      {selectedId && (
        <>
          <SectionTitle>Review</SectionTitle>
          <CandidateDetail candidateId={selectedId} />
        </>
      )}
    </Page>
  );
}
