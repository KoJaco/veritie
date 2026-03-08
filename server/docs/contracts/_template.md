# Contract: <Name>

## Purpose

What boundary does this contract define and who/what consumes it?

## Scope

What is included in this contract? What is explicitly out of scope?

## Versioning

-   **Current version:** vX
-   **Compatibility:** Backward compatible | Breaking
-   **Change policy:** When and how versions bump

## Definitions

Define important terms used in the contract (if needed).

## Contract Shape (Conceptual)

Describe the structure at a human level (avoid duplicating code schemas verbatim).

### Required fields

-   `field_name` — purpose, type (conceptually), constraints

### Optional fields

-   `field_name` — purpose, constraints

## Invariants (Must Always Hold)

List rules that must always be true.

-   Unknown component types must be handled safely (no render crash)
-   Validation occurs at the boundary before rendering
-   …

## Error Handling

How are invalid payloads handled? What is rendered/logged/returned?

## Examples

Provide one minimal valid example and (optionally) one invalid example with expected handling.

### Minimal valid example

```json
{
    "example": true
}
```

### Invalid example

```json
{
    "example": "nope"
}
```

### Operational notes

Anything relevant to performance, caching, telemetry, debug, etc.

### References

-   Related ADRs: `docs/adr/XXXX-...`
-   Issue/PR: #
