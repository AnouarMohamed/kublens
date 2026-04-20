export interface AssistantDocRef {
  title: string;
  url: string;
  source: string;
  snippet?: string;
}

export interface AssistantMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: string;
  query?: string;
  hints?: string[];
  resources?: string[];
  references?: AssistantDocRef[];
  isError?: boolean;
}

export interface ChatSession {
  id: string;
  title: string;
  startedAt: string;
  messages: AssistantMessage[];
}
