// Package showcase implements the sidecar `.showcase/` convention:
// repos may carry a `.showcase/showcase.yaml` manifest associating
// media assets (images, video, interactive HTML) with the repo landing
// page, individual files, or commits.
package showcase

import (
	"fmt"
	"path"
	"strings"

	"github.com/jxc2000-b/git-preview/git"
	"gopkg.in/yaml.v3"
)

const Dir = ".showcase"
const manifestPath = Dir + "/showcase.yaml"

type Asset struct {
	Asset   string `yaml:"asset"`
	Caption string `yaml:"caption"`
	Height  int    `yaml:"height"`
}

// FileEntry is the per-file manifest value. It accepts either a plain
// asset list:
//
//	"src/foo.ts":
//	  - asset: demo.png
//
// or a mapping with an optional preview policy:
//
//	"src/foo.ts":
//	  preview: full | none | truncated
//	  assets:
//	    - asset: demo.png
type FileEntry struct {
	Assets  []Asset
	Preview string
}

func (f *FileEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		return value.Decode(&f.Assets)
	}
	var aux struct {
		Assets  []Asset `yaml:"assets"`
		Preview string  `yaml:"preview"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	f.Assets = aux.Assets
	f.Preview = aux.Preview
	return nil
}

type Manifest struct {
	DefaultPreview string               `yaml:"defaultPreview"`
	Landing        []Asset              `yaml:"landing"`
	Files          map[string]FileEntry `yaml:"files"`
	Commits        map[string][]Asset   `yaml:"commits"`
}

// Kind classifies an asset by extension: "image", "video", "html" or "".
func (a Asset) Kind() string {
	switch strings.ToLower(strings.TrimPrefix(path.Ext(a.Asset), ".")) {
	case "png", "jpg", "jpeg", "gif", "svg", "webp", "avif":
		return "image"
	case "mp4", "webm", "mov", "ogv":
		return "video"
	case "html", "htm":
		return "html"
	}
	return ""
}

// IframeHeight returns the iframe height in px, defaulting to 360.
func (a Asset) IframeHeight() int {
	if a.Height > 0 {
		return a.Height
	}
	return 360
}

// safe rejects asset paths that could escape the .showcase directory.
func safe(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return false
	}
	clean := path.Clean(p)
	return clean == p && clean != ".." && !strings.HasPrefix(clean, "../")
}

func filterSafe(as []Asset) []Asset {
	out := make([]Asset, 0, len(as))
	for _, a := range as {
		if safe(a.Asset) && a.Kind() != "" {
			out = append(out, a)
		}
	}
	return out
}

// Load reads the showcase manifest from the repo at its opened ref.
// A repo without a manifest yields (nil, nil).
func Load(gr *git.GitRepo) (*Manifest, error) {
	raw, err := gr.FileContentBytes(manifestPath)
	if err != nil {
		// Missing manifest just means the feature is unused.
		return nil, nil
	}

	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", manifestPath, err)
	}

	m.Landing = filterSafe(m.Landing)
	for k, v := range m.Files {
		v.Assets = filterSafe(v.Assets)
		m.Files[k] = v
	}
	for k, v := range m.Commits {
		m.Commits[k] = filterSafe(v)
	}
	return &m, nil
}

// ForFile returns assets associated with a repo-relative file path.
func (m *Manifest) ForFile(p string) []Asset {
	if m == nil {
		return nil
	}
	return m.Files[path.Clean(p)].Assets
}

// PreviewMode returns the blob-view policy for a file: "full", "none",
// or "truncated". A per-file policy takes precedence over defaultPreview;
// when neither is set, the mode remains "truncated" for compatibility.
func (m *Manifest) PreviewMode(p string) string {
	if m != nil {
		switch m.Files[path.Clean(p)].Preview {
		case "full":
			return "full"
		case "none":
			return "none"
		case "truncated":
			return "truncated"
		}
		switch m.DefaultPreview {
		case "full":
			return "full"
		case "none":
			return "none"
		}
	}
	return "truncated"
}

// ForCommit returns assets associated with a commit hash; manifest keys
// may be abbreviated (>= 7 chars) prefixes of the full hash.
func (m *Manifest) ForCommit(hash string) []Asset {
	if m == nil {
		return nil
	}
	if as, ok := m.Commits[hash]; ok {
		return as
	}
	for k, as := range m.Commits {
		if len(k) >= 7 && strings.HasPrefix(hash, k) {
			return as
		}
	}
	return nil
}
