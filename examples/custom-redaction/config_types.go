package main

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/smegg99/s99config"
)

const (
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
	cyan   = "\x1b[36m"
	reset  = "\x1b[0m"
)

type RedMarkerSecret struct {
	s99config.PresentedSecret
}

type PartialSecret struct {
	s99config.PresentedSecret
}

type FingerprintSecret struct {
	s99config.PresentedSecret
}

func newStyledSecret(value s99config.SecretValue) s99config.Secret {
	switch value.Path() {
	case "/red_marker":
		return RedMarkerSecret{s99config.NewPresentedSecret(value, func(s99config.SecretValue) string {
			return red + "[REDACTED]" + reset
		})}
	case "/partial":
		return PartialSecret{s99config.NewPresentedSecret(value, func(value s99config.SecretValue) string {
			text := []rune(value.Reveal())
			if len(text) <= 8 {
				return yellow + "..." + reset
			}
			return yellow + string(text[:4]) + "..." + string(text[len(text)-4:]) + reset
		})}
	case "/fingerprint":
		return FingerprintSecret{s99config.NewPresentedSecret(value, func(value s99config.SecretValue) string {
			sum := sha256.Sum256([]byte(value.Reveal()))
			return cyan + "[sha256:" + hex.EncodeToString(sum[:])[:12] + "]" + reset
		})}
	default:
		return s99config.NewPresentedSecret(value, nil)
	}
}

var (
	_ s99config.Secret = RedMarkerSecret{}
	_ s99config.Secret = PartialSecret{}
	_ s99config.Secret = FingerprintSecret{}
)
