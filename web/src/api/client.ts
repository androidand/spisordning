import createClient from "openapi-fetch";
import type { paths } from "../generated/spisordning";

const baseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const apiClient = createClient<paths>({ baseUrl });

export function errorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message.trim()) return message;
  }
  return "Request failed";
}
