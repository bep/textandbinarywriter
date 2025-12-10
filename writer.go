// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package textandbinaryreader

import (
	"bytes"
	"io"
)

const byteHeaderByteLength = 8 + 4 + 4 // marker + id + size

type Writer struct {
	// Default writer, for text content.
	textw io.Writer
	// When we receive a blob header, we switch to this writer.
	// The header and blob data is written to this writer.
	binaryw io.Writer

	// state
	mode    writerMode
	header  [byteHeaderByteLength]byte
	headerN int // number of header bytes read

	binaryCurrentWriteSize  int64
	binaryCurrentWriteCount int64
}

type writerMode int

const (
	modeText writerMode = iota
	modeHeader
	modeBinary
)

func NewWriter(textw, binaryw io.Writer) *Writer {
	return &Writer{
		textw:   textw,
		binaryw: binaryw,
		mode:    modeText,
	}
}

var _ io.Writer = (*Writer)(nil)

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	var nn int
	for len(p) > 0 {
		switch w.mode {
		case modeText:
			idx := bytes.Index(p, BlobMarker[:])
			if idx == -1 {
				// No marker found, write all to textw.
				nn, err = w.textw.Write(p)
				n += nn
				return n, err
			}

			if idx > 0 {
				// Marker found, write up to the marker to textw.
				nn, err = w.textw.Write(p[:idx])
				n += nn
				if err != nil {
					return n, err
				}
			}

			p = p[idx:]
			w.mode = modeHeader
			w.headerN = 0

		case modeHeader:
			toRead := byteHeaderByteLength - w.headerN
			canRead := len(p)
			if canRead >= toRead {
				copy(w.header[w.headerN:], p[:toRead])
				p = p[toRead:]
				n += toRead

				_, size, err := ReadBlobHeader(bytes.NewReader(w.header[:]))
				if err != nil {
					// Invalid header, reset to text mode and write the buffered header as text.
					w.mode = modeText
					nn, err2 := w.textw.Write(w.header[:w.headerN])
					n += nn
					if err2 != nil {
						return n, err2
					}
					return n, err
				}
				w.binaryCurrentWriteSize = int64(size)
				w.binaryCurrentWriteCount = 0

				_, err = w.binaryw.Write(w.header[:])
				// n is the bytes read from p, not what's written to the underlying writers.
				if err != nil {
					return n, err
				}
				w.mode = modeBinary

			} else {
				copy(w.header[w.headerN:], p)
				w.headerN += canRead
				n += canRead
				p = nil
			}

		case modeBinary:
			remaining := w.binaryCurrentWriteSize - w.binaryCurrentWriteCount
			if remaining <= 0 {
				w.mode = modeText
				continue
			}

			toWrite := int64(len(p))
			if toWrite > remaining {
				toWrite = remaining
			}

			nn, err = w.binaryw.Write(p[:toWrite])
			n += nn
			if err != nil {
				return n, err
			}
			w.binaryCurrentWriteCount += int64(nn)
			p = p[nn:]

		}
	}

	return n, nil
}

func (w *Writer) Close() (err error) {
	if closer, ok := w.textw.(io.Closer); ok {
		if err = closer.Close(); err != nil {
			return
		}
	}
	if closer, ok := w.binaryw.(io.Closer); ok {
		err = closer.Close()
	}
	return
}
