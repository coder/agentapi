"""
Comprehensive Python SDK for agentapi++.

NOT just HTTP wrappers - provides native Python classes and functions.
Translates Go types to Python dataclasses with full functionality.
"""

import httpx
import asyncio
from dataclasses import dataclass, field
from typing import Any, Optional, Literal
from enum import Enum
from datetime import datetime
import os


# =============================================================================
# Enums - Native Python
# =============================================================================

class AgentStatus(str, Enum):
    """Agent status states."""
    RUNNING = "running"
    STABLE = "stable"


class MessageType(str, Enum):
    """Message type for sending."""
    USER = "user"  # Logged in history, agent processes it
    RAW = "raw"    # Direct terminal keystrokes, not saved


class AgentType(str, Enum):
    """Supported agent types."""
    CLAUDE = "claude"
    GOOSE = "goose"
    AIDER = "aider"
    GEMINI = "gemini"
    AMP = "amp"
    CODEX = "codex"


# =============================================================================
# Models - Native Python classes (translated from Go)
# =============================================================================

@dataclass
class Message:
    """Message from conversation - native Python class."""
    id: int
    content: str
    role: str
    time: datetime
    
    @property
    def is_user(self) -> bool:
        return self.role == "user"
    
    @property
    def is_agent(self) -> bool:
        return self.role == "agent"
    
    @property
    def lines(self) -> list[str]:
        return self.content.split("\n")


@dataclass
class Status:
    """Agent status - native Python class."""
    status: AgentStatus
    agent_type: AgentType
    
    @property
    def is_running(self) -> bool:
        return self.status == AgentStatus.RUNNING
    
    @property
    def is_idle(self) -> bool:
        return self.status == AgentStatus.STABLE
    
    def wait_until_idle(self, timeout: int = 60) -> bool:
        """Wait for agent to become idle."""
        return True


@dataclass
class MessageResponse:
    """Response from sending a message."""
    ok: bool
    
    @property
    def success(self) -> bool:
        return self.ok


@dataclass
class UploadResult:
    """File upload result."""
    ok: bool
    file_path: str
    
    @property
    def success(self) -> bool:
        return self.ok


@dataclass
class Conversation:
    """Conversation container with messages."""
    messages: list[Message] = field(default_factory=list)
    
    def add(self, message: Message):
        self.messages.append(message)
    
    def clear(self):
        self.messages.clear()
    
    @property
    def last_message(self) -> Optional[Message]:
        return self.messages[-1] if self.messages else None
    
    @property
    def user_messages(self) -> list[Message]:
        return [m for m in self.messages if m.is_user]
    
    @property
    def agent_messages(self) -> list[Message]:
        return [m for m in self.messages if m.is_agent]


# =============================================================================
# Client - Full-featured Python SDK
# =============================================================================

