package codec

import (
	"github.com/ugorji/go/codec"
)

var (
	// MsgPack is a Codec that uses the `ugorji/go/codec` package.
	MsgPack = Codec{msgPackMarshal, msgPackUnmarshal}
)

func msgPackMarshal(v interface{}) (out []byte, err error) {
	var h codec.Handle = new(codec.MsgpackHandle)
	err = codec.NewEncoderBytes(&out, h).Encode(v)
	return
}

func msgPackUnmarshal(in []byte, v interface{}) error {
	var h codec.Handle = new(codec.MsgpackHandle)
	return codec.NewDecoderBytes(in, h).Decode(v)
}
