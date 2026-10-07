package graphql

import (
	"context"
	"platrium/internal/fsops"
	"platrium/internal/identity"
)

// TODO: Too many helpers, organize this!
func (r *Resolver) resolveItemPath(ctx context.Context, itemID string) ([]*Folder, error) {
	p, err := r.Actors.PrincipalOrAnonymous(ctx)
	if err != nil {
		return nil, err
	}
	folders, err := r.FSOps.GetItemPath(ctx, p, itemID)
	if err != nil {
		return nil, err
	}
	return mapFsopsFolders(folders), nil
}

func mapFolderRecord(f *fsops.Folder) *Folder {
	return &Folder{
		ID:        f.ID,
		ParentID:  f.ParentID,
		Name:      f.Name,
		Type:      DriveItemTypeFolder,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,

		MyCapabilities: f.Caps.Verbs(),
	}
}

func mapFsopsFolders(fsopsFolders []*fsops.Folder) []*Folder {
	var folders []*Folder
	for _, f := range fsopsFolders {
		folderType := DriveItemTypeFolder
		folders = append(folders, &Folder{
			ID:        f.ID,
			ParentID:  f.ParentID,
			Name:      f.Name,
			Type:      folderType,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,

			MyCapabilities: f.Caps.Verbs(),
		})
	}
	return folders
}

func mapDriveItemRecord(item *fsops.DriveItemRecord) DriveItem {
	if item.IsFolder() {
		return &Folder{
			ID:        item.ID,
			ParentID:  item.ParentID,
			Name:      item.Name,
			Type:      DriveItemTypeFolder,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,

			MyCapabilities: item.Caps.Verbs(),
		}
	} else {
		var size int64
		if item.Size != nil {
			size = *item.Size
		}
		mime := "application/octet-stream"
		if item.MimeType != nil {
			mime = *item.MimeType
		}
		pid := ""
		if item.ParentID != nil {
			pid = *item.ParentID
		}
		return &File{
			ID:        item.ID,
			ParentID:  pid,
			Name:      item.Name,
			Type:      DriveItemTypeFile,
			Size:      size,
			MimeType:  mime,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,

			MyCapabilities: item.Caps.Verbs(),
		}
	}
}

func mapFileRecord(file *fsops.File) *File {
	if file == nil {
		return nil
	}
	return &File{
		ID:        file.ID,
		Name:      file.Name,
		Type:      DriveItemTypeFile,
		Size:      file.Size,
		MimeType:  file.MimeType,
		CreatedAt: file.CreatedAt,
		UpdatedAt: file.UpdatedAt,

		MyCapabilities: file.Caps.Verbs(),
	}
}

func MapPublicTenantAuthConfig(cfg *identity.PublicTenantAuthConfig) *TenantAuthConfig {
	if cfg == nil {
		return nil
	}
	var providers []*IdpProvider
	for _, p := range cfg.Providers {
		providers = append(providers, &IdpProvider{
			ID:   p.ID,
			Name: p.Name,
			Type: p.Type,
		})
	}
	return &TenantAuthConfig{
		TenantID:     cfg.TenantID,
		Name:         cfg.Name,
		Alias:        cfg.Alias,
		DefaultIdpID: cfg.DefaultIdpID,
		Providers:    providers,
	}
}

// mapDrive maps a drive to the folder that represents its root.
func mapDrive(d *fsops.Drive) *Folder {
	driveType := DriveTypePrivate
	if d.Type == fsops.DriveTypeShared {
		driveType = DriveTypeShared
	}
	var quota *int64
	if d.StorageQuota > 0 {
		q := d.StorageQuota
		quota = &q
	}
	return &Folder{
		ID:   d.ID,
		Name: d.Name,
		Type: DriveItemTypeFolder,
		DriveMetadata: &DriveMetadata{
			DriveType:    driveType,
			StorageUsed:  d.StorageUsed,
			StorageQuota: quota,
		},
		MyCapabilities: d.Caps.Verbs(),
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.CreatedAt,
	}
}
