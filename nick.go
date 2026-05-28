package main

import (
	"fmt"
	"math/rand"
	"strings"
)

// ============================================================================
// nick.go — Bot nickname generation system
// Supports multiple modes: plain, numbered, random suffix, random word, custom list
// ============================================================================

// maxNickRunes is gartic's hard limit for a nickname, counted in CHARACTERS
// (runes), not bytes — important for Arabic names where one character is
// several bytes. A nick longer than this is silently rejected and the bot
// never joins the room.
const maxNickRunes = 18

// arabicHiddenChars are the zero-WIDTH characters used to make plain ("mode 0")
// nicks unique WITHOUT adding any visible gap — this matches the extension's
// addHiddenCharToArabicName exactly.
//
// Why this is needed: gartic refuses a join when the nickname is already
// present in the room. With mode 0 every bot would otherwise join with the
// exact same plain name, collide, and only the first one would get in.
//
// Why these specific characters: the old approach used HANGUL FILLER / BRAILLE
// BLANK, which actually take up width and show up as an ugly space/gap inside
// the name. These four — WORD JOINER, FUNCTION APPLICATION, INVISIBLE TIMES,
// INVISIBLE SEPARATOR — have ZERO width, so the name renders with no gap at all
// while still being unique on the wire.
var arabicHiddenChars = []rune{'\u2060', '\u2061', '\u2062', '\u2063'}

// NickResult contains the generated nickname and optional avatar override.
type NickResult struct {
	Nick   string
	Avatar *int
}

// addHiddenChar inserts a run of 1–3 identical zero-width characters at a random
// position inside name, mirroring the extension's addHiddenCharToArabicName.
// The inserted characters are invisible and have no width, so the name shows
// with no gap, while the random character / position / repeat count keep each
// bot's nick unique so gartic does not reject the duplicate-name joins.
func addHiddenChar(name string) string {
	chars := []rune(name)
	idx := rand.Intn(len(chars) + 1)
	hidden := arabicHiddenChars[rand.Intn(len(arabicHiddenChars))]
	count := rand.Intn(3) + 1
	run := strings.Repeat(string(hidden), count)
	return string(chars[:idx]) + run + string(chars[idx:])
}

// clampNick builds a final nick from a base part and a suffix while
// guaranteeing the result is valid for gartic:
//   - empty base falls back to "Bot"
//   - surrounding whitespace is trimmed from the base
//   - the total length never exceeds maxNickRunes — if it would, the BASE is
//     shortened (by runes, so multi-byte/Arabic names are never cut in half)
//     so the suffix (the part that keeps bots unique) is preserved.
func clampNick(base, suffix string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Bot"
	}
	br := []rune(base)
	sr := []rune(suffix)
	if len(br)+len(sr) > maxNickRunes {
		keep := maxNickRunes - len(sr)
		if keep < 0 {
			keep = 0
		}
		br = br[:keep]
	}
	return string(br) + suffix
}

// generateNick creates a bot nickname based on the mode.
//
// Modes (as sent by the extension — note the UI buttons are labelled +1):
//
//	"0" = plain name, suffix-free        (UI button "Bot nick 1")
//	"1" = name + sequential number       (UI button "Bot nick 2")
//	"2" = name + random 3 chars          (UI button "Bot nick 3")
//	"3" = random word + random num
//	"4" = pick from custom nick list (with optional avatar per nick)
//	"5" = name + random number
func generateNick(baseName, mode string, customNicks []CustomNick, queueNum *int) NickResult {
	switch mode {
	case "1": // Sequential number suffix
		// queueNum may be nil on some call paths — dereferencing it directly
		// would panic and kill the bot's goroutine before it ever joins.
		n := 1
		if queueNum != nil {
			*queueNum++
			n = *queueNum
		}
		return NickResult{Nick: clampNick(baseName, fmt.Sprintf("%d", n))}

	case "2": // Random suffix
		chars := "abcdefghijklmnopqrstuvwxyz0123456789"
		suffix := make([]byte, 3)
		for i := range suffix {
			suffix[i] = chars[rand.Intn(len(chars))]
		}
		return NickResult{Nick: clampNick(baseName, string(suffix))}

	case "3": // Random word
		words := []string{
			"Shadow", "Storm", "Phoenix", "Ghost", "Frost", "Blaze",
			"Nova", "Viper", "Wolf", "Hawk", "Tiger", "Bear",
			"Cobra", "Dragon", "Eagle", "Falcon", "Panther", "Raven",
			"Mystic", "Spark", "Ember", "Cyber", "Neon", "Pixel",
			"Turbo", "Flash", "Bolt", "Drift", "Pulse", "Fury",
		}
		word := words[rand.Intn(len(words))]
		return NickResult{Nick: clampNick(word, fmt.Sprintf("%d", rand.Intn(999)))}

	case "4": // Custom nick list
		if len(customNicks) > 0 {
			cn := customNicks[rand.Intn(len(customNicks))]
			return NickResult{Nick: clampNick(cn.Nick, ""), Avatar: cn.Avatar}
		}
		return NickResult{Nick: clampNick(baseName, "")}

	case "5": // Random number suffix (e.g. hi482, hi73)
		return NickResult{Nick: clampNick(baseName, fmt.Sprintf("%d", rand.Intn(1000)))}

	default: // "0" or anything else — plain name with NO visible gap
		// Insert zero-width characters into the name (exactly like the
		// extension's addHiddenCharToArabicName) so the name stays clean with
		// no space/gap, while every bot still gets a unique nick on the wire —
		// otherwise gartic rejects the duplicate-name joins and only one bot
		// gets in.
		base := baseName
		if strings.TrimSpace(base) == "" {
			base = "Bot"
		}
		return NickResult{Nick: addHiddenChar(base)}
	}
}