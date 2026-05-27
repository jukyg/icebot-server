package main

import (
	"fmt"
	"math/rand"
	"strings"
)

var aiGreetings = []string{
	"hey", "hi", "hello", "yo", "sup", "heya", "howdy", "ello",
}

var aiFarewells = []string{
	"bye", "goodbye", "cya", "later", "peace", "see ya",
}

var aiAgreements = []string{
	"true", "facts", "for real", "exactly", "same", "yep", "yeah",
	"totally", "agreed", "no cap", "real talk",
}

var aiDisagreements = []string{
	"nah", "no way", "cap", "you sure", "doubt it", "nope",
	"not really", "i disagree", "different opinion",
}

var aiLaughs = []string{
	"lol", "lmao", "lmfao", "haha", "hehe", "xd", "lolz",
}

var aiConfusion = []string{
	"huh", "what", "idk", "not sure", "confused", "explain",
	"i don't get it", "wym", "wait what",
}

var aiGeneric = []string{
	"fr fr", "real", "ong", "that's wild", "bruh", "bet",
	"say less", "sheesh", "no shot", "deadass",
	"that's crazy", "word", "i see you",
	"bro really", "aight", "bet bet",
}

var aiQuestions = []string{
	"what do you mean", "how so", "why's that", "for real though",
	"explain yourself", "what makes you say that",
	"what's your reasoning", "tell me more",
}

var aiSarcastic = []string{
	"oh wow", "really", "no way", "i'm shocked", "who would've thought",
	"groundbreaking", "incredible", "what a surprise",
}

var aiSupportive = []string{
	"you got this", "keep going", "nice one", "good job",
	"proud of you", "you're doing great", "don't give up",
}

func aiChatResponse(userMsg string, persona string) string {
	msg := strings.ToLower(strings.TrimSpace(userMsg))
	if msg == "" {
		return ""
	}

	pool := aiGeneric

	if strings.HasPrefix(msg, "!") || strings.HasPrefix(msg, "/") {
		return ""
	}

	switch {
	case isGreeting(msg):
		pool = append(pool, aiGreetings...)
	case isFarewell(msg):
		pool = append(pool, aiFarewells...)
	case isQuestion(msg):
		pool = append(pool, aiQuestions...)
		pool = append(pool, aiConfusion...)
	case isAgreement(msg):
		pool = append(pool, aiAgreements...)
	case isDisagreement(msg):
		pool = append(pool, aiDisagreements...)
	case isLaugh(msg):
		pool = append(pool, aiLaughs...)
	case isSarcastic(msg, persona):
		pool = append(pool, aiSarcastic...)
	case isSupportive(msg):
		pool = append(pool, aiSupportive...)
	}

	pool = applyPersona(pool, persona)

	if len(pool) == 0 {
		pool = aiGeneric
	}

	return pool[rand.Intn(len(pool))]
}

func aiChatResponseWithGemini(userMsg, persona, targetName string) string {
	if !GeminiReady() {
		return aiChatResponseForTargetLocal(userMsg, persona, targetName)
	}

	text, err := geminiGenerate(persona, userMsg, targetName)
	if err != nil {
		return aiChatResponseForTargetLocal(userMsg, persona, targetName)
	}

	targets := []string{
		text,
		fmt.Sprintf("@%s %s", targetName, text),
		fmt.Sprintf("%s @%s", text, targetName),
	}
	return targets[rand.Intn(len(targets))]
}

func aiChatResponseForTarget(userMsg, persona, targetName string) string {
	return aiChatResponseWithGemini(userMsg, persona, targetName)
}

func aiChatResponseForTargetLocal(userMsg, persona, targetName string) string {
	base := aiChatResponse(userMsg, persona)
	if base == "" {
		return ""
	}
	targets := []string{
		fmt.Sprintf("@%s %s", targetName, base),
		fmt.Sprintf("%s @%s", base, targetName),
		fmt.Sprintf("%s — %s", targetName, base),
	}
	return targets[rand.Intn(len(targets))]
}

func isGreeting(msg string) bool {
	for _, g := range aiGreetings {
		if strings.Contains(msg, g) {
			return true
		}
	}
	return false
}

