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

type Person = components["schemas"]["Person"];
type Preference = components["schemas"]["PersonPreference"];

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

function usePreferences(personId?: string) {
  return useQuery({
    queryKey: ["preferences", personId],
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/preferences", {
        params: { query: personId ? { personId } : undefined },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to load preferences (${response.status})`);
      return data;
    },
  });
}

function sentimentTone(s: number): "good" | "warn" | "bad" | "neutral" {
  if (s >= 1) return "good";
  if (s === 0) return "neutral";
  if (s === -1) return "warn";
  return "bad";
}

export default function PreferencesPage() {
  const queryClient = useQueryClient();
  const peopleQuery = usePeople();
  const people: Person[] = peopleQuery.data ?? [];
  const [personId, setPersonId] = useState<string | undefined>(people.at(0)?.id);
  const prefsQuery = usePreferences(personId);
  const prefs: Preference[] = prefsQuery.data ?? [];

  const [tag, setTag] = useState("");
  const [sentiment, setSentiment] = useState(1);
  const [confidence, setConfidence] = useState(0.8);

  const selected = people.find((p) => p.id === personId);
  const [editName, setEditName] = useState<string | null>(null);
  const [editWeight, setEditWeight] = useState<string | null>(null);

  const savePerson = useMutation({
    mutationFn: async () => {
      if (!personId || editName === null) throw new Error("Nothing to save");
      const weight = editWeight === null || editWeight === "" ? 0 : Number(editWeight);
      const { data, error, response } = await apiClient.PATCH("/people/{id}", {
        params: { path: { id: personId } },
        body: { name: editName, weight },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to save person (${response.status})`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["people"] });
      setEditName(null);
      setEditWeight(null);
    },
  });

  const setPref = useMutation({
    mutationFn: async () => {
      if (!personId) throw new Error("Select a person first");
      const { data, error, response } = await apiClient.POST("/preferences", {
        body: { person_id: personId, tag, sentiment, confidence },
      });
      if (error) throw new Error(errorMessage(error));
      if (!response.ok || !data) throw new Error(`Failed to save preference (${response.status})`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["preferences"] });
      setTag("");
    },
  });

  if (peopleQuery.isLoading) return <Spinner label="Loading people" />;
  if (peopleQuery.isError) return <ErrorState message={peopleQuery.error.message} />;

  return (
    <Page
      title="Preferences"
      subtitle="What each person likes and dislikes."
      actions={
        people.length > 0 ? (
          <select className="input" value={personId ?? ""} onChange={(e) => setPersonId(e.target.value)}>
            {people.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        ) : undefined
      }
    >
      {people.length === 0 ? (
        <EmptyState>No household members yet.</EmptyState>
      ) : prefsQuery.isLoading ? (
        <Spinner label="Loading preferences" />
      ) : prefsQuery.isError ? (
        <ErrorState message={prefsQuery.error.message} />
      ) : prefs.length === 0 ? (
        <EmptyState>No learned preferences for this person yet.</EmptyState>
      ) : (
        <div className="items">
          {prefs.map((p, i) => (
            <Card key={i} className="item">
              <span className="item-name">{p.tag}</span>
              <Badge tone={sentimentTone(p.sentiment)}>
                {p.sentiment > 0 ? `+${p.sentiment}` : p.sentiment}
              </Badge>
              <span className="item-forms">
                confidence {Math.round(p.confidence * 100)}% · {formatDate(p.updated_at)}
              </span>
            </Card>
          ))}
        </div>
      )}

      {selected && (
        <>
          <SectionTitle>Edit profile</SectionTitle>
          <div className="row">
            <TextInput
              value={editName ?? selected.name}
              onChange={(e) => setEditName(e.target.value)}
              placeholder="Name"
            />
            <TextInput
              value={editWeight ?? String(selected.weight)}
              onChange={(e) => setEditWeight(e.target.value)}
              placeholder="Weight (e.g. 1.0)"
              type="number"
            />
            <Button
              onClick={() => savePerson.mutate()}
              disabled={
                savePerson.isPending ||
                (editName === null && editWeight === null) ||
                (editName !== null && !editName.trim())
              }
            >
              Save profile
            </Button>
          </div>
          {savePerson.isError && <ErrorState message={savePerson.error.message} />}
        </>
      )}

      <SectionTitle>Edit preferences</SectionTitle>
      {personId ? (
        <div className="row">
          <TextInput
            placeholder="Tag (e.g. spicy, dairy, vegetarian)"
            value={tag}
            onChange={(e) => setTag(e.target.value)}
          />
          <select
            className="input"
            value={sentiment}
            onChange={(e) => setSentiment(Number(e.target.value))}
          >
            <option value={2}>love (+2)</option>
            <option value={1}>like (+1)</option>
            <option value={0}>neutral (0)</option>
            <option value={-1}>dislike (−1)</option>
            <option value={-2}>hate (−2)</option>
          </select>
          <select
            className="input"
            value={confidence}
            onChange={(e) => setConfidence(Number(e.target.value))}
          >
            <option value={0.3}>confidence 30%</option>
            <option value={0.5}>confidence 50%</option>
            <option value={0.7}>confidence 70%</option>
            <option value={0.8}>confidence 80%</option>
            <option value={1}>confidence 100%</option>
          </select>
          <Button
            onClick={() => setPref.mutate()}
            disabled={!tag.trim() || setPref.isPending}
          >
            Save
          </Button>
        </div>
      ) : (
        <EmptyState>Add a household member first to record their preferences.</EmptyState>
      )}
      {setPref.isError && <ErrorState message={setPref.error.message} />}
    </Page>
  );
}
