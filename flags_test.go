package flags

import (
	"context"
	"testing"
)

func newService() *Service {
	return &Service{flags: map[string]*Flag{}, evals: map[string]map[string]int64{}}
}

func TestOnOff(t *testing.T) {
	s := newService()
	ctx := context.Background()
	s.Set(Flag{Key: "f", Enabled: false})
	if s.Enabled(ctx, "f", Subject{ID: "u1"}) {
		t.Error("disabled flag should be off")
	}
	if s.Enabled(ctx, "missing", Subject{ID: "u1"}) {
		t.Error("undefined flag should be off")
	}
	s.Set(Flag{Key: "f", Enabled: true}) // enabled, no rollout → 100%
	if got, _ := s.Get("f"); got.Rollout != 100 {
		t.Fatalf("enabled flag should default to 100%% rollout, got %d", got.Rollout)
	}
	if !s.Enabled(ctx, "f", Subject{ID: "u1"}) {
		t.Error("fully-enabled flag should be on")
	}
}

func TestPercentageDeterministicAndDistributed(t *testing.T) {
	s := newService()
	ctx := context.Background()
	s.Set(Flag{Key: "p", Enabled: true, Rollout: 30})

	// Deterministic: same subject → same result across many calls.
	first := s.Enabled(ctx, "p", Subject{ID: "stable-user"})
	for i := 0; i < 50; i++ {
		if s.Enabled(ctx, "p", Subject{ID: "stable-user"}) != first {
			t.Fatal("evaluation is not deterministic for a subject")
		}
	}

	// Distribution: ~30% of many subjects enabled (allow a wide band).
	on := 0
	const n = 4000
	for i := 0; i < n; i++ {
		if s.Enabled(ctx, "p", Subject{ID: "user-" + itoa(i)}) {
			on++
		}
	}
	pct := float64(on) / float64(n) * 100
	if pct < 24 || pct > 36 {
		t.Errorf("30%% rollout produced %.1f%% (expected ~30)", pct)
	}
}

func TestTargetingRules(t *testing.T) {
	s := newService()
	ctx := context.Background()
	s.Set(Flag{
		Key:     "beta",
		Enabled: true,
		Rollout: 100,
		Rules:   []Rule{{Attribute: "plan", Values: []string{"pro", "enterprise"}}},
	})
	pro := Subject{ID: "u1", Attributes: map[string]string{"plan": "pro"}}
	free := Subject{ID: "u2", Attributes: map[string]string{"plan": "free"}}
	if !s.Enabled(ctx, "beta", pro) {
		t.Error("pro subject should match the targeting rule")
	}
	if s.Enabled(ctx, "beta", free) {
		t.Error("free subject should NOT match the targeting rule")
	}
}

func TestVariantAssignment(t *testing.T) {
	s := newService()
	ctx := context.Background()
	s.Set(Flag{Key: "exp", Enabled: true, Rollout: 100, Variants: []string{"control", "test"}})

	v := s.Variant(ctx, "exp", Subject{ID: "u1"})
	if v != "control" && v != "test" {
		t.Fatalf("variant %q not in set", v)
	}
	// Deterministic per subject.
	for i := 0; i < 20; i++ {
		if s.Variant(ctx, "exp", Subject{ID: "u1"}) != v {
			t.Fatal("variant assignment not deterministic")
		}
	}
	// Both variants appear across subjects.
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[s.Variant(ctx, "exp", Subject{ID: "u-" + itoa(i)})] = true
	}
	if !seen["control"] || !seen["test"] {
		t.Errorf("expected both variants across subjects, saw %v", seen)
	}
	// No variant when disabled.
	if s.Variant(ctx, "missing", Subject{ID: "u1"}) != "" {
		t.Error("missing flag should yield empty variant")
	}
}

func TestResults(t *testing.T) {
	s := newService()
	ctx := context.Background()
	s.Set(Flag{Key: "r", Enabled: true, Rollout: 100})
	s.Enabled(ctx, "r", Subject{ID: "a"})
	s.Enabled(ctx, "r", Subject{ID: "b"})
	if got := s.Results("r")["true"]; got != 2 {
		t.Errorf("expected 2 true evaluations recorded, got %d", got)
	}
}

func TestAllAndDelete(t *testing.T) {
	s := newService()
	s.Set(Flag{Key: "b", Enabled: true})
	s.Set(Flag{Key: "a", Enabled: true})
	all := s.All()
	if len(all) != 2 || all[0].Key != "a" || all[1].Key != "b" {
		t.Fatalf("All() not sorted/complete: %+v", all)
	}
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Error("deleted flag still present")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
