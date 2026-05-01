/**
 * Shared types for the AgentAPI Chat interface
 */

// =============================================================================
// Core Message Types
// =============================================================================

export interface Message {
  id: number;
  role: string;
  content: string;
}

/**
 * Draft messages are used to optimistically update the UI
 * before the server responds.
 */
export interface DraftMessage extends Omit<Message, "id"> {
  id?: number;
}

export type MessageType = "user" | "raw";

// =============================================================================
// SSE Event Types
// =============================================================================

export interface MessageUpdateEvent {
  id: number;
  role: string;
  message: string;
  time: string;
}

export interface StatusChangeEvent {
  status: string;
  agent_type: string;
}

export interface ErrorEventData {
  message: string;
  level: string;
  time: string;
}

// =============================================================================
// API Error Types
// =============================================================================

export interface APIErrorDetail {
  location: string;
  message: string;
  value: null | string | number | boolean | object;
}

export interface APIErrorModel {
  $schema: string;
  detail: string;
  errors: APIErrorDetail[];
  instance: string;
  status: number;
  title: string;
  type: string;
}

// =============================================================================
// Server Status
// =============================================================================

export type ServerStatus = "stable" | "running" | "offline" | "unknown";

// =============================================================================
// File Upload
// =============================================================================

export interface FileUploadResponse {
  ok: boolean;
  filePath?: string;
}

// =============================================================================
// Agent Types
// =============================================================================

export type AgentType =
  | "claude"
  | "goose"
  | "aider"
  | "gemini"
  | "amp"
  | "codex"
  | "cursor"
  | "cursor-agent"
  | "copilot"
  | "auggie"
  | "amazonq"
  | "opencode"
  | "custom"
  | "unknown";

export type AgentColorDisplayNamePair = {
  displayName: string;
};

export const AgentTypeDisplayNames: Record<
  Exclude<AgentType, "unknown">,
  AgentColorDisplayNamePair
> = {
  claude: { displayName: "Claude Code" },
  goose: { displayName: "Goose" },
  aider: { displayName: "Aider" },
  gemini: { displayName: "Gemini" },
  amp: { displayName: "Amp" },
  codex: { displayName: "Codex" },
  cursor: { displayName: "Cursor Agent" },
  "cursor-agent": { displayName: "Cursor Agent" },
  copilot: { displayName: "Copilot" },
  auggie: { displayName: "Auggie" },
  amazonq: { displayName: "Amazon Q" },
  opencode: { displayName: "Opencode" },
  custom: { displayName: "Custom" },
};

// =============================================================================
// Utility Type Guards
// =============================================================================

export function isDraftMessage(message: Message | DraftMessage): boolean {
  return message.id === undefined;
}
