import { Chat } from "@/components/chat";
import { ChatProvider } from "@/components/chat-provider";
import { Header } from "./header";
import { Suspense } from "react";

export default function Home() {
  return (
    <Suspense
      fallback={
        <div className="text-center p-4 text-sm">Loading chat interface...</div>
      }
    >
      <ChatProvider>
        <div className="flex flex-col h-svh">
          <Header />
          <Chat />
        </div>
      </ChatProvider>
    </Suspense>
  );
}
