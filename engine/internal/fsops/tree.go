package fsops

import (
	"context"
	"database/sql"
	"fmt"
)

// maxTreeDepth bounds the recursive ancestor walk. The tree is acyclic by
// construction (moves are cycle-checked), so this is a backstop against
// corruption and runaway queries, not an expected limit.
const maxTreeDepth = 1024

// rawQuerier is satisfied by both *ent.Client and *ent.Tx (ent's execquery
// feature), so the ancestor walk can run on the pool or inside a transaction.
type rawQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ancestorIDs returns the IDs on the path from itemID up to its drive root,
// starting with itemID itself (index 0) and ending at the root. It is empty if
// the item does not exist in the tenant.
//
// This is the only raw SQL in the filesystem layer. It is a recursive CTE,
// supported identically by Postgres, MySQL 8+, MariaDB 10.2+ and SQLite, and
// every hop is tenant-scoped. It selects IDs only, so there is no
// driver-specific timestamp scanning; rows are loaded through ent afterwards.
func (f *FSOps) ancestorIDs(ctx context.Context, q rawQuerier, tenantID, itemID string) ([]string, error) {
	query := f.db.Rebind(`
WITH RECURSIVE ancestors(id, parent_id, depth) AS (
	SELECT id, parent_id, 0 FROM drive_items WHERE id = ? AND tenant_id = ?
	UNION ALL
	SELECT p.id, p.parent_id, a.depth + 1
	FROM drive_items p
	JOIN ancestors a ON p.id = a.parent_id
	WHERE p.tenant_id = ? AND a.depth < ?
)
SELECT id FROM ancestors ORDER BY depth ASC`)

	rows, err := q.QueryContext(ctx, query, itemID, tenantID, tenantID, maxTreeDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to walk ancestors: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan ancestor: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
