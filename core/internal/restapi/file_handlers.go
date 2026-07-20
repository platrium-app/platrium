package restapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"platrium/internal/fsops"
)

type UploadSessionPassportClaims struct {
	SessionID      string `json:"session_id"`
	ParentFolderID string `json:"parent_folder_id"`
	Filename       string `json:"filename"`
	FileSize       int64  `json:"file_size"`
	TenantID       string `json:"tenant_id"`
	jwt.RegisteredClaims
}

// GenerateUploadSessionPassport issues a cryptographically signed JWT passport for the upload session.
func (api *RestAPI) GenerateUploadSessionPassport(sessionID, parentFolderID, filename string, fileSize int64, tenantID string) (string, error) {
	claims := UploadSessionPassportClaims{
		SessionID:      sessionID,
		ParentFolderID: parentFolderID,
		Filename:       filename,
		FileSize:       fileSize,
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
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(api.HMACSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session token: %w", err)
	}

	if claims, ok := token.Claims.(*UploadSessionPassportClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid session passport claims")
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
	if request.Body == nil || request.Body.FileName == "" || request.Body.ParentId == "" {
		return UploadSessionInitialize500JSONResponse{Debuginfo: "file_name and parent_id are required"}, nil
	}

	sessionID := uuid.New().String()
	// TODO: Replace with authenticated tenant ID from JWT middleware
	tenantID := "f6105961-cd9f-492a-8657-33f7e14ff1b2"

	token, err := api.GenerateUploadSessionPassport(sessionID, request.Body.ParentId, request.Body.FileName, request.Body.FileSize, tenantID)
	if err != nil {
		return UploadSessionInitialize500JSONResponse{Debuginfo: fmt.Sprintf("failed to generate session passport: %v", err)}, nil
	}

	return UploadSessionInitialize201JSONResponse{
		SessionId: token,
	}, nil
}

// UploadSessionChunks handles POST /files/uploadsession/{sessionId}/chunks (Stage 2: Batch Presigning).
func (api *RestAPI) UploadSessionChunks(ctx context.Context, request UploadSessionChunksRequestObject) (UploadSessionChunksResponseObject, error) {
	claims, err := api.VerifyUploadSessionPassport(request.SessionId)
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

// UploadSessionCommit handles POST /files/uploadsession/{sessionId}/commit (Stage 3: Zero-Read Commit).
func (api *RestAPI) UploadSessionCommit(ctx context.Context, request UploadSessionCommitRequestObject) (UploadSessionCommitResponseObject, error) {
	claims, err := api.VerifyUploadSessionPassport(request.SessionId)
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
	fileId, err := api.FSOps.CreateFile(ctx, claims.TenantID, claims.ParentFolderID, claims.Filename, hexHashes)
	if err != nil {
		return UploadSessionCommit500JSONResponse{
			Debuginfo: fmt.Sprintf("failed to commit file node: %v", err),
		}, nil
	}

	return UploadSessionCommit200JSONResponse{
		FileId: fileId,
	}, nil
}
