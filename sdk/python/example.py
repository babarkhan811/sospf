"""Build an S-OSPF scenario with the Python SDK and write it to disk.

Run:  python sdk/python/example.py > /dev/null && \
      sospfctl resolve-intent -scenario /tmp/py-scenario.json
"""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))

from sospf_agent import Agent, Capability, Intent, Constraints  # noqa: E402


def main() -> None:
    agent = Agent()
    agent.add_subclass("ont:ct-segmentation", ["ont:image-segmentation"])

    agent.register(Capability(
        agent_id="seg-trusted", prefix="medical/imaging/segmentation/seg-trusted",
        domain="medical", capabilities=["ont:image-segmentation"],
        disclosure=1, trust=0.93, igp_cost=2, latency_ms=20, next_hop=2,
    ))
    agent.register(Capability(
        agent_id="ct-subclass", prefix="medical/imaging/segmentation/ct-1",
        domain="medical", capabilities=["ont:ct-segmentation"],
        disclosure=1, trust=0.80, igp_cost=3, latency_ms=30, next_hop=2,
    ))
    agent.register(Capability(
        agent_id="ocr-wrong", prefix="medical/ocr/ocr-9",
        domain="medical", capabilities=["ont:ocr"],
        disclosure=1, trust=0.99, igp_cost=1, latency_ms=5, next_hop=3,
    ))

    intent = Intent(
        task_type_iri="ont:image-segmentation", domain="medical",
        required_capabilities=["ont:image-segmentation"],
        constraints=Constraints(max_latency_ms=50, min_trust=0.70,
                                allowed_domains=["medical"], disclosure_limit=2),
    )

    out = os.path.join(os.path.dirname(__file__), "py-scenario.json")
    agent.write_scenario(out, intent)
    print(f"wrote {out}")
    print("resolve with:  sospfctl resolve-intent -scenario", out)


if __name__ == "__main__":
    main()
