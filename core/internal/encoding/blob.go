package encoding

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var FileMagicHeader = []byte("PLB1")

const (
	// ReservedFlag is kept for 38-byte alignment (previously FlagFinalized).
	ReservedFlag byte = 0x00
)

const (
	EncAlgoPlaintext byte = 0x00
	EncAlgoAESGCM    byte = 0x01
)

const (
	HashLengthSHA256 = 32
	HeaderLength     = 4 + 1 + 1 + HashLengthSHA256 // 38 bytes total
)

var (
	ErrInvalidFormat = errors.New("invalid file format: missing PLB1 magic header")
	ErrShortHeader   = errors.New("file too short to contain a complete header")
	ErrInvalidHash   = errors.New("invalid SHA256 hash length")
)

// BlobMetadata represents the parsed header of a PLB1 file.
type BlobMetadata struct {
	EncAlgo      byte
	ExpectedHash []byte // Always exactly 32 bytes for SHA-256
}

// GetExpectedHashHex returns the ExpectedHash as a standard hex string for easy comparison.
func (m *BlobMetadata) GetExpectedHashHex() string {
	return hex.EncodeToString(m.ExpectedHash)
}

// BlobEncodingOptions defines strictly what transformations to apply to a Blob
type BlobEncodingOptions struct {
	Encrypt bool
}

// PrependBlobHeader prepends the standard PLB1 38-byte header onto an incoming
// payload stream. It does not load the payload into memory.
//
// TODO (Future): If encAlgo is EncAlgoAESGCM, wrap rawStream in a cipher stream before
// passing it to the MultiReader.
func PrependBlobHeader(hashHex string, encAlgo byte, rawStream io.Reader) (io.Reader, error) {
	// 1. Decode and validate the SHA256 Hash
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex hash: %w", err)
	}
	if len(hashBytes) != HashLengthSHA256 {
		return nil, ErrInvalidHash
	}

	// 2. Prepare the Flags bitmask (Currently just Reserved)
	var flags byte = ReservedFlag

	// 3. Build the 38-byte binary header exactly
	var header bytes.Buffer
	header.Write(FileMagicHeader) // 4 bytes
	header.WriteByte(flags)       // 1 byte
	header.WriteByte(encAlgo)     // 1 byte
	header.Write(hashBytes)       // 32 bytes

	// Validate our math just in case
	if header.Len() != HeaderLength {
		return nil, fmt.Errorf("developer error: header produced %d bytes, expected %d", header.Len(), HeaderLength)
	}

	// 4. Wrap the header buffer and aggressively concatenate the streams.
	// When io.Copy consumes this returned MultiReader, it gets 38 bytes of header,
	// then seamlessly drops down to pulling from rawStream.
	headerReader := bytes.NewReader(header.Bytes())

	// (Future: wrap rawStream with AES if encAlgo == EncAlgoAESGCM here)
	finalStream := io.MultiReader(headerReader, rawStream)

	return finalStream, nil
}

// ConsumeBlobHeader completely consumes the 38-byte PLB1 header from a disk stream,
// parses the metadata, and returns a stream perfectly positioned at the start of the payload.
func ConsumeBlobHeader(diskStream io.Reader) (*BlobMetadata, io.Reader, error) {
	// 1. Allocate a strictly sized buffer for the 38-byte header
	headerBuf := make([]byte, HeaderLength)

	// 2. Read exactly 38 bytes from the disk stream
	// io.ReadFull correctly handles partial reads until the buffer is perfectly full
	_, err := io.ReadFull(diskStream, headerBuf)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, nil, ErrShortHeader
		}
		return nil, nil, fmt.Errorf("failed to read PLB1 header: %w", err)
	}

	// 3. Validate Magic Bytes (Offsets 0-3)
	if !bytes.Equal(headerBuf[0:4], FileMagicHeader) {
		return nil, nil, ErrInvalidFormat
	}

	// 4. Parse Flags (Offset 4)
	// flags := headerBuf[4] // Currently Reserved

	// 5. Parse Encryption Algo (Offset 5)
	encAlgo := headerBuf[5]

	// 6. Extract Hash (Offsets 6 to 37)
	hashBytes := make([]byte, HashLengthSHA256)
	copy(hashBytes, headerBuf[6:6+HashLengthSHA256])

	// 7. Assemble the Metadata Struct
	meta := &BlobMetadata{
		EncAlgo:      encAlgo,
		ExpectedHash: hashBytes,
	}

	// 8. Return the original disk stream as the payload stream.
	// Since io.ReadFull advanced the cursor by exactly 38 bytes, reading from
	// this diskStream going forward will flawlessly return pure payload data!

	// (Future: Check meta.EncAlgo and wrap diskStream in AES decipher if needed here)
	payloadStream := diskStream

	return meta, payloadStream, nil
}

// EncodeToBlobStream maps Blob-specific options into the universal EncodingPipeline,
// executes the pipeline, and seamlessly attaches the PLB1 binary header.
func EncodeToBlobStream(expectedHash string, rawStream io.Reader, opts BlobEncodingOptions) (io.Reader, error) {
	// 1. Initialize universal builder (Hash checking is mandatory for PLB1 uploads)
	pipeline := NewEncodingPipeline(rawStream).
		EnableHashVerification(expectedHash)

	// 2. Map Blob-specific options
	var encAlgo byte = EncAlgoPlaintext
	if opts.Encrypt {
		pipeline.SetEncryption(EncAlgoAESGCM)
		encAlgo = EncAlgoAESGCM
	}

	// 3. Finalize the raw transformed payload
	secureStream, err := pipeline.Build()
	if err != nil {
		return nil, err
	}

	// 4. Attach the PLB1 Header perfectly securely!
	return PrependBlobHeader(expectedHash, encAlgo, secureStream)
}

// DecodeFromBlobStream is the Director for decoding a PLB1 file back into a raw stream.
func DecodeFromBlobStream(diskStream io.Reader) (*BlobMetadata, io.Reader, error) {
	// 1. Dumb binary parser consumes the 38-byte header
	meta, rawPayloadStream, err := ConsumeBlobHeader(diskStream)
	if err != nil {
		return nil, nil, err
	}

	// 2. Dynamically build the Decoding Pipeline based on the metadata in the file!
	pipeline := NewDecodingPipeline(rawPayloadStream)

	_, err = pipeline.SetDecryption(meta.EncAlgo)
	if err != nil {
		return nil, nil, err
	}

	// 3. Finalize the plaintext, ready-to-serve stream
	nakedStream, err := pipeline.Build()
	if err != nil {
		return nil, nil, err
	}

	return meta, nakedStream, nil
}
