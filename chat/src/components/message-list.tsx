"use client";

import { useLayoutEffect, useRef, useEffect, useCallback } from "react";

interface Message {
  role: string;
  content: string;
  id: number;
}

// Draft messages are used to optmistically update the UI
// before the server responds.
interface DraftMessage extends Omit<Message, "id"> {
  id?: number;
}

interface MessageListProps {
  messages: (Message | DraftMessage)[];
}

export default function MessageList({ messages }: MessageListProps) {
  const scrollAreaRef = useRef<HTMLDivElement>(null);
  // Avoid the message list to change its height all the time. It causes some
  // flickering in the screen because some messages, as the ones displaying
  // progress statuses, are changing the content(the number of lines) and size
  // constantily. To minimize it, we keep track of the biggest scroll height of
  // the content, and use that as the min height of the scroll area.
  const contentMinHeight = useRef(0);

  // Track if user is at bottom - default to true for initial scroll
  const isAtBottomRef = useRef(true);
  // Track the last known scroll height to detect new content
  const lastScrollHeightRef = useRef(0);

  const checkIfAtBottom = useCallback(() => {
    if (!scrollAreaRef.current) return false;
    const { scrollTop, scrollHeight, clientHeight } = scrollAreaRef.current;
    return scrollTop + clientHeight >= scrollHeight - 10; // 10px tolerance
  }, []);

  // Update isAtBottom on scroll
  useEffect(() => {
    const scrollContainer = scrollAreaRef.current;
    if (!scrollContainer) return;

    const handleScroll = () => {
      isAtBottomRef.current = checkIfAtBottom();
    };

    // Initial check
    handleScroll();

    scrollContainer.addEventListener("scroll", handleScroll);
    return () => scrollContainer.removeEventListener("scroll", handleScroll);
  }, [checkIfAtBottom]);

  // Handle auto-scrolling when messages change
  useLayoutEffect(() => {
    if (!scrollAreaRef.current) return;

    const scrollContainer = scrollAreaRef.current;
    const currentScrollHeight = scrollContainer.scrollHeight;

    // Check if this is new content (scroll height increased)
    const hasNewContent = currentScrollHeight > lastScrollHeightRef.current;
    const isFirstRender = lastScrollHeightRef.current === 0;
    const isNewUserMessage =
      messages.length > 0 && messages[messages.length - 1].role === "user";

    // Update content min height if needed
    if (currentScrollHeight > contentMinHeight.current) {
      contentMinHeight.current = currentScrollHeight;
    }

    // Auto-scroll only if:
    // 1. It's the first render, OR
    // 2. There's new content AND user was at the bottom, OR
    // 3. The user sent a new message
    if (
      hasNewContent &&
      (isFirstRender || isAtBottomRef.current || isNewUserMessage)
    ) {
      scrollContainer.scrollTo({
        top: currentScrollHeight,
        behavior: isFirstRender ? "instant" : "smooth",
      });
      // After scrolling, we're at the bottom
      isAtBottomRef.current = true;
    }

    // Update the last known scroll height
    lastScrollHeightRef.current = currentScrollHeight;
  }, [messages]);

  // If no messages, show a placeholder
  if (messages.length === 0) {
    return (
      <div className="flex-1 p-6 flex items-center justify-center text-muted-foreground">
        <p>No messages yet. Start the conversation!</p>
      </div>
    );
  }

  return (
    <div className="overflow-y-auto flex-1" ref={scrollAreaRef}>
      <div
        className="p-4 flex flex-col gap-4 max-w-4xl mx-auto"
        style={{ minHeight: contentMinHeight.current }}
      >
        {messages.map((message) => (
          <div
            key={message.id ?? "draft"}
            className={`${message.role === "user" ? "text-right" : ""}`}
          >
            <div
              className={`inline-block rounded-lg ${
                message.role === "user"
                  ? "bg-accent-foreground rounded-lg max-w-[90%] px-4 py-3 text-accent"
                  : "max-w-[80ch]"
              } ${message.id === undefined ? "animate-pulse" : ""}`}
            >
              <div
                className={`whitespace-pre-wrap break-words text-left text-xs md:text-sm leading-relaxed md:leading-normal ${
                  message.role === "user" ? "" : "font-mono"
                }`}
              >
                {message.role !== "user" && message.content === "" ? (
                  <LoadingDots />
                ) : (
                  message.content.trimEnd()
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

const LoadingDots = () => (
  <div className="flex space-x-1">
    <div
      aria-hidden="true"
      className={`size-2 rounded-full bg-foreground animate-pulse [animation-delay:0ms]`}
    />
    <div
      aria-hidden="true"
      className={`size-2 rounded-full bg-foreground animate-pulse [animation-delay:300ms]`}
    />
    <div
      aria-hidden="true"
      className={`size-2 rounded-full bg-foreground animate-pulse [animation-delay:600ms]`}
    />
    <span className="sr-only">Loading...</span>
  </div>
);
