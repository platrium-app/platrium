package fsops

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/authz"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/driveitem"
)

// FSOps encapsulates the domain logic for Platrium file system operations.
type FSOps struct {
	db           *db.DB
	manifestRepo *ManifestRepo
	authz        authz.Authorizer
}

// File represents a file in a drive's tree.
type File struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	InlineChunks []string  `json:"inline_chunks,omitempty"`
	// Caps is what the calling principal may do with the file.
	Caps authz.Capability `json:"-"`
}

// CreateFileParams encapsulates the input fields required to create a new File node.
type CreateFileParams struct {
	Actor     authz.Principal
	ParentID  string
	Name      string
	Size      int64
	MimeType  string
	HexHashes []string
}

func NewFSOps(d *db.DB, m *ManifestRepo, az authz.Authorizer) *FSOps {
	return &FSOps{db: d, manifestRepo: m, authz: az}
}

// processHashes decodes hex strings and executes the Hybrid Manifest strategy.
// If <= 4 chunks, they are returned to be stored inline on the file row.
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

// CreateFile adds a file under a parent folder. The actor needs CREATE on the
// parent, and the parent must exist in their tenant and be a folder.
func (f *FSOps) CreateFile(ctx context.Context, params CreateFileParams) (string, error) {
	// Authorize before anything is written, including the KV manifest.
	if err := f.Require(ctx, params.Actor, params.ParentID, authz.CapCreate); err != nil {
		return "", err
	}

	fileId := nanoid.Must()
	version := "1" // TODO: Change to NanoID or smth else, Initial creation is always v1

	// TODO: a manifest written for a file whose insert then fails is orphaned in the KV store.
	inlineChunks, err := f.processHashes(ctx, fileId, version, params.HexHashes)
	if err != nil {
		return "", err
	}

	err = f.db.WithTx(ctx, func(tx *ent.Tx) error {
		parent, err := tx.DriveItem.Query().
			Where(driveitem.ID(params.ParentID), driveitem.TenantID(params.Actor.TenantID), driveitem.KindEQ(driveitem.KindFOLDER)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: invalid parent resource or permission denied", ErrNotFound)
			}
			return err
		}

		create := tx.DriveItem.Create().
			SetID(fileId).
			SetTenantID(params.Actor.TenantID).
			SetDriveID(parent.DriveID).
			SetParentID(parent.ID).
			SetKind(driveitem.KindFILE).
			SetName(params.Name).
			SetSize(params.Size).
			SetMimeType(params.MimeType)
		if inlineChunks != nil {
			create.SetInlineChunks(inlineChunks)
		}
		if err := create.Exec(ctx); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fileId, nil
}

func fileFromEnt(i *ent.DriveItem, caps authz.Capability) *File {
	file := &File{
		Caps:         caps,
		ID:           i.ID,
		TenantID:     i.TenantID,
		Name:         i.Name,
		CreatedAt:    i.CreatedAt,
		UpdatedAt:    i.UpdatedAt,
		InlineChunks: i.InlineChunks,
	}
	if i.Size != nil {
		file.Size = *i.Size
	}
	if i.MimeType != nil {
		file.MimeType = *i.MimeType
	}
	return file
}

// GetFile retrieves file metadata. Requires VIEW. Anonymous callers may read
// publicly shared files.
func (f *FSOps) GetFile(ctx context.Context, p authz.Principal, fileId string) (*File, error) {
	return f.getFile(ctx, p, fileId, authz.CapView)
}

// GetFileForDownload retrieves a file and the chunk list needed to download
// it. Requires DOWNLOAD, which a view-only share does not carry. Anonymous
// callers may download publicly shared files.
func (f *FSOps) GetFileForDownload(ctx context.Context, p authz.Principal, fileId string) (*File, error) {
	return f.getFile(ctx, p, fileId, authz.CapDownload)
}

func (f *FSOps) getFile(ctx context.Context, p authz.Principal, fileId string, need authz.Capability) (*File, error) {
	tenantID, caps, err := f.readAccess(ctx, p, fileId, need)
	if err != nil {
		return nil, err
	}
	i, err := f.db.DriveItem.Query().
		Where(driveitem.ID(fileId), driveitem.TenantID(tenantID), driveitem.KindEQ(driveitem.KindFILE)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: file not found or access denied", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch file: %w", err)
	}
	return fileFromEnt(i, caps), nil
}

// CopyFile copies a file to a new parent folder. Requires DOWNLOAD on the
// source (copying exposes the content) and CREATE on the destination.
func (f *FSOps) CopyFile(ctx context.Context, p authz.Principal, fileID string, newParentID string, newName string) (*File, error) {
	if err := requireActor(p); err != nil {
		return nil, err
	}
	caps, err := f.authz.CapsMany(ctx, p, []string{fileID, newParentID})
	if err != nil {
		return nil, err
	}
	if err := denyUnless(caps[fileID], authz.CapDownload, "source file"); err != nil {
		return nil, err
	}
	if err := denyUnless(caps[newParentID], authz.CapCreate, "destination folder"); err != nil {
		return nil, err
	}

	var copied *ent.DriveItem
	err = f.db.WithTx(ctx, func(tx *ent.Tx) error {
		src, err := tx.DriveItem.Query().
			Where(driveitem.ID(fileID), driveitem.TenantID(p.TenantID), driveitem.KindEQ(driveitem.KindFILE)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: source file not found", ErrNotFound)
			}
			return err
		}
		dest, err := tx.DriveItem.Query().
			Where(driveitem.ID(newParentID), driveitem.TenantID(p.TenantID), driveitem.KindEQ(driveitem.KindFOLDER)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: destination folder not found", ErrNotFound)
			}
			return err
		}

		name := newName
		if name == "" {
			name = src.Name
		}

		create := tx.DriveItem.Create().
			SetTenantID(p.TenantID).
			SetDriveID(dest.DriveID).
			SetParentID(dest.ID).
			SetKind(driveitem.KindFILE).
			SetName(name).
			SetNillableSize(src.Size).
			SetNillableMimeType(src.MimeType)
		if src.InlineChunks != nil {
			create.SetInlineChunks(slices.Clone(src.InlineChunks))
		}
		copied, err = create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// TODO: If the file was > 4 chunks, we must also duplicate the KVStore manifest
	// pointing to the new file_id. For now, this perfectly copies inline_chunks!

	return fileFromEnt(copied, caps[newParentID]), nil
}
