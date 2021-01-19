package lint

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/errata-ai/vale/v2/internal/core"
	"golang.org/x/net/html"
)

// A node is a scoped block of text in Vale's AST.
type node struct {
	Classes     []string
	Sentences   []string
	Scope       string
	Text        string
	Words       []string
	Descendants []node
}

// An ast is Vale's internal representation of text documents.
type ast struct {
	nodes []node

	z *html.Tokenizer

	activeTag    string
	activeInline bool

	tagHistory []string
	clsHistory []string

	storage []node
}

func (t *ast) walk() (html.TokenType, html.Token, string) {
	tokt := t.z.Next()
	tok := t.z.Token()
	return tokt, tok, html.UnescapeString(strings.TrimSpace(tok.Data))
}

func (t *ast) addTag(tag string) {
	t.tagHistory = append(t.tagHistory, tag)
	t.activeTag = tag
	t.activeInline = core.StringInSlice(tag, inlineTags)
}

func (t *ast) addClass(class string) {
	if class != "" {
		t.clsHistory = append(t.clsHistory, class)
	}
}

func (t *ast) addNode(text, scope string) {
	n := node{Text: text, Scope: scope, Classes: t.clsHistory}
	for _, child := range t.storage {
		n.Descendants = append(n.Descendants, child)
	}
	t.nodes = append(t.nodes, n)
}

func (t *ast) addChild(text string) {
	tag := t.activeTag

	inl := core.StringInSlice(tag, inlineTags)
	if _, match := tagToScope[tag]; match && inl {
		t.storage = append(t.storage,
			node{Text: text, Scope: t.activeTag, Classes: t.clsHistory})
		t.activeTag = ""
	}
}

func (t *ast) getKnownScope() string {
	for _, tag := range t.tagHistory {
		scope, match := tagToScope[tag]
		if match && !core.StringInSlice(tag, inlineTags) {
			return scope
		} else if heading.MatchString(tag) {
			return "text.heading." + tag
		}
	}

	if core.AllStringsInSlice([]string{"pre", "code"}, t.tagHistory) {
		return "pre"
	}

	return ""
}

func (t *ast) addNodeFromScope(text string) {
	text = strings.TrimLeft(text, " ")
	if text == "" {
		// TODO: should this happend?
		return
	} else if s := t.getKnownScope(); s != "" {
		t.addNode(text, s)
	} else {
		t.addNode(text, "text")
	}
}

func (t *ast) addNodeFromData(tok html.Token) *node {
	if tok.Data == "img" {
		for _, a := range tok.Attr {
			if a.Key == "alt" {
				t.addNode(a.Val, "text.attr."+a.Key)
			}
		}
	}
	return nil
}

func (t *ast) reset() {
	t.tagHistory = []string{}
	t.clsHistory = []string{}
	t.storage = []node{}
}

func newAST(raw []byte) (ast, error) {
	var buf bytes.Buffer

	tree := ast{z: html.NewTokenizer(bytes.NewReader(raw))}
	for {
		tokt, tok, txt := tree.walk()
		switch tokt {
		case html.ErrorToken:
			return tree, tree.z.Err()
		case html.StartTagToken:
			tree.addTag(txt)
		case html.CommentToken:
			tree.addNode(txt, "comment")
		case html.TextToken:
			tree.addChild(txt)
			buf.WriteString(offset(txt, tree.activeInline))
		case html.EndTagToken:
			if !core.StringInSlice(txt, inlineTags) {
				content := buf.String()
				tree.addNodeFromScope(content)
				tree.reset()
				buf.Reset()
			}
		}
		tree.addClass(getAttribute(tok, "class"))
		tree.addNodeFromData(tok)
	}
}

func offset(txt string, inline bool) string {
	punct := []string{".", "?", "!", ",", ":", ";"}
	first, _ := utf8.DecodeRuneInString(txt)

	starter := core.StringInSlice(string(first), punct)
	if inline && !starter {
		txt = " " + txt
	}

	return txt
}
