// Package export renders the whole site to static files by crawling
// the HTTP mux in-process. The output is suitable for any static host
// (e.g. Cloudflare Pages with output directory pointing here).
package export

import (
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"

	"github.com/jxc2000-b/git-preview/config"
	"github.com/jxc2000-b/git-preview/git"
	"github.com/jxc2000-b/git-preview/routes"
	"github.com/jxc2000-b/git-preview/showcase"
	securejoin "github.com/cyphar/filepath-securejoin"
)

// Run exports the site for config c into outDir.
func Run(c *config.Config, outDir string) error {
	mux := routes.Handlers(c)

	fetch := func(urlPath string) ([]byte, int, string) {
		req := httptest.NewRequest("GET", urlPath, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Body.Bytes(), rec.Code, rec.Header().Get("Content-Type")
	}

	// writePage stores an HTML page as <urlPath>/index.html so static
	// hosts serve it at the same URL the live server used.
	writePage := func(urlPath string) error {
		body, code, _ := fetch(urlPath)
		if code != 200 {
			return fmt.Errorf("GET %s: status %d", urlPath, code)
		}
		rel := path.Clean("/" + urlPath)[1:]
		fp := filepath.Join(outDir, filepath.FromSlash(rel), "index.html")
		if rel == "" {
			fp = filepath.Join(outDir, "index.html")
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}
		return os.WriteFile(fp, body, 0644)
	}

	// writeAsset stores a response at its literal path (real filename,
	// real extension) — used for showcase media and static files.
	writeAsset := func(urlPath string) error {
		body, code, _ := fetch(urlPath)
		if code != 200 {
			return fmt.Errorf("GET %s: status %d", urlPath, code)
		}
		fp := filepath.Join(outDir, filepath.FromSlash(path.Clean("/"+urlPath)[1:]))
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}
		return os.WriteFile(fp, body, 0644)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	if err := writePage("/"); err != nil {
		return err
	}

	// Static assets, straight from the static dir.
	statics, err := os.ReadDir(c.Dirs.Static)
	if err != nil {
		return fmt.Errorf("static dir: %w", err)
	}
	for _, s := range statics {
		if s.IsDir() {
			continue
		}
		if err := writeAsset("/static/" + s.Name()); err != nil {
			return err
		}
	}

	// Repos.
	dirs, err := os.ReadDir(c.Repo.ScanPath)
	if err != nil {
		return fmt.Errorf("scan path: %w", err)
	}

	var headers []string

	for _, dir := range dirs {
		name := dir.Name()
		if !dir.IsDir() || ignored(c, name) {
			continue
		}

		repoPath, err := securejoin.SecureJoin(c.Repo.ScanPath, name)
		if err != nil {
			return err
		}

		gr, err := git.Open(repoPath, "")
		if err != nil {
			log.Printf("skipping %s: %s", name, err)
			continue
		}

		ref, err := gr.FindMainBranch(c.Repo.MainBranch)
		if err != nil {
			log.Printf("skipping %s: %s", name, err)
			continue
		}
		// Re-open at the named ref so blob/tree URLs match live ones.
		gr, err = git.Open(repoPath, ref)
		if err != nil {
			return err
		}

		base := "/" + name
		if err := writePage(base); err != nil {
			return err
		}
		if err := writePage(base + "/log/" + ref); err != nil {
			return err
		}

		// Tree and blob pages.
		var walk func(prefix string) error
		walk = func(prefix string) error {
			entries, err := gr.FileTree(prefix)
			if err != nil {
				return err
			}
			if err := writePage(base + "/tree/" + ref + "/" + prefix); err != nil {
				return err
			}
			for _, e := range entries {
				p := path.Join(prefix, e.Name)
				if prefix == "" && e.Name == showcase.Dir {
					continue
				}
				if e.IsFile {
					if err := writePage(base + "/blob/" + ref + "/" + p); err != nil {
						return err
					}
				} else {
					if err := walk(p); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := walk(""); err != nil {
			return err
		}

		// Commit pages.
		commits, err := gr.Commits()
		if err != nil {
			return err
		}
		for _, cm := range commits {
			if err := writePage(base + "/commit/" + cm.Hash.String()); err != nil {
				return err
			}
		}

		// Showcase assets referenced by the manifest.
		sc, err := showcase.Load(gr)
		if err != nil {
			log.Printf("showcase manifest for %s: %s", name, err)
		}
		if sc != nil {
			seen := map[string]bool{}
			var assets []showcase.Asset
			assets = append(assets, sc.Landing...)
			for _, fe := range sc.Files {
				assets = append(assets, fe.Assets...)
			}
			for _, as := range sc.Commits {
				assets = append(assets, as...)
			}
			for _, a := range assets {
				if seen[a.Asset] {
					continue
				}
				seen[a.Asset] = true
				if err := writeAsset(base + "/showcase/" + ref + "/" + a.Asset); err != nil {
					return err
				}
			}
			// Sandbox interactive widgets on the static host too.
			headers = append(headers,
				fmt.Sprintf("%s/showcase/%s/*", base, ref),
				"  Content-Security-Policy: sandbox allow-scripts",
				"  X-Content-Type-Options: nosniff",
				"")
		}
	}

	if len(headers) > 0 {
		out := ""
		for _, l := range headers {
			out += l + "\n"
		}
		if err := os.WriteFile(filepath.Join(outDir, "_headers"), []byte(out), 0644); err != nil {
			return err
		}
	}

	return nil
}

func ignored(c *config.Config, name string) bool {
	for _, i := range c.Repo.Ignore {
		if name == i {
			return true
		}
	}
	return false
}
