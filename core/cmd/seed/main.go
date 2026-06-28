package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"platrium/internal/infra/graph"
)

func main() {
	graphStore, err := graph.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize Graph store: %v", err)
	}
	defer graphStore.Close(context.Background())

	tenantID := "demo"
	parentID := uuid.New().String()

	cypher := `
		MERGE (p:Resource {tenant_id: $tenant_id, name: 'My Drive'})
		ON CREATE SET
			p:PrivateDrive,
			p.id = $parent_id
		RETURN p.id AS id
	`
	params := map[string]any{
		"tenant_id": tenantID,
		"parent_id": parentID,
	}

	err = graphStore.WriteTx(context.Background(), func(tx graph.Tx) error {
		res, err := tx.Query(context.Background(), cypher, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if res.Next() {
			var out map[string]any
			if err := res.Scan(&out); err != nil {
				return err
			}
			
			id := out["id"].(string)

			fmt.Printf("\n✅ Successfully seeded database!\n")
			fmt.Printf("--------------------------------------------------\n")
			fmt.Printf("Tenant ID: %s\n", tenantID)
			fmt.Printf("Parent ID: %s\n", id)
			fmt.Printf("--------------------------------------------------\n")
			fmt.Printf("Use this Parent ID in your curl test!\n\n")
		}

		return res.Err()
	})

	if err != nil {
		log.Fatalf("Seed failed: %v", err)
	}
}
