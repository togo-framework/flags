---
name: flags
description: Create, roll out, target, and A/B-test feature flags in a togo app using the flags plugin (percentage rollouts, targeting rules, variant experiments, evaluation API).
---

# togo feature flags

Use this skill to work with the `togo-framework/flags` plugin.

## Define / roll out
```go
s, _ := flags.FromKernel(k)
s.Set(flags.Flag{Key: "<key>", Enabled: true, Rollout: 25})        // 25% rollout
```
Increase `Rollout` over time — bucketing is deterministic, so it only adds subjects.

## Target
```go
s.Set(flags.Flag{Key: "<key>", Enabled: true, Rollout: 100,
  Rules: []flags.Rule{{Attribute: "plan", Values: []string{"pro"}}}})
```

## Evaluate
```go
s.Enabled(ctx, "<key>", flags.Subject{ID: userID, Attributes: attrs})
```

## A/B test
```go
s.Set(flags.Flag{Key: "<key>", Enabled: true, Rollout: 100, Variants: []string{"control","test"}})
s.Variant(ctx, "<key>", subj)   // stable per subject
s.Results("<key>")              // counts per result/variant
```

## REST: `/api/flags` (list/create/get/delete), `/{key}/evaluate`, `/{key}/results`.

## Tips
- Prefer rolling out gradually (5 → 25 → 50 → 100) and watch `Results`.
- Use targeting rules for plan/region/beta cohorts; keep flag keys descriptive.
- Gate routes with `s.Middleware(key, subjectFn)` to dark-launch endpoints.
