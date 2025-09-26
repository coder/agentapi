"use client";

import { useChat } from "./chat-provider";
import MessageInput from "./message-input";
import MessageList from "./message-list";
import {UppyContextProvider} from "@uppy/react";
import {useState} from "react";
import {Uppy} from "@uppy/core";

export function Chat() {
  const { messages, loading, sendMessage, serverStatus } = useChat();
  const [uppy] = useState(() => new Uppy());

  return (
    <UppyContextProvider uppy={uppy}>
      <MessageList messages={messages} />
      <MessageInput
        onSendMessage={sendMessage}
        disabled={loading}
        serverStatus={serverStatus}
      />
    </UppyContextProvider>
  );
}
