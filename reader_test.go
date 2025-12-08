// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package textandbinaryreader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"
)

var writeBlob = func(c *qt.C, w io.Writer, id uint32, data []byte) {
	c.Helper()
	err := WriteBlobHeader(w, id, uint32(len(data)))
	c.Assert(err, qt.IsNil)
	_, err = w.Write(data)
	c.Assert(err, qt.IsNil)
}

func TestReader(t *testing.T) {
	c := qt.New(t)

	const testJSONTemplate = `{"item":"Item %d","price":%d}`
	json := func(i int) string {
		return fmt.Sprintf(testJSONTemplate, i, i*10)
	}

	blob1 := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	blob2 := []byte{0xCA, 0xFE, 0xBA, 0xBE}

	testCases := []struct {
		name        string
		buildInput  func(c *qt.C) io.Reader
		readBufSize int
		handler     string // default, error, partialRead
		wantOutput  string
		wantBlobIDs []uint32
		wantBlobs   [][]byte
		wantErr     string
	}{
		{
			name: "text-blob-text",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 101, blob1)
				buf.WriteString(json(2))
				return &buf
			},
			wantOutput:  json(1) + json(2),
			wantBlobIDs: []uint32{101},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "starts with blob",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				writeBlob(c, &buf, 102, blob1)
				buf.WriteString(json(1))
				return &buf
			},
			wantOutput:  json(1),
			wantBlobIDs: []uint32{102},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "ends with blob",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 103, blob1)
				return &buf
			},
			wantOutput:  json(1),
			wantBlobIDs: []uint32{103},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "multiple consecutive blobs",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				writeBlob(c, &buf, 104, blob1)
				writeBlob(c, &buf, 201, blob2)
				return &buf
			},
			wantOutput:  "",
			wantBlobIDs: []uint32{104, 201},
			wantBlobs:   [][]byte{blob1, blob2},
		},
		{
			name: "multiple blobs not in sequence",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 105, blob1)
				buf.WriteString(json(2))
				writeBlob(c, &buf, 202, blob2)
				buf.WriteString(json(3))
				return &buf
			},
			wantOutput:  json(1) + json(2) + json(3),
			wantBlobIDs: []uint32{105, 202},
			wantBlobs:   [][]byte{blob1, blob2},
		},
		{
			name: "zero-length blob",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 106, []byte{})
				buf.WriteString(json(2))
				return &buf
			},
			wantOutput:  json(1) + json(2),
			wantBlobIDs: []uint32{106},
			wantBlobs:   [][]byte{{}},
		},
		{
			name: "partial marker in text",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				buf.Write(BlobMarker[:4])
				buf.WriteString(json(2))
				writeBlob(c, &buf, 107, blob1)
				return &buf
			},
			wantOutput:  json(1) + string(BlobMarker[:4]) + json(2),
			wantBlobIDs: []uint32{107},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "empty stream",
			buildInput: func(c *qt.C) io.Reader {
				return &bytes.Buffer{}
			},
			wantOutput: "",
		},
		{
			name: "text only",
			buildInput: func(c *qt.C) io.Reader {
				return bytes.NewBufferString(json(1) + json(2))
			},
			wantOutput: json(1) + json(2),
		},
		{
			name: "blob only",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				writeBlob(c, &buf, 108, blob1)
				return &buf
			},
			wantOutput:  "",
			wantBlobIDs: []uint32{108},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "small read buffer",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 109, blob1)
				buf.WriteString(json(2))
				return &buf
			},
			readBufSize: 5,
			wantOutput:  json(1) + json(2),
			wantBlobIDs: []uint32{109},
			wantBlobs:   [][]byte{blob1},
		},
		{
			name: "blob handler error",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 110, blob1)
				return &buf
			},
			handler:    "error",
			wantOutput: json(1),
			wantErr:    "handler error",
		},
		{
			name: "blob not fully read",
			buildInput: func(c *qt.C) io.Reader {
				var buf bytes.Buffer
				buf.WriteString(json(1))
				writeBlob(c, &buf, 111, blob1)
				buf.WriteString(json(2))
				return &buf
			},
			handler:     "partialRead",
			wantOutput:  json(1) + json(2),
			wantBlobIDs: []uint32{111},
			wantBlobs:   [][]byte{blob1[:1]},
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			var (
				handledBlobs   [][]byte
				handledBlobIDs []uint32
			)

			var handleBlob func(id uint32, r io.Reader) error
			switch tc.handler {
			case "error":
				handleBlob = func(id uint32, r io.Reader) error {
					return fmt.Errorf("handler error")
				}
			case "partialRead":
				handleBlob = func(id uint32, r io.Reader) error {
					handledBlobIDs = append(handledBlobIDs, id)
					// Read only one byte.
					b := make([]byte, 1)
					_, err := r.Read(b)
					handledBlobs = append(handledBlobs, b)
					return err
				}
			default:
				handleBlob = func(id uint32, r io.Reader) error {
					b, err := io.ReadAll(r)
					c.Assert(err, qt.IsNil)
					handledBlobIDs = append(handledBlobIDs, id)
					handledBlobs = append(handledBlobs, b)
					return nil
				}
			}

			r := New(tc.buildInput(c), handleBlob)
			var outBuf bytes.Buffer
			var err error

			if tc.readBufSize > 0 {
				_, err = io.CopyBuffer(&outBuf, r, make([]byte, tc.readBufSize))
			} else {
				_, err = io.Copy(&outBuf, r)
			}

			if tc.wantErr != "" {
				c.Assert(err, qt.ErrorMatches, tc.wantErr)
			} else {
				c.Assert(err, qt.IsNil)
			}

			c.Assert(outBuf.String(), qt.Equals, tc.wantOutput)

			if tc.wantBlobs != nil {
				c.Assert(handledBlobs, qt.DeepEquals, tc.wantBlobs)
			}
			if tc.wantBlobIDs != nil {
				c.Assert(handledBlobIDs, qt.DeepEquals, tc.wantBlobIDs)
			}
		})
	}
}

