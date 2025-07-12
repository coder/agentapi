"use client";

import {
  useState,
  useEffect,
  useRef,
  createContext,
  PropsWithChildren,
  useContext,
} from "react";
import { toast } from "sonner";

interface Message {
  id: number;
  role: string;
  content: string;
}

// Draft messages are used to optmistically update the UI
// before the server responds.
interface DraftMessage extends Omit<Message, "id"> {
  id?: number;
}

interface MessageUpdateEvent {
  id: number;
  role: string;
  message: string;
  time: string;
}

interface StatusChangeEvent {
  status: string;
}

function isDraftMessage(message: Message | DraftMessage): boolean {
  return message.id === undefined;
}

type MessageType = "user" | "raw";

type ServerStatus = "online" | "offline" | "unknown";

interface ChatContextValue {
  messages: (Message | DraftMessage)[];
  loading: boolean;
  serverStatus: ServerStatus;
  apiKey: string;
  setApiKey: (key: string) => void;
  authRequired: boolean;
  baseUrl: string;
  setBaseUrl: (url: string) => void;
  sendMessage: (message: string, type?: MessageType) => void;
}

const ChatContext = createContext<ChatContextValue | undefined>(undefined);

export function ChatProvider({ children }: PropsWithChildren) {
  const [messages, setMessages] = useState<(Message | DraftMessage)[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [serverStatus, setServerStatus] = useState<ServerStatus>("unknown");
  const [apiKey, setApiKey] = useState<string>("");
  const [authRequired, setAuthRequired] = useState<boolean>(false);
  const [baseUrl, setBaseUrl] = useState<string>("");

  // Initialize baseUrl on client side
  useEffect(() => {
    if (typeof window !== "undefined" && !baseUrl) {
      const searchParams = new URLSearchParams(window.location.search);
      const urlFromQuery = searchParams.get("url") || window.location.origin;
      setBaseUrl(urlFromQuery);
    }
  }, [baseUrl]);
  const eventSourceRef = useRef<EventSource | null>(null);
  const agentAPIUrl = baseUrl;

  // Helper function to get headers with optional authentication
  const getHeaders = () => {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (apiKey) {
      headers["Authorization"] = `Bearer ${apiKey}`;
    }
    return headers;
  };

  // Check if authentication is required on first load
  useEffect(() => {
    const checkAuthRequired = async () => {
      try {
        const response = await fetch(`${agentAPIUrl}/status`);
        if (response.status === 401) {
          setAuthRequired(true);
        } else {
          setAuthRequired(false);
        }
      } catch (error) {
        console.warn("Could not check auth requirements:", error);
      }
    };

    if (agentAPIUrl) {
      checkAuthRequired();
    }
  }, [agentAPIUrl]);

  // Set up SSE connection to the events endpoint
  useEffect(() => {
    // Function to create and set up EventSource
    const setupEventSource = () => {
      // Close existing connection
      if (eventSourceRef.current) {
        console.log("Closing existing EventSource connection");
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }

      // Reset messages when establishing a new connection
      setMessages([]);

      if (!agentAPIUrl) {
        console.warn(
          "agentAPIUrl is not set, SSE connection cannot be established."
        );
        setServerStatus("offline");
        return null;
      }

      // If auth is required but no API key provided, don't attempt connection
      if (authRequired && !apiKey) {
        console.log("Auth required but no API key provided, skipping SSE connection");
        setServerStatus("offline");
        return null;
      }

      // For SSE, we need to pass API key as a query parameter since EventSource doesn't support custom headers
      const eventsUrl = apiKey 
        ? `${agentAPIUrl}/events?api_key=${encodeURIComponent(apiKey)}`
        : `${agentAPIUrl}/events`;
      const eventSource = new EventSource(eventsUrl);
      eventSourceRef.current = eventSource;

      // Handle message updates
      eventSource.addEventListener("message_update", (event) => {
        const data: MessageUpdateEvent = JSON.parse(event.data);

        setMessages((prevMessages) => {
          // Clean up draft messages
          const updatedMessages = [...prevMessages].filter(
            (m) => !isDraftMessage(m)
          );

          // Check if message with this ID already exists
          const existingIndex = updatedMessages.findIndex(
            (m) => m.id === data.id
          );

          if (existingIndex !== -1) {
            // Update existing message
            updatedMessages[existingIndex] = {
              role: data.role,
              content: data.message,
              id: data.id,
            };
            return updatedMessages;
          } else {
            // Add new message
            return [
              ...updatedMessages,
              {
                role: data.role,
                content: data.message,
                id: data.id,
              },
            ];
          }
        });
      });

      // Handle status changes
      eventSource.addEventListener("status_change", (event) => {
        const data: StatusChangeEvent = JSON.parse(event.data);
        setServerStatus(data.status as ServerStatus);
      });

      // Handle connection open (server is online)
      eventSource.onopen = () => {
        // Connection is established, but we'll wait for status_change event
        // for the actual server status
        console.log("EventSource connection established:", eventsUrl);
        setServerStatus("online");
      };

      // Handle connection errors
      eventSource.onerror = (error) => {
        console.error("EventSource error:", error, "ReadyState:", eventSource.readyState, "URL:", eventsUrl);
        setServerStatus("offline");

        // Check if this might be an authentication error
        if (eventSource.readyState === EventSource.CLOSED) {
          console.log("EventSource closed immediately - possible auth error");
          // Don't automatically set authRequired=true here as it might cause loops
        }

        // Don't auto-reconnect if we have an API key and connection failed
        // This prevents infinite reconnection loops on auth errors
        if (!apiKey) {
          setTimeout(() => {
            if (eventSourceRef.current === eventSource) {
              console.log("Attempting to reconnect...");
              setupEventSource();
            }
          }, 3000);
        }
      };

      return eventSource;
    };

    // Initial setup
    const eventSource = setupEventSource();

    // Clean up on component unmount
    return () => {
      if (eventSource) {
        // Check if eventSource was successfully created
        eventSource.close();
      }
    };
  }, [agentAPIUrl, apiKey, authRequired]);

  // Send a new message
  const sendMessage = async (
    content: string,
    type: "user" | "raw" = "user"
  ) => {
    // For user messages, require non-empty content
    if (type === "user" && !content.trim()) return;

    // For raw messages, don't set loading state as it's usually fast
    if (type === "user") {
      setMessages((prevMessages) => [
        ...prevMessages,
        { role: "user", content },
      ]);
      setLoading(true);
    }

    try {
      const response = await fetch(`${agentAPIUrl}/message`, {
        method: "POST",
        headers: getHeaders(),
        body: JSON.stringify({
          content: content,
          type,
        }),
      });

      if (!response.ok) {
        if (response.status === 401) {
          setAuthRequired(true);
          toast.error("Authentication required", {
            description: "Please provide a valid API key to continue.",
          });
          return;
        }

        const errorData = await response.json();
        console.error("Failed to send message:", errorData);
        const detail = errorData.detail;
        const messages =
          "errors" in errorData
            ? // eslint-disable-next-line @typescript-eslint/no-explicit-any
              errorData.errors.map((e: any) => e.message).join(", ")
            : "";

        const fullDetail = `${detail}: ${messages}`;
        toast.error(`Failed to send message`, {
          description: fullDetail,
        });
      }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (error: any) {
      console.error("Error sending message:", error);
      const detail = error.detail;
      const messages =
        "errors" in error
          ? // eslint-disable-next-line @typescript-eslint/no-explicit-any
            error.errors.map((e: any) => e.message).join("\n")
          : "";

      const fullDetail = `${detail}: ${messages}`;

      toast.error(`Error sending message`, {
        description: fullDetail,
      });
    } finally {
      if (type === "user") {
        setMessages((prevMessages) =>
          prevMessages.filter((m) => !isDraftMessage(m))
        );
        setLoading(false);
      }
    }
  };

  return (
    <ChatContext.Provider
      value={{
        messages,
        loading,
        sendMessage,
        serverStatus,
        apiKey,
        setApiKey,
        authRequired,
        baseUrl,
        setBaseUrl,
      }}
    >
      {children}
    </ChatContext.Provider>
  );
}

export function useChat() {
  const context = useContext(ChatContext);
  if (!context) {
    throw new Error("useChat must be used within a ChatProvider");
  }
  return context;
}
