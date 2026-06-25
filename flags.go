// Package flags is a feature-flag + A/B-testing plugin for togo (Laravel
// Pennant / Flipper / django-waffle for Go).
//
// Define flags with a master on/off, a percentage rollout, and targeting rules;
// evaluate them per subject with deterministic bucketing (a subject stays in
// the same bucket across calls). Flags with variants drive A/B experiments.
//
//	s, _ := flags.FromKernel(k)
//	s.Set(flags.Flag{Key: "new-checkout", Enabled: true, Rollout: 25})
//	if s.Enabled(ctx, "new-checkout", flags.Subject{ID: user.ID}) { ... }
package flags

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/togo-framework/togo"
)

// Subject is who a flag is evaluated for.
type Subject struct {
	ID         string            `json:"id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Rule is a targeting rule: the subject's Attribute must equal one of Values.
// A flag matches only when ALL its rules match.
type Rule struct {
	Attribute string   `json:"attribute"`
	Values    []string `json:"values"`
}

// Flag is a feature-flag definition.
type Flag struct {
	Key         string   `json:"key"`
	Enabled     bool     `json:"enabled"`             // master on/off
	Rollout     int      `json:"rollout"`             // 0-100 percentage when enabled
	Rules       []Rule   `json:"rules,omitempty"`     // targeting (all must match)
	Variants    []string `json:"variants,omitempty"`  // A/B variants (deterministic)
	Description string   `json:"description,omitempty"`
}

// Service is the flags runtime stored on the kernel (k.Get("flags")).
type Service struct {
	k     *togo.Kernel
	mu    sync.RWMutex
	flags map[string]*Flag
	evals map[string]map[string]int64 // key -> result -> count
	store Store // optional durable backing (write-through)
}

func init() {
	togo.RegisterProviderFunc("flags", togo.PriorityLate+10, func(k *togo.Kernel) error {
		s := &Service{k: k, flags: map[string]*Flag{}, evals: map[string]map[string]int64{}}
		k.Set("flags", s)
		if k.Router != nil {
			s.mountRoutes(k.Router)
		}
		return nil
	})
}

// FromKernel returns the flags Service registered on the kernel.
func FromKernel(k *togo.Kernel) (*Service, bool) {
	v, ok := k.Get("flags")
	if !ok {
		return nil, false
	}
	s, ok := v.(*Service)
	return s, ok
}

// Set defines or replaces a flag. An enabled flag with Rollout==0 is treated
// as 100% (fully on); set Rollout explicitly for a partial rollout.
func (s *Service) Set(f Flag) {
	if f.Enabled && f.Rollout == 0 {
		f.Rollout = 100
	}
	if f.Rollout < 0 {
		f.Rollout = 0
	} else if f.Rollout > 100 {
		f.Rollout = 100
	}
	s.mu.Lock()
	s.flags[f.Key] = &f
	if s.store != nil {
		s.store.Save(f)
	}
	s.mu.Unlock()
}

// Get returns a flag definition.
func (s *Service) Get(key string) (Flag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flags[key]
	if !ok {
		return Flag{}, false
	}
	return *f, true
}

// All returns every flag, sorted by key.
func (s *Service) All() []Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Flag, 0, len(s.flags))
	for _, f := range s.flags {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Delete removes a flag.
func (s *Service) Delete(key string) {
	s.mu.Lock()
	delete(s.flags, key)
	if s.store != nil {
		s.store.Delete(key)
	}
	s.mu.Unlock()
}

// Enabled evaluates a flag for a subject. Deterministic: the same (key,subject)
// always yields the same bucket, so a partial rollout is stable per subject.
func (s *Service) Enabled(ctx context.Context, key string, subj Subject) bool {
	s.mu.RLock()
	f, ok := s.flags[key]
	s.mu.RUnlock()

	res := false
	if ok && f.Enabled && matchRules(f.Rules, subj) {
		switch {
		case f.Rollout >= 100:
			res = true
		case f.Rollout > 0:
			res = bucket(key, subj.ID) < f.Rollout
		}
	}
	s.recordEval(key, strconv.FormatBool(res))
	return res
}

// Variant returns the deterministic A/B variant for a subject, or "" when the
// flag is missing/disabled, has no variants, or the subject doesn't match.
func (s *Service) Variant(ctx context.Context, key string, subj Subject) string {
	s.mu.RLock()
	f, ok := s.flags[key]
	s.mu.RUnlock()
	if !ok || !f.Enabled || len(f.Variants) == 0 || !matchRules(f.Rules, subj) {
		return ""
	}
	v := f.Variants[int(hashU32(key+":variant:"+subj.ID)%uint32(len(f.Variants)))]
	s.recordEval(key, v)
	return v
}

// Results returns the recorded evaluation counts for a flag (result -> count),
// e.g. {"true": 120, "false": 380} or per-variant counts for A/B experiments.
func (s *Service) Results(key string) map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]int64{}
	for r, c := range s.evals[key] {
		out[r] = c
	}
	return out
}

func matchRules(rules []Rule, subj Subject) bool {
	for _, r := range rules {
		got := subj.Attributes[r.Attribute]
		hit := false
		for _, v := range r.Values {
			if v == got {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// bucket maps (flag, subject) deterministically to 0..99.
func bucket(key, id string) int { return int(hashU32(key+":"+id) % 100) }

func hashU32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func (s *Service) recordEval(key, result string) {
	s.mu.Lock()
	if s.evals[key] == nil {
		s.evals[key] = map[string]int64{}
	}
	s.evals[key][result]++
	s.mu.Unlock()
}

// Middleware gates a handler behind a flag. subjectFn extracts the Subject from
// the request (e.g. the authenticated user); when the flag is off it responds
// 404. Use to dark-launch routes.
func (s *Service) Middleware(key string, subjectFn func(*http.Request) Subject) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.Enabled(r.Context(), key, subjectFn(r)) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- REST admin + evaluation API ---

func (s *Service) mountRoutes(r chi.Router) {
	r.Route("/api/flags", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, s.All()) })
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var f Flag
			if err := json.NewDecoder(req.Body).Decode(&f); err != nil || f.Key == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid flag (key required)"})
				return
			}
			s.Set(f)
			writeJSON(w, 200, f)
		})
		r.Get("/{key}", func(w http.ResponseWriter, req *http.Request) {
			if f, ok := s.Get(chi.URLParam(req, "key")); ok {
				writeJSON(w, 200, f)
			} else {
				writeJSON(w, 404, map[string]string{"error": "not found"})
			}
		})
		r.Delete("/{key}", func(w http.ResponseWriter, req *http.Request) {
			s.Delete(chi.URLParam(req, "key"))
			writeJSON(w, 200, map[string]string{"status": "deleted"})
		})
		r.Post("/{key}/evaluate", func(w http.ResponseWriter, req *http.Request) {
			var subj Subject
			_ = json.NewDecoder(req.Body).Decode(&subj)
			key := chi.URLParam(req, "key")
			writeJSON(w, 200, map[string]any{
				"enabled": s.Enabled(req.Context(), key, subj),
				"variant": s.Variant(req.Context(), key, subj),
			})
		})
		r.Get("/{key}/results", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, 200, s.Results(chi.URLParam(req, "key")))
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
