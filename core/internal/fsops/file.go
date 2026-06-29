package fsops

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
	"platrium/internal/infra/kvstore"
)

// FSOps encapsulates the domain logic for Platrium file system operations.
type FSOps struct {
	graph        graph.Graph
	manifestRepo *ManifestRepo
}

func NewFSOps(g graph.Graph, m *ManifestRepo) *FSOps {
	return &FSOps{graph: g, manifestRepo: m}
}

// processHashes decodes hex strings and executes the Hybrid Manifest strategy.
// If <= 4 chunks, they are returned for inline Graph caching.
// If > 4 chunks, they are paged out to the KVStore.
func (f *FSOps) processHashes(ctx context.Context, fileId string, version int, hexHashes []string) (string, []string, error) {
	var binaryHashes [][]byte
	for _, hexHash := range hexHashes {
		bin, err := hex.DecodeString(hexHash)
		if err != nil {
			return "", nil, fmt.Errorf("invalid hex hash %s: %v", hexHash, err)
		}
		binaryHashes = append(binaryHashes, bin)
	}

	manifestPath := ""
	var inlineChunks []string

	if len(hexHashes) <= 4 {
		inlineChunks = hexHashes
	} else {
		manifestPath = fmt.Sprintf("%s:%s:v%d", kvstore.NSManifest, fileId, version)
		if err := f.manifestRepo.SaveManifest(ctx, fileId, version, binaryHashes); err != nil {
			return "", nil, fmt.Errorf("failed to save manifest: %v", err)
		}
	}

	return manifestPath, inlineChunks, nil
}

// CreateFile assigns a file into the resource graph, strictly verifying the parent
// container exists and isn't a file, all within a single Neo4j transaction.
func (f *FSOps) CreateFile(ctx context.Context, tenantId, parentId, name string, hexHashes []string) (string, error) {
	fileId := uuid.New().String()
	version := 1 // Initial creation is always v1

	manifestPath, inlineChunks, err := f.processHashes(ctx, fileId, version, hexHashes)
	if err != nil {
		return "", err
	}
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

	err = f.graph.WriteTx(ctx, func(tx graph.Tx) error {
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

	return fileId, err
}
