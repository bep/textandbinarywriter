// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package textandbinaryreader

import (
	"context"
	"encoding/binary"
	"io"

	"golang.org/x/sync/errgroup"
)

// BlobMarker is used to identify start of a binary blob.
var BlobMarker [8]byte = [8]byte{'T', 'A', 'K', '3', '5', 'E', 'M', '1'}

// NewReader creates a new io.ReadCloser that reads from r and calls handleBlob
// when a binary blob is encountered. Note that the io.Reader passed to handleBlob
// is only valid during the call and any non-consumed data will be discarded.
// The returned Reader represents the text stream with blobs filtered out.
// 
// Note: handleBlob may be called from a different goroutine. If your handleBlob
// implementation accesses shared state, it must be thread-safe.
func NewReader(r io.Reader, handleBlob func(id uint32, r io.Reader) error) io.ReadCloser {
	pr, pw := io.Pipe()
	blobR, blobW := io.Pipe()

	writer := NewWriter(pw, blobW)

	g, _ := errgroup.WithContext(context.Background())

	rc := &reader{
		PipeReader: pr,
		g:          g,
	}

	// Goroutine to process blobs
	g.Go(func() error {
		defer blobR.Close()
		for {
			id, size, err := ReadBlobHeader(blobR)
			if err != nil {
				return firstRealError(err)
			}

			lr := io.LimitReader(blobR, int64(size))
			err = handleBlob(id, lr)
			if err != nil {
				blobW.Close()
				return err
			}

			// Ensure the entire blob is consumed.
			_, err = io.Copy(io.Discard, lr)
			if err != nil {
				blobW.Close()
				return err
			}
		}
	})

	// Goroutine to feed the writer
	g.Go(func() error {
		_, err := io.Copy(writer, r)
		return firstRealError(err, pw.Close(), blobW.Close())
	})

	return rc
}

func firstRealError(errs ...error) error {
	for _, err := range errs {
		if err != nil && err != io.EOF && err != io.ErrClosedPipe {
			return err
		}
	}
	return nil
}

type reader struct {
	*io.PipeReader
	g *errgroup.Group
}

func (r *reader) Close() error {
	r.PipeReader.Close()
	err := r.g.Wait()
	return err
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
