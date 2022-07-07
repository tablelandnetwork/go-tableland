package sqlstore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserData(t *testing.T) {
	ud := &UserData{}

	var input0 int64 = 100
	require.NoError(t, ud.Scan(input0))
	val := ud.Value()
	v0, ok := val.(int64)
	require.True(t, ok)
	require.Equal(t, input0, v0)

	input1 := 100.0
	require.NoError(t, ud.Scan(input1))
	val = ud.Value()
	v1, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, input1, v1)

	input2 := true
	require.NoError(t, ud.Scan(input2))
	val = ud.Value()
	v2, ok := val.(bool)
	require.True(t, ok)
	require.Equal(t, input2, v2)

	input3 := []byte("hello there")
	require.NoError(t, ud.Scan(input3))
	val = ud.Value()
	v3, ok := val.([]byte)
	require.True(t, ok)
	require.Equal(t, input3, v3)

	input4 := "hello"
	require.NoError(t, ud.Scan(input4))
	val = ud.Value()
	v4, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, input4, v4)

	input5 := time.Now()
	require.NoError(t, ud.Scan(input5))
	val = ud.Value()
	v5, ok := val.(time.Time)
	require.True(t, ok)
	require.Equal(t, input5, v5)

	var input6 interface{}
	require.NoError(t, ud.Scan(input6))
	val = ud.Value()
	require.Nil(t, val)
	require.Equal(t, input6, val)

	input7 := "{ \"hello"
	require.NoError(t, ud.Scan(input7))
	val = ud.Value()
	v7, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, input7, v7)

	input8 := "[ \"hello"
	require.NoError(t, ud.Scan(input8))
	val = ud.Value()
	v8, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, input8, v8)

	input9 := "{ \"name\": \"aaron\" }"
	require.NoError(t, ud.Scan(input9))
	val = ud.Value()
	v9, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v9), 0)

	input10 := "[ \"one\", \"two\" ]"
	require.NoError(t, ud.Scan(input10))
	val = ud.Value()
	v10, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v10), 0)
}
