package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServer(t *testing.T) {
	rec := httptest.NewRecorder()

	body := strings.NewReader(`{
		"node_id" : "4f9393d5-18a5-4dab-bf29-e82f91d600ce",
		"metrics" : [
			{
				"version" : 1,
				"timestamp" : "2018-09-22T12:42:31+07:00",
				"type" : 0, 
				"payload" : {
					"version":1,
					"chain_id":69,
					"block_number":7724201,
					"hash":"f49cb8ed68020595cfb517635663785e47b120c9"
				}
			}
		]
	}`)

	req := httptest.NewRequest("POST", "/", body)

	m := &mock{}
	makeHandler(m)(rec, req)
	if rec.Code != 200 && !m.called {
		t.Fail()
	}
}

type mock struct {
	called bool
}

func (m *mock) insert(_ context.Context, _ request) error {
	m.called = true
	return nil
}
