# ADR 0002 — Provider abstraction and BYOK

Status: Accepted

## Decision
SynVideo domain behavior is provider-neutral. AI/media/platform providers are accessed through explicit capability interfaces/adapters. BYOK is supported as the initial credential model.

## Why
Model/provider quality, pricing and availability change rapidly. Product semantics must survive provider replacement and future managed-credit billing.

## Consequences
- Core entities do not encode vendor names as domain types.
- Provider-specific configuration lives at boundaries.
- Provider capabilities are explicit; UI must not assume every provider supports identical options.
- Secrets must never be exposed to client surfaces that do not need them.
