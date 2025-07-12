"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useChat } from "./chat-provider";

interface ApiKeyInputProps {
  className?: string;
}

export function ApiKeyInput({ className }: ApiKeyInputProps) {
  const { apiKey, setApiKey, authRequired } = useChat();
  const [localKey, setLocalKey] = useState(apiKey);
  const [showKey, setShowKey] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setApiKey(localKey);
  };

  if (!authRequired) {
    return null;
  }

  return (
    <div className={`space-y-3 ${className}`}>
      <Alert>
        <AlertDescription>
          This server requires an API key for authentication.
        </AlertDescription>
      </Alert>
      
      <form onSubmit={handleSubmit} className="flex gap-2">
        <div className="flex-1 relative">
          <input
            type={showKey ? "text" : "password"}
            value={localKey}
            onChange={(e) => setLocalKey(e.target.value)}
            placeholder="Enter API key..."
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent dark:border-gray-600 dark:bg-gray-800 dark:text-white"
          />
          <button
            type="button"
            onClick={() => setShowKey(!showKey)}
            className="absolute right-2 top-1/2 transform -translate-y-1/2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            {showKey ? "🙈" : "👁️"}
          </button>
        </div>
        <Button type="submit" variant="outline">
          Set Key
        </Button>
      </form>
      
      {apiKey && (
        <div className="text-sm text-green-600 dark:text-green-400">
          ✓ API key configured
        </div>
      )}
    </div>
  );
}