"use client";

import React, {useLayoutEffect, useRef, useEffect, useCallback, useMemo, useState} from "react";

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

interface ProcessedMessageProps {
  messageContent: string;
  index: number;
}

export default function MessageList({messages}: MessageListProps) {
  const [scrollAreaRef, setScrollAreaRef] = useState<HTMLDivElement | null>(null);

  // Track if user is at bottom - default to true for initial scroll
  const isAtBottomRef = useRef(true);

  const checkIfAtBottom = useCallback(() => {
    if (!scrollAreaRef) return false;
    const { scrollTop, scrollHeight, clientHeight } = scrollAreaRef;
    return scrollTop + clientHeight >= scrollHeight - 10; // 10px tolerance
  }, [scrollAreaRef]);

  // Track Ctrl (Windows/Linux) or Cmd (Mac) key state
  // This is so that underline is only visible when hover + cmd/ctrl
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey) document.documentElement.classList.add('modifier-pressed');
    };
    const handleKeyUp = (e: KeyboardEvent) => {
      if (!e.ctrlKey && !e.metaKey) document.documentElement.classList.remove('modifier-pressed');
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("keyup", handleKeyUp);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("keyup", handleKeyUp);
      document.documentElement.classList.remove('modifier-pressed');

    };
  }, []);

  // Track whether the user is scrolled to the bottom. Every scroll event
  // updates the ref so auto-scroll decisions are always based on the
  // user's actual position.
  useEffect(() => {
    if (!scrollAreaRef) return;
    const handleScroll = () => {
      isAtBottomRef.current = checkIfAtBottom();
    };
    handleScroll();
    scrollAreaRef.addEventListener("scroll", handleScroll);
    return () => scrollAreaRef.removeEventListener("scroll", handleScroll);
  }, [checkIfAtBottom, scrollAreaRef]);

  // Pin to bottom when new content arrives, but only if the user hasn't
  // scrolled away. Always scroll when the latest message is from the user
  // (they just sent it and should see it). Direct scrollTop assignment is
  // synchronous and avoids the animation conflicts that smooth scrollTo
  // causes during streaming.
  useLayoutEffect(() => {
    if (!scrollAreaRef) return;
    const lastMessage = messages[messages.length - 1];
    const isUserMessage = lastMessage && lastMessage.role === "user";
    if (!isAtBottomRef.current && !isUserMessage) return;
    scrollAreaRef.scrollTop = scrollAreaRef.scrollHeight;
    isAtBottomRef.current = true;
  }, [messages, scrollAreaRef]);

  // If no messages, show a placeholder
  if (messages.length === 0) {
    return (
      <div className="flex-1 p-6 flex items-center justify-center text-muted-foreground">
        <p>No messages yet. Start the conversation!</p>
      </div>
    );
  }

  return (
    <div className="overflow-y-auto flex-1" ref={setScrollAreaRef}>
      <div
        className="p-4 flex flex-col gap-4 max-w-4xl mx-auto min-h-0">
        {messages.map((message, index) => (
          <div
            key={message.id ?? "draft"}
            className={`${message.role === "user" ? "text-right" : ""}`}
          >
            <div
              className={`inline-block rounded-lg ${
                message.role === "user"
                  ? "bg-accent-foreground rounded-lg max-w-[90%] px-4 py-3 text-accent"
                  : "max-w-[80ch]"
              }`}
            >
              <div
                className={`whitespace-pre-wrap break-words text-left text-xs md:text-sm leading-relaxed md:leading-normal ${
                  message.role === "user" ? "" : "font-mono"
                }`}
              >
                {message.role !== "user" && message.content === "" ? (
                  <LoadingDots />
                ) : (
                  <ProcessedMessage
                    messageContent={message.content}
                    index={index}
                  />
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


const ProcessedMessage = React.memo(function ProcessedMessage({
                                                                messageContent,
                                                                index,
                                                              }: ProcessedMessageProps) {
  // Regex to find URLs
  // https://stackoverflow.com/a/17773849
  const urlRegex = useMemo<RegExp>(() => /(https?:\/\/(?:www\.|(?!www))[a-zA-Z0-9][a-zA-Z0-9-]+[a-zA-Z0-9]\.[^\s]{2,}|www\.[a-zA-Z0-9][a-zA-Z0-9-]+[a-zA-Z0-9]\.[^\s]{2,}|https?:\/\/(?:www\.|(?!www))[a-zA-Z0-9]+\.[^\s]{2,}|www\.[a-zA-Z0-9]+\.[^\s]{2,})/, []);

  const handleClick = (e: React.MouseEvent<HTMLAnchorElement>, url: string) => {
    if (e.metaKey || e.ctrlKey) {
      window.open(url, "_blank");
    } else {
      e.preventDefault(); // disable normal click to emulate terminal behaviour
    }
  }

  const linkedContent = useMemo(() => {
    return messageContent.split(urlRegex).map((content, idx) => {
      if (urlRegex.test(content)) {
        return (
          <a
            key={`${index}-${idx}`}
            href={content}
            onClick={(e) => handleClick(e, content)}
            className="cursor-default [.modifier-pressed_&]:hover:underline [.modifier-pressed_&]:hover:cursor-pointer"
          >
            {content}
          </a>
        );
      }
      return <span key={`${index}-${idx}`}>{content}</span>;
    });
  }, [index, messageContent, urlRegex]);

  return <>{linkedContent}</>;
});