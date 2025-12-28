package signer_test

import (
	"testing"

	"github.com/pseudomuto/sumdb"
	. "github.com/pseudomuto/sumdb/internal/signer"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/sumdb/tlog"
)

func TestNewSigner(t *testing.T) {
	skey, _, err := sumdb.GenerateKeys("test.example.com")
	require.NoError(t, err)

	// Should be able to recreate signer from encoded key
	s, err := NewSigner(skey)
	require.NoError(t, err)
	require.Equal(t, "test.example.com", s.Name())
}

func TestNewSigner_InvalidKey(t *testing.T) {
	_, err := NewSigner("invalid key format")
	require.Error(t, err)
}

func TestNewVerifier(t *testing.T) {
	_, vkey, err := sumdb.GenerateKeys("test.example.com")
	require.NoError(t, err)

	// Should be able to create verifier from encoded key
	v, err := NewVerifier(vkey)
	require.NoError(t, err)
	require.Equal(t, "test.example.com", v.Name())
}

func TestNewVerifier_InvalidKey(t *testing.T) {
	_, err := NewVerifier("invalid key format")
	require.Error(t, err)
}

func TestSignAndVerifyTreeHead(t *testing.T) {
	skey, vkey, err := sumdb.GenerateKeys("test.example.com")
	require.NoError(t, err)

	s, err := NewSigner(skey)
	require.NoError(t, err)

	v, err := NewVerifier(vkey)
	require.NoError(t, err)

	tree := tlog.Tree{
		N:    42,
		Hash: tlog.RecordHash([]byte("test record data")),
	}

	signed, err := SignTreeHead(s, tree)
	require.NoError(t, err)
	require.NotEmpty(t, signed)

	// Verify the signature
	verified, err := VerifyTreeHead(v, signed)
	require.NoError(t, err)
	require.Equal(t, tree.N, verified.N)
	require.Equal(t, tree.Hash, verified.Hash)
}

func TestSignAndVerifyTreeHead_RoundTrip(t *testing.T) {
	// Generate key, persist as strings, reload, sign, verify
	skey, vkey, err := sumdb.GenerateKeys("round.trip.test")
	require.NoError(t, err)

	// Simulate persistence and reload
	s, err := NewSigner(skey)
	require.NoError(t, err)

	v, err := NewVerifier(vkey)
	require.NoError(t, err)

	tree := tlog.Tree{N: 100, Hash: tlog.Hash{1, 2, 3}}

	signed, err := SignTreeHead(s, tree)
	require.NoError(t, err)

	verified, err := VerifyTreeHead(v, signed)
	require.NoError(t, err)
	require.Equal(t, tree.N, verified.N)
	require.Equal(t, tree.Hash, verified.Hash)
}

func TestVerifyTreeHead_WrongKey(t *testing.T) {
	skey, _, err := sumdb.GenerateKeys("signer.example.com")
	require.NoError(t, err)

	_, wrongVKey, err := sumdb.GenerateKeys("other.example.com")
	require.NoError(t, err)

	s, err := NewSigner(skey)
	require.NoError(t, err)

	wrongV, err := NewVerifier(wrongVKey)
	require.NoError(t, err)

	tree := tlog.Tree{N: 1, Hash: tlog.Hash{}}
	signed, err := SignTreeHead(s, tree)
	require.NoError(t, err)

	_, err = VerifyTreeHead(wrongV, signed)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrVerifyFailed)
}

func TestVerifyTreeHead_TamperedData(t *testing.T) {
	skey, vkey, err := sumdb.GenerateKeys("test.example.com")
	require.NoError(t, err)

	s, err := NewSigner(skey)
	require.NoError(t, err)

	v, err := NewVerifier(vkey)
	require.NoError(t, err)

	tree := tlog.Tree{N: 42, Hash: tlog.Hash{}}
	signed, err := SignTreeHead(s, tree)
	require.NoError(t, err)

	// Tamper with the signed data
	signed[10] ^= 0xFF

	_, err = VerifyTreeHead(v, signed)
	require.Error(t, err)
}

func TestVerifyTreeHead_InvalidNoteFormat(t *testing.T) {
	_, vkey, err := sumdb.GenerateKeys("test.example.com")
	require.NoError(t, err)

	v, err := NewVerifier(vkey)
	require.NoError(t, err)

	// Pass garbage that isn't a valid signed note
	_, err = VerifyTreeHead(v, []byte("not a valid note"))
	require.Error(t, err)
}
