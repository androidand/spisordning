import { Card, EmptyState, Page, SectionTitle } from "../components/ui";

export default function SyncPage() {
  return (
    <Page title="Sync" subtitle="External data sources.">
      <SectionTitle>Mealie recipes</SectionTitle>
      <EmptyState>
        Recipe sync from Mealie is a backend job (
        <code>food-brain sync recipes</code>). A trigger/status endpoint is not exposed over
        HTTP yet, so this will light up once one lands.
      </EmptyState>

      <SectionTitle>Retailer offers</SectionTitle>
      <EmptyState>
        Offer sync (
        <code>food-brain sync-offers</code>) is a backend job. No HTTP trigger yet.
      </EmptyState>

      <SectionTitle>Apple Notes checklist</SectionTitle>
      <Card className="notice">
        The Apple Notes checklist bridge is live: <code>POST /shopping-lists/from-checklist</code>{" "}
        ingests a named checklist as a shopping list. It is driven by the Mac-local reader, not
        this UI.
      </Card>
    </Page>
  );
}
