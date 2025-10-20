"use client";

import {useState, FormEvent, KeyboardEvent, MouseEvent, useEffect, useRef, ChangeEvent} from "react";
import {Button} from "./ui/button";
import {
  ArrowDownIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpIcon,
  CornerDownLeftIcon,
  DeleteIcon,
  SendIcon,
  Upload,
  Square,
} from "lucide-react";
import {Tabs, TabsList, TabsTrigger} from "./ui/tabs";
import type {ServerStatus} from "./chat-provider";
import TextareaAutosize from "react-textarea-autosize";
import {useChat} from "./chat-provider";
import {DragDrop} from "./drag-drop";
import {toast} from "sonner";
import {getErrorMessage} from "@/lib/error-utils";

interface MessageInputProps {
  onSendMessage: (message: string, type: "user" | "raw") => void;
  disabled?: boolean;
  serverStatus: ServerStatus;
}

interface SentChar {
  char: string;
  id: number;
  timestamp: number;
}

// List of keys to send as raw input when in control mode

const specialKeys: Record<string, string> = {
  ArrowUp: "\x1b[A", // Escape sequence for up arrow
  ArrowDown: "\x1b[B", // Escape sequence for down arrow
  ArrowRight: "\x1b[C", // Escape sequence for right arrow
  ArrowLeft: "\x1b[D", // Escape sequence for left arrow
  Escape: "\x1b", // Escape key
  Tab: "\t", // Tab key
  Delete: "\x1b[3~", // Delete key
  Home: "\x1b[H", // Home key
  End: "\x1b[F", // End key
  PageUp: "\x1b[5~", // Page Up
  PageDown: "\x1b[6~", // Page Down
  Backspace: "\b", // Backspace key
};

