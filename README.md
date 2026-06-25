<div align="center">
  <img src=".github/assets/togo-mark.svg" alt="togo" height="64" />
  <h1>togo-framework/flags</h1>
  <p>
    <a href="https://to-go.dev/marketplace"><img src="https://img.shields.io/badge/marketplace-to--go.dev-1FC7DC" alt="marketplace" /></a>
    <a href="https://pkg.go.dev/github.com/togo-framework/flags"><img src="https://pkg.go.dev/badge/github.com/togo-framework/flags.svg" alt="pkg.go.dev" /></a>
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT" />
  </p>
  <p><strong>Feature flags + A/B testing for <a href="https://to-go.dev">togo</a> — rollouts, targeting, experiments.</strong></p>
</div>

## Install

```bash
togo install togo-framework/flags
```

`flags` is the togo answer to Laravel Pennant / Flipper / django-waffle. Define flags with a master on/off, a **percentage rollout**, and **targeting rules**, then evaluate them per subject with **deterministic bucketing** — a subject stays in the same bucket across calls, so partial rollouts are stable. Flags with **variants** drive A/B experiments.

## Usage

```go
import "github.com/togo-framework/flags"

s, _ := flags.FromKernel(k)

// A 25% rollout of a new checkout.
s.Set(flags.Flag{Key: "new-checkout", Enabled: true, Rollout: 25})

// Targeted to pro/enterprise plans only.
s.Set(flags.Flag{
    Key:     "beta-api",
    Enabled: true, Rollout: 100,
    Rules:   []flags.Rule{{Attribute: "plan", Values: []string{"pro", "enterprise"}}},
})

// An A/B experiment.
s.Set(flags.Flag{Key: "cta-copy", Enabled: true, Rollout: 100,
    Variants: []string{"control", "test"}})

// Evaluate (deterministic per subject):
subj := flags.Subject{ID: user.ID, Attributes: map[string]string{"plan": user.Plan}}
if s.Enabled(ctx, "new-checkout", subj) {
    // ...new flow
}
variant := s.Variant(ctx, "cta-copy", subj) // "control" or "test"
```

### Gate a route behind a flag

```go
k.Router.With(s.Middleware("new-checkout", func(r *http.Request) flags.Subject {
    return flags.Subject{ID: userID(r)}
})).Get("/checkout/v2", handler) // 404 when the flag is off
```

## REST API

Mounted automatically under `/api/flags`:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/flags` | list flags |
| `POST` | `/api/flags` | create/replace a flag (JSON body) |
| `GET` | `/api/flags/{key}` | get a flag |
| `DELETE` | `/api/flags/{key}` | delete a flag |
| `POST` | `/api/flags/{key}/evaluate` | evaluate for a subject → `{enabled, variant}` |
| `GET` | `/api/flags/{key}/results` | evaluation counts (A/B results) |

## How evaluation works

1. Flag missing or `enabled=false` → **off**.
2. Targeting `rules` must **all** match the subject's attributes.
3. `rollout >= 100` → on; otherwise the subject's deterministic bucket (`0–99`, FNV hash of `key:subjectID`) must be `< rollout`.
4. `Variant` assigns one of `variants` deterministically per subject for A/B tests.

Every evaluation is counted (`Results(key)`) so you can read experiment outcomes.

## Configuration

No required env. Flags live in the kernel service (in-memory, fast); manage them via the Go API or the REST endpoints. The `flags` service registers at `PriorityService` and mounts its routes on boot.

---

<div align="center">
  <h3>Premium sponsors</h3>
  <p>
    <a href="https://id8media.com"><strong>ID8 Media</strong></a> &nbsp;·&nbsp;
    <a href="https://one-studio.co"><strong>One Studio</strong></a>
  </p>
  <p><sub>Support togo — <a href="https://github.com/sponsors/fadymondy">become a sponsor</a>.</sub></p>
</div>
