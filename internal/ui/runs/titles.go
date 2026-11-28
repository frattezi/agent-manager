package runs

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Number is the one a row shows for a Run: its pull request's when it has one,
// else its Ticket's.
type Number struct {
	Digits string
	// PullRequest is true when Digits came from the pull request.
	PullRequest bool
}

// Project is the name of the repository the Run's work belongs to, from its
// checkout rather than its Ticket Ref.
func (r Record) Project() string {
	if r.RepoPath == "" {
		return ""
	}
	return filepath.Base(r.RepoPath)
}

// RunTitle is the record's own Title, else the branch de-slugged: up to the
// last "/" dropped, a leading ticket number dropped, hyphens as spaces, first
// letter capitalised. Empty when neither exists.
func (r Record) RunTitle() string {
	if r.Title != "" {
		return r.Title
	}
	slug := r.Branch[strings.LastIndex(r.Branch, "/")+1:]
	if rest := strings.TrimLeft(slug, "0123456789"); rest != slug && strings.HasPrefix(rest, "-") {
		slug = rest[1:]
	}
	title := strings.ReplaceAll(slug, "-", " ")
	if title == "" {
		return ""
	}
	first, size := utf8.DecodeRuneInString(title)
	return string(unicode.ToUpper(first)) + title[size:]
}

// Number is the pull request's number when the URL ends in digits, else the
// digits after the last "#" of the Ticket Ref; zero when neither exists.
func (r Record) Number() Number {
	if last := r.PRURL[strings.LastIndex(r.PRURL, "/")+1:]; allDigits(last) {
		return Number{Digits: last, PullRequest: true}
	}
	if i := strings.LastIndex(r.TicketRef, "#"); i >= 0 && allDigits(r.TicketRef[i+1:]) {
		return Number{Digits: r.TicketRef[i+1:]}
	}
	return Number{}
}

// String is "PR 293" for a pull request and "#270" for a Ticket, so the two
// are told apart.
func (n Number) String() string {
	switch {
	case n.Digits == "":
		return ""
	case n.PullRequest:
		return "PR " + n.Digits
	}
	return "#" + n.Digits
}

func allDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
