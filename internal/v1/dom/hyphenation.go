package dom

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Trie Node struct
type liangTrieNode struct {
	strengths []int                   // present at the end of the pattern, corresponding array of strengths at each position of the pattern
	children  map[rune]*liangTrieNode // connection
}

// TEX pattern files
type hyphenationPatternFile struct {
	Leftmin    int               `json:"leftmin"`
	Rightmin   int               `json:"rightmin"`
	Exceptions map[string]string `json:"exceptions"`
	Patterns   map[int]string    `json:"patterns"`
}

// used to cache the hyphenationPatternFile metrics
type hyphenationPatternFileMetrics struct {
	Leftmin    int               `json:"leftmin"`
	Rightmin   int               `json:"rightmin"`
	Exceptions map[string]string `json:"exceptions"`
}

// to embed all paths to the hyphenation json files
//
//go:embed hyphen-patterns/*.json
var hyphenPatternsFS embed.FS

// cached trie roots for langCodes
var langCodeTrieCache map[LangCode]*liangTrieNode

// cached exceptions for langCodes
// also includes already parsed words
var hyphenationMetricsCache map[LangCode]*hyphenationPatternFileMetrics

func hyphenateWord(word string, langCode LangCode) (string, error) {
	var trie *liangTrieNode
	trie, ok := langCodeTrieCache[langCode]
	if !ok {
		lp, err := readHyphenationPatternFile(langCode)
		if err != nil {
			return "", err
		}
		trie = createLiangHyphenTrie(lp)
		langCodeTrieCache[langCode] = trie
		hyphenationMetricsCache[langCode] = &hyphenationPatternFileMetrics{
			Leftmin:    lp.Leftmin,
			Rightmin:   lp.Rightmin,
			Exceptions: lp.Exceptions,
		}
	}

	// check if word is in exceptions
	metrics := hyphenationMetricsCache[langCode]
	if metrics.Exceptions != nil && len(metrics.Exceptions) > 0 {
		if hyphenated, ok := metrics.Exceptions[word]; ok {
			return hyphenated, nil
		}
	}

	// check if word has hyphen
	for _, r := range word {
		switch r {
		case '-', '\u00AD':
			return word, nil
		default:
		}
	}

	word = "." + word + "."
	lowerWord := strings.ToLower(word)

	originalCharacters := []rune(word)
	lowerCharacters := []rune(lowerWord)
	strengths := make([]int, len(lowerCharacters))

	for i := 0; i < len(lowerCharacters); i++ {
		node := trie
		for j := i; j < len(lowerCharacters); j++ {
			node = node.children[lowerCharacters[j]]

			if node != nil {
				if node.strengths != nil && len(node.strengths) > 0 {
					for k := 0; k < len(node.strengths); k++ {
						strengths[i+k] = max(strengths[i+k], node.strengths[k])
					}
				}
			} else {
				break
			}
		}
	}

	var result strings.Builder
	for i := 1; i < len(originalCharacters)-1; i++ {
		result.WriteRune(originalCharacters[i])
		if i > metrics.Leftmin && i < (len(originalCharacters)-metrics.Rightmin) && strengths[i]%2 == 1 {
			result.WriteRune('\u00AD')
		}
	}

	return result.String(), nil
}

func createLiangHyphenTrie(lp *hyphenationPatternFile) *liangTrieNode {
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
					if len(strengths) == 0 {
						strengths = append(strengths, int(r-'0'))
					} else {
						strengths[len(strengths)-1] = int(r - '0')
					}
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

	return trie
}

// read hyphenation pattern file
func readHyphenationPatternFile(langCode LangCode) (*hyphenationPatternFile, error) {
	fpath := filepath.Join("hyphen-patterns", string(langCode))

	fbytes, err := hyphenPatternsFS.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pattern for %s: %w", langCode, err)
	}

	var lp hyphenationPatternFile
	err = json.Unmarshal(fbytes, &lp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pattern for %s: %w", langCode, err)
	}

	return &lp, nil
}
