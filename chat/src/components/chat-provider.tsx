"use client";

import { useSearchParams } from "next/navigation";
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

export type ServerStatus = "stable" | "running" | "offline" | "unknown";

interface ChatContextValue {
  messages: (Message | DraftMessage)[];
  loading: boolean;
  serverStatus: ServerStatus;
  sendMessage: (message: string, type?: MessageType) => void;
}

const ChatContext = createContext<ChatContextValue | undefined>(undefined);

const useAgentAPIUrl = (): string => {
  const searchParams = useSearchParams();
  const paramsUrl = searchParams.get("url");
  if (paramsUrl) {
    return paramsUrl;
  }
  const basePath = process.env.NEXT_PUBLIC_BASE_PATH;
  if (!basePath) {
    throw new Error(
      "agentAPIUrl is not set. Please set the url query parameter to the URL of the AgentAPI or the NEXT_PUBLIC_BASE_PATH environment variable."
    );
  }
  // NOTE(cian): We use '../' here to construct the agent API URL relative
  // to the chat's location. Let's say the app is hosted on a subpath
  // `/@admin/workspace.agent/apps/ccw/`. When you visit this URL you get
  // redirected to `/@admin/workspace.agent/apps/ccw/chat/embed`. This serves
  // this React application, but it needs to know where the agent API is hosted.
  // This will be at the root of where the application is mounted e.g.
  // `/@admin/workspace.agent/apps/ccw/`. Previously we used
  // `window.location.origin` but this assumes that the application owns the
  // entire origin.
  // See: https://github.com/coder/coder/issues/18779#issuecomment-3133290494 for more context.
  let chatURL: string = new URL(basePath, window.location.origin).toString();
  // NOTE: trailing slashes and relative URLs are tricky.
  // https://developer.mozilla.org/en-US/docs/Web/API/URL_API/Resolving_relative_references#current_directory_relative
  if (!chatURL.endsWith("/")) {
    chatURL += "/";
  }
  const agentAPIURL = new URL("..", chatURL).toString();
  if (agentAPIURL.endsWith("/")) {
    return agentAPIURL.slice(0, -1);
  }
  return agentAPIURL;
};

export function ChatProvider({ children }: PropsWithChildren) {
  const [messages, setMessages] = useState<(Message | DraftMessage)[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [serverStatus, setServerStatus] = useState<ServerStatus>("unknown");
  const eventSourceRef = useRef<EventSource | null>(null);
  const agentAPIUrl = useAgentAPIUrl();

  // Set up SSE connection to the events endpoint
  useEffect(() => {
    // Function to create and set up EventSource
    const setupEventSource = () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }

      // Reset messages when establishing a new connection
      setMessages([]);

      if (!agentAPIUrl) {
        console.warn(
          "agentAPIUrl is not set, SSE connection cannot be established."
        );
        setServerStatus("offline"); // Or some other appropriate status
        return null; // Don't try to connect if URL is empty
      }

      const eventSource = new EventSource(`${agentAPIUrl}/events`);
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
        if (data.status === "stable") {
          setServerStatus("stable");
        } else if (data.status === "running") {
          setServerStatus("running");
        } else {
          setServerStatus("unknown");
        }
      });

      // Handle connection open (server is online)
      eventSource.onopen = () => {
        // Connection is established, but we'll wait for status_change event
        // for the actual server status
        console.log("EventSource connection established - messages reset");
      };

      // Handle connection errors
      eventSource.onerror = (error) => {
        console.error("EventSource error:", error);
        setServerStatus("offline");

        // Try to reconnect after delay
        setTimeout(() => {
          if (eventSourceRef.current) {
            setupEventSource();
          }
        }, 3000);
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
  }, [agentAPIUrl]);

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
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          content: content,
          type,
        }),
      });

      if (!response.ok) {
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
