// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package textandbinaryreader

import (
	"bytes"
	"io"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestWriter(t *testing.T) {
	c := qt.New(t)

	// A helper to create the binary representation of a blob for the expected output.
	newExpectedBinary := func(id, size uint32, data []byte) []byte {
		var buf bytes.Buffer
		_ = WriteBlobHeader(&buf, id, size)
		buf.Write(data)
		return buf.Bytes()
	}

	testCases := []struct {
		name           string
		writes         []func(w io.Writer) error
		expectedText   string
		expectedBinary []byte
	}{
		{
			name: "text only",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					_, err := w.Write([]byte("Hello, World!"))
					return err
				},
			},
			expectedText: "Hello, World!",
		},
		{
			name: "blob only",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					return WriteBlobHeader(w, 1, 4)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{1, 2, 3, 4})
					return err
				},
			},
			expectedBinary: newExpectedBinary(1, 4, []byte{1, 2, 3, 4}),
		},
		{
			name: "text, blob, text",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					_, err := w.Write([]byte("one"))
					return err
				},
				func(w io.Writer) error {
					return WriteBlobHeader(w, 2, 4)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{5, 6, 7, 8})
					return err
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte("two"))
					return err
				},
			},
			expectedText:   "onetwo",
			expectedBinary: newExpectedBinary(2, 4, []byte{5, 6, 7, 8}),
		},
		{
			name: "split data write",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					return WriteBlobHeader(w, 4, 4)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{1, 2})
					return err
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{3, 4})
					return err
				},
			},
			expectedBinary: newExpectedBinary(4, 4, []byte{1, 2, 3, 4}),
		},
		{
			name: "multiple blobs",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					_, err := w.Write([]byte("a"))
					return err
				},
				func(w io.Writer) error {
					return WriteBlobHeader(w, 5, 2)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{1, 2})
					return err
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte("b"))
					return err
				},
				func(w io.Writer) error {
					return WriteBlobHeader(w, 6, 2)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte{3, 4})
					return err
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte("c"))
					return err
				},
			},
			expectedText:   "abc",
			expectedBinary: append(newExpectedBinary(5, 2, []byte{1, 2}), newExpectedBinary(6, 2, []byte{3, 4})...),
		},
		{
			name: "zero-length blob",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					_, err := w.Write([]byte("a"))
					return err
				},
				func(w io.Writer) error {
					return WriteBlobHeader(w, 7, 0)
				},
				func(w io.Writer) error {
					_, err := w.Write([]byte("b"))
					return err
				},
			},
			expectedText:   "ab",
			expectedBinary: newExpectedBinary(7, 0, []byte{}),
		},
		{
			name: "text with partial marker",
			writes: []func(w io.Writer) error{
				func(w io.Writer) error {
					_, err := w.Write([]byte("Hello TAK35EM World"))
					return err
				},
			},
			expectedText: "Hello TAK35EM World",
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			textReader, textWriter := io.Pipe()
			binaryReader, binaryWriter := io.Pipe()

			w := NewWriter(textWriter, binaryWriter)

			var textOut, binaryOut bytes.Buffer
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				_, err := io.Copy(&textOut, textReader)
				if err != nil && (err != io.EOF && err != io.ErrClosedPipe) {
					c.Errorf("error copying text: %v", err)
				}
			}()
			go func() {
				defer wg.Done()
				_, err := io.Copy(&binaryOut, binaryReader)
				if err != nil && (err != io.EOF && err != io.ErrClosedPipe) {
					c.Errorf("error copying binary: %v", err)
				}
			}()

			go func() {
				for _, writeFn := range tc.writes {
					err := writeFn(w)
					c.Assert(err, qt.IsNil)
				}
				textWriter.Close()
				binaryWriter.Close()
			}()

			// Wait for readers to finish.
			wg.Wait()

			if len(tc.expectedBinary) == 0 {
				c.Assert(binaryOut.Len(), qt.Equals, 0)
			} else {
				c.Assert(binaryOut.Bytes(), qt.DeepEquals, tc.expectedBinary)
			}
			c.Assert(textOut.String(), qt.Equals, tc.expectedText)
		})
	}
}

func BenchmarkWriter(b *testing.B) {
	textBuf := bytes.Repeat([]byte("Hello, World!\n"), 1000)
	blobData := bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var textOut bytes.Buffer
		var binaryOut bytes.Buffer

		w := NewWriter(&textOut, &binaryOut)

		// Write text
		_, _ = w.Write(textBuf)

		// Write blob
		_ = WriteBlobHeader(w, 42, uint32(len(blobData)))
		_, _ = w.Write(blobData)
	}
}
