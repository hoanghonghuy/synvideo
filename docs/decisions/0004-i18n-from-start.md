# ADR 0004 — i18n architecture from the start

Status: Accepted

## Decision
All user-facing UI copy must be compatible with localization infrastructure from initial implementation. Vietnamese may be the first fully authored locale; English follows without architectural retrofit.

## Why
Hard-coded UI text creates expensive cleanup and inconsistent product language later.

## Consequences
- Components use message keys/resources instead of scattered literal UI copy.
- Validation/error messages intended for users follow the same approach.
- Internal logs/debug strings do not need localization.
