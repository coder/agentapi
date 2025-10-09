"use client";

import {AgentType, useChat} from "@/components/chat-provider";
import {ModeToggle} from "@/components/mode-toggle";

export function Header() {
  const {serverStatus, agentType} = useChat();

  return (
    <header className="p-4 flex items-center justify-between border-b">
      <span className="font-bold">AgentAPI Chat</span>

      <div className="flex items-center gap-4">
        {serverStatus !== "unknown" && (
          <div className="flex items-center gap-2 text-sm font-medium">
            <span
              className={`text-secondary w-2 h-2 rounded-full ${
                ["offline", "unknown"].includes(serverStatus)
                  ? "bg-red-500 ring-2 ring-red-500/35"
                  : "bg-green-500 ring-2 ring-green-500/35"
              }`}
            />
            <span className="sr-only">Status:</span>
            <span className="first-letter:uppercase">{serverStatus}</span>
          </div>
        )}

        {agentType !== "unknown" && (
          <div className="flex items-center gap-2 text-sm font-medium">
            <span>{AgentType[agentType].displayName}</span>
          </div>
        )}
        <ModeToggle/>
      </div>
    </header>
  );
}
