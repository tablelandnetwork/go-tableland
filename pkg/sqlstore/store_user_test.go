package sqlstore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
)

func TestUserValue(t *testing.T) {
	uv := &tableland.ColValue{}

	var in0 int64 = 100
	require.NoError(t, uv.Scan(in0))
	val := uv.Value()
	v0, ok := val.(int64)
	require.True(t, ok)
	require.Equal(t, in0, v0)
	b, err := json.Marshal(uv)
	require.NoError(t, err)
	var out0 int64
	require.NoError(t, json.Unmarshal(b, &out0))
	require.Equal(t, in0, out0)

	in1 := 100.0
	require.NoError(t, uv.Scan(in1))
	val = uv.Value()
	v1, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, in1, v1)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out1 float64
	require.NoError(t, json.Unmarshal(b, &out1))
	require.Equal(t, in1, out1)

	in2 := true
	require.NoError(t, uv.Scan(in2))
	val = uv.Value()
	v2, ok := val.(bool)
	require.True(t, ok)
	require.Equal(t, in2, v2)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out2 bool
	require.NoError(t, json.Unmarshal(b, &out2))
	require.Equal(t, in2, out2)

	in3 := []byte("hello there")
	require.NoError(t, uv.Scan(in3))
	val = uv.Value()
	v3, ok := val.([]byte)
	require.True(t, ok)
	require.Equal(t, in3, v3)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out3 []byte
	require.NoError(t, json.Unmarshal(b, &out3))
	require.Equal(t, in3, out3)

	in4 := "hello"
	require.NoError(t, uv.Scan(in4))
	val = uv.Value()
	v4, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in4, v4)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out4 string
	require.NoError(t, json.Unmarshal(b, &out4))
	require.Equal(t, in4, out4)

	in5 := time.Now()
	require.NoError(t, uv.Scan(in5))
	val = uv.Value()
	v5, ok := val.(time.Time)
	require.True(t, ok)
	require.Equal(t, in5, v5)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out5 time.Time
	require.NoError(t, json.Unmarshal(b, &out5))
	require.Equal(t, in5.Unix(), out5.Unix())

	var in6 interface{}
	require.NoError(t, uv.Scan(in6))
	val = uv.Value()
	require.Nil(t, val)
	require.Equal(t, in6, val)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out6 interface{}
	require.NoError(t, json.Unmarshal(b, &out6))
	require.Equal(t, in6, out6)

	in7 := "{ \"hello"
	require.NoError(t, uv.Scan(in7))
	val = uv.Value()
	v7, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in7, v7)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out7 string
	require.NoError(t, json.Unmarshal(b, &out7))
	require.Equal(t, in7, out7)

	in8 := "[ \"hello"
	require.NoError(t, uv.Scan(in8))
	val = uv.Value()
	v8, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in8, v8)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out8 string
	require.NoError(t, json.Unmarshal(b, &out8))
	require.Equal(t, in8, out8)

	in9 := "{\"name\":\"aaron\"}"
	require.NoError(t, uv.Scan(in9))
	val = uv.Value()
	v9, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v9), 0)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	require.Equal(t, in9, string(b))

	in10 := "[\"one\",\"two\"]"
	require.NoError(t, uv.Scan(in10))
	val = uv.Value()
	v10, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v10), 0)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	require.Equal(t, in10, string(b))
}
