# Contributing Guide

**Veritie**

This repository follows a lightweight but production-grade Git workflow optimised for MVP velocity while maintaining strong traceability and auditability.

Even though the frontend MVP is currently just me, all rules below are enforced to ensure the codebase remains clean, reviewable, and scalable to a team environment.

---

## 1. Branching Strategy

### Primary Branch

- **`main`**
    - Always deployable
    - Protected
    - No direct commits

There is **no `dev` branch** during the MVP phase.

---

### Short-Lived Branches (Required)

All work must be done on short-lived branches and merged via Pull Request.

#### Branch Naming Convention

Examples:

- `chore/server-foundation`
- `feat/auth-principal-core`
- `refactor/match-obs-metrics`

---

### When to Create a Branch

Create a branch for anything that should be **isolated for review or rollback**, including:

- Features (always)
- Refactors (especially registry, schema, validation, runtime logic)
- Build or tooling changes
- CI configuration
- Design system changes (rare)
- Documentation that changes decisions (ADRs, architectural notes)

Nothing is committed directly to `main`.

---

## 2. Commit Message Standards

This repository uses **Conventional Commits** with **mandatory scopes**.

### Allowed Commit Types

- `feat` — user-facing functionality
- `fix` — bug fixes
- `refactor` — behavior-preserving restructuring
- `perf` — performance improvements
- `chore` — tooling, dependencies, housekeeping
- `docs` — documentation / ADRs
- `test` — tests only
- `ci` — CI/CD configuration
- `build` — build tooling (Next.js config, bundler)
- `style` — formatting only (no logic changes)

> Do not mix logic changes with `style`.

---

### Commit Format

Examples:

- `feat(sdui): render registry-driven layouts`
- `refactor(registry): split schema parsing from rendering`
- `fix(validation): handle unknown component types safely`
- `docs(adr): define schema versioning strategy`

---

### Scope Guidelines

Scopes should reflect **logical ownership**, not file paths.

Common scopes:

- `layout`
- `sdui`
- `registry`
- `schema`
- `validation`
- `ci`
- `build`
- `docs`

---

## 3. Traceability Requirements

Every change must be traceable.

### Issues / Work Items

Each Pull Request must reference a work item using one of:

- `Closes #123`
- `Refs #123`

This establishes a clear chain:

isse -> PR -> Commit -> release -> Deployed SHA

---

### Release Traceability

- Production releases are tagged:
- `v0.x.y`
- CI injects the commit SHA into the build (e.g. `NEXT_PUBLIC_GIT_SHA`)
- Every deployed artifact must map back to a commit

---

## 4. Pull Request Requirements

All merges into `main` **must** go through a Pull Request, even for solo development.

PRs act as:

- Review checkpoints
- Change logs
- Audit records
- Context for future contributors

A PR template is enforced.

### PR Template

#### Intent

What problem does this PR solve? Why is this change necessary?

#### Scope

What areas of the codebase are touched?

#### Risk

What could break as a result of this change? Are there edge cases or backward-compatibility concerns?

#### Test Plan

How was this validated?

- Manual testing steps
- Automated tests
- Screenshots or recordings (for UI changes)... if applicable...

#### Rollout

- Safe to deploy immediately?
- Behind a feature flag?
- Follow-up required?

#### Traceability

Closes / Refs: #

---

## 5. Self-Review & Diff Review Checklist

Before merging a PR, the author must perform a self-review.

Minimum checklist:

- Diff sanity check (no accidental changes)
- Dead code removed
- Naming consistency
- Error / empty / loading states considered
- Schema or contract changes reviewed for compatibility
- No debug logging left behind

This is the primary quality gate during MVP.

---

## 6. Merge Strategy

### Default

- **Squash merge** into `main`

Why:

- Clean history
- One commit per PR
- Clear narrative per change

The squash commit message **must** follow conventional commit rules.

---

## 7. Branch Protection (`main`)

The `main` branch is protected with the following rules:

- No direct commits
- PR required to merge
- Required CI checks:
- Lint
- Typecheck
- Tests (if present)
- Build
- Squash merge only

---

## 8. CI Requirements (MVP)

The CI pipeline is intentionally lightweight.

Required:

- `lint`
- `typecheck`
- `build`
- `test` (where applicable)

Advanced analysis (dead code pruning, bundle analysis) is deferred until post-MVP.

---

## 9. Releases

- Releases are cut from `main`
- Tagged as `v0.x.y`
- Tags represent production-ready snapshots

---

## 10. Summary

- Trunk-based development on `main`
- Short-lived branches for all work
- Conventional commits with enforced scopes
- PRs required for all merges
- Squash merge only
- Lightweight CI
- Strong end-to-end traceability

This workflow prioritises speed during MVP while preventing future cleanup or process rewrites.
