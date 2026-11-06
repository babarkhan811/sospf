# sospf-agent (Python SDK, experimental)

A small Python SDK for S-OSPF agents. It models capability registration and
intent submission, and provides MCP/A2A **handoff adapters**. Until the `sospfd`
gRPC/REST endpoint lands, the SDK bridges to the Go resolver through the shared
*scenario* JSON that `sospfctl resolve-intent` consumes.

## Layout

```
sospf_agent/
  agent.py      Capability, Intent, Constraints, Agent (scenario builder)
  adapters.py   Adapter base + MCPAdapter / A2AAdapter (perform a prepared handoff)
example.py      Build a scenario and write it to disk
```

## Use

```python
from sospf_agent import Agent, Capability, Intent, Constraints

agent = Agent()
agent.register(Capability(
    agent_id="seg-1", prefix="medical/imaging/segmentation/seg-1",
    domain="medical", capabilities=["ont:image-segmentation"],
    trust=0.9, latency_ms=20, next_hop=2,
))
intent = Intent("ont:image-segmentation", "medical", ["ont:image-segmentation"],
                Constraints(min_trust=0.7, disclosure_limit=2))
agent.write_scenario("scenario.json", intent)
```

Then resolve it with the Go CLI:

```
sospfctl resolve-intent -scenario scenario.json
```

## Adapters

The Gateway Mediation Layer *prepares* a handoff; the adapters perform it over
the real protocol. The adapters here are thin and must not be mistaken for full
MCP/A2A implementations — they resolve the scoped-token *reference* to a
short-lived credential out of band and invoke the target.

> Experimental. No PyPI release; no network calls. Requires Python 3.10+.
