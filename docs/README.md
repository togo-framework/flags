# flags — docs

The `togo-framework/flags` plugin provides feature flags + A/B testing for togo.

- **Install:** `togo install togo-framework/flags`
- **Marketplace:** https://to-go.dev/plugins/flags
- **Source:** https://github.com/togo-framework/flags

## Contents
- [Usage](usage.md) — defining flags, rollouts, targeting, A/B experiments, REST

## Overview
Define flags (on/off + percentage rollout + targeting rules), evaluate per
subject with deterministic bucketing, and run A/B experiments via variants.
Resolve the service with `flags.FromKernel(k)` or use the `/api/flags` REST API.
