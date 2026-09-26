package restapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/auth/session"
	"platrium/internal/fsops"
)

type UploadSessionPassportClaims struct {
	SessionID      string `json:"session_id"`
	ParentFolderID string `json:"parent_folder_id"`
	Filename       string `json:"filename"`
	FileSize       int64  `json:"file_size"`
	MimeType       string `json:"mime_type"`
	TenantID       string `json:"tenant_id"`
	jwt.RegisteredClaims
}

type DownloadSessionPassportClaims struct {
	FileID       string   `json:"file_id"`
	Version      string   `json:"version,omitempty"`
	InlineChunks []string `json:"inline_chunks"`
	jwt.RegisteredClaims
}

// GenerateUploadSessionPassport issues a cryptographically signed JWT passport for the upload session.
func (api *RestAPI) GenerateUploadSessionPassport(sessionID, parentFolderID, filename string, fileSize int64, mimeType string, tenantID string) (string, error) {
	claims := UploadSessionPassportClaims{
		SessionID:      sessionID,
		ParentFolderID: parentFolderID,
		Filename:       filename,
		FileSize:       fileSize,
		MimeType:       mimeType,
		TenantID:       tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(api.HMACSecret))
}

// VerifyUploadSessionPassport parses and validates an incoming JWT session passport token.
func (api *RestAPI) VerifyUploadSessionPassport(sessionToken string) (*UploadSessionPassportClaims, error) {
	token, err := jwt.ParseWithClaims(sessionToken, &UploadSessionPassportClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(api.HMACSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UploadSessionPassportClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateDownloadSessionPassport issues a cryptographically signed JWT passport for the download session.
func (api *RestAPI) GenerateDownloadSessionPassport(fileID string, version string, inlineChunks []string) (string, error) {
	claims := DownloadSessionPassportClaims{
		FileID:       fileID,
		Version:      version,
		InlineChunks: inlineChunks,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(api.HMACSecret))
}

// VerifyDownloadSessionPassport parses and validates an incoming JWT session passport token.
func (api *RestAPI) VerifyDownloadSessionPassport(sessionToken string) (*DownloadSessionPassportClaims, error) {
	token, err := jwt.ParseWithClaims(sessionToken, &DownloadSessionPassportClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(api.HMACSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*DownloadSessionPassportClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateHMACReceipt calculates a symmetric cryptographic authorization signature for a chunk hash.
func (api *RestAPI) GenerateHMACReceipt(sessionID, hash string, isEOF bool) string {
	h := hmac.New(sha256.New, []byte(api.HMACSecret))
	payload := sessionID + ":" + hash
	if isEOF {
		payload += ":EOF_CHUNK"
	}
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// UploadSessionInitialize handles POST /files/uploadsession (Stage 1: Session Init).
func (api *RestAPI) UploadSessionInitialize(ctx context.Context, request UploadSessionInitializeRequestObject) (UploadSessionInitializeResponseObject, error) {
	// TODO: see if we can use a validator to make this simpler.
	if request.Body == nil || request.Body.FileName == "" || request.Body.ParentId == "" || request.Body.MimeType == "" {
		return UploadSessionInitialize500JSONResponse{Debuginfo: "file_name, parent_id, and mime_type are required"}, nil
	}

	sessionID := nanoid.Must()
	sess, ok := session.FromContext(ctx)
	if !ok {
		return UploadSessionInitialize500JSONResponse{Debuginfo: "unauthorized: missing session"}, nil
	}

	tenantID := sess.TenantID
	token, err := api.GenerateUploadSessionPassport(sessionID, request.Body.ParentId, request.Body.FileName, request.Body.FileSize, request.Body.MimeType, tenantID)
	if err != nil {
		return UploadSessionInitialize500JSONResponse{Debuginfo: fmt.Sprintf("failed to generate session passport: %v", err)}, nil
	}

	return UploadSessionInitialize201JSONResponse{
		SessionId: token,
	}, nil
}

// UploadSessionChunks handles POST /files/uploadsession/chunks (Stage 2: Batch Presigning).
func (api *RestAPI) UploadSessionChunks(ctx context.Context, request UploadSessionChunksRequestObject) (UploadSessionChunksResponseObject, error) {
	claims, err := api.VerifyUploadSessionPassport(request.Params.XPlatriumUploadsession)
	if err != nil {
		return UploadSessionChunks500JSONResponse{Debuginfo: err.Error()}, nil
	}

	if request.Body == nil || len(request.Body.Hashes) == 0 {
		return UploadSessionChunks500JSONResponse{Debuginfo: "hashes array cannot be empty"}, nil
	}

	hashes := request.Body.Hashes
	containsEOF := request.Body.ContainsEofChunk != nil && *request.Body.ContainsEofChunk

	// 1. Query ChunkStore for existing VALIDATED chunks (Read-Only Stateless Presign)
	existingChunks, err := api.ChunkStore.ReferenceExistingChunks(ctx, hashes)
	if err != nil {
		return UploadSessionChunks500JSONResponse{Debuginfo: fmt.Sprintf("failed to query chunk store: %v", err)}, nil
	}

	// 2. Identify missing chunk hashes that require presigned upload URLs
	missingHashes := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		meta, exists := existingChunks[hash]
		if !exists || meta.State != fsops.ChunkStateValidated {
			missingHashes = append(missingHashes, hash)
		}
	}

	// 3. Request presigned upload targets from StorageManager for missing chunks
	var presignedURLs map[string]string
	if len(missingHashes) > 0 {
		urls, err := api.StorageManager.GenerateChunkUploadURLs(ctx, missingHashes)
		if err != nil {
			return UploadSessionChunks500JSONResponse{Debuginfo: fmt.Sprintf("failed to generate upload URLs: %v", err)}, nil
		}
		presignedURLs = urls
	}

	// 4. Construct presigned metadata and HMAC receipts for every chunk in the batch
	responseMap := make(map[string]FilesUploadSessionPresignedChunk, len(hashes))
	lastIndex := len(hashes) - 1

	for i, hash := range hashes {
		meta, exists := existingChunks[hash]
		isValidated := exists && meta.State == fsops.ChunkStateValidated

		isEOF := containsEOF && (i == lastIndex)
		receipt := api.GenerateHMACReceipt(claims.SessionID, hash, isEOF)

		if isValidated {
			// Chunk already exists on disk -> Deduplicate (UploadUrl = nil)
			responseMap[hash] = FilesUploadSessionPresignedChunk{
				UploadUrl: nil,
				Receipt:   receipt,
			}
		} else {
			url, ok := presignedURLs[hash]
			if !ok {
				return UploadSessionChunks500JSONResponse{Debuginfo: fmt.Sprintf("missing presigned URL for chunk %s", hash)}, nil
			}
			urlPtr := url
			responseMap[hash] = FilesUploadSessionPresignedChunk{
				UploadUrl: &urlPtr,
				Receipt:   receipt,
			}
		}
	}

	return UploadSessionChunks200JSONResponse{
		Chunks: responseMap,
	}, nil
}

// UploadSessionCommit handles POST /files/uploadsession/commit (Stage 3: Zero-Read Commit).
func (api *RestAPI) UploadSessionCommit(ctx context.Context, request UploadSessionCommitRequestObject) (UploadSessionCommitResponseObject, error) {
	claims, err := api.VerifyUploadSessionPassport(request.Params.XPlatriumUploadsession)
	if err != nil {
		return UploadSessionCommit500JSONResponse{Debuginfo: err.Error()}, nil
	}

	if request.Body == nil || len(request.Body.Chunks) == 0 {
		return UploadSessionCommit500JSONResponse{Debuginfo: "chunks manifest array cannot be empty"}, nil
	}

	chunks := request.Body.Chunks
	lastIndex := len(chunks) - 1
	hexHashes := make([]string, len(chunks))

	// 1. Zero-Read In-Memory Structural Geometry Validation
	for i, chunk := range chunks {
		isEOF := (i == lastIndex)
		expectedReceipt := api.GenerateHMACReceipt(claims.SessionID, chunk.Hash, isEOF)

		if chunk.Receipt != expectedReceipt {
			return UploadSessionCommit500JSONResponse{
				Debuginfo: fmt.Sprintf("unauthorized or invalid HMAC receipt for chunk index %d (%s)", i, chunk.Hash),
			}, nil
		}
		hexHashes[i] = chunk.Hash
	}

	// 2. Commit file node and chunk manifest sequence to Graph DB / Manifest KV Store
	fileId, err := api.FSOps.CreateFile(ctx, fsops.CreateFileParams{
		TenantID:  claims.TenantID,
		ParentID:  claims.ParentFolderID,
		Name:      claims.Filename,
		Size:      claims.FileSize,
		MimeType:  claims.MimeType,
		HexHashes: hexHashes,
	})
	if err != nil {
		return UploadSessionCommit500JSONResponse{
			Debuginfo: fmt.Sprintf("failed to commit file node: %v", err),
		}, nil
	}

	return UploadSessionCommit200JSONResponse{
		FileId: fileId,
	}, nil
}

// DownloadSessionInitialize handles POST /files/downloadsession (Stage 1: Passport Issuance).
func (api *RestAPI) DownloadSessionInitialize(ctx context.Context, request DownloadSessionInitializeRequestObject) (DownloadSessionInitializeResponseObject, error) {
	sessionInfo, ok := session.FromContext(ctx)
	if !ok {
		return DownloadSessionInitialize404JSONResponse{}, nil // Standard 404/Unauthorized placeholder
	}

	if request.Body == nil {
		return DownloadSessionInitialize500JSONResponse{Debuginfo: "missing request body"}, nil
	}
	fileID := request.Body.FileId

	// 1. Fetch File Metadata from FSOps to verify permissions and get file info
	fileNode, err := api.FSOps.GetFile(ctx, sessionInfo.TenantID, fileID)
	if err != nil {
		// Log error internally, but return generic 404 to client
		return DownloadSessionInitialize404JSONResponse{}, nil
	}

	// // Default to version "" if not provided
	// // TODO: In the future, fsops.File will consist of current and existing versions where a version is a UUID.
	// var version string = ""
	// if request.Body.Version != nil {
	// 	version = *request.Body.Version
	// }
	var version string = "" // Temporarily hardcoded until versioning is fully implemented

	// 2. Generate the Download Passport
	token, err := api.GenerateDownloadSessionPassport(fileID, version, fileNode.InlineChunks)
	if err != nil {
		return DownloadSessionInitialize500JSONResponse{Debuginfo: "failed to sign session passport"}, nil
	}

	return DownloadSessionInitialize200JSONResponse{
		SessionId: token,
		FileName:  fileNode.Name,
		FileSize:  fileNode.Size,
		MimeType:  fileNode.MimeType,
	}, nil
}

// DownloadSessionChunks handles POST /files/downloadsession/chunks (Stage 2: Batch Presigning).
func (api *RestAPI) DownloadSessionChunks(ctx context.Context, request DownloadSessionChunksRequestObject) (DownloadSessionChunksResponseObject, error) {
	// 1. Verify Passport
	claims, err := api.VerifyDownloadSessionPassport(request.Params.XPlatriumDownloadsession)
	if err != nil {
		return DownloadSessionChunks500JSONResponse{Debuginfo: "invalid or expired download session"}, nil
	}

	// 2. Extract requested indices
	if request.Body == nil || len(request.Body.Indices) == 0 {
		return DownloadSessionChunks500JSONResponse{Debuginfo: "indices array cannot be empty"}, nil
	}

	indices := request.Body.Indices
	fileID := claims.FileID
	version := claims.Version
	inlineChunks := claims.InlineChunks

	// 3. Map to hold our resolved hashes (index to SHA256 Chunk Hash)
	resolvedHashes := make(map[int32]string)

	if inlineChunks != nil {
		for _, idx := range indices {
			if int(idx) >= 0 && int(idx) < len(inlineChunks) {
				resolvedHashes[idx] = inlineChunks[idx]
			}
		}
	} else {
		// TODO: File > 4 chunks -> Read manifest from ManifestRepo
		// manifestRepo.ResolveIndices(fileID, version, indices)
	}

	// 4. Extract the hashes for the StorageManager
	var chunkHashes []string
	for _, hash := range resolvedHashes {
		chunkHashes = append(chunkHashes, hash)
	}

	// 5. Fetch presigned URLs from StorageManager
	downloadUrls, err := api.StorageManager.GenerateChunkDownloadURLs(ctx, chunkHashes)
	if err != nil {
		return DownloadSessionChunks500JSONResponse{Debuginfo: "failed to generate chunk download URLs"}, nil
	}

	// 6. Map URLs back to the requested indices
	responseChunks := make(map[string]FilesDownloadSessionPresignedChunk)
	for idx, hash := range resolvedHashes {
		if url, ok := downloadUrls[hash]; ok {
			responseChunks[fmt.Sprintf("%d", idx)] = FilesDownloadSessionPresignedChunk{
				DownloadUrl: url,
			}
		}
	}

	// TODO: Temporary ignore to pass compiler checks until manifest logic is added
	_ = fileID
	_ = version

	return DownloadSessionChunks200JSONResponse{
		Chunks: responseChunks,
	}, nil
}
