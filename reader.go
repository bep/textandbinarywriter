// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package textandbinaryreader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

// BlobMarker is used to identify start of a binary blob.
var BlobMarker [8]byte = [8]byte{'T', 'A', 'K', '3', '5', 'E', 'M', '1'}

// New creates a new Reader that reads from r and calls handleBlob
// when a binary blob is encountered. Note that the io.Reader passed to handleBlob
// is only valid during the call and any non-consumed data will be discarded.
// The returned Reader represents the text stream with blobs filtered out.
func New(r io.Reader, handleBlob func(id uint32, r io.Reader) error) io.Reader {
	return &Reader{
		r:          bufio.NewReader(r),
		handleBlob: handleBlob,
	}
}

var _ io.Reader = (*Reader)(nil)

// Reader is a reader that filters out binary blobs from a stream.
type Reader struct {
	// underlying reader
	r *bufio.Reader
	// handleBlob is called when a blob is encountered.
	// The reader passed to it receives the blob. Any non-consumed data will be discarded.
	handleBlob func(id uint32, r io.Reader) error
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		peeked, err := r.r.Peek(len(BlobMarker))
		if err != nil {
			// Not enough bytes to form a marker, so the rest is text.
			read, readErr := r.r.Read(p[n:])
			n += read
			return n, readErr
		}

		if bytes.Equal(peeked, BlobMarker[:]) {
			// Discard the marker.
			_, err = r.r.Discard(len(BlobMarker))
			if err != nil {
				return n, err
			}

			// Read the blob header.
			id, size, err := ReadBlobHeaderExcludingMarker(r.r)
			if err != nil {
				return n, err
			}

			// Create a limited reader for the blob and handle it.
			lr := io.LimitReader(r.r, int64(size))
			if err := r.handleBlob(id, lr); err != nil {
				return n, err
			}

			// Ensure the entire blob is consumed.
			if _, err := io.Copy(io.Discard, lr); err != nil {
				return n, err
			}

			continue

		}

		// Not a marker, read one byte.
		p[n], err = r.r.ReadByte()
		if err != nil {
			return n, err
		}
		n++
	}

	return n, nil
}

// WriteBlobHeader writes a blob header to w with the given id and size using little-endian encoding.
func WriteBlobHeader(w io.Writer, id, size uint32) error {
	if err := binary.Write(w, binary.LittleEndian, BlobMarker); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, id); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, size); err != nil {
		return err
	}
	return nil
}

// ReadBlobHeader reads a blob header from r and returns the id and size using little-endian encoding.
func ReadBlobHeader(r io.Reader) (id, size uint32, err error) {
	var marker [8]byte
	if _, err := io.ReadFull(r, marker[:]); err != nil {
		return 0, 0, err
	}
	if marker != BlobMarker {
		return 0, 0, io.ErrUnexpectedEOF
	}
	return ReadBlobHeaderExcludingMarker(r)
}

// ReadBlobHeaderExcludingMarker reads a blob header from r excluding the marker using little-endian encoding.
func ReadBlobHeaderExcludingMarker(r io.Reader) (id, size uint32, err error) {
	if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
		return 0, 0, err
	}
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return 0, 0, err
	}
	return
}
