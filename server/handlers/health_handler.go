// Package handlers contains HTTP handlers: the functions that receive web
// requests, call the right application logic, and write HTTP responses.
package handlers

import (
	"fmt"      // formats strings; used to build SQL and Prometheus text lines
	"net/http" // Go's standard HTTP request/response package
	"time"     // used to put a generated_at timestamp on freshness responses

	"pokemontool/pkg" // local response helpers for JSON and error responses

	"github.com/jackc/pgx/v5/pgtype"  // nullable PostgreSQL value types
	"github.com/jackc/pgx/v5/pgxpool" // PostgreSQL connection pool
	"github.com/redis/go-redis/v9"    // Redis client
)

// HealthHandler is the object that owns health-related HTTP endpoints.
//
// Handler intuition:
// A handler sits at the edge of your service. It receives an HTTP request,
// uses dependencies like Postgres/Redis, then writes an HTTP response.
type HealthHandler struct {
	db  *pgxpool.Pool // db is the shared Postgres connection pool
	rdb *redis.Client // rdb is the Redis client
}

// NewHealthHandler builds a HealthHandler with the dependencies it needs.
//
// This is dependency injection: main.go creates db/redis once, then hands them
// to the handler instead of the handler secretly creating its own connections.
func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb} // return a pointer so routes can call methods on it
}

// Check answers: "Can the API reach its core dependencies?"
//
// This is process health. It does not prove the business data is fresh.
// It only proves the API can reach Postgres and Redis right now.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	// r.Context() carries cancellation/timeouts from the HTTP request into DB work.
	if err := h.db.Ping(r.Context()); err != nil {
		// If Postgres cannot be reached, the service is unavailable.
		pkg.Error(w, http.StatusServiceUnavailable, "database down")
		return
	}

	// Redis Ping checks whether the Redis dependency responds.
	if err := h.rdb.Ping(r.Context()).Err(); err != nil {
		// If Redis cannot be reached, the health endpoint reports unavailable.
		pkg.Error(w, http.StatusServiceUnavailable, "redis down")
		return
	}

	// If both dependency checks passed, return a small JSON success response.
	pkg.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",             // machine-readable health result
		"service": "pokemontool-go", // identifies which service answered
	})
}

// freshnessTarget defines one source of business data we want to inspect.
//
// This is not the output. This is the configuration/input model:
// "Which table should we check, which timestamp column matters, and what
// thresholds decide whether the data is trustworthy?"
type freshnessTarget struct {
	Name            string // friendly check name, like "price_history"
	Table           string // database table to query
	Column          string // timestamp column that represents freshness
	WarningMinutes  int    // age where confidence should drop
	CriticalMinutes int    // age where automated decisions should not trust it
	BusinessRisk    string // human explanation of what stale data breaks
}

// freshnessCheck is one result returned to users and operators.
//
// This is the output model for a single freshness target:
// "Here is what we found when we checked this table."
type freshnessCheck struct {
	Name            string   `json:"name"`             // same friendly name from freshnessTarget
	Table           string   `json:"table"`            // table that was checked
	TimestampColumn string   `json:"timestamp_column"` // timestamp column that was checked
	WarningMinutes  int      `json:"warning_minutes"`  // warning threshold copied into output
	CriticalMinutes int      `json:"critical_minutes"` // critical threshold copied into output
	NewestSeen      *string  `json:"newest_seen"`      // newest timestamp found, nil when table has no rows
	AgeMinutes      *float64 `json:"age_minutes"`      // age of newest timestamp, nil when there is no data
	Status          string   `json:"status"`           // fresh, warning, critical, or no_data
	BusinessRisk    string   `json:"business_risk"`    // why this table matters to the business
}

// freshnessResponse is the complete JSON body for GET /api/health/freshness.
//
// It wraps all individual checks with service-level metadata.
type freshnessResponse struct {
	Service       string           `json:"service"`        // service name, here "pokemon"
	Check         string           `json:"check"`          // check type, here "data_freshness"
	OverallStatus string           `json:"overall_status"` // collapsed result across all checks
	GeneratedAt   string           `json:"generated_at"`   // UTC time when this response was produced
	Checks        []freshnessCheck `json:"checks"`         // one result per freshness target
}

