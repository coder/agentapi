"use client";

import {useChat} from "./chat-provider";
import MessageInput from "./message-input";
import MessageList from "./message-list";

export function Chat() {
  const {messages, loading, sendMessage, serverStatus} = useChat();

  return (
    <>
      <MessageList messages={messages}/>
      <MessageInput
        onSendMessage={sendMessage}
        disabled={loading}
        serverStatus={serverStatus}
      />
    </>
  );
}
