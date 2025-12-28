// Package signer provides Ed25519 signing and verification for sumdb tree heads.
package signer

import (
	"errors"
	"fmt"

	"golang.org/x/mod/sumdb/note"
	"golang.org/x/mod/sumdb/tlog"
)

var (
	ErrInvalidNote  = errors.New("invalid note format")
	ErrVerifyFailed = errors.New("signature verification failed")
)

// NewSigner creates a Signer from an encoded signer key.
// The skey must be in the format "PRIVATE+KEY+<name>+<hash>+<keydata>".
func NewSigner(skey string) (note.Signer, error) {
	return note.NewSigner(skey)
}

// NewVerifier creates a Verifier from an encoded verifier key.
// The vkey must be in the format "<name>+<hash>+<keydata>".
func NewVerifier(vkey string) (note.Verifier, error) {
	return note.NewVerifier(vkey)
}

// SignTreeHead signs a tree and returns the signed note bytes.
func SignTreeHead(signer note.Signer, tree tlog.Tree) ([]byte, error) {
	text := tlog.FormatTree(tree)
	return note.Sign(&note.Note{Text: string(text)}, signer)
}

// VerifyTreeHead verifies a signed tree head and returns the parsed tree.
func VerifyTreeHead(verifier note.Verifier, signed []byte) (tlog.Tree, error) {
	verifiers := note.VerifierList(verifier)
	n, err := note.Open(signed, verifiers)
	if err != nil {
		return tlog.Tree{}, fmt.Errorf("%w: %w", ErrVerifyFailed, err)
	}

	tree, err := tlog.ParseTree([]byte(n.Text))
	if err != nil {
		return tlog.Tree{}, fmt.Errorf("%w: %w", ErrInvalidNote, err)
	}

	return tree, nil
}
