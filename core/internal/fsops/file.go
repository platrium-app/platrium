package fsops

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
)

// FSOps encapsulates the domain logic for Platrium file system operations.
type FSOps struct {
	graph        graph.Graph
	manifestRepo *ManifestRepo
}

// File represents a file node in the graph database.
type File struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	InlineChunks []string  `json:"inline_chunks,omitempty"`
}

// CreateFileParams encapsulates the input fields required to create a new File node.
type CreateFileParams struct {
	TenantID  string
	ParentID  string
	Name      string
	Size      int64
	MimeType  string
	HexHashes []string
}

func NewFSOps(g graph.Graph, m *ManifestRepo) *FSOps {
	return &FSOps{graph: g, manifestRepo: m}
}

// processHashes decodes hex strings and executes the Hybrid Manifest strategy.
// If <= 4 chunks, they are returned for inline Graph caching.
// If > 4 chunks, they are paged out to the KVStore.
func (f *FSOps) processHashes(ctx context.Context, fileId string, version string, hexHashes []string) ([]string, error) {
	var binaryHashes [][]byte
	for _, hexHash := range hexHashes {
		bin, err := hex.DecodeString(hexHash)
		if err != nil {
			return nil, fmt.Errorf("invalid hex hash %s: %v", hexHash, err)
		}
		binaryHashes = append(binaryHashes, bin)
	}

	var inlineChunks []string

	if len(hexHashes) == 0 {
		inlineChunks = []string{} // Explicitly non-nil empty array for empty files
	} else if len(hexHashes) <= 4 {
		inlineChunks = hexHashes
	} else {
		if err := f.manifestRepo.SaveManifest(ctx, fileId, version, binaryHashes); err != nil {
			return nil, fmt.Errorf("failed to save manifest: %v", err)
		}
	}

	return inlineChunks, nil
}

// CreateFile assigns a file into the resource graph, strictly verifying the parent
// container exists and isn't a file, all within a single Neo4j transaction.
func (f *FSOps) CreateFile(ctx context.Context, params CreateFileParams) (string, error) {
	fileId := uuid.New().String()
	version := "1" // TODO: Change to NanoID or smth else, Initial creation is always v1

	inlineChunks, err := f.processHashes(ctx, fileId, version, params.HexHashes)
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
			file.size = $size,
			file.mime_type = $mime_type,
			file.created_at = datetime(),
			file.updated_at = datetime(),
			file.inline_chunks = $inline_chunks
		
		MERGE (file)-[:CHILD_OF]->(parent)
		RETURN file
	`

	cypherParams := map[string]any{
		"tenant_id":     params.TenantID,
		"parent_id":     params.ParentID,
		"file_id":       fileId,
		"name":          params.Name,
		"size":          params.Size,
		"mime_type":     params.MimeType,
		"inline_chunks": inlineChunks,
	}

	err = f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, cypher, cypherParams)
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

	if err != nil {
		return "", err
	}

	return fileId, nil
}

// GetFile retrieves the full file metadata, ensuring tenant isolation.
func (f *FSOps) GetFile(ctx context.Context, tenantId, fileId string) (*File, error) {
	query := `
		MATCH (file:File {id: $file_id, tenant_id: $tenant_id})
		RETURN 
			file.id AS id,
			file.tenant_id AS tenant_id,
			file.name AS name,
			file.size AS size,
			file.mime_type AS mime_type,
			file.created_at AS created_at,
			file.updated_at AS updated_at,
			file.inline_chunks AS inline_chunks
	`
	params := map[string]interface{}{
		"file_id":   fileId,
		"tenant_id": tenantId,
	}

	var file File
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("file not found or access denied")
		}

		if err := res.Scan(&file); err != nil {
			return fmt.Errorf("failed to scan file: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &file, nil
}
