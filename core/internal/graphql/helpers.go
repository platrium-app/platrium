package graphql

import (
	"context"
	"fmt"
	"platrium/internal/auth/session"
	"platrium/internal/fsops"
)

func (r *Resolver) resolveItemPath(ctx context.Context, itemID string) ([]*Folder, error) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}
	folders, err := r.FSOps.GetItemPath(ctx, sess.TenantID, itemID)
	if err != nil {
		return nil, err
	}
	return mapFsopsFolders(folders), nil
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
		})
	}
	return folders
}

func mapDriveItemRecord(item *fsops.DriveItemRecord) DriveItem {
	isFolder := false
	for _, label := range item.Labels {
		if label == "Folder" || label == "PrivateDrive" || label == "SharedDrive" {
			isFolder = true
		}
	}

	if isFolder {
		return &Folder{
			ID:        item.ID,
			ParentID:  item.ParentID,
			Name:      item.Name,
			Type:      DriveItemTypeFolder,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
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
		}
	}
}
