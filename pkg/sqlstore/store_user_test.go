package sqlstore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserData(t *testing.T) {
	ud := &UserData{}

	var in0 int64 = 100
	require.NoError(t, ud.Scan(in0))
	val := ud.Value()
	v0, ok := val.(int64)
	require.True(t, ok)
	require.Equal(t, in0, v0)
	b, err := json.Marshal(ud)
	require.NoError(t, err)
	var out0 int64
	require.NoError(t, json.Unmarshal(b, &out0))
	require.Equal(t, in0, out0)

	in1 := 100.0
	require.NoError(t, ud.Scan(in1))
	val = ud.Value()
	v1, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, in1, v1)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out1 float64
	require.NoError(t, json.Unmarshal(b, &out1))
	require.Equal(t, in1, out1)

	in2 := true
	require.NoError(t, ud.Scan(in2))
	val = ud.Value()
	v2, ok := val.(bool)
	require.True(t, ok)
	require.Equal(t, in2, v2)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out2 bool
	require.NoError(t, json.Unmarshal(b, &out2))
	require.Equal(t, in2, out2)

	in3 := []byte("hello there")
	require.NoError(t, ud.Scan(in3))
	val = ud.Value()
	v3, ok := val.([]byte)
	require.True(t, ok)
	require.Equal(t, in3, v3)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out3 []byte
	require.NoError(t, json.Unmarshal(b, &out3))
	require.Equal(t, in3, out3)

	in4 := "hello"
	require.NoError(t, ud.Scan(in4))
	val = ud.Value()
	v4, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in4, v4)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out4 string
	require.NoError(t, json.Unmarshal(b, &out4))
	require.Equal(t, in4, out4)

	in5 := time.Now()
	require.NoError(t, ud.Scan(in5))
	val = ud.Value()
	v5, ok := val.(time.Time)
	require.True(t, ok)
	require.Equal(t, in5, v5)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out5 time.Time
	require.NoError(t, json.Unmarshal(b, &out5))
	require.Equal(t, in5.Unix(), out5.Unix())

	var in6 interface{}
	require.NoError(t, ud.Scan(in6))
	val = ud.Value()
	require.Nil(t, val)
	require.Equal(t, in6, val)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out6 interface{}
	require.NoError(t, json.Unmarshal(b, &out6))
	require.Equal(t, in6, out6)

	in7 := "{ \"hello"
	require.NoError(t, ud.Scan(in7))
	val = ud.Value()
	v7, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in7, v7)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out7 string
	require.NoError(t, json.Unmarshal(b, &out7))
	require.Equal(t, in7, out7)

	in8 := "[ \"hello"
	require.NoError(t, ud.Scan(in8))
	val = ud.Value()
	v8, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in8, v8)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	var out8 string
	require.NoError(t, json.Unmarshal(b, &out8))
	require.Equal(t, in8, out8)

	in9 := "{\"name\":\"aaron\"}"
	require.NoError(t, ud.Scan(in9))
	val = ud.Value()
	v9, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v9), 0)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	require.Equal(t, in9, string(b))

	in10 := "[\"one\",\"two\"]"
	require.NoError(t, ud.Scan(in10))
	val = ud.Value()
	v10, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v10), 0)
	b, err = json.Marshal(ud)
	require.NoError(t, err)
	require.Equal(t, in10, string(b))
}
