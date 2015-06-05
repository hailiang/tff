package core

import (
	"errors"
	"io"
	"strings"
	"unicode"
)

var (
	errInvalidCodePoint = errors.New("invalid code point")
)

type TokenType int

const (
	Invalid TokenType = iota
	Annotation
	LineString
	Indent
	Unindent
	EOF
	_SOF
)

type Token struct {
	Type  TokenType
	Value string
}

type Scanner struct {
	r       io.RuneScanner
	ch      rune
	indents []string
	toks    []Token
	err     error
}

func NewScanner(r io.RuneScanner) *Scanner {
	return &Scanner{
		r:       r,
		indents: []string{""},
		toks:    []Token{Token{Type: _SOF}},
	}
}

func (s *Scanner) Scan() bool {
	s.toks = s.toks[1:]
	if len(s.toks) > 0 {
		return true
	}
	if s.err != nil {
		return false
	}
	s.scanLine()
	s.handleEOF()
	return len(s.toks) > 0
}

func (s *Scanner) scanLine() {
	indent, ok := s.scanIndent()
	if !ok {
		return
	}
	n, ok := s.calcIndent(indent)
	if !ok {
		s.err = errors.New("mismatch indent")
		return
	}
	switch n {
	case 0: // same
		s.afterIndent()
	case 1: // indent
		s.indents = append(s.indents, indent)
		s.addTok(Token{Type: Indent})
		s.afterIndent()
	default: // unindent
		n = -n
		s.indents = s.indents[:len(s.indents)-n]
		for i := 0; i < n; i++ {
			s.addTok(Token{Type: Unindent})
		}
	}
}

func (s *Scanner) scanIndent() (indent string, ok bool) {
	for {
		indent, ok = s.indentSpaces()
		if !ok {
			return
		}
		var hasNewline bool
		hasNewline, ok = s.newlineSpaces()
		if !ok {
			return
		} else if !hasNewline {
			ok = true
			return
		}
	}
}
func (s *Scanner) newlineSpaces() (hasNewline bool, ok bool) {
	for s.next() {
		switch s.ch {
		case '\r', '\n':
			hasNewline = true
		default:
			s.prev()
			return hasNewline, true
		}
	}
	return hasNewline, false
}
func (s *Scanner) indentSpaces() (indent string, ok bool) {
	rs := []rune{}
	for s.next() {
		if s.ch != ' ' && s.ch != '\t' {
			s.prev()
			return string(rs), true
		}
		rs = append(rs, s.ch)
	}
	return "", false
}

func (s *Scanner) calcIndent(indent string) (int, bool) {
	last := s.indents[len(s.indents)-1]
	if indent == last {
		return 0, true
	} else if strings.HasPrefix(indent, last) {
		return 1, true
	}
	for i := 1; i < len(s.indents); i++ {
		if indent == s.indents[len(s.indents)-i-1] {
			return -i, true
		}
	}
	return 0, false
}

func (s *Scanner) afterIndent() {
	if s.ch == '#' {
		s.addTok(Token{Type: Annotation, Value: s.inline()[1:]})
	} else {
		s.addTok(Token{Type: LineString, Value: s.inline()})
	}
}
func (s *Scanner) inline() string {
	rs := []rune{}
	for s.next() {
		if s.ch == '\r' || s.ch == '\n' {
			s.prev()
			break
		}
		rs = append(rs, s.ch)
	}
	return string(rs)
}

func (s *Scanner) handleEOF() {
	if s.err == io.EOF {
		if len(s.indents) > 1 {
			for i := 0; i < len(s.indents)-1; i++ {
				s.addTok(Token{Type: Unindent})
			}
			s.indents = s.indents[:1]
		}
		s.addTok(Token{Type: EOF})
	}
}

func (s *Scanner) addTok(tok Token) {
	s.toks = append(s.toks, tok)
}

func (s *Scanner) Token() Token {
	return s.toks[0]
}

func (s *Scanner) next() bool {
	s.ch, _, s.err = s.r.ReadRune()
	if s.err != nil {
		return false
	}
	switch s.ch {
	case '\t', ' ', '\r', '\n':
	case unicode.ReplacementChar:
		s.err = errInvalidCodePoint
		return false
	default:
		if '\x00' <= s.ch && s.ch <= '\x19' {
			s.err = errInvalidCodePoint
			return false
		}
	}
	return true
}

func (s *Scanner) prev() bool {
	s.err = s.r.UnreadRune()
	return s.err == nil
}

func (s *Scanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}
