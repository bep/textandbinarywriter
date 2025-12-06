// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package golibtemplate

import (
	"encoding/binary"
	"io"
)

// BlobMarker is used to identify start of a binary blob.
var BlobMarker [8]byte = [8]byte{'T', 'A', '5', 'B', 'L', 'O', 'B', '1'}

// Chunk holds a chunk of data.
type Chunk struct {
	ID   uint32
	Body io.Reader
}

func New() io.ReadCloser {
	return &Reader{}
}

var _ io.ReadCloser = (*Reader)(nil)

type Reader struct {
	textr    io.Reader
	binaryr  io.Reader
	currentr io.Reader // current reader: text or binary

	buffIdx int      // current index in buff
	buff    [12]byte // big enough for blob marker.
}

func (r *Reader) Read(p []byte) (n int, err error) {
	// Read into buff until it is full, then decide what to do.

	// Check if we have detected a blob marker.

	// If we have detected a blob marker, write up to the marker to the current reader.

	// Switch to binary reader.

	// Read binary blob.

	// Switch back to text reader.

	panic("not implemented")
}

func (r *Reader) Close() error {
	// Flush any remaining data.
	// Close any underlying readers if needed.
	panic("not implemented")
}

func BlobHeaderWrite(w io.Writer, id uint32, size uint64) error {
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
