package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/owenrumney/go-lsp/servertest"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConvertErrorToDiagnostics(t *testing.T) {
	// Standard parse error with line X char Y.
	err := errors.New("failed to parse: line 2 char 5: unexpected token")
	diags := convertErrorToDiagnostics("dummy", err)

	assert.Len(t, diags, 1)
	assert.Equal(t, 1, diags[0].Range.Start.Line)
	assert.Equal(t, 4, diags[0].Range.Start.Character)
	assert.Equal(t, "unexpected token", diags[0].Message)

	// Fallback when line / char not found.
	err2 := errors.New("something went wrong entirely")
	diags2 := convertErrorToDiagnostics("dummy", err2)

	assert.Len(t, diags2, 1)
	assert.Equal(t, 0, diags2[0].Range.Start.Line)
	assert.Equal(t, 0, diags2[0].Range.Start.Character)
	assert.Equal(t, "something went wrong entirely", diags2[0].Message)
}

func TestDiagnosticUnicodeColumns(t *testing.T) {
	text := "root = \"😀\" + ("
	h, _ := featureHandler(t)
	_, err := h.benv.Parse(text)
	assert.Error(t, err)
	diags := convertErrorToDiagnostics(text, err)
	assert.Len(t, diags, 1)
	assert.Equal(t, bytePosition(text, len(text)), diags[0].Range.Start)
}

func TestEmptyDiagnosticPublicationIsArrayOnWire(t *testing.T) {
	h, uri := featureHandler(t)
	client := servertest.New(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, publish := range []func(){func() { h.publishDiagnostics(ctx, uri, nil) }, func() { h.publishExecDiagnostics(ctx, uri, nil) }} {
		client.ClearDiagnostics()
		publish()
		ds, err := client.WaitForDiagnostics(ctx, uri)
		require.NoError(t, err)
		require.NotNil(t, ds, "empty publication must decode from [] rather than null")
		require.Empty(t, ds)
		notifications := client.AllDiagnostics()
		require.NotEmpty(t, notifications)
		encoded, err := json.Marshal(notifications[len(notifications)-1])
		require.NoError(t, err)
		require.Contains(t, string(encoded), `"diagnostics":[]`)
	}
}
func TestInputRepairClearsDiagnosticOnWire(t *testing.T) {
	h, uri := featureHandler(t)
	client := servertest.New(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, client.DidOpen(uri, "bloblang", "#!input {\"foo\":\nroot = this"))
	ds, err := client.WaitForDiagnostics(ctx, uri)
	require.NoError(t, err)
	require.NotEmpty(t, ds)
	client.ClearDiagnostics()
	require.NoError(t, client.DidChange(uri, 2, "#!input {\"foo\":\"bar\"}\nroot = this"))
	ds, err = client.WaitForDiagnostics(ctx, uri)
	require.NoError(t, err)
	require.NotNil(t, ds)
	require.Empty(t, ds)
}
