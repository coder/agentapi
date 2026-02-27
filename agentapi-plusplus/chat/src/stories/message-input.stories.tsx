import type { Meta, StoryObj } from "@storybook/nextjs";

import MessageInput from "../components/message-input";

const meta = {
  title: "Components/MessageInput",
  component: MessageInput,
  parameters: {
    // More on how to position stories at: https://storybook.js.org/docs/configure/story-layout
    layout: "centered",
  },
} satisfies Meta<typeof MessageInput>;

export default meta;
type Story = StoryObj<typeof meta>;

const defaultArgs = {
  onSendMessage: () => {},
};

export const ServerStatusStable: Story = {
  args: {
    ...defaultArgs,
    serverStatus: "stable",
  },
};
export const ServerStatusRunning: Story = {
  args: {
    ...defaultArgs,
    serverStatus: "running",
  },
};
export const ServerStatusOffline: Story = {
  args: {
    ...defaultArgs,
    serverStatus: "offline",
  },
};
export const ServerStatusUnknown: Story = {
  args: {
    ...defaultArgs,
    serverStatus: "unknown",
  },
};
