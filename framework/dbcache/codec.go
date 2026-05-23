package dbcache

import (
	"encoding/gob"
	"encoding/json"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

// Codec 缓存编解码器接口
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSONCodec 默认的 JSON 编解码器
type JSONCodec struct{}

func (JSONCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (JSONCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// GobCodec 基于 encoding/gob 的编解码器，比 JSON 更高效
type GobCodec struct{}

func (GobCodec) Marshal(v any) ([]byte, error) {
	var buf buffer
	err := gob.NewEncoder(&buf).Encode(v)
	return buf.Bytes(), err
}

func (GobCodec) Unmarshal(data []byte, v any) error {
	return gob.NewDecoder(&reader{data: data}).Decode(v)
}

// MsgpackCodec 基于 msgpack 的编解码器，比 JSON 更高效且体积更小
type MsgpackCodec struct{}

func (MsgpackCodec) Marshal(v any) ([]byte, error)      { return msgpack.Marshal(v) }
func (MsgpackCodec) Unmarshal(data []byte, v any) error { return msgpack.Unmarshal(data, v) }

// buffer 实现了 io.Writer 的简单 byte buffer
type buffer struct {
	data []byte
}

func (b *buffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *buffer) Bytes() []byte { return b.data }

// reader 实现了 io.Reader，从 byte slice 读取
type reader struct {
	data   []byte
	offset int
}

func (r *reader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
