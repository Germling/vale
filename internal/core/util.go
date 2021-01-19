package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jdkato/prose/tag"
	"github.com/jdkato/prose/tokenize"
	"github.com/jdkato/regexp"
)

var defaultIgnoreDirectories = []string{
	"node_modules", ".git",
}

var sanitizer = strings.NewReplacer(
	"&rsquo;", "'",
	"\r\n", "\n",
	"\r", "\n")

var spaces = regexp.MustCompile(" +")

var reANSI = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

// StripANSI removes all ANSI characters from the given string.
func StripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// WhitespaceToSpace converts newlines and multiple spaces (e.g., "  ") into a
// single space.
func WhitespaceToSpace(msg string) string {
	msg = strings.Replace(msg, "\n", " ", -1)
	msg = spaces.ReplaceAllString(msg, " ")
	return msg
}

// ShouldIgnoreDirectory will check if directory should be ignored
func ShouldIgnoreDirectory(directoryName string) bool {
	for _, directory := range defaultIgnoreDirectories {
		if directory == directoryName {
			return true
		}
	}
	return false
}

// PrintJSON prints the data type `t` as JSON.
func PrintJSON(t interface{}) error {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		fmt.Println("{}")
		return err
	}
	fmt.Println(string(b))
	return nil
}

// ToSentence converts a slice of terms into sentence.
func ToSentence(words []string, andOrOr string) string {
	l := len(words)

	if l == 1 {
		return fmt.Sprintf("'%s'", words[0])
	} else if l == 2 {
		return fmt.Sprintf("'%s' or '%s'", words[0], words[1])
	}

	wordsForSentence := []string{}
	for _, w := range words {
		wordsForSentence = append(wordsForSentence, fmt.Sprintf("'%s'", w))
	}

	wordsForSentence[l-1] = andOrOr + " " + wordsForSentence[l-1]
	return strings.Join(wordsForSentence, ", ")
}

// IsLetter returns `true` if s contains all letter characters and false if not.
func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// IsPhrase returns `true` is s is a phrase-like token.
//
// This is used to differentiate regex tokens from non-regex.
func IsPhrase(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) && r != ' ') {
			return false
		}
	}
	return true
}

// InRange determines if the range r contains the integer n.
func InRange(n int, r []int) bool {
	return len(r) == 2 && (r[0] <= n && n <= r[1])
}

// Tag assigns part-of-speech tags to `words`.
func Tag(words []string) []tag.Token {
	if Tagger == nil {
		Tagger = tag.NewPerceptronTagger()
	}
	return Tagger.Tag(words)
}

// TextToWords convert raw text into a slice of words.
func TextToWords(text string, nlp bool) []string {
	// TODO: Replace with iterTokenizer?
	tok := tokenize.NewTreebankWordTokenizer()

	words := []string{}
	for _, s := range SentenceTokenizer.Tokenize(text) {
		if nlp {
			words = append(words, tok.Tokenize(s)...)
		} else {
			words = append(words, strings.Fields(s)...)
		}
	}

	return words
}

// TextToTokens converts a string to a slice of tokens.
func TextToTokens(text string, needsTagging bool) []tag.Token {
	if needsTagging {
		return Tag(TextToWords(text, true))
	}
	tokens := []tag.Token{}
	for _, word := range TextToWords(text, true) {
		tokens = append(tokens, tag.Token{Text: word})
	}
	return tokens
}

// CheckPOS determines if a match (as found by an extension point) also matches
// the expected part-of-speech in text.
func CheckPOS(loc []int, expected, text string) bool {
	pos := 1

	observed := []string{}
	for _, tok := range TextToTokens(text, true) {
		if InRange(pos, loc) {
			observed = append(observed, (tok.Text + "/" + tok.Tag))
		}
		pos += len(tok.Text)
		if !StringInSlice(tok.Tag, []string{"POS", ".", ",", ":", ";", "?"}) {
			// Space-bounded ...
			pos++
		}
	}

	match, _ := regexp.MatchString(expected, strings.Join(observed, " "))
	return !match
}

// Which checks for the existence of any command in `cmds`.
func Which(cmds []string) string {
	for _, cmd := range cmds {
		path, err := exec.LookPath(cmd)
		if err == nil {
			return path
		}
	}
	return ""
}

