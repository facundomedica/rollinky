package utils

import (
	"bytes"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		a       []byte
		b       []byte
		wantErr bool
	}{
		{
			name:    "valid data",
			a:       []byte("hello"),
			b:       []byte("world"),
			wantErr: false,
		},
		{
			name:    "empty slices",
			a:       []byte{},
			b:       []byte{},
			wantErr: false,
		},
		{
			name:    "nil slices",
			a:       nil,
			b:       nil,
			wantErr: false,
		},
		{
			name:    "large data",
			a:       bytes.Repeat([]byte("a"), 1000),
			b:       bytes.Repeat([]byte("b"), 1000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.a, tt.b)
			a, b, err := Decode(encoded)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !bytes.Equal(a, tt.a) {
				t.Errorf("first slice mismatch: got %v, want %v", a, tt.a)
			}

			if !bytes.Equal(b, tt.b) {
				t.Errorf("second slice mismatch: got %v, want %v", b, tt.b)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{
			name:    "too short",
			data:    []byte{1, 2, 3},
			wantErr: "invalid data: too short",
		},
		{
			name:    "invalid first length",
			data:    []byte{255, 255, 255, 255, 1, 2, 3, 4},
			wantErr: "invalid a length",
		},
		{
			name:    "invalid second length",
			data:    []byte{0, 0, 0, 1, 1, 255, 255, 255, 255},
			wantErr: "invalid b length",
		},
		{
			name:    "extra data",
			data:    []byte{0, 0, 0, 1, 1, 0, 0, 0, 1, 2, 3},
			wantErr: "extra data at end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Decode(tt.data)
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}
