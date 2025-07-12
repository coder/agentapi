"use client";

import { useChat } from "@/components/chat-provider";
import { SettingsMenu } from "../components/settings-menu";

export function Header() {
  const { serverStatus, authRequired, apiKey } = useChat();

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
        {authRequired && !apiKey && (
          <div className="text-xs text-amber-600 dark:text-amber-400 font-medium">
            API key required
          </div>
        )}
        <SettingsMenu />
      </div>
    </header>
  );
}
