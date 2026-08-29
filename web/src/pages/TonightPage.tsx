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
  Spinner,
} from "../components/ui";
import { formatDate } from "../lib/format";

type TonightView = components["schemas"]["TonightView"];
type Person = components["schemas"]["Person"];

function useTonight() {
  return useQuery({
    queryKey: ["tonight"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/tonight");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`No meal tonight (${response.status})`);
      return data;
    },
    retry: false,
  });
}

function usePeople() {
  return useQuery({
    queryKey: ["people"],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/people");
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load people (${response.status})`);
      return data;
    },
  });
}

const sentiments = [
  { value: -2, label: "Hated" },
  { value: -1, label: "Disliked" },
  { value: 1, label: "Liked" },
  { value: 2, label: "Loved" },
];

export default function TonightPage() {
  const queryClient = useQueryClient();
  const tonightQuery = useTonight();
  const peopleQuery = usePeople();
  const people: Person[] = peopleQuery.data ?? [];
  const [personId, setPersonId] = useState<string>("");
  const [sentiment, setSentiment] = useState<number>(1);

  const react = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await apiClient.POST("/reactions", {
        body: { person_id: personId, sentiment },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to record reaction (${response.status})`);
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["tonight"] }),
  });

  const view: TonightView | undefined = tonightQuery.data;

  if (tonightQuery.isLoading) return <Spinner label="Loading tonight" />;
  if (tonightQuery.isError) {
    return (
      <Page title="Tonight">
        <EmptyState>No meal planned for tonight.</EmptyState>
      </Page>
    );
  }

  return (
    <Page title="Tonight" subtitle={`Served ${formatDate(view!.served_on)}`}>
      <Card className="tonight-card">
        <h3>{view!.recipe.title || view!.recipe.mealie_recipe_id}</h3>
        {view!.reactions.length > 0 && (
          <p className="tonight-reactions">
            {view!.reactions.length} reaction{view!.reactions.length > 1 ? "s" : ""} so far
          </p>
        )}
      </Card>

      <div className="row">
        <select className="input" value={personId} onChange={(e) => setPersonId(e.target.value)}>
          <option value="">Who?</option>
          {people.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>
      <div className="row">
        {sentiments.map((s) => (
          <Button
            key={s.value}
            variant={sentiment === s.value ? "primary" : "ghost"}
            onClick={() => setSentiment(s.value)}
          >
            {s.label}
          </Button>
        ))}
        <Button
          onClick={() => react.mutate()}
          disabled={!personId || react.isPending}
        >
          {react.isPending ? "Saving…" : "React"}
        </Button>
      </div>
      {react.isError && <ErrorState message={react.error.message} />}
    </Page>
  );
}
