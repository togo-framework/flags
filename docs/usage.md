# flags — usage

## Define a flag
```go
s, _ := flags.FromKernel(k)
s.Set(flags.Flag{Key: "new-checkout", Enabled: true, Rollout: 25}) // 25% rollout
```
An enabled flag with `Rollout == 0` is treated as 100%.

## Targeting rules
All rules must match the subject's attributes:
```go
s.Set(flags.Flag{Key: "beta", Enabled: true, Rollout: 100,
  Rules: []flags.Rule{{Attribute: "plan", Values: []string{"pro","enterprise"}}}})
```

## Evaluate
```go
subj := flags.Subject{ID: user.ID, Attributes: map[string]string{"plan": user.Plan}}
on := s.Enabled(ctx, "new-checkout", subj)   // deterministic per subject
```

## A/B experiments
```go
s.Set(flags.Flag{Key: "cta", Enabled: true, Rollout: 100, Variants: []string{"control","test"}})
v := s.Variant(ctx, "cta", subj)  // "control" or "test", stable per subject
counts := s.Results("cta")        // {"control": N, "test": M}
```

## Gate a route
```go
k.Router.With(s.Middleware("new-checkout", subjFromReq)).Get("/v2", handler)
```

## REST API (`/api/flags`)
- `GET /api/flags` · `POST /api/flags` · `GET|DELETE /api/flags/{key}`
- `POST /api/flags/{key}/evaluate` → `{enabled, variant}`
- `GET /api/flags/{key}/results` → evaluation counts

## Determinism
Bucketing uses an FNV hash of `"<key>:<subjectID>"` mod 100, so a subject keeps
the same bucket across calls — increasing a rollout only ever adds subjects.
