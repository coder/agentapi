"""
Comprehensive Python SDK for agentapi.

Provides native Python classes - not just HTTP wrappers.
Translates Go types to Python classes with full functionality.
"""

import httpx
from dataclasses import dataclass, field
from typing import Any, Optional
import os


# =============================================================================
# Models - Native Python classes
# =============================================================================

@dataclass
class AgentSession:
    """Agent session with full state."""
    id: str
    agent: str
    started: int
    models: list[str] = field(default_factory=list)
    metadata: dict = field(default_factory=dict)


@dataclass
class RoutingRule:
    """Routing rule for agent."""
    agent: str
    preferred_model: str = "claude-3-5-sonnet-20241022"
    fallback_models: list[str] = field(default_factory=list)
    max_retries: int = 3
    timeout_seconds: int = 30


@dataclass
class ChatResponse:
    """Chat response as Python class."""
    id: str
    model: str
    choices: list[dict]
    usage: dict = field(default_factory=dict)
    
    @property
    def text(self) -> str:
        return self.choices[0]["message"]["content"] if self.choices else ""


# =============================================================================
# Client - Full-featured Python SDK
# =============================================================================

class AgentAPIClient:
    """Comprehensive agentapi SDK - not just HTTP wrapper.
    
    Provides native Python classes and functions.
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
    
    # -------------------------------------------------------------------------
    # High-level Python functions
    # -------------------------------------------------------------------------
    
    def chat(
        self,
        agent: str,
        prompt: str,
        model: Optional[str] = None,
        **kwargs
    ) -> ChatResponse:
        """Native Python chat - returns ChatResponse object."""
        messages = [{"role": "user", "content": prompt}]
        
        resp = self.completions_create(
            agent=agent,
            messages=messages,
            model=model,
            **kwargs
        )
        return ChatResponse(
            id=resp["id"],
            model=resp["model"],
            choices=resp.get("choices", []),
            usage=resp.get("usage", {})
        )
    
    def complete(self, prompt: str, **kwargs) -> str:
        """Simple completion - returns string."""
        return self.chat("default", prompt, **kwargs).text
    
    # -------------------------------------------------------------------------
    # Mid-level operations
    # -------------------------------------------------------------------------
    
    def rules_set(self, rule: RoutingRule) -> dict:
        """Set routing rule as Python object."""
        return self.admin_rules_set(rule.__dict__)
    
    def rules_get(self, agent: str) -> RoutingRule:
        """Get rule as Python object."""
        rules = self.admin_rules_list()
        for r in rules.get("rules", []):
            if r.get("agent") == agent:
                return RoutingRule(**r)
        return RoutingRule(agent=agent)
    
    # -------------------------------------------------------------------------
    # Low-level HTTP
    # -------------------------------------------------------------------------
    
    def completions_create(self, **kwargs) -> dict:
        """POST /v1/chat/completions."""
        return self._request("POST", "/v1/chat/completions", json=kwargs)
    
    def admin_rules_list(self) -> dict:
        """GET /admin/rules."""
        return self._request("GET", "/admin/rules")
    
    def admin_rules_set(self, rule: dict) -> dict:
        """POST /admin/rules."""
        return self._request("POST", "/admin/rules", json=rule)
    
    def sessions_list(self) -> list[AgentSession]:
        """List sessions as Python objects."""
        data = self._request("GET", "/admin/sessions")
        return [AgentSession(**s) for s in data.get("sessions", [])]
    
    def health(self) -> dict:
        """Health check."""
        return self._request("GET", "/health")
    
    # -------------------------------------------------------------------------
    # HTTP layer
    # -------------------------------------------------------------------------
    
    def _request(
        self,
        method: str,
        path: str,
        **kwargs
    ) -> dict:
        """Base HTTP."""
        url = f"{self.base_url}{path}"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        resp = self._client.request(method, url, headers=headers, **kwargs)
        resp.raise_for_status()
        return resp.json()
    
    def close(self):
        self._client.close()
    
    def __enter__(self):
        return self
    
    def __exit__(self, *args):
        self.close()


# =============================================================================
# Convenience functions
# =============================================================================

def chat(prompt: str, **kwargs) -> str:
    """One-liner chat."""
    with AgentAPIClient() as client:
        return client.complete(prompt, **kwargs)


def session(agent: str) -> AgentAPIClient:
    """Create session-scoped client."""
    return AgentAPIClient()
