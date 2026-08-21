package main

import "net/http"

// ruleFixtureHandler is intentionally local-only; Task 9 reuses it when API
// flows need a deterministic downloadable rule set.
func ruleFixtureHandler() http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`{"version":1,"rules":[]}`))
  })
}
