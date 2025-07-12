"use client";

import * as React from "react";
import { Settings, Sun, Moon, Key } from "lucide-react";
import { useTheme } from "next-themes";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import { useChat } from "./chat-provider";

export function SettingsMenu() {
  const { setTheme } = useTheme();
  const { apiKey, setApiKey, authRequired } = useChat();
  const [localKey, setLocalKey] = useState(apiKey);
  const [showKeyInput, setShowKeyInput] = useState(false);

  const handleSaveKey = () => {
    setApiKey(localKey);
    setShowKeyInput(false);
  };

  const handleClearKey = () => {
    setLocalKey("");
    setApiKey("");
    setShowKeyInput(false);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="icon">
          <Settings className="h-[1.2rem] w-[1.2rem]" />
          <span className="sr-only">Settings</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel>Settings</DropdownMenuLabel>
        <DropdownMenuSeparator />
        
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Sun className="mr-2 h-4 w-4" />
            Theme
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem onClick={() => setTheme("light")}>
              <Sun className="mr-2 h-4 w-4" />
              Light
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme("dark")}>
              <Moon className="mr-2 h-4 w-4" />
              Dark
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme("system")}>
              <Settings className="mr-2 h-4 w-4" />
              System
            </DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSeparator />

        <DropdownMenuLabel className="flex items-center">
          <Key className="mr-2 h-4 w-4" />
          API Key
          {authRequired && (
            <span className="ml-auto text-xs text-red-500">Required</span>
          )}
        </DropdownMenuLabel>

        {apiKey ? (
          <>
            <DropdownMenuItem disabled className="text-xs text-green-600">
              ✓ API key configured
            </DropdownMenuItem>
            <DropdownMenuItem 
              onSelect={(e) => {
                e.preventDefault();
                setShowKeyInput(true);
              }}
            >
              Update API key
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleClearKey} className="text-red-600">
              Clear API key
            </DropdownMenuItem>
          </>
        ) : (
          <DropdownMenuItem 
            onSelect={(e) => {
              e.preventDefault();
              setShowKeyInput(true);
            }}
          >
            {authRequired ? "Set required API key" : "Set API key"}
          </DropdownMenuItem>
        )}

        {showKeyInput && (
          <div className="p-2 space-y-2" onClick={(e) => e.stopPropagation()}>
            <input
              type="text"
              value={localKey}
              onChange={(e) => setLocalKey(e.target.value)}
              placeholder="Enter API key..."
              className="w-full px-2 py-1 text-xs border rounded focus:outline-none focus:ring-1 focus:ring-blue-500 dark:bg-gray-800 dark:border-gray-600"
              autoFocus
              onKeyDown={(e) => {
                e.stopPropagation();
                if (e.key === 'Enter') {
                  handleSaveKey();
                } else if (e.key === 'Escape') {
                  setShowKeyInput(false);
                }
              }}
            />
            <div className="flex gap-1">
              <Button 
                size="sm" 
                onClick={handleSaveKey}
                className="flex-1 h-6 text-xs"
              >
                Save
              </Button>
              <Button 
                size="sm" 
                variant="outline" 
                onClick={() => setShowKeyInput(false)}
                className="flex-1 h-6 text-xs"
              >
                Cancel
              </Button>
            </div>
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}