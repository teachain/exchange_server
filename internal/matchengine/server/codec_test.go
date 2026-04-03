package server

import (
	"bytes"
	"testing"
)

func TestRPCPkgPackUnpack(t *testing.T) {
	original := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  12345,
		PkgType:  RPCPkgTypeReq,
		Result:   0,
		CRC32:    0,
		Sequence: 100,
		ReqID:    999,
		BodySize: 0,
		ExtSize:  0,
		Body:     []byte(`{"method":"ping"}`),
	}

	packed, err := original.Pack()
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	if len(packed) != RPCPkgHeadSize+len(original.Body) {
		t.Fatalf("Pack() returned wrong size: got %d, want %d",
			len(packed), RPCPkgHeadSize+len(original.Body))
	}

	var unpacked RPCPkg
	err = unpacked.Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}

	if unpacked.Magic != original.Magic {
		t.Errorf("Magic mismatch: got %x, want %x", unpacked.Magic, original.Magic)
	}
	if unpacked.Command != original.Command {
		t.Errorf("Command mismatch: got %d, want %d", unpacked.Command, original.Command)
	}
	if unpacked.PkgType != original.PkgType {
		t.Errorf("PkgType mismatch: got %d, want %d", unpacked.PkgType, original.PkgType)
	}
	if unpacked.Sequence != original.Sequence {
		t.Errorf("Sequence mismatch: got %d, want %d", unpacked.Sequence, original.Sequence)
	}
	if unpacked.ReqID != original.ReqID {
		t.Errorf("ReqID mismatch: got %d, want %d", unpacked.ReqID, original.ReqID)
	}
	if !bytes.Equal(unpacked.Body, original.Body) {
		t.Errorf("Body mismatch: got %s, want %s", unpacked.Body, original.Body)
	}
}

func TestRPCPkgUnpackInvalidMagic(t *testing.T) {
	data := make([]byte, RPCPkgHeadSize)
	data[0] = 0xFF
	data[1] = 0xFF
	data[2] = 0xFF
	data[3] = 0xFF

	var pkg RPCPkg
	err := pkg.Unpack(data)
	if err != ErrInvalidMagic {
		t.Errorf("Unpack() error = %v, want ErrInvalidMagic", err)
	}
}

func TestRPCPkgUnpackBufferTooSmall(t *testing.T) {
	data := make([]byte, RPCPkgHeadSize-1)

	var pkg RPCPkg
	err := pkg.Unpack(data)
	if err != ErrBufferTooSmall {
		t.Errorf("Unpack() error = %v, want ErrBufferTooSmall", err)
	}
}

func TestRPCPkgPackWithNilBody(t *testing.T) {
	pkg := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  1,
		PkgType:  RPCPkgTypePush,
		Sequence: 1,
		ReqID:    1,
	}

	packed, err := pkg.Pack()
	if err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	if len(packed) != RPCPkgHeadSize {
		t.Errorf("Pack() size = %d, want %d", len(packed), RPCPkgHeadSize)
	}

	var unpacked RPCPkg
	err = unpacked.Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack() error = %v", err)
	}

	if unpacked.BodySize != 0 {
		t.Errorf("BodySize = %d, want 0", unpacked.BodySize)
	}
	if unpacked.Body != nil {
		t.Errorf("Body = %v, want nil", unpacked.Body)
	}
}

func TestRPCPkgConstants(t *testing.T) {
	if RPCPkgMagic != 0x70656562 {
		t.Errorf("RPCPkgMagic = %x, want 0x70656562", RPCPkgMagic)
	}
	if RPCPkgTypeReq != 0 {
		t.Errorf("RPCPkgTypeReq = %d, want 0", RPCPkgTypeReq)
	}
	if RPCPkgTypeResp != 1 {
		t.Errorf("RPCPkgTypeResp = %d, want 1", RPCPkgTypeResp)
	}
	if RPCPkgTypePush != 2 {
		t.Errorf("RPCPkgTypePush = %d, want 2", RPCPkgTypePush)
	}
	if RPCPkgHeadSize != 36 {
		t.Errorf("RPCPkgHeadSize = %d, want 36", RPCPkgHeadSize)
	}
}