func TestReaderJSONDecode(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	buf.WriteString("{\"item\":\"Item 1\",\"price\":10}\n")
	writeBlob(c, &buf, 201, []byte{0xAA, 0xBB, 0xCC})
	buf.WriteString("{\"item\":\"Item 2\",\"price\":20}\n")

	var decodedItems []struct {
		Item  string `json:"item"`
		Price int    `json:"price"`
	}

	handleBlob := func(id uint32, r io.Reader) error {
		io.Copy(io.Discard, r)
		return nil
	}

	r := New(&buf, handleBlob)
	decoder := json.NewDecoder(r)

	for {
		var item struct {
			Item  string `json:"item"`
			Price int    `json:"price"`
		}
		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		decodedItems = append(decodedItems, item)
	}

	c.Assert(decodedItems, qt.DeepEquals, []struct {
		Item  string `json:"item"`
		Price int    `json:"price"`
	}{
		{"Item 1", 10},
		{"Item 2", 20},
	})
}

func TestBlobHeaderWriteAndRead(t *testing.T) {
	c := qt.New(t)

	var b bytes.Buffer
	err := WriteBlobHeader(&b, 42, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(b.Len(), qt.Equals, 8+4+4) // marker + id + size

	id, size, err := ReadBlobHeader(&b)
	c.Assert(err, qt.IsNil)
	c.Assert(id, qt.Equals, uint32(42))
	c.Assert(size, qt.Equals, uint32(100))
}

func BenchmarkReader(b *testing.B) {
	bh := func(id uint32, r io.Reader) error {
		// Do nothing.
		return nil
	}

	var buf bytes.Buffer
	for i := 0; i < 1000; i++ {
		buf.WriteString(fmt.Sprintf(`{"item":"Item %d","price":%d}`, i, i*10))

		// Write a blob.
		blobData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		err := WriteBlobHeader(&buf, uint32(i), uint32(len(blobData)))
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(blobData)
	}

	b.ResetTimer()

	for b.Loop() {
		r := New(&buf, bh)
		_, err := io.Copy(io.Discard, r)
		if err != nil {
			b.Fatal(err)
		}
	}
}