// freshnessTargets returns the list of business-data sources we care about.
//
// Your intuition is right: before we can check freshness, we model what
// "freshness" means for each data source.
func freshnessTargets() []freshnessTarget {
	return []freshnessTarget{
		{
			Name:            "price_history",                                          // market price history signal
			Table:           "price_history",                                          // SQL table name
			Column:          "created_at",                                             // newest row creation time means newest price history
			WarningMinutes:  60,                                                       // after 1 hour, confidence drops
			CriticalMinutes: 180,                                                      // after 3 hours, too stale for automation
			BusinessRisk:    "Pricing and trend decisions may use stale market data.", // why operators should care
		},
		{
			Name:            "card_listings",                                         // marketplace listing ingestion signal
			Table:           "card_listings",                                         // SQL table name
			Column:          "discovered_at",                                         // when this listing was discovered
			WarningMinutes:  60,                                                      // after 1 hour, opportunity detection is suspicious
			CriticalMinutes: 240,                                                     // after 4 hours, listing data is too old
			BusinessRisk:    "Deal detection may miss current market opportunities.", // why operators should care
		},
		{
			Name:            "alerts",                                     // user alert creation signal
			Table:           "alerts",                                     // SQL table name
			Column:          "created_at",                                 // when latest alert was created
			WarningMinutes:  1440,                                         // after 1 day, alerting may be quiet
			CriticalMinutes: 2880,                                         // after 2 days, alerting may be inactive
			BusinessRisk:    "User alerting may be inactive or unproven.", // why operators should care
		},
		{
			Name:            "deals",                                         // deal-of-the-day generation signal
			Table:           "deals",                                         // SQL table name
			Column:          "created_at",                                    // when latest deal row was created
			WarningMinutes:  1440,                                            // after 1 day, daily deal freshness is questionable
			CriticalMinutes: 2880,                                            // after 2 days, daily deals are stale
			BusinessRisk:    "Deal-of-the-day recommendations may be stale.", // why operators should care
		},
		{
			Name:            "cards",                                              // card catalog pricing/trend signal
			Table:           "cards",                                              // SQL table name
			Column:          "last_updated",                                       // when card data was last updated
			WarningMinutes:  60,                                                   // after 1 hour, card facts may be stale
			CriticalMinutes: 180,                                                  // after 3 hours, too stale for automation
			BusinessRisk:    "Current card prices and trend labels may be stale.", // why operators should care
		},
	}
}

// freshnessStatus converts a numeric age into a business status.
//
// This function is the policy/rule engine for one check.
func freshnessStatus(ageMinutes float64, warningMinutes int, criticalMinutes int) string {
	// If age is greater than the critical threshold, automated trust should stop.
	if ageMinutes > float64(criticalMinutes) {
		return "critical"
	}

	// If age is greater than the warning threshold, confidence should drop.
	if ageMinutes > float64(warningMinutes) {
		return "warning"
	}

	// If age is within the warning window, this data source is fresh.
	return "fresh"
}

// overallFreshnessStatus collapses many table-level checks into one service status.
//
// This is the executive summary: one red/yellow/green signal for Pokemon data.
func overallFreshnessStatus(checks []freshnessCheck) string {
	overall := "fresh" // optimistic default; downgrade as checks reveal risk

	// Look at every individual freshness check.
	for _, check := range checks {
		// Critical, missing table, and no_data all mean the platform cannot fully trust the data.
		if check.Status == "critical" || check.Status == "missing_table" || check.Status == "no_data" {
			return "critical"
		}

		// Warning does not immediately stop the loop because a later check might be critical.
		if check.Status == "warning" {
			overall = "warning"
		}
	}

	// Return fresh if nothing was bad, or warning if at least one check warned.
	return overall
}

// freshnessStatusValue turns business words into Prometheus-friendly numbers.
//
// Prometheus stores numeric time series. Labels can hold words like "warning",
// but alert rules compare numbers more naturally.
func freshnessStatusValue(status string) float64 {
	// switch is a clean way to map a small set of string states to numeric codes.
	switch status {
	case "fresh":
		return 0 // 0 means healthy/fresh
	case "warning":
		return 1 // 1 means degraded confidence
	case "critical", "no_data", "missing_table":
		return 2 // 2 means unsafe/stale/no trustworthy data
	default:
		return 3 // 3 means unexpected state, useful for catching bugs
	}
}

