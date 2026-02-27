import { Chat } from "@/components/chat";
import { ChatProvider } from "@/components/chat-provider";
import { Suspense } from "react";

export default function EmbedPage() {
  return (
    <Suspense
      fallback={
        <div className="text-center p-4 text-sm">Loading chat interface...</div>
      }
    >
      <ChatProvider>
        <div className="flex flex-col h-svh">
          <Chat />
        </div>
      </ChatProvider>
    </Suspense>
  );
}
