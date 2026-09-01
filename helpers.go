package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
)

// tokenHex genera un token aleatorio en hexadecimal (equivalente a secrets.token_hex).
func tokenHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(filepath.Base(os.TempDir())))[:n*2]
	}
	return hex.EncodeToString(b)
}

// tokenURLSafe genera un token aleatorio URL-safe (equivalente a secrets.token_urlsafe).
func tokenURLSafe(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	// base64 URL-safe sin padding
	const alfabeto = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, n*4/3+1)
	for _, c := range b {
		out = append(out, alfabeto[int(c)%64])
	}
	return string(out)
}

// escribirArchivo crea el archivo (y su carpeta) con el contenido dado.
func escribirArchivo(ruta string, contenido []byte) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ruta, contenido, 0o644)
}

var (
	osStat   = os.Stat
	osRemove = os.Remove
)
