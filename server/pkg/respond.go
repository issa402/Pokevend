// ============================================================
// FILE: server/pkg/respond.go
// TYPE: Shared Utility — HTTP Response Helpers
//
// WHAT IS THE pkg/ PACKAGE?
// "pkg" = shared utilities used by MORE THAN ONE package.
// Rule: if only ONE package uses it → put it in that package.
//       if TWO+ packages need it → put it in pkg/ (Don't Repeat Yourself).
//
// WHY RESPONSE HELPERS?
// Without helpers, every handler writes the same boilerplate:
//   w.Header().Set("Content-Type", "application/json")
//   w.WriteHeader(status)
//   json.NewEncoder(w).Encode(body)
// That's 3 lines repeated in every handler (20 handlers × 2 responses = 120 lines).
// With helpers: pkg.JSON(w, 200, data) — compact and consistent.
//
// FAANG PRINCIPLE: DRY (Don't Repeat Yourself)
// If you find yourself writing the same code in multiple places,
// extract it into a shared helper. This is one of the most important
// engineering principles — repeated code = bugs in multiple places.
//
// GO CONCEPTS:
//   json.NewEncoder(w).Encode(v) — stream JSON to ResponseWriter
//   w.Header().Set() — must be called BEFORE WriteHeader
//   http.ResponseWriter interface
// ============================================================
package pkg

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given HTTP status code.
// This is called in every handler for successful responses.
//
// GO NOTE: w.Header().Set() MUST come before w.WriteHeader().
// Once you call WriteHeader (or write any body), headers are locked.
// This is a common Go beginner mistake: setting headers after writing = silently ignored.
func JSON(w http.ResponseWriter, status int, v interface{}) {
	// Set the Content-Type header so the client (browser/React) knows it's JSON
	w.Header().Set("Content-Type", "application/json")
	// Write the HTTP status code line (e.g., "HTTP/1.1 200 OK")
	w.WriteHeader(status)
	// json.NewEncoder(w).Encode(v) = serialize v to JSON and stream to response body
	// More efficient than json.Marshal → json.Write because it doesn't buffer
	json.NewEncoder(w).Encode(v)
}

// Error writes a JSON error response.
// Standardizes error format across all handlers.
//
// ALL errors in this API look like: {"error": "some message"}
// This consistency is critical for the frontend — it can always
// check response.error to show the right message to users.
//
// interface{} = accepts any type (we pass string messages here,
// but could pass structured error objects in the future)
func Error(w http.ResponseWriter, status int, message interface{}) {
	JSON(w, status, map[string]interface{}{"error": message})
}

// TODO #1 (Practice): Add a NoContent helper
// Some API actions return no body — they just signal success/failure.
// Examples: DELETE /api/alerts/{id}, POST /api/alerts/read-all
// Add: func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }
// HTTP 204 No Content = success with no response body.
// The frontend should handle 204 responses differently than 200+JSON.

// TODO #2 (Practice): Add response envelope wrapping
// Many APIs wrap responses consistently:
//   {"data": {...}, "meta": {"requestId": "abc", "timestamp": "..."}}
// This makes logging and debugging much easier in production.
// Add: func Envelope(w http.ResponseWriter, status int, data interface{})
// The envelope should include: data, meta.timestamp, meta.status_code
// Then update all handlers to use Envelope instead of JSON.
// Research: "JSON API spec" — a popular standard for REST API response format
