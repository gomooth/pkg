package dbcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type codecTestStruct struct {
	Name  string
	Value int
	Tags  []string
}

func testCodecRoundTrip(t *testing.T, codec Codec) {
	t.Helper()

	original := &codecTestStruct{
		Name:  "test",
		Value: 42,
		Tags:  []string{"go", "cache"},
	}

	data, err := codec.Marshal(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded codecTestStruct
	err = codec.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Value, decoded.Value)
	assert.Equal(t, original.Tags, decoded.Tags)
}

func TestJSONCodec_RoundTrip(t *testing.T) {
	testCodecRoundTrip(t, JSONCodec{})
}

func TestGobCodec_RoundTrip(t *testing.T) {
	testCodecRoundTrip(t, GobCodec{})
}

func TestMsgpackCodec_RoundTrip(t *testing.T) {
	testCodecRoundTrip(t, MsgpackCodec{})
}

func TestJSONCodec_NilInput(t *testing.T) {
	codec := JSONCodec{}
	_, err := codec.Marshal(nil)
	assert.NoError(t, err)
}

func TestGobCodec_NilInput(t *testing.T) {
	codec := GobCodec{}
	err := codec.Unmarshal([]byte{}, nil)
	assert.Error(t, err)
}

func TestMsgpackCodec_NilInput(t *testing.T) {
	codec := MsgpackCodec{}
	_, err := codec.Marshal(nil)
	assert.NoError(t, err)
}

func BenchmarkJSONCodec_Marshal(b *testing.B) {
	codec := JSONCodec{}
	data := &codecTestStruct{Name: "benchmark", Value: 123, Tags: []string{"a", "b"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Marshal(data)
	}
}

func BenchmarkGobCodec_Marshal(b *testing.B) {
	codec := GobCodec{}
	data := &codecTestStruct{Name: "benchmark", Value: 123, Tags: []string{"a", "b"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Marshal(data)
	}
}

func BenchmarkMsgpackCodec_Marshal(b *testing.B) {
	codec := MsgpackCodec{}
	data := &codecTestStruct{Name: "benchmark", Value: 123, Tags: []string{"a", "b"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Marshal(data)
	}
}
