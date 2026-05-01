/**
 * AgentAPI++ Library - Barrel Export
 *
 * This file re-exports types and utilities from the various submodules.
 * The actual implementations are in Go, but these types provide
 * TypeScript definitions for consumers of this library.
 */

// Re-export types from message formatting module
export type { AgentType } from './msgfmt/msgfmt';

// Re-export types from screen tracker module
export type {
  Conversation,
  ConversationMessage,
  ConversationRole,
  ConversationStatus,
  Emitter,
  ErrorLevel,
  MessagePart,
  AgentIO,
  StatePersistenceConfig,
} from './screentracker/conversation';

// Re-export types from HTTP API module
export type {
  MessagePart as HTTPMsgPart,
} from './httpapi/models';

// Re-export utilities
export type {
  WaitTimeout,
} from './util/util';

// Re-export logging context utilities
export type {
  LoggerContext,
} from './logctx/logctx';
