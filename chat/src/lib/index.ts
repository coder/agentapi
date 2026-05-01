/**
 * AgentAPI Chat Library
 *
 * A collection of utilities and shared types for the chat interface.
 */

// Types
export {
  type Message,
  type DraftMessage,
  type MessageType,
  type MessageUpdateEvent,
  type StatusChangeEvent,
  type ErrorEventData,
  type APIErrorDetail,
  type APIErrorModel,
  type ServerStatus,
  type FileUploadResponse,
  type AgentType,
  type AgentColorDisplayNamePair,
  isDraftMessage,
} from "./types";

export { AgentTypeDisplayNames } from "./types";

// Utilities
export { getErrorMessage, type ErrorWithMessage } from "./error-utils";
export { cn } from "./utils";
