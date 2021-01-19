package lint

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/errata-ai/vale/v2/internal/core"
	"golang.org/x/net/html"
)

// A Node is a scoped block of text in Vale's AST.
type Node struct {
	Classes     []string
	Sentences   []string
	Scope       string
	Text        string
	Words       []string
	Descendants []Node
}

// An AST is Vale's internal representation of text documents.
type AST struct {
	Nodes []Node

	z *html.Tokenizer

	activeTag    string
	activeInline bool

	tagHistory []string
	clsHistory []string

	storage []Node
}

func (t *AST) walk() (html.TokenType, html.Token, string) {
	tokt := t.z.Next()
	tok := t.z.Token()
	return tokt, tok, html.UnescapeString(strings.TrimSpace(tok.Data))
}

func (t *AST) addTag(tag string) {
	t.tagHistory = append(t.tagHistory, tag)
	t.activeTag = tag
	t.activeInline = core.StringInSlice(tag, inlineTags)
}

func (t *AST) addClass(class string) {
	if class != "" {
		t.clsHistory = append(t.clsHistory, class)
	}
}

func (t *AST) addNode(text, scope string) {
	t.Nodes = append(t.Nodes,
		Node{Text: text, Scope: scope, Classes: t.clsHistory})
}

func (t *AST) addChild(text string) {
	tag := t.activeTag

	inl := core.StringInSlice(tag, inlineTags)
	if _, match := tagToScope[tag]; match && inl {
		t.storage = append(t.storage,
			Node{Text: text, Scope: t.activeTag, Classes: t.clsHistory})
		t.activeTag = ""
	}
}

func (t *AST) addNodeFromScope(text string) {
	text = strings.TrimLeft(text, " ")

	for _, tag := range t.tagHistory {
		scope, match := tagToScope[tag]

		inl := core.StringInSlice(tag, inlineTags)
		if (match && !inl) || heading.MatchString(tag) {
			if !match {
				scope = "text.heading." + tag
			}
			t.addNode(text, scope)
			return
		}
	}

	if core.AllStringsInSlice([]string{"pre", "code"}, t.tagHistory) {
		t.addNode(text, "pre")
		return
	}

	n := Node{Text: text, Scope: "p", Classes: t.clsHistory}
	for _, child := range t.storage {
		n.Descendants = append(n.Descendants, child)
	}
	t.Nodes = append(t.Nodes, n)
}

func (t *AST) addNodeFromData(tok html.Token) *Node {
	if tok.Data == "img" {
		for _, a := range tok.Attr {
			if a.Key == "alt" {
				t.addNode(a.Val, "text.attr."+a.Key)
			}
		}
	}
	return nil
}

func (t *AST) reset() {
	t.tagHistory = []string{}
	t.clsHistory = []string{}
	t.storage = []Node{}
}

func newAST(raw []byte) AST {
	var buf bytes.Buffer

	tree := AST{z: html.NewTokenizer(bytes.NewReader(raw))}
	for {
		tokt, tok, txt := tree.walk()
		if tokt == html.ErrorToken {
			break
		} else if tokt == html.StartTagToken {
			tree.addTag(txt)
		} else if tokt == html.CommentToken {
			tree.addNode(txt, "comment")
		} else if tokt == html.TextToken {
			tree.addChild(txt)
			buf.WriteString(offset(txt, tree.activeInline))
		}

		if tokt == html.EndTagToken && !core.StringInSlice(txt, inlineTags) {
			content := buf.String()
			tree.addNodeFromScope(content)
			tree.reset()
			buf.Reset()
		}

		tree.addClass(getAttribute(tok, "class"))
		tree.addNodeFromData(tok)
	}

	return tree
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
