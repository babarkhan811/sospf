"""Agent-facing data model and scenario builder.

Status values and disclosure levels mirror the Go ``internal/semantic`` wire
constants (WIRE_FORMAT §5.1), so a capability built here encodes to the same
meaning the router expects.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Optional

# Agent status (WIRE_FORMAT §5.1).
STATUS_ACTIVE = 0
STATUS_PENDING = 1
STATUS_SUSPENDED = 2
STATUS_UNREACHABLE = 3

# Disclosure levels.
DISCLOSURE_PUBLIC = 0
DISCLOSURE_DOMAIN = 1
DISCLOSURE_TENANT = 2
DISCLOSURE_RESTRICTED = 3


@dataclass
class Capability:
    """A capability profile an agent advertises."""

    agent_id: str
    prefix: str  # e.g. "medical/imaging/segmentation/agent-1"
    domain: str
    capabilities: list[str]
    status: int = STATUS_ACTIVE
    disclosure: int = DISCLOSURE_DOMAIN
    # Live metrics the router/gateway attaches; defaults are placeholders.
    trust: float = 0.5
    igp_cost: int = 1
    latency_ms: int = 10
    load: float = 0.1
    next_hop: int = 0
    embedding: Optional[list[float]] = None

    def to_scenario_agent(self) -> dict:
        d = {
            "agent_id": self.agent_id,
            "prefix": self.prefix,
            "domain": self.domain,
            "status": self.status,
            "disclosure": self.disclosure,
            "capabilities": self.capabilities,
            "trust": self.trust,
            "igp_cost": self.igp_cost,
            "latency_ms": self.latency_ms,
            "load": self.load,
            "next_hop": self.next_hop,
        }
        if self.embedding is not None:
            d["embedding"] = self.embedding
        return d


@dataclass
class Constraints:
    max_latency_ms: int = 0
    min_trust: float = 0.0
    allowed_domains: list[str] = field(default_factory=list)
    disclosure_limit: int = DISCLOSURE_RESTRICTED


@dataclass
class Intent:
    """An intent an agent submits (WIRE_FORMAT §5.6)."""

    task_type_iri: str
    domain: str
    required_capabilities: list[str]
    constraints: Constraints = field(default_factory=Constraints)
    embedding: Optional[list[float]] = None

    def to_scenario_intent(self) -> dict:
        d = {
            "task_type_iri": self.task_type_iri,
            "domain": self.domain,
            "required_capabilities": self.required_capabilities,
            "max_latency_ms": self.constraints.max_latency_ms,
            "min_trust": self.constraints.min_trust,
            "allowed_domains": self.constraints.allowed_domains,
            "disclosure_limit": self.constraints.disclosure_limit,
        }
        if self.embedding is not None:
            d["embedding"] = self.embedding
        return d


class Agent:
    """A local agent registry + scenario builder.

    Register the capabilities visible to this router, set an intent, then write
    a scenario file that ``sospfctl resolve-intent -scenario <file>`` resolves.
    """

    def __init__(self) -> None:
        self._caps: list[Capability] = []
        self._subclasses: dict[str, list[str]] = {}

    def register(self, cap: Capability) -> None:
        self._caps.append(cap)

    def add_subclass(self, child_iri: str, ancestor_iris: list[str]) -> None:
        self._subclasses[child_iri] = ancestor_iris

    def scenario(self, intent: Intent) -> dict:
        return {
            "intent": intent.to_scenario_intent(),
            "subclasses": self._subclasses,
            "agents": [c.to_scenario_agent() for c in self._caps],
        }

    def write_scenario(self, path: str, intent: Intent) -> None:
        with open(path, "w", encoding="utf-8") as fh:
            json.dump(self.scenario(intent), fh, indent=2)
