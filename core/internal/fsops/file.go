package fsops

import (
	"context"
	"fmt"

	"platrium/internal/infra/graph"
)

// FSOps encapsulates the domain logic for Platrium file system operations.
type FSOps struct {
	graph graph.Graph
}

func NewFSOps(g graph.Graph) *FSOps {
	return &FSOps{graph: g}
}

// CreateFile assigns a file into the resource graph, strictly verifying the parent
// container exists and isn't a file, all within a single Neo4j transaction.
func (f *FSOps) CreateFile(ctx context.Context, tenantId, parentId, fileId, name, manifestPath string, inlineChunks []string) error {
	cypher := `
		MATCH (parent:Resource {id: $parent_id, tenant_id: $tenant_id})
		WHERE parent:PrivateDrive OR parent:SharedDrive OR parent:Folder

		MERGE (file:Resource {id: $file_id})
		ON CREATE SET 
			file:File,
			file.name = $name, 
			file.tenant_id = $tenant_id,
			file.manifest_path = $manifest_path,
			file.inline_chunks = $inline_chunks
		
		MERGE (file)-[:CHILD_OF]->(parent)
		RETURN file
	`

	params := map[string]any{
		"tenant_id":     tenantId,
		"parent_id":     parentId,
		"file_id":       fileId,
		"name":          name,
		"manifest_path": manifestPath,
		"inline_chunks": inlineChunks,
	}

	return f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, cypher, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			if res.Err() != nil {
				return res.Err()
			}
			// Safe failure: the parent lookup failed the type constraints or didn't exist.
			return fmt.Errorf("invalid parent resource or permission denied")
		}

		return nil
	})
}