// CondSprintf is sprintf, ignores extra arguments.
func CondSprintf(format string, v ...interface{}) string {
	v = append(v, "")
	format += fmt.Sprint("%[", len(v), "]s")
	return fmt.Sprintf(format, v...)
}

// FormatMessage inserts `subs` into `msg`.
func FormatMessage(msg string, subs ...string) string {
	return CondSprintf(msg, StringsToInterface(subs)...)
}

// Substitute replaces the substring `sub` with a string of asterisks.
func Substitute(src, sub string, char rune) (string, bool) {
	idx := strings.Index(src, sub)
	if idx < 0 {
		return src, false
	}
	repl := strings.Map(func(r rune) rune {
		if r != '\n' {
			return char
		}
		return r
	}, sub)
	return src[:idx] + repl + src[idx+len(sub):], true
}

// StringsToInterface converts a slice of strings to an interface.
func StringsToInterface(strings []string) []interface{} {
	intf := make([]interface{}, len(strings))
	for i, v := range strings {
		intf[i] = v
	}
	return intf
}

// Sanitize prepares text for our check functions.
func Sanitize(txt string) string {
	return sanitizer.Replace(txt)
}

// Indent adds padding to every line of `text`.
func Indent(text, indent string) string {
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}

// IsDir determines if the path given by `filename` is a directory.
func IsDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

// FileExists determines if the path given by `filename` exists.
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// StringInSlice determines if `slice` contains the string `a`.
func StringInSlice(a string, slice []string) bool {
	for _, b := range slice {
		if a == b {
			return true
		}
	}
	return false
}

// IntInSlice determines if `slice` contains the int `a`.
func IntInSlice(a int, slice []int) bool {
	for _, b := range slice {
		if a == b {
			return true
		}
	}
	return false
}

// AllStringsInSlice determines if `slice` contains the `strings`.
func AllStringsInSlice(strings []string, slice []string) bool {
	for _, s := range strings {
		if !StringInSlice(s, slice) {
			return false
		}
	}
	return true
}

// SplitLines splits on CRLF, CR not followed by LF, and LF.
func SplitLines(data []byte, atEOF bool) (adv int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\n' {
			return i + 1, data[0:i], nil
		}
		adv = i + 1
		if len(data) > i+1 && data[i+1] == '\n' {
			adv++
		}
		return adv, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func determinePath(configPath string, keyPath string) string {
	if !IsDir(configPath) {
		configPath = filepath.Dir(configPath)
	}
	sep := string(filepath.Separator)
	abs, _ := filepath.Abs(keyPath)
	rel := strings.TrimRight(keyPath, sep)
	if abs != rel || !strings.Contains(keyPath, sep) {
		// The path was relative
		return filepath.Join(configPath, keyPath)
	}
	return abs
}

func mergeValues(shadows []string) []string {
	values := []string{}
	for _, v := range shadows {
		entry := strings.TrimSpace(v)
		if entry != "" && !StringInSlice(entry, values) {
			values = append(values, entry)
		}
	}
	return values
}

func validateLevel(key, val string, cfg *Config) bool {
	options := []string{"YES", "suggestion", "warning", "error"}
	if val == "NO" || !StringInSlice(val, options) {
		return false
	} else if val != "YES" {
		cfg.RuleToLevel[key] = val
	}
	return true
}

func loadVocab(root string, cfg *Config) error {
	target := ""
	for _, p := range cfg.Paths {
		opt := filepath.Join(p, "Vocab", root)
		if IsDir(opt) {
			target = opt
			break
		}
	}

	if target == "" {
		return NewE100("vocab", fmt.Errorf("Vocab '%s' does not exist", root))
	}

	err := filepath.Walk(target, func(fp string, fi os.FileInfo, err error) error {
		if filepath.Base(fp) == "accept.txt" {
			return cfg.AddWordListFile(fp, true)
		} else if filepath.Base(fp) == "reject.txt" {
			return cfg.AddWordListFile(fp, false)
		}
		return err
	})

	return err
}

// TextToSentences splits the given block of text into a slice of sentences.
func TextToSentences(text string) []string {
	return SentenceTokenizer.Tokenize(text)
}

// SentencesToWords splits the given sentences into a slice of words.
func SentencesToWords(sents []string) []string {
	// TODO: Replace with iterTokenizer?
	tok := tokenize.NewTreebankWordTokenizer()

	words := []string{}
	for _, s := range sents {
		words = append(words, tok.Tokenize(s)...)
	}

	return words
}