// Freshness answers: "Is Pokemon business data fresh enough to trust?"
//
// This is different from /health:
// /health proves the API can reach DB/Redis.
// /api/health/freshness proves the decision data is recent enough.
func (h *HealthHandler) Freshness(w http.ResponseWriter, r *http.Request) {
	// Make an empty slice with enough capacity for every configured target.
	checks := make([]freshnessCheck, 0, len(freshnessTargets()))

	// Run the same freshness query for each configured table/column pair.
	for _, target := range freshnessTargets() {
		// Ask Postgres for the newest timestamp and turn it into a freshnessCheck.
		check, err := h.checkTableFreshness(r, target)
		if err != nil {
			// If any query fails, return 500 because the endpoint could not complete.
			pkg.Error(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Add this table's result to the response list.
		checks = append(checks, check)
	}

	// Build the full JSON response object.
	response := freshnessResponse{
		Service:       "pokemon",                             // this service's domain name
		Check:         "data_freshness",                      // type of health check
		OverallStatus: overallFreshnessStatus(checks),        // collapse many checks to one status
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339), // timestamp in standard UTC format
		Checks:        checks,                                // all individual table checks
	}

	// Default to HTTP 200 when the endpoint ran successfully.
	statusCode := http.StatusOK

	// If data is critically stale, make monitoring see this as service unavailable.
	if response.OverallStatus == "critical" {
		statusCode = http.StatusServiceUnavailable
	}

	// Write the response as JSON.
	pkg.JSON(w, statusCode, response)
}

// FreshnessMetrics exposes business-data freshness for Prometheus.
//
// Same truth as Freshness(), different output format:
// /api/health/freshness is JSON for people.
// /metrics is Prometheus text for Prometheus/Grafana.
func (h *HealthHandler) FreshnessMetrics(w http.ResponseWriter, r *http.Request) {
	// Build the same set of freshness checks used by the JSON endpoint.
	checks := make([]freshnessCheck, 0, len(freshnessTargets()))

	// Query every configured target.
	for _, target := range freshnessTargets() {
		// Reuse the core table-checking function so JSON and metrics cannot drift apart.
		check, err := h.checkTableFreshness(r, target)
		if err != nil {
			// Prometheus expects plain HTTP responses, so use http.Error here.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Store this result for metric rendering.
		checks = append(checks, check)
	}

	// Compute one overall status for a simple dashboard/alert signal.
	overallStatus := overallFreshnessStatus(checks)

	// Prometheus scrapers expect the Prometheus text exposition content type.
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// HELP describes the metric for humans and Grafana metric explorers.
	fmt.Fprintln(w, "# HELP pokemon_data_freshness_status Data freshness status code by source. 0=fresh, 1=warning, 2=critical_or_no_data, 3=unknown.")
	// TYPE tells Prometheus this metric is a gauge: a value that can go up or down.
	fmt.Fprintln(w, "# TYPE pokemon_data_freshness_status gauge")
	// Emit one status metric per check.
	for _, check := range checks {
		// Labels identify which table/check this number belongs to.
		fmt.Fprintf(
			w,
			"pokemon_data_freshness_status{check=%q,table=%q,timestamp_column=%q,status=%q} %.0f\n",
			check.Name,                         // label: logical check name
			check.Table,                        // label: database table
			check.TimestampColumn,              // label: freshness timestamp column
			check.Status,                       // label: human-readable state
			freshnessStatusValue(check.Status), // numeric value for Prometheus alerts
		)
	}

	// HELP for the age metric.
	fmt.Fprintln(w, "# HELP pokemon_data_freshness_age_minutes Age in minutes of the newest row for each source.")
	// TYPE for the age metric.
	fmt.Fprintln(w, "# TYPE pokemon_data_freshness_age_minutes gauge")
	// Emit one age metric per check that actually has data.
	for _, check := range checks {
		// If a table has no rows, age is nil, so skip the age metric for that table.
		if check.AgeMinutes == nil {
			continue
		}

		// Write the age in minutes as a Prometheus gauge.
		fmt.Fprintf(
			w,
			"pokemon_data_freshness_age_minutes{check=%q,table=%q,timestamp_column=%q} %.2f\n",
			check.Name,            // label: logical check name
			check.Table,           // label: database table
			check.TimestampColumn, // label: freshness timestamp column
			*check.AgeMinutes,     // value: age of newest row in minutes
		)
	}

	// HELP for warning threshold metric.
	fmt.Fprintln(w, "# HELP pokemon_data_freshness_warning_threshold_minutes Warning threshold in minutes for each source.")
	// TYPE for warning threshold metric.
	fmt.Fprintln(w, "# TYPE pokemon_data_freshness_warning_threshold_minutes gauge")
	// HELP for critical threshold metric.
	fmt.Fprintln(w, "# HELP pokemon_data_freshness_critical_threshold_minutes Critical threshold in minutes for each source.")
	// TYPE for critical threshold metric.
	fmt.Fprintln(w, "# TYPE pokemon_data_freshness_critical_threshold_minutes gauge")
	// Emit configured thresholds so dashboards can compare current age to policy.
	for _, check := range checks {
		// Warning threshold line.
		fmt.Fprintf(
			w,
			"pokemon_data_freshness_warning_threshold_minutes{check=%q,table=%q} %d\n",
			check.Name,           // label: logical check name
			check.Table,          // label: database table
			check.WarningMinutes, // value: warning threshold
		)
		// Critical threshold line.
		fmt.Fprintf(
			w,
			"pokemon_data_freshness_critical_threshold_minutes{check=%q,table=%q} %d\n",
			check.Name,            // label: logical check name
			check.Table,           // label: database table
			check.CriticalMinutes, // value: critical threshold
		)
	}

	// HELP for one overall service-level freshness signal.
	fmt.Fprintln(w, "# HELP pokemon_data_freshness_overall_status Overall data freshness status. 0=fresh, 1=warning, 2=critical_or_no_data, 3=unknown.")
	// TYPE for the overall signal.
	fmt.Fprintln(w, "# TYPE pokemon_data_freshness_overall_status gauge")
	// Emit one overall metric for simple stat panels and alert rules.
	fmt.Fprintf(
		w,
		"pokemon_data_freshness_overall_status{service=%q,status=%q} %.0f\n",
		"pokemon",                           // label: service name
		overallStatus,                       // label: human-readable state
		freshnessStatusValue(overallStatus), // value: numeric state
	)
}

// checkTableFreshness checks one table by finding its newest timestamp.
//
// This is the core data-access function:
// Given a target like "price_history.created_at", ask Postgres:
// "What is the newest timestamp, and how many minutes old is it?"
func (h *HealthHandler) checkTableFreshness(r *http.Request, target freshnessTarget) (freshnessCheck, error) {
	// Build SQL from trusted internal config, not user input.
	query := fmt.Sprintf(
		`
		SELECT
			MAX(%s)::text AS newest_seen,
			EXTRACT(EPOCH FROM (NOW() - MAX(%s))) / 60 AS age_minutes
		FROM %s
		`,
		target.Column, // first %s: timestamp column for MAX(...)
		target.Column, // second %s: same timestamp column for age math
		target.Table,  // third %s: table name
	)

	// pgtype.Text can represent either a string value or SQL NULL.
	var newestSeen pgtype.Text
	// pgtype.Float8 can represent either a float64 value or SQL NULL.
	var ageMinutes pgtype.Float8

	// Run the query and scan the two selected columns into nullable Go values.
	if err := h.db.QueryRow(r.Context(), query).Scan(&newestSeen, &ageMinutes); err != nil {
		// Wrap the error with the check name so debugging tells us which table failed.
		return freshnessCheck{}, fmt.Errorf("freshness query failed for %s: %w", target.Name, err)
	}

	// Start with a no_data result. If Postgres returned real values, update it below.
	check := freshnessCheck{
		Name:            target.Name,            // copy configured check name
		Table:           target.Table,           // copy configured table
		TimestampColumn: target.Column,          // copy configured timestamp column
		WarningMinutes:  target.WarningMinutes,  // copy configured warning threshold
		CriticalMinutes: target.CriticalMinutes, // copy configured critical threshold
		NewestSeen:      nil,                    // nil means JSON null
		AgeMinutes:      nil,                    // nil means JSON null
		Status:          "no_data",              // default when MAX(timestamp) is NULL
		BusinessRisk:    target.BusinessRisk,    // copy configured risk explanation
	}

	// If MAX(timestamp)::text was not NULL, store it in the response.
	if newestSeen.Valid {
		value := newestSeen.String // create local variable so we can take its address
		check.NewestSeen = &value  // pointer lets JSON encode null vs string
	}

	// If age_minutes was not NULL, compute the freshness status from thresholds.
	if ageMinutes.Valid {
		value := ageMinutes.Float64                                                          // raw age in minutes
		check.AgeMinutes = &value                                                            // pointer lets JSON encode null vs number
		check.Status = freshnessStatus(value, target.WarningMinutes, target.CriticalMinutes) // convert age into fresh/warning/critical
	}

	// Return the completed result for this one target.
	return check, nil
}
