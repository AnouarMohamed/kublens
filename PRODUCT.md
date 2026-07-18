# KubeLens Product Context

## Register

product

## Product Purpose

KubeLens is an incident and change-risk copilot for Kubernetes operators. It helps an SRE move from live signals to evidence, simulation, governed remediation, and incident records without switching between a dozen tools.

## Primary Users

- On-call SREs triaging degraded services under time pressure.
- Platform engineers reviewing risky Kubernetes changes before they reach production.
- Engineering managers and incident commanders who need a concise evidence trail after an event.

## Product Promise

KubeLens should make risky cluster decisions more explainable and safer. The product is only credible when every recommendation is backed by deterministic evidence, explicit confidence, visible limitations, audit history, and a reversible GitOps artifact.

## Strategic Principles

1. The main workflow is detect, simulate, explain, remediate, export.
2. Deterministic evidence is the floor. AI and ML may enrich the workflow, but must not hide weak signals or lower deterministic risk without governance.
3. Simulation is valuable only when confidence and known limitations are visible.
4. Writes are always governed: RBAC, write gate, four-eyes policy in production, audit, and rollback context.
5. Enterprise readiness means durable storage, observable health, verifiable audit records, documented operations, and honest feature maturity labels.

## Anti-References

- A generic Kubernetes dashboard that simply restyles `kubectl`.
- An AI chatbot that gives unsupported operational advice.
- A remediation bot that executes changes before trust has been earned.
- A dense collection of equal-weight views with no primary workflow.

## Tone

Direct, calm, operational, and evidence-first. The interface should read like a reliable control room, not a marketing demo.
