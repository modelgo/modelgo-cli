package tenantcmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/modelgo/modelgo-cli/internal/auth"
)

// runListRemote fetches every tenant the account belongs to via
// GET {BaseURL}/open/v1/tenants (authenticated with the env's active session)
// and merges it with the locally logged-in tenants. Logged-in tenants are
// marked with a ✓; tenants present only remotely are flagged with the hint to
// run `modelgo auth login` to add them.
//
// The /open/v1/tenants route is reverse-proxied by model-gateway to
// web-api/permissions. If that route is not yet wired in a given deployment the
// call returns a non-2xx and we surface a clear error to stderr (caller logs
// it) rather than silently degrading — but the local `tenant list` (without
// --remote) always works regardless.
func runListRemote(stdout, stderr io.Writer, envName, path string) error {
	// Need a logged-in session for this env to authenticate the request.
	active, err := auth.LoadActive(envName, path)
	if err != nil {
		return fmt.Errorf("no active session for env %q; run `modelgo auth login` first: %w", envName, err)
	}

	localCreds, activeID, err := auth.ListTenants(envName, path)
	if err != nil {
		return err
	}
	localByID := make(map[string]auth.Credential, len(localCreds))
	for _, c := range localCreds {
		localByID[c.TenantID] = c
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	remote, err := auth.FetchTenants(ctx, nil, active)
	if err != nil {
		return err
	}

	type row struct {
		id, slug, name, role string
		loggedIn, isActive   bool
	}
	rows := make([]row, 0, len(remote))
	seen := map[string]bool{}
	for _, rt := range remote {
		_, ok := localByID[rt.TenantID]
		rows = append(rows, row{
			id:       rt.TenantID,
			slug:     rt.Slug,
			name:     rt.Name,
			role:     rt.Role,
			loggedIn: ok,
			isActive: rt.TenantID == activeID,
		})
		seen[rt.TenantID] = true
	}
	// Include any locally logged-in tenants the server did not return (e.g.
	// membership revoked but credential still cached locally).
	for _, c := range localCreds {
		if seen[c.TenantID] {
			continue
		}
		rows = append(rows, row{
			id:       c.TenantID,
			slug:     c.TenantSlug,
			name:     c.TenantName,
			role:     "",
			loggedIn: true,
			isActive: c.TenantID == activeID,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })

	for _, r := range rows {
		marker := "  "
		if r.isActive {
			marker = "* "
		}
		status := "needs `auth login`"
		if r.loggedIn {
			status = "logged in ✓"
		}
		fmt.Fprintf(stdout, "%s%-26s %-16s %-20s %-10s %s\n",
			marker, r.id, dash(r.slug), dash(r.name), dash(r.role), status)
	}
	return nil
}