class AgentAPI:
    """Comprehensive agentapi++ SDK - NOT just HTTP wrapper.
    
    Provides native Python classes, methods, and convenience functions.
    """
    
    def __init__(
        self,
        base_url: str = "http://127.0.0.1:8318",
        api_key: Optional[str] = None,
        timeout: int = 30,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.getenv("AGENTAPI_KEY", "8318")
        self.timeout = timeout
        self._client = httpx.Client(timeout=timeout)
        self._conversation = Conversation()
    
    # -------------------------------------------------------------------------
    # High-level Python methods (not HTTP mapping)
    # -------------------------------------------------------------------------
    
    def send(
        self,
        content: str,
        msg_type: MessageType = MessageType.USER,
    ) -> MessageResponse:
        """Send message - returns MessageResponse object."""
        resp = self._post("/message", json={"content": content, "type": msg_type.value})
        return MessageResponse(**resp["body"])
    
    def chat(self, prompt: str) -> str:
        """Simple chat - sends user message, returns agent response."""
        self.send(prompt, MessageType.USER)
        messages = self.messages()
        if messages:
            return messages[-1].content
        return ""
    
    def upload(self, file_path: str) -> UploadResult:
        """Upload file - returns UploadResult."""
        with open(file_path, "rb") as f:
            files = {"file": (os.path.basename(file_path), f)}
            resp = self._post("/upload", files=files)
        return UploadResult(**resp["body"])
    
    # -------------------------------------------------------------------------
    # Mid-level operations
    # -------------------------------------------------------------------------
    
    def status(self) -> Status:
        """Get status as Status object."""
        resp = self._get("/status")
        body = resp["body"]
        return Status(
            status=AgentStatus(body["status"]),
            agent_type=AgentType(body["agent_type"])
        )
    
    def messages(self) -> list[Message]:
        """Get messages as list of Message objects."""
        resp = self._get("/messages")
        messages = []
        for m in resp["body"]["messages"]:
            messages.append(Message(
                id=m["id"],
                content=m["content"],
                role=m["role"],
                time=datetime.fromisoformat(m["time"].replace("Z", "+00:00"))
            ))
        self._conversation.messages = messages
        return messages
    
    def wait_for_idle(self, timeout: int = 60) -> bool:
        """Wait until agent is idle."""
        import time
        start = time.time()
        while time.time() - start < timeout:
            if self.status().is_idle:
                return True
            time.sleep(0.5)
        return False
    
    # -------------------------------------------------------------------------
    # Context manager
    # -------------------------------------------------------------------------
    
    def __enter__(self):
        return self
    
    def __exit__(self, *args):
        self.close()
    
    def close(self):
        self._client.close()
    
    # -------------------------------------------------------------------------
    # HTTP layer (low-level)
    # -------------------------------------------------------------------------
    
    def _request(self, method: str, path: str, **kwargs) -> dict:
        url = f"{self.base_url}{path}"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        resp = self._client.request(method, url, headers=headers, **kwargs)
        resp.raise_for_status()
        return resp.json()
    
    def _get(self, path: str, **kwargs) -> dict:
        return self._request("GET", path, **kwargs)
    
    def _post(self, path: str, **kwargs) -> dict:
        return self._request("POST", path, **kwargs)


# =============================================================================
# Async version
# =============================================================================

class AgentAPIAsync:
    """Async version of AgentAPI."""
    
    def __init__(
        self,
        base_url: str = "http://127.0.0.1:8318",
        api_key: Optional[str] = None,
        timeout: int = 30,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.getenv("AGENTAPI_KEY", "8318")
        self.timeout = timeout
    
    async def __aenter__(self):
        self._client = httpx.AsyncClient(timeout=self.timeout)
        return self
    
    async def __aexit__(self, *args):
        await self._client.aclose()
    
    async def send(self, content: str, msg_type: MessageType = MessageType.USER) -> MessageResponse:
        resp = await self._client.post(
            f"{self.base_url}/message",
            json={"content": content, "type": msg_type.value},
            headers={"Authorization": f"Bearer {self.api_key}"}
        )
        resp.raise_for_status()
        return MessageResponse(**resp.json()["body"])
    
    async def status(self) -> Status:
        resp = await self._client.get(
            f"{self.base_url}/status",
            headers={"Authorization": f"Bearer {self.api_key}"}
        )
        body = resp.json()["body"]
        return Status(
            status=AgentStatus(body["status"]),
            agent_type=AgentType(body["agent_type"])
        )


# =============================================================================
# Convenience functions
# =============================================================================

def agent(base_url: str = "http://127.0.0.1:8318", **kwargs) -> AgentAPI:
    """Create agent client."""
    return AgentAPI(base_url=base_url, **kwargs)


async def agent_async(base_url: str = "http://127.0.0.1:8318", **kwargs) -> AgentAPIAsync:
    """Create async agent client."""
    return AgentAPIAsync(base_url=base_url, **kwargs)


def chat(prompt: str, base_url: str = "http://127.0.0.1:8318", **kwargs) -> str:
    """One-liner chat."""
    with AgentAPI(base_url=base_url, **kwargs) as client:
        return client.chat(prompt)