func isFarewell(msg string) bool {
	for _, f := range aiFarewells {
		if strings.Contains(msg, f) {
			return true
		}
	}
	return false
}

func isQuestion(msg string) bool {
	return strings.HasSuffix(msg, "?") ||
		strings.HasPrefix(msg, "what") ||
		strings.HasPrefix(msg, "why") ||
		strings.HasPrefix(msg, "how") ||
		strings.HasPrefix(msg, "who") ||
		strings.HasPrefix(msg, "when") ||
		strings.HasPrefix(msg, "where")
}

func isAgreement(msg string) bool {
	for _, a := range aiAgreements {
		if strings.Contains(msg, a) {
			return true
		}
	}
	return false
}

func isDisagreement(msg string) bool {
	for _, d := range aiDisagreements {
		if strings.Contains(msg, d) {
			return true
		}
	}
	return false
}

func isLaugh(msg string) bool {
	return strings.Contains(msg, "lol") ||
		strings.Contains(msg, "lmao") ||
		strings.Contains(msg, "haha") ||
		strings.Contains(msg, "hehe") ||
		strings.Contains(msg, "xd") ||
		strings.Contains(msg, "lolz")
}

func isSarcastic(msg string, persona string) bool {
	lp := strings.ToLower(persona)
	if strings.Contains(lp, "sarcastic") || strings.Contains(lp, "snarky") || strings.Contains(lp, "edgy") {
		return true
	}
	return strings.Contains(msg, "omg") ||
		strings.Contains(msg, "wow") ||
		strings.Contains(msg, "seriously") ||
		strings.Contains(msg, "unbelievable")
}

func isSupportive(msg string) bool {
	return strings.Contains(msg, "help") ||
		strings.Contains(msg, "stuck") ||
		strings.Contains(msg, "cant") ||
		strings.Contains(msg, "can't") ||
		strings.Contains(msg, "hard") ||
		strings.Contains(msg, "difficult")
}

func applyPersona(pool []string, persona string) []string {
	p := strings.ToLower(persona)
	if p == "" {
		return pool
	}

	switch {
	case strings.Contains(p, "sarcastic") || strings.Contains(p, "snarky") || strings.Contains(p, "edgy"):
		pool = append(pool, aiSarcastic...)
		pool = append(pool, "oh really? tell me more", "wow, groundbreaking", "i'm sure that was very important", "cool story bro", "anyways...")
	case strings.Contains(p, "shy") || strings.Contains(p, "quiet") || strings.Contains(p, "timid"):
		pool = append(pool, "...", "um", "i-i think so?", "maybe?", "if you say so", "s-sure")
	case strings.Contains(p, "friendly") || strings.Contains(p, "nice") || strings.Contains(p, "kind") || strings.Contains(p, "sweet"):
		pool = append(pool, aiSupportive...)
		pool = append(pool, "that's wonderful!", "i'm happy for you", "you're amazing!", "keep being awesome", "sending good vibes")
	case strings.Contains(p, "angry") || strings.Contains(p, "aggressive") || strings.Contains(p, "mean"):
		pool = append(pool, "shut up", "nobody asked", "stfu", "whatever", "you're wrong", "cap", "cringe")
	case strings.Contains(p, "funny") || strings.Contains(p, "comedian") || strings.Contains(p, "jokester"):
		pool = append(pool, aiLaughs...)
		pool = append(pool, "that's what she said", "name checks out", "i have a joke but...", "knock knock", "you heard that one about the chicken?")
	case strings.Contains(p, "weeb") || strings.Contains(p, "anime") || strings.Contains(p, "otaku"):
		pool = append(pool, "desu", "nya~", "senpai noticed me", "it's not like i wanted to talk to you", "baka", "sugoi", "nani?!")
	case strings.Contains(p, "gen z") || strings.Contains(p, "young") || strings.Contains(p, "hip"):
		pool = append(pool, "fr fr", "no cap", "based", "cringe", "slay", "period", "periodt", "let's go", "yeet", "bet", "sheesh", "bussin", "hits different")
	}

	return pool
}
