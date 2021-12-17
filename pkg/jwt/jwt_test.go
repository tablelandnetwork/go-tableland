package jwt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const validToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJFVEgiLCJraWQiOiJldGg6MTMzNzoweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.eyJuYmYiOjE2Mzk3NTQ2NDIsImlhdCI6MTYzOTc1NDY1MiwiZXhwIjoxNjc1NzU0NjUyLCJpc3MiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYiLCJhdWQiOiJicm9rZXIuaWQiLCJzdWIiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.DdEKSyiiLS-E2LWT0Qv9KkvoxA86F_HsWs7wu9qwv35YnSZJjcaf3EfNwMNUn2HLHsI1ZoHp-HDm1dYcXmwqKRw"   //nolint
const expiredToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJFVEgiLCJraWQiOiJldGg6MTMzNzoweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.eyJuYmYiOjE2Mzk3NTQ4MTEsImlhdCI6MTYzOTc1NDgyMSwiZXhwIjoxNjM5NzU0ODgxLCJpc3MiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYiLCJhdWQiOiJicm9rZXIuaWQiLCJzdWIiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.i56mBqFMEBggFjm1l7Iy_shTTHr0DEx-iOEoJTbc25VIioI6DDe-n6LKqVKuBLuoP_fx87KEFLko13Mhs9iSbRs" //nolint
const invalidToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJFVEgiLCJraWQiOiJldGg6MTMzNzoweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.eyJuYmYiOjE2Mzk3NTQ2NDIsImlhdCI6MTYzOTc1NDY1MiwiZXhwIjoxNjc1NzU0NjUyLCJpc3MiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYiLCJhdWQiOiJicm9rZXIuaWQiLCJzdWIiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.i56mBqFMEBggFjm1l7Iy_shTTHr0DEx-iOEoJTbc25VIioI6DDe-n6LKqVKuBLuoP_fx87KEFLko13Mhs9iSbRs" //nolint
const misformattedToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJFVEgiLCJraWQiOiJldGg6MTMzNzoweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.eyJuYmYiOjE2Mzk3NoxNjc1NzU0NjUyLCJpc3MiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYiLCJhdWQiOiJicm9rZXIuaWQiLCJzdWIiOiIweDE3ZWM4NTk3ZmY5MkMzRjQ0NTIzYkRjNjVCRjBmMWJFNjMyOTE3ZmYifQ.DdEKSyiiLS-E2LWT0Qv9KkvoxA86F_HsWs7wu9qwv35YnSZJjcaf3EfNwMNUn2HLHsI1ZoHp-HDm1dYcXmwqKRw"                                 //nolint

func TestValidToken(t *testing.T) {
	jwt, err := Parse(validToken)
	require.NoError(t, err)
	require.NoError(t, jwt.Verify())
}

func TestInvalidToken(t *testing.T) {
	jwt, err := Parse(invalidToken)
	require.NoError(t, err)
	require.Error(t, jwt.Verify())
}

func TestExpiredToken(t *testing.T) {
	jwt, err := Parse(expiredToken)
	require.NoError(t, err)
	require.Error(t, jwt.Verify())
}

func TestMisformattedToken(t *testing.T) {
	_, err := Parse(misformattedToken)
	require.Error(t, err)
}
