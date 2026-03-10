# Code Comment Style

## Purpose

Keep comments short, useful, and focused on intent where code alone is not obvious.

## Rules

- Prefer no comment when the code is self-explanatory.
- Add comments for:
    - startup/shutdown lifecycle boundaries
    - validation shortcuts or bypass flags
    - non-obvious defaults and fallback behavior
    - temporary scaffolding decisions (for example, no-op backends)
- Keep comments to one sentence where possible.
- Avoid narrating every line or repeating symbol names.

## Tone and Format

- Simple and direct.
- Lowercase sentence fragments are acceptable for inline comments.
- Examples:
    - `// Init core config and deps`
    - `// Parse strongly typed env values first so we fail early on bad process config.`
    - `// JSON logs keep output machine-readable for CI and future log aggregation.`

## Scope

This style applies to the refactor branches unless a package has stricter local conventions.
