import createClient from "openapi-fetch";
import type { components, paths } from "../generated/spisordning";

const baseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const apiClient = createClient<paths>({ baseUrl });

export function errorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message.trim()) return message;
  }
  return "Request failed";
}

// ── Recipe discovery ─────────────────────────────────────────────────────────

export type ImportCandidate = components["schemas"]["ImportCandidate"];
export type PromoteCandidateResponse = components["schemas"]["PromoteCandidateResponse"];

/** Fetch an external recipe URL and stage it as a review candidate. */
export async function discoverRecipe(url: string, sourceId?: string): Promise<ImportCandidate> {
  const { data, error, response } = await apiClient.POST("/recipes/discover", {
    body: sourceId ? { url, source_id: sourceId } : { url },
  });
  if (error) throw new Error(errorMessage(error));
  if (!response.ok || !data) throw new Error(`Failed to discover recipe (${response.status})`);
  return data;
}

/** List staged recipe import candidates, optionally filtered by status. */
export async function listImportCandidates(status?: string): Promise<ImportCandidate[]> {
  const { data, error, response } = await apiClient.GET("/recipes/discovery/candidates", {
    params: status ? { query: { status } } : undefined,
  });
  if (error) throw new Error(errorMessage(error));
  if (!response.ok || !data) throw new Error(`Failed to list candidates (${response.status})`);
  return data;
}

/** Fetch one staged recipe import candidate by id. */
export async function getImportCandidate(id: string): Promise<ImportCandidate> {
  const { data, error, response } = await apiClient.GET("/recipes/discovery/candidates/{id}", {
    params: { path: { id } },
  });
  if (error) throw new Error(errorMessage(error));
  if (!response.ok || !data) throw new Error(`Failed to load candidate (${response.status})`);
  return data;
}

/** Reject a staged recipe import candidate that has not been promoted. */
export async function rejectImportCandidate(id: string): Promise<void> {
  const { error, response } = await apiClient.POST("/recipes/discovery/candidates/{id}/reject", {
    params: { path: { id } },
  });
  if (error) throw new Error(errorMessage(error));
  if (!response.ok) throw new Error(`Failed to reject candidate (${response.status})`);
}

/** Promote a staged candidate into the native recipe_family hierarchy. */
export async function promoteImportCandidate(id: string, familyId?: string): Promise<PromoteCandidateResponse> {
  const { data, error, response } = await apiClient.POST("/recipes/discovery/candidates/{id}/promote", {
    params: { path: { id } },
    body: familyId ? { family_id: familyId } : {},
  });
  if (error) throw new Error(errorMessage(error));
  if (!response.ok || !data) throw new Error(`Failed to promote candidate (${response.status})`);
  return data;
}
