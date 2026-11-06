"""Handoff adapters.

The Gateway Mediation Layer *prepares* a handoff; it does not perform the
protocol (draft-yang §6 non-goal). These adapters are where an agent actually
reaches the selected target over MCP, A2A, IAIP or HTTP. The base class defines
the contract; the concrete adapters are deliberately thin and must not be
treated as full MCP/A2A implementations.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class HandoffContext:
    """Mirror of the Go gateway.HandoffContext (draft-yang -02 §10.2)."""

    task_meaning: str
    context_summary: str  # already redacted by the gateway
    mechanism: str  # "MCP" | "A2A" | "IAIP" | "HTTP"
    target_ref: str
    scoped_token_ref: str  # a reference, never the token itself
    disclosure_limit: int = 0


class Adapter:
    """Performs a handoff prepared by the gateway."""

    mechanism = "HTTP"

    def perform(self, hc: HandoffContext) -> dict:
        raise NotImplementedError

    def _check(self, hc: HandoffContext) -> None:
        if hc.mechanism != self.mechanism:
            raise ValueError(f"{self.mechanism} adapter got a {hc.mechanism} handoff")


class MCPAdapter(Adapter):
    mechanism = "MCP"

    def perform(self, hc: HandoffContext) -> dict:
        self._check(hc)
        # A real implementation would open an MCP session to the target and
        # invoke the tool named by task_meaning, resolving scoped_token_ref to a
        # short-lived credential out of band. Here we only describe the call.
        return {
            "adapter": "mcp",
            "target": hc.target_ref,
            "tool": hc.task_meaning,
            "status": "prepared",
        }


class A2AAdapter(Adapter):
    mechanism = "A2A"

    def perform(self, hc: HandoffContext) -> dict:
        self._check(hc)
        return {
            "adapter": "a2a",
            "target": hc.target_ref,
            "skill": hc.task_meaning,
            "status": "prepared",
        }
