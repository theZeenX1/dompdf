package dom

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type LangCode string

const (
	EnglishUS LangCode = "en-us"
)

// Trie Node struct
type liangTrieNode struct {
	strengths []int                   // present at the end of the pattern, corresponding array of strengths at each position of the pattern
	children  map[rune]*liangTrieNode // connection
}

// TEX pattern files
type hyphernationPatternFile struct {
	Leftmin    int            `json:"leftmin"`
	Rightmin   int            `json:"rightmin"`
	Exceptions []string       `json:"exceptions"`
	Patterns   map[int]string `json:"patterns"`
}

//go:embed hyphen-patterns/*.json
var hyphenPatternsFS embed.FS

func createLiangHyphenTrie(langCode LangCode, exceptions ...string) (*liangTrieNode, error) {
	lp, err := readHyphenationPatternFile(langCode, exceptions...)

	trie := &liangTrieNode{
		children: make(map[rune]*liangTrieNode),
	}
	for sz, pattern := range lp.Patterns {
		runes := []rune(pattern) // to safely handle non utf8 languages

		// create "sz" sized pattern chunks
		chunks := []string{}
		for i := 0; i < len(runes); i += sz {
			end := min(i+sz, len(runes))
			chunk := string(runes[i:end])
			chunks = append(chunks, chunk)
		}

		// iterated through all chunks:
		for _, chunk := range chunks {
			t := trie
			strengths := []int{}
			characters := []rune{}
			for _, r := range []rune(chunk) {
				if r >= '0' && r <= '9' {
					strengths[len(strengths)-1] = int(r - '0')
				} else {
					characters = append(characters, r)
					strengths = append(strengths, 0)
				}
			}

			for _, c := range characters {
				if _, ok := t.children[c]; !ok {
					t.children[c] = &liangTrieNode{
						children: make(map[rune]*liangTrieNode),
					}
				}
				t = t.children[c]
			}

			t.strengths = strengths
		}

	}

	return trie, err
}

// read hyphenation pattern file
func readHyphenationPatternFile(langCode LangCode, exceptions ...string) (*hyphernationPatternFile, error) {
	fpath := filepath.Join("hyphen-patterns", string(langCode))

	fbytes, err := hyphenPatternsFS.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pattern for %s: %w", langCode, err)
	}

	var lp hyphernationPatternFile
	err = json.Unmarshal(fbytes, &lp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pattern for %s: %w", langCode, err)
	}

	for _, ex := range exceptions {
		lp.Exceptions = append(lp.Exceptions, ex)
	}

	return &lp, nil
}
