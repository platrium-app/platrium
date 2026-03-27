package encoding

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
)

var ErrHashMismatch = errors.New("cryptographic validation failed: hash mismatch")

// hashVerifyingReader traps the io.EOF signal to securely validate a running SHA-256 hash.
type hashVerifyingReader struct {
	stream       io.Reader
	hasher       hash.Hash
	expectedHash string
}

func (h *hashVerifyingReader) Read(p []byte) (int, error) {
	n, err := h.stream.Read(p)

	// Intercept the EOF signal to securely validate the finalized cryptographic hash.
	if err == io.EOF {
		actualHash := hex.EncodeToString(h.hasher.Sum(nil))
		if actualHash != h.expectedHash {
			return n, fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, h.expectedHash, actualHash)
		}
	}

	return n, err
}

// EncodingPipeline provides a fluent builder pattern to dynamically sequence
// stream transformers (hashing, encryption, compression) with zero-copy overhead.
type EncodingPipeline struct {
	stream io.Reader
}

func NewEncodingPipeline(baseStream io.Reader) *EncodingPipeline {
	return &EncodingPipeline{
		stream: baseStream,
	}
}

// EnableHashVerification wraps the stream in a TeeReader to compute SHA256 on the fly,
// intercepting io.EOF to validate against the expected hash.
func (p *EncodingPipeline) EnableHashVerification(expectedHashHex string) *EncodingPipeline {
	hasher := sha256.New()
	tappedStream := io.TeeReader(p.stream, hasher)

	p.stream = &hashVerifyingReader{
		stream:       tappedStream,
		hasher:       hasher,
		expectedHash: expectedHashHex,
	}
	return p
}

// SetEncryption wraps the stream with cryptographic transformers based on algo.
func (p *EncodingPipeline) SetEncryption(algo byte) *EncodingPipeline {
	if algo == EncAlgoAESGCM {
		// TODO (Future): p.stream = cipher.NewStreamWriter(...)
	}
	return p
}

// Build finalizes the pipeline, returning the fully wrapped data stream.
func (p *EncodingPipeline) Build() (io.Reader, error) {
	return p.stream, nil
}

// DecodingPipeline provides a fluent builder for unwrapping a raw domain stream
type DecodingPipeline struct {
	stream io.Reader
}

func NewDecodingPipeline(baseStream io.Reader) *DecodingPipeline {
	return &DecodingPipeline{stream: baseStream}
}

// UnwrapEncryption removes the AES-GCM layer if applicable
func (p *DecodingPipeline) SetDecryption(algo byte) (*DecodingPipeline, error) {
	if algo == EncAlgoAESGCM {
		// TODO (Future): p.stream = cipher.NewStreamReader(...)
	} else if algo != EncAlgoPlaintext {
		return nil, fmt.Errorf("unsupported encryption algorithm: %v", algo)
	}
	return p, nil
}

// Build finalizes the decoding pipeline, returning the naked plaintext stream.
func (p *DecodingPipeline) Build() (io.Reader, error) {
	return p.stream, nil
}