export default function MessageInput({
  onSendMessage,
  disabled = false,
  serverStatus,
}: MessageInputProps) {
  const [message, setMessage] = useState("");
  const [inputMode, setInputMode] = useState("text");
  const [sentChars, setSentChars] = useState<SentChar[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const nextCharId = useRef(0);
  const [controlAreaFocused, setControlAreaFocused] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const {uploadFiles} = useChat();

  const handleFilesAdded = async (files: File[]) => {
    for (const file of files) {
      const arrayBuffer = await file.arrayBuffer();

      try {
        // Create Blob from ArrayBuffer
        const blob = new Blob([arrayBuffer], {type: file.type});

        // Create FormData for upload
        const formData = new FormData();
        formData.append('file', blob, file.name);

        // Upload to agent API
        const response = await uploadFiles(formData);
        if (response.ok) {
          setMessage(oldMessage => oldMessage + ' @"' + response.filePath + '"');
        }
      } catch (error) {
        toast.error("Failed to and upload file:", {
          description: getErrorMessage(error),
        });
      }
    }
    textareaRef.current?.focus();
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (message.trim() && !disabled) {
      onSendMessage(message, "user");
      setMessage("");
    }
  };

  // Remove sent characters after they expire (2 seconds)
  useEffect(() => {
    if (sentChars.length === 0) return;

    const interval = setInterval(() => {
      const now = Date.now();
      setSentChars((chars) =>
        chars.filter((char) => now - char.timestamp < 2000)
      );
    }, 100);

    return () => clearInterval(interval);
  }, [sentChars]);

  const addSentChar = (char: string) => {
    const newChar: SentChar = {
      char,
      id: nextCharId.current++,
      timestamp: Date.now(),
    };
    setSentChars((prev) => [...prev, newChar]);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    // In control mode, send special keys as raw messages
    if (inputMode === "control" && !disabled) {
      // Check if the pressed key is in our special keys map
      if (specialKeys[e.key]) {
        e.preventDefault();
        addSentChar(e.key);
        onSendMessage(specialKeys[e.key], "raw");
        return;
      }

      // Handle Enter as raw newline when in control mode
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        addSentChar("⏎");
        onSendMessage("\r", "raw");
        return;
      }

      // Handle Ctrl+key combinations
      if (e.ctrlKey) {
        const ctrlMappings: Record<string, string> = {
          c: "\x03", // Ctrl+C (SIGINT)
          d: "\x04", // Ctrl+D (EOF)
          z: "\x1A", // Ctrl+Z (SIGTSTP)
          l: "\x0C", // Ctrl+L (clear screen)
          a: "\x01", // Ctrl+A (beginning of line)
          e: "\x05", // Ctrl+E (end of line)
          w: "\x17", // Ctrl+W (delete word)
          u: "\x15", // Ctrl+U (clear line)
          r: "\x12", // Ctrl+R (reverse history search)
        };

        if (ctrlMappings[e.key.toLowerCase()]) {
          e.preventDefault();
          addSentChar(`Ctrl+${e.key.toUpperCase()}`);
          onSendMessage(ctrlMappings[e.key.toLowerCase()], "raw");
          return;
        }
      }

      // If it's a printable character (length 1), send it as raw input
      if (e.key.length === 1) {
        e.preventDefault();
        addSentChar(e.key);
        onSendMessage(e.key, "raw");
        return;
      }
    } else if (e.key === "Enter" && !e.shiftKey) {
      // Normal Enter handling for text mode with non-empty message
      e.preventDefault();
      handleSubmit(e);
    }
  };

  const handleUploadClick = (e: MouseEvent<HTMLButtonElement>) => {
    e.preventDefault();
    fileInputRef.current?.click();
  };

  const handleFileInputChange = async (e: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length > 0) {
      await handleFilesAdded(files);
    }
    e.target.value = '';
  };

  return (
    <Tabs value={inputMode} onValueChange={setInputMode}>
      <div className="max-w-4xl mx-auto w-full p-4 pt-0">
        <DragDrop
          onFilesAdded={handleFilesAdded}
          disabled={disabled || inputMode === "control"}
        >
          <input
            ref={fileInputRef}
            type="file"
            multiple={true}
            className={"hidden"}
            onChange={handleFileInputChange}
          />
          <form onSubmit={handleSubmit}
                className={"rounded-lg border text-base shadow-sm placeholder:text-muted-foreground focus-within:outline-none focus-within:ring-1 focus-within:ring-ring disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"}>
            <div className="flex flex-col">
              <div className="flex">
                {inputMode === "control" && !disabled ? (
                  <div
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    ref={textareaRef as any}
                    tabIndex={0}
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    onKeyDown={handleKeyDown as any}
                    onFocus={() => setControlAreaFocused(true)}
                    onBlur={() => setControlAreaFocused(false)}
                    className="cursor-text p-4 h-20 text-muted-foreground flex items-center justify-center w-full outline-none text-sm"
                  >
                    {controlAreaFocused
                      ? "Press any key to send to terminal (arrows, Ctrl+C, Ctrl+R, etc.)"
                      : "Click or focus this area to send keystrokes to terminal"}
                  </div>
                ) : (
                  <TextareaAutosize
                    autoFocus
                    ref={textareaRef}
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={
                      serverStatus === "running"
                        ? "Running..."
                        : "Type a message..."
                    }
                    className="resize-none w-full text-sm outline-none p-4 h-20 max-h-[400px]"
                    disabled={serverStatus !== "stable"}
                  />
                )}
              </div>

              <div className="flex items-center justify-between p-4">
                <TabsList className="bg-transparent">
                  <TabsTrigger
                    value="text"
                    onClick={() => {
                      textareaRef.current?.focus();
                    }}
                  >
                    Text
                  </TabsTrigger>
                  <TabsTrigger
                    value="control"
                    onClick={() => {
                      textareaRef.current?.focus();
                    }}
                  >
                    Control
                  </TabsTrigger>
                </TabsList>

                <div className={"flex flex-row gap-3"}>
                  <Button
                    type="submit"
                    size="icon"
                    className="rounded-full"
                    onClick={handleUploadClick}
                  >
                    <Upload/>
                    <span className="sr-only">Upload</span>
                  </Button>

                  {inputMode === "text" && serverStatus !== "running" && (
                    <Button
                      type="submit"
                      disabled={disabled || !message.trim()}
                      size="icon"
                      className="rounded-full"
                    >
                      <SendIcon/>
                      <span className="sr-only">Send</span>
                    </Button>
                  )}

                  {inputMode === "text" && serverStatus === "running" && (
                    <Button
                      size="icon"
                      className="rounded-full"
                      disabled={disabled}
                      onClick={() => {
                        onSendMessage(specialKeys.Escape, "raw");
                      }}
                    >
                      <Square/>
                      <span className="sr-only">Stop</span>
                    </Button>
                  )}

                  {inputMode === "control" && !disabled && (
                    <div className="flex items-center gap-1">
                      {sentChars.map((char) => (
                        <span
                          key={char.id}
                          className="min-w-9 h-9 px-2 rounded border font-mono font-medium text-xs flex items-center justify-center animate-pulse"
                        >
                      <Char char={char.char}/>
                    </span>
                      ))}
                    </div>
                  )}
                </div>

              </div>
            </div>
          </form>
        </DragDrop>

        <span className="text-xs text-muted-foreground mt-2 block text-center">
          {inputMode === "text" ? (
            <>
              Switch to <span className="font-medium">Control</span> mode to
              send raw keystrokes (↑,↓,Tab,Ctrl+C,Ctrl+R) directly to the
              terminal. Drag and drop files onto the input area to upload.
            </>
          ) : (
            <>Control mode - keystrokes sent directly to terminal</>
          )}
        </span>
      </div>
    </Tabs>
  );
}

function Char({ char }: { char: string }) {
  switch (char) {
    case "ArrowUp":
      return <ArrowUpIcon className="h-4 w-4" />;
    case "ArrowDown":
      return <ArrowDownIcon className="h-4 w-4" />;
    case "ArrowRight":
      return <ArrowRightIcon className="h-4 w-4" />;
    case "ArrowLeft":
      return <ArrowLeftIcon className="h-4 w-4" />;
    case "⏎":
      return <CornerDownLeftIcon className="h-4 w-4" />;
    case "Backspace":
      return <DeleteIcon className="h-4 w-4" />;
    default:
      return char;
  }
}
