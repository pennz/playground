package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestLimitedWriter(t *testing.T) {
	cases := []struct {
		desc          string
		lw            *limitedWriter
		in            []byte
		want          []byte
		wantN         int64
		wantRemaining int64
		err           error
	}{
		{
			desc:          "simple",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte("hi"),
			want:          []byte("hi"),
			wantN:         2,
			wantRemaining: 8,
		},
		{
			desc:          "writing nothing",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte(""),
			want:          []byte(""),
			wantN:         0,
			wantRemaining: 10,
		},
		{
			desc:          "writing exactly enough",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 6},
			in:            []byte("enough"),
			want:          []byte("enough"),
			wantN:         6,
			wantRemaining: 0,
			err:           nil,
		},
		{
			desc:          "writing too much",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte("this is much longer than 10"),
			want:          []byte("this is mu"),
			wantN:         10,
			wantRemaining: -1,
			err:           errTooMuchOutput,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			n, err := io.Copy(c.lw, iotest.OneByteReader(bytes.NewReader(c.in)))
			if err != c.err {
				t.Errorf("c.lw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if n != c.wantN {
				t.Errorf("c.lw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if c.lw.n != c.wantRemaining {
				t.Errorf("c.lw.n = %d, wanted %d", c.lw.n, c.wantRemaining)
			}
			if string(c.lw.dst.Bytes()) != string(c.want) {
				t.Errorf("c.lw.dst.Bytes() = %q, wanted %q", c.lw.dst.Bytes(), c.want)
			}
		})
	}
}

func TestSwitchWriter(t *testing.T) {
	cases := []struct {
		desc      string
		sw        *switchWriter
		in        []byte
		want1     []byte
		want2     []byte
		wantN     int64
		wantFound bool
		err       error
	}{
		{
			desc:      "not found",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("hi"),
			want1:     []byte("hi"),
			want2:     []byte(""),
			wantN:     2,
			wantFound: false,
		},
		{
			desc:      "writing nothing",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte(""),
			want1:     []byte(""),
			want2:     []byte(""),
			wantN:     0,
			wantFound: false,
		},
		{
			desc:      "writing exactly switchAfter",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("UNIQUE"),
			want1:     []byte("UNIQUE"),
			want2:     []byte(""),
			wantN:     6,
			wantFound: true,
		},
		{
			desc:      "writing before and after switchAfter",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("this is before UNIQUE and this is after"),
			want1:     []byte("this is before UNIQUE"),
			want2:     []byte(" and this is after"),
			wantN:     39,
			wantFound: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			dst1, dst2 := &bytes.Buffer{}, &bytes.Buffer{}
			c.sw.dst1, c.sw.dst2 = dst1, dst2
			n, err := io.Copy(c.sw, iotest.OneByteReader(bytes.NewReader(c.in)))
			if err != c.err {
				t.Errorf("c.sw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if n != c.wantN {
				t.Errorf("c.sw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if c.sw.found != c.wantFound {
				t.Errorf("c.sw.found = %v, wanted %v", c.sw.found, c.wantFound)
			}
			if string(dst1.Bytes()) != string(c.want1) {
				t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), c.want1)
			}
			if string(dst2.Bytes()) != string(c.want2) {
				t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), c.want2)
			}
		})
	}
}

func TestSwitchWriterMultipleWrites(t *testing.T) {
	dst1, dst2 := &bytes.Buffer{}, &bytes.Buffer{}
	sw := &switchWriter{
		dst1:        dst1,
		dst2:        dst2,
		switchAfter: []byte("GOPHER"),
	}
	n, err := io.Copy(sw, iotest.OneByteReader(strings.NewReader("this is before GO")))
	if err != nil || n != 17 {
		t.Errorf("sw.Write(%q) = %d, %q, wanted %d, no error", "this is before GO", n, err, 17)
	}
	if sw.found {
		t.Errorf("sw.found = %v, wanted %v", sw.found, false)
	}
	if string(dst1.Bytes()) != "this is before GO" {
		t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), "this is before GO")
	}
	if string(dst2.Bytes()) != "" {
		t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), "")
	}
	n, err = io.Copy(sw, iotest.OneByteReader(strings.NewReader("PHER and this is after")))
	if err != nil || n != 22 {
		t.Errorf("sw.Write(%q) = %d, %q, wanted %d, no error", "this is before GO", n, err, 22)
	}
	if !sw.found {
		t.Errorf("sw.found = %v, wanted %v", sw.found, true)
	}
	if string(dst1.Bytes()) != "this is before GOPHER" {
		t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), "this is before GOPHEr")
	}
	if string(dst2.Bytes()) != " and this is after" {
		t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), " and this is after")
	}
}
