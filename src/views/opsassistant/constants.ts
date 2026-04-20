export type AssistantIntent = "triage" | "remediate" | "verify";

export { ASSISTANT_DRAFT_KEY } from "../../features/opsassistant/constants";

export const intentOptions: Array<{ value: AssistantIntent; label: string }> = [
  { value: "triage", label: "Triage" },
  { value: "remediate", label: "Remediate" },
  { value: "verify", label: "Verify" },
];
