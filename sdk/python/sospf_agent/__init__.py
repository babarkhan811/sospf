"""sospf-agent: a small Python SDK for S-OSPF agents (experimental).

It models the agent-facing concepts — capability registration, intent
submission — and the MCP/A2A handoff adapters. Until the ``sospfd`` gRPC/REST
endpoint lands (a later phase), the SDK produces the same *scenario* JSON that
``sospfctl resolve-intent`` consumes, so agents and the Go resolver already
interoperate through a shared format.
"""

from .agent import Agent, Capability, Intent, Constraints
from .adapters import Adapter, MCPAdapter, A2AAdapter, HandoffContext

__all__ = [
    "Agent",
    "Capability",
    "Intent",
    "Constraints",
    "Adapter",
    "MCPAdapter",
    "A2AAdapter",
    "HandoffContext",
]

__version__ = "0.1.0"
