// Copyright 2025 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package golibtemplate

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestReader(t *testing.T) {
	c := qt.New(t)
	rc := New()
	c.Assert(rc, qt.Not(qt.IsNil))
	c.Assert(rc.Close(), qt.IsNil)
}

func TestBlobHeaderWrite(t *testing.T) {
	c := qt.New(t)

	var b bytes.Buffer
	err := BlobHeaderWrite(&b, 42, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(b.Len(), qt.Equals, 8+4+8) // marker + id + size
}
